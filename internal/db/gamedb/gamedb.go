// Package gamedb provides a unified query interface that wraps either SQLite
// (solo worlds) or Postgres (shared worlds) behind a single concrete struct.
// Command handlers use *GameDB instead of raw *sql.DB.
package gamedb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adam-stokes/gl1tch-mud/internal/db/pgq"
	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
)

// GameDB wraps either a SQLite or Postgres backend. Exactly one of sqlite/pg
// will be non-nil.
type GameDB struct {
	sqlite    *sqliteq.Queries
	pg        *pgq.Queries
	accountID string        // UUID string for shared worlds
	worldID   string        // world name for shared worlds
	sqliteDB  *sql.DB       // underlying SQLite conn (for transactions)
	pgPool    *pgxpool.Pool // underlying Postgres pool (for transactions)
}

// NewSQLite creates a GameDB backed by SQLite (for solo worlds / CLI mode).
func NewSQLite(db *sql.DB) *GameDB {
	return &GameDB{
		sqlite:   sqliteq.New(db),
		sqliteDB: db,
	}
}

// NewPostgres creates a GameDB backed by Postgres (for shared worlds).
// Panics if accountID is not a valid UUID — callers must validate before reaching this point.
func NewPostgres(pool *pgxpool.Pool, accountID, worldID string) *GameDB {
	if _, err := uuid.Parse(accountID); err != nil {
		panic("gamedb: invalid account UUID: " + accountID)
	}
	return &GameDB{
		pg:        pgq.New(pool),
		accountID: accountID,
		worldID:   worldID,
		pgPool:    pool,
	}
}

// IsShared reports whether this GameDB is backed by Postgres (shared world).
func (g *GameDB) IsShared() bool {
	return g.pg != nil
}

// AccountID returns the account UUID string (empty for solo).
func (g *GameDB) AccountID() string { return g.accountID }

// WorldID returns the world name string (empty for solo).
func (g *GameDB) WorldID() string { return g.worldID }

// pgUUID converts the stored accountID string to a pgtype.UUID.
func (g *GameDB) pgUUID() pgtype.UUID {
	parsed, err := uuid.Parse(g.accountID)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

// pgText converts a string to pgtype.Text.
func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// normErr converts pgx.ErrNoRows to sql.ErrNoRows so callers can use a single
// sentinel check regardless of backend.
func normErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}

// ─── Transaction Support ──────────────────────────────────────────────────

// TxFunc is a function that runs inside a transaction.
// For SQLite, tx is *sql.Tx; for Postgres, a pgx.Tx wraps the connection.
type TxFunc func(txGDB *GameDB) error

// RunTx executes fn within a database transaction.
// The GameDB passed to fn wraps the transaction's query object.
func (g *GameDB) RunTx(ctx context.Context, fn TxFunc) error {
	if g.pg != nil {
		tx, err := g.pgPool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		txGDB := &GameDB{
			pg:        pgq.New(tx),
			accountID: g.accountID,
			worldID:   g.worldID,
			pgPool:    g.pgPool,
		}
		if err := fn(txGDB); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	tx, err := g.sqliteDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	txGDB := &GameDB{
		sqlite:   sqliteq.New(tx),
		sqliteDB: g.sqliteDB,
	}
	if err := fn(txGDB); err != nil {
		return err
	}
	return tx.Commit()
}

// ─── Player State ──────────────────────────────────────────────────────────

// GetPlayer returns core player state. For solo: reads from the single-row
// player table. For shared: reads from shared_player_state.
func (g *GameDB) GetPlayer(ctx context.Context) (roomID string, hp, maxHP int, worldName, class string, err error) {
	if g.pg != nil {
		row, qerr := g.pg.GetSharedPlayerState(ctx, pgq.GetSharedPlayerStateParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if qerr != nil {
			return "", 0, 0, "", "", normErr(qerr)
		}
		// class lives in a column the sqlc model doesn't know about — query separately.
		var c string
		if g.pgPool != nil {
			_ = g.pgPool.QueryRow(ctx,
				`SELECT class FROM shared_player_state WHERE account_id=$1 AND world_id=$2`,
				g.pgUUID(), g.worldID,
			).Scan(&c)
		}
		return row.RoomID, int(row.Hp), int(row.MaxHp), g.worldID, c, nil
	}
	row, qerr := g.sqlite.GetPlayer(ctx)
	if qerr != nil {
		return "", 0, 0, "", "", qerr
	}
	// class lives in a column the sqlc model doesn't know about — query separately.
	var c string
	_ = g.sqliteDB.QueryRow(`SELECT class FROM player WHERE id=1`).Scan(&c)
	return row.RoomID, int(row.Hp), int(row.MaxHp), row.World, c, nil
}

// SavePlayer persists current player state.
func (g *GameDB) SavePlayer(ctx context.Context, roomID string, hp, maxHP int, worldName, class string) error {
	if g.pg != nil {
		if err := g.pg.UpsertSharedPlayerState(ctx, pgq.UpsertSharedPlayerStateParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			RoomID:    roomID,
			Hp:        int32(hp),
			MaxHp:     int32(maxHP),
			Credits:   0, // credits managed separately
		}); err != nil {
			return err
		}
		if g.pgPool != nil {
			_, err := g.pgPool.Exec(ctx,
				`UPDATE shared_player_state SET class=$1 WHERE account_id=$2 AND world_id=$3`,
				class, g.pgUUID(), g.worldID,
			)
			return err
		}
		return nil
	}
	if err := g.sqlite.SavePlayer(ctx, sqliteq.SavePlayerParams{
		RoomID: roomID,
		Hp:     int64(hp),
		MaxHp:  int64(maxHP),
		World:  worldName,
	}); err != nil {
		return err
	}
	_, err := g.sqliteDB.Exec(`UPDATE player SET class=? WHERE id=1`, class)
	return err
}

// SeedPlayer creates the initial player record (SQLite only — Postgres uses upsert).
func (g *GameDB) SeedPlayer(ctx context.Context, name, roomID string, hp, maxHP int, worldName, class string) error {
	if g.pg != nil {
		if err := g.pg.UpsertSharedPlayerState(ctx, pgq.UpsertSharedPlayerStateParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			RoomID:    roomID,
			Hp:        int32(hp),
			MaxHp:     int32(maxHP),
		}); err != nil {
			return err
		}
		if g.pgPool != nil {
			_, err := g.pgPool.Exec(ctx,
				`UPDATE shared_player_state SET class=$1 WHERE account_id=$2 AND world_id=$3`,
				class, g.pgUUID(), g.worldID,
			)
			return err
		}
		return nil
	}
	if err := g.sqlite.SeedPlayer(ctx, sqliteq.SeedPlayerParams{
		Name:   name,
		RoomID: roomID,
		Hp:     int64(hp),
		MaxHp:  int64(maxHP),
		World:  worldName,
	}); err != nil {
		return err
	}
	_, err := g.sqliteDB.Exec(`UPDATE player SET class=? WHERE id=1`, class)
	return err
}

// ─── Inventory ─────────────────────────────────────────────────────────────

// InventoryItem is a carried item.
type InventoryItem struct {
	ID   string
	Name string
	Desc string
}

// ListInventory returns all items the player is carrying.
func (g *GameDB) ListInventory(ctx context.Context) ([]InventoryItem, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedInventory(ctx, pgq.ListSharedInventoryParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		items := make([]InventoryItem, len(rows))
		for i, r := range rows {
			items[i] = InventoryItem{ID: r.ItemID, Name: r.ItemName, Desc: r.ItemDesc}
		}
		return items, nil
	}
	rows, err := g.sqlite.ListInventory(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]InventoryItem, len(rows))
	for i, r := range rows {
		items[i] = InventoryItem{ID: r.ItemID, Name: r.ItemName, Desc: r.ItemDesc}
	}
	return items, nil
}

// AddItem adds an item to inventory.
func (g *GameDB) AddItem(ctx context.Context, id, name, desc string) error {
	if g.pg != nil {
		return g.pg.AddSharedItem(ctx, pgq.AddSharedItemParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    id,
			ItemName:  name,
			ItemDesc:  desc,
		})
	}
	return g.sqlite.AddItem(ctx, sqliteq.AddItemParams{
		ItemID:   id,
		ItemName: name,
		ItemDesc: desc,
	})
}

// RemoveItem removes an item from inventory.
func (g *GameDB) RemoveItem(ctx context.Context, itemID string) error {
	if g.pg != nil {
		tag, err := g.pg.RemoveSharedItem(ctx, pgq.RemoveSharedItemParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
		})
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("item %q not in inventory", itemID)
		}
		return nil
	}
	res, err := g.sqlite.RemoveItem(ctx, itemID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("item %q not in inventory", itemID)
	}
	return nil
}

// RemoveOneItem removes exactly one copy of an item from inventory.
func (g *GameDB) RemoveOneItem(ctx context.Context, itemID string) error {
	if g.pg != nil {
		return g.pg.RemoveOneSharedItem(ctx, pgq.RemoveOneSharedItemParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
		})
	}
	return g.sqlite.DeleteOneInventoryItem(ctx, itemID)
}

// DeleteInventoryItem removes an item from inventory by ID (used by building transactions).
func (g *GameDB) DeleteInventoryItem(ctx context.Context, itemID string) error {
	if g.pg != nil {
		return g.pg.RemoveOneSharedItem(ctx, pgq.RemoveOneSharedItemParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
		})
	}
	return g.sqlite.DeleteInventoryItem(ctx, itemID)
}

// InsertInventoryItem adds an item directly (used by building transactions).
func (g *GameDB) InsertInventoryItem(ctx context.Context, itemID, itemName, itemDesc string) error {
	if g.pg != nil {
		return g.pg.AddSharedItem(ctx, pgq.AddSharedItemParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
			ItemName:  itemName,
			ItemDesc:  itemDesc,
		})
	}
	return g.sqlite.InsertInventoryItem(ctx, sqliteq.InsertInventoryItemParams{
		ItemID:   itemID,
		ItemName: itemName,
		ItemDesc: itemDesc,
	})
}

// InsertInventoryItemCraft adds a crafted item to inventory.
func (g *GameDB) InsertInventoryItemCraft(ctx context.Context, itemID, itemName, itemDesc string) error {
	if g.pg != nil {
		return g.pg.AddSharedItem(ctx, pgq.AddSharedItemParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
			ItemName:  itemName,
			ItemDesc:  itemDesc,
		})
	}
	return g.sqlite.InsertInventoryItemCraft(ctx, sqliteq.InsertInventoryItemCraftParams{
		ItemID:   itemID,
		ItemName: itemName,
		ItemDesc: itemDesc,
	})
}

// ClearInventory deletes all inventory items.
func (g *GameDB) ClearInventory(ctx context.Context) error {
	if g.pg != nil {
		return g.pg.ClearSharedInventory(ctx, pgq.ClearSharedInventoryParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
	}
	return g.sqlite.ClearInventory(ctx)
}

// ─── NPC State ─────────────────────────────────────────────────────────────

// NPCAlive reports whether an NPC is alive (default true if no record).
func (g *GameDB) NPCAlive(ctx context.Context, roomID, npcID string) bool {
	if g.pg != nil {
		row, err := g.pg.GetSharedNPCState(ctx, pgq.GetSharedNPCStateParams{
			WorldID: g.worldID,
			NpcID:   npcID,
		})
		if err != nil {
			return true // default alive
		}
		return row.Alive
	}
	row, err := g.sqlite.GetNPCState(ctx, sqliteq.GetNPCStateParams{
		RoomID: roomID,
		NpcID:  npcID,
	})
	if err != nil {
		return true // default alive
	}
	return row.Alive == 1
}

// NPCCurrentHP returns the current HP for an NPC, defaulting to defaultHP.
func (g *GameDB) NPCCurrentHP(ctx context.Context, roomID, npcID string, defaultHP int) int {
	if g.pg != nil {
		row, err := g.pg.GetSharedNPCState(ctx, pgq.GetSharedNPCStateParams{
			WorldID: g.worldID,
			NpcID:   npcID,
		})
		if err != nil {
			return defaultHP
		}
		return int(row.Hp)
	}
	row, err := g.sqlite.GetNPCState(ctx, sqliteq.GetNPCStateParams{
		RoomID: roomID,
		NpcID:  npcID,
	})
	if err != nil {
		return defaultHP
	}
	return int(row.Hp)
}

// KillNPC marks an NPC as dead.
func (g *GameDB) KillNPC(ctx context.Context, roomID, npcID string, finalHP int) error {
	if g.pg != nil {
		return g.pg.UpsertSharedNPCDead(ctx, pgq.UpsertSharedNPCDeadParams{
			WorldID: g.worldID,
			NpcID:   npcID,
			RoomID:  roomID,
			Hp:      int32(finalHP),
		})
	}
	return g.sqlite.UpsertNPCDead(ctx, sqliteq.UpsertNPCDeadParams{
		RoomID: roomID,
		NpcID:  npcID,
		Hp:     int64(finalHP),
	})
}

// SetNPCHP updates an NPC's HP without killing it.
func (g *GameDB) SetNPCHP(ctx context.Context, roomID, npcID string, hp int) error {
	if g.pg != nil {
		return g.pg.UpsertSharedNPCAlive(ctx, pgq.UpsertSharedNPCAliveParams{
			WorldID: g.worldID,
			NpcID:   npcID,
			RoomID:  roomID,
			Hp:      int32(hp),
		})
	}
	return g.sqlite.UpsertNPCAlive(ctx, sqliteq.UpsertNPCAliveParams{
		RoomID: roomID,
		NpcID:  npcID,
		Hp:     int64(hp),
	})
}

// ─── Visited ───────────────────────────────────────────────────────────────

// MarkVisited records that the player has visited a room.
func (g *GameDB) MarkVisited(ctx context.Context, roomID string) {
	if g.pg != nil {
		_ = g.pg.MarkSharedVisited(ctx, pgq.MarkSharedVisitedParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			RoomID:    roomID,
		})
		return
	}
	_ = g.sqlite.MarkVisited(ctx, roomID)
}

// HasVisited reports whether the player has previously visited a room.
func (g *GameDB) HasVisited(ctx context.Context, roomID string) bool {
	if g.pg != nil {
		_, err := g.pg.HasSharedVisited(ctx, pgq.HasSharedVisitedParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			RoomID:    roomID,
		})
		return err == nil
	}
	_, err := g.sqlite.HasVisited(ctx, roomID)
	return err == nil
}

// ─── Skills ────────────────────────────────────────────────────────────────

// LoadSkill returns level and XP for the given skill. Returns 0,0 if not found.
func (g *GameDB) LoadSkill(ctx context.Context, skill string) (level, xp int, err error) {
	if g.pg != nil {
		row, qerr := g.pg.GetSharedSkill(ctx, pgq.GetSharedSkillParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Skill:     skill,
		})
		if qerr != nil {
			return 0, 0, nil // not found → zero
		}
		return int(row.Level), int(row.Xp), nil
	}
	row, qerr := g.sqlite.LoadSkill(ctx, skill)
	if qerr == sql.ErrNoRows {
		return 0, 0, nil
	}
	if qerr != nil {
		return 0, 0, qerr
	}
	return int(row.Level), int(row.Xp), nil
}

// UpsertSkill saves a skill level and XP.
func (g *GameDB) UpsertSkill(ctx context.Context, skill string, level, xp int) error {
	if g.pg != nil {
		return g.pg.UpsertSharedSkill(ctx, pgq.UpsertSharedSkillParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Skill:     skill,
			Level:     int32(level),
			Xp:        int32(xp),
		})
	}
	return g.sqlite.UpsertSkill(ctx, sqliteq.UpsertSkillParams{
		Skill: skill,
		Level: int64(level),
		Xp:    int64(xp),
	})
}

// ListAllSkills returns all skills as map[name] → [level, xp].
func (g *GameDB) ListAllSkills(ctx context.Context) (map[string][2]int, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedSkills(ctx, pgq.ListSharedSkillsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		result := make(map[string][2]int, len(rows))
		for _, r := range rows {
			result[r.Skill] = [2]int{int(r.Level), int(r.Xp)}
		}
		return result, nil
	}
	rows, err := g.sqlite.ListAllSkills(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string][2]int, len(rows))
	for _, r := range rows {
		result[r.Skill] = [2]int{int(r.Level), int(r.Xp)}
	}
	return result, nil
}

// ─── Credits ───────────────────────────────────────────────────────────────

// GetCredits returns the current credit balance (0 on error).
func (g *GameDB) GetCredits(ctx context.Context) int {
	if g.pg != nil {
		c, err := g.pg.GetSharedCredits(ctx, pgq.GetSharedCreditsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return 0
		}
		return int(c)
	}
	c, err := g.sqlite.GetCredits(ctx)
	if err != nil {
		return 0
	}
	return int(c)
}

// AddCredits adds amount credits (may be negative).
func (g *GameDB) AddCredits(ctx context.Context, amount int) error {
	if g.pg != nil {
		return g.pg.AddSharedCredits(ctx, pgq.AddSharedCreditsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Credits:   int32(amount),
		})
	}
	return g.sqlite.UpsertCredits(ctx, int64(amount))
}

// ─── Action Counter ────────────────────────────────────────────────────────

// GetActionCount returns the player's action counter.
func (g *GameDB) GetActionCount(ctx context.Context) int {
	if g.pg != nil {
		c, err := g.pg.GetSharedActionCount(ctx, pgq.GetSharedActionCountParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return 0
		}
		return int(c)
	}
	c, err := g.sqlite.GetActionCount(ctx)
	if err != nil || !c.Valid {
		return 0
	}
	return int(c.Int64)
}

// BumpActions increments the action counter.
func (g *GameDB) BumpActions(ctx context.Context) {
	if g.pg != nil {
		_ = g.pg.BumpSharedActions(ctx, pgq.BumpSharedActionsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		return
	}
	_ = g.sqlite.BumpActions(ctx)
}

// ─── Equipped Armor ────────────────────────────────────────────────────────

// EquippedArmorRecord holds equipped armor data.
type EquippedArmorRecord struct {
	ItemID   string
	ItemName string
	Defense  int
}

// GetEquippedArmor returns the currently equipped armor, or nil.
func (g *GameDB) GetEquippedArmor(ctx context.Context) (*EquippedArmorRecord, error) {
	if g.pg != nil {
		row, err := g.pg.GetSharedEquippedArmor(ctx, pgq.GetSharedEquippedArmorParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, nil // no row = no armor
		}
		return &EquippedArmorRecord{
			ItemID:   row.ItemID,
			ItemName: row.ItemName,
			Defense:  int(row.Defense),
		}, nil
	}
	row, err := g.sqlite.GetEquippedArmor(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &EquippedArmorRecord{
		ItemID:   row.ItemID,
		ItemName: row.ItemName,
		Defense:  int(row.Defense),
	}, nil
}

// EquipArmor upserts the equipped armor.
func (g *GameDB) EquipArmor(ctx context.Context, itemID, itemName string, defense int) error {
	if g.pg != nil {
		return g.pg.UpsertSharedEquippedArmor(ctx, pgq.UpsertSharedEquippedArmorParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
			ItemName:  itemName,
			Defense:   int32(defense),
		})
	}
	return g.sqlite.EquipArmor(ctx, sqliteq.EquipArmorParams{
		ItemID:   itemID,
		ItemName: itemName,
		Defense:  int64(defense),
	})
}

// UnequipArmor removes equipped armor.
func (g *GameDB) UnequipArmor(ctx context.Context) error {
	if g.pg != nil {
		return g.pg.DeleteSharedEquippedArmor(ctx, pgq.DeleteSharedEquippedArmorParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
	}
	return g.sqlite.UnequipArmor(ctx)
}

// ─── Death Pile ────────────────────────────────────────────────────────────

// InsertDeathPile adds an item to the death pile.
func (g *GameDB) InsertDeathPile(ctx context.Context, roomID, itemID, itemName, itemDesc string, diedAt int) error {
	if g.pg != nil {
		return g.pg.InsertSharedDeathPile(ctx, pgq.InsertSharedDeathPileParams{
			WorldID:  g.worldID,
			RoomID:   roomID,
			ItemID:   itemID,
			ItemName: itemName,
			ItemDesc: itemDesc,
		})
	}
	return g.sqlite.InsertDeathPile(ctx, sqliteq.InsertDeathPileParams{
		RoomID:   roomID,
		ItemID:   itemID,
		ItemName: itemName,
		ItemDesc: itemDesc,
		DiedAt:   int64(diedAt),
	})
}

// GetDeathPile returns death pile items for a room.
func (g *GameDB) GetDeathPile(ctx context.Context, roomID string) ([]InventoryItem, error) {
	if g.pg != nil {
		rows, err := g.pg.GetSharedDeathPile(ctx, pgq.GetSharedDeathPileParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
		if err != nil {
			return nil, err
		}
		items := make([]InventoryItem, len(rows))
		for i, r := range rows {
			items[i] = InventoryItem{ID: r.ItemID, Name: r.ItemName, Desc: r.ItemDesc}
		}
		return items, nil
	}
	rows, err := g.sqlite.GetDeathPile(ctx, roomID)
	if err != nil {
		return nil, err
	}
	items := make([]InventoryItem, len(rows))
	for i, r := range rows {
		items[i] = InventoryItem{ID: r.ItemID, Name: r.ItemName, Desc: r.ItemDesc}
	}
	return items, nil
}

// DeleteDeathPile removes all death pile items for a room.
func (g *GameDB) DeleteDeathPile(ctx context.Context, roomID string) error {
	if g.pg != nil {
		return g.pg.DeleteSharedDeathPile(ctx, pgq.DeleteSharedDeathPileParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
	}
	return g.sqlite.DeleteDeathPile(ctx, roomID)
}

// AnyDeathPile returns the room_id and count of the most recent death pile.
// Returns ("", 0, nil) if none.
func (g *GameDB) AnyDeathPile(ctx context.Context) (roomID string, count int, err error) {
	if g.pg != nil {
		row, qerr := g.pg.AnySharedDeathPile(ctx, g.worldID)
		if errors.Is(qerr, pgx.ErrNoRows) {
			return "", 0, nil
		}
		if qerr != nil {
			return "", 0, qerr
		}
		return row.RoomID, int(row.Count), nil
	}
	row, qerr := g.sqlite.AnyDeathPile(ctx)
	if qerr == sql.ErrNoRows {
		return "", 0, nil
	}
	if qerr != nil {
		return "", 0, qerr
	}
	return row.RoomID, int(row.Count), nil
}

// DumpToDeathPile moves all inventory items to death pile for roomID.
func (g *GameDB) DumpToDeathPile(ctx context.Context, roomID string, actionCount int) error {
	items, err := g.ListInventory(ctx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	return g.RunTx(ctx, func(txGDB *GameDB) error {
		for _, it := range items {
			if err := txGDB.InsertDeathPile(ctx, roomID, it.ID, it.Name, it.Desc, actionCount); err != nil {
				return err
			}
		}
		return txGDB.ClearInventory(ctx)
	})
}

// ClaimDeathPile recovers all death pile items back to inventory.
func (g *GameDB) ClaimDeathPile(ctx context.Context, roomID string) error {
	items, err := g.GetDeathPile(ctx, roomID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	return g.RunTx(ctx, func(txGDB *GameDB) error {
		for _, it := range items {
			if err := txGDB.AddItem(ctx, it.ID, it.Name, it.Desc); err != nil {
				return err
			}
		}
		return txGDB.DeleteDeathPile(ctx, roomID)
	})
}

// ─── Crystal Shards ────────────────────────────────────────────────────────

// MarkShardCollected marks a crystal shard as collected.
func (g *GameDB) MarkShardCollected(ctx context.Context, shardID string) error {
	if g.pg != nil {
		return g.pg.MarkSharedShardCollected(ctx, pgq.MarkSharedShardCollectedParams{
			WorldID:     g.worldID,
			ShardID:     shardID,
			CollectedAt: int32(g.GetActionCount(ctx)),
		})
	}
	actionCnt, _ := g.sqlite.GetActionCount(ctx) //nolint:errcheck
	var cnt int64
	if actionCnt.Valid {
		cnt = actionCnt.Int64
	}
	return g.sqlite.MarkShardCollected(ctx, sqliteq.MarkShardCollectedParams{
		CollectedAt: cnt,
		ShardID:     shardID,
	})
}

// SeedCrystalShard inserts a single crystal shard row.
func (g *GameDB) SeedCrystalShard(ctx context.Context, shardID, biome string) error {
	if g.pg != nil {
		return g.pg.SeedSharedCrystalShard(ctx, pgq.SeedSharedCrystalShardParams{
			WorldID: g.worldID,
			ShardID: shardID,
			Biome:   biome,
		})
	}
	return g.sqlite.SeedCrystalShard(ctx, sqliteq.SeedCrystalShardParams{
		ShardID: shardID,
		Biome:   biome,
	})
}

// CountCollectedShards returns the count of collected crystal shards.
func (g *GameDB) CountCollectedShards(ctx context.Context) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedCollectedShards(ctx, g.worldID)
	}
	return g.sqlite.CountCollectedShards(ctx)
}

// CountTotalShards returns the total count of crystal shards.
func (g *GameDB) CountTotalShards(ctx context.Context) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedTotalShards(ctx, g.worldID)
	}
	return g.sqlite.CountTotalShards(ctx)
}

// ─── Starting Items ────────────────────────────────────────────────────────

// CountStartingItem returns the count of starting items (for seed check).
func (g *GameDB) CountStartingItem(ctx context.Context) (int64, error) {
	if g.pg != nil {
		// For PG, we check inventory count.
		rows, err := g.pg.ListSharedInventory(ctx, pgq.ListSharedInventoryParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return 0, err
		}
		return int64(len(rows)), nil
	}
	return g.sqlite.CountStartingItem(ctx)
}

// InsertStartingItem adds a starting item to inventory.
func (g *GameDB) InsertStartingItem(ctx context.Context, itemID, itemName, itemDesc string) error {
	if g.pg != nil {
		return g.pg.AddSharedItem(ctx, pgq.AddSharedItemParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
			ItemName:  itemName,
			ItemDesc:  itemDesc,
		})
	}
	return g.sqlite.InsertStartingItem(ctx, sqliteq.InsertStartingItemParams{
		ItemID:   itemID,
		ItemName: itemName,
		ItemDesc: itemDesc,
	})
}

// ─── Arena ─────────────────────────────────────────────────────────────────

// ArenaMatch is the common arena session struct.
type ArenaMatch struct {
	ID             string
	GameType       string
	Phase          string
	Wave           int
	EnemiesJSON    string
	RewardCredits  int
	RewardItemID   string
	RewardItemName string
	RewardItemDesc string
	Status         string
	StartedAt      int64
}

// arenaExtra is the extra metadata stored in the enemies blob for PG mode.
type arenaExtra struct {
	Phase          string `json:"phase"`
	RewardCredits  int    `json:"reward_credits"`
	RewardItemID   string `json:"reward_item_id"`
	RewardItemName string `json:"reward_item_name"`
	RewardItemDesc string `json:"reward_item_desc"`
	Status         string `json:"status"`
	Enemies        json.RawMessage `json:"enemies"`
}

// InsertArenaSession creates a new arena session.
func (g *GameDB) InsertArenaSession(ctx context.Context, m ArenaMatch) error {
	if g.pg != nil {
		// Pack extra fields into enemies JSON for PG.
		extra := arenaExtra{
			Phase:          m.Phase,
			RewardCredits:  m.RewardCredits,
			RewardItemID:   m.RewardItemID,
			RewardItemName: m.RewardItemName,
			RewardItemDesc: m.RewardItemDesc,
			Status:         m.Status,
			Enemies:        json.RawMessage(m.EnemiesJSON),
		}
		blob, _ := json.Marshal(extra)
		return g.pg.StartSharedArena(ctx, pgq.StartSharedArenaParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			GameType:  m.GameType,
			Wave:      int32(m.Wave),
			Enemies:   string(blob),
		})
	}
	return g.sqlite.InsertArenaSession(ctx, sqliteq.InsertArenaSessionParams{
		ID:             m.ID,
		GameType:       m.GameType,
		Phase:          m.Phase,
		Wave:           int64(m.Wave),
		EnemiesJson:    m.EnemiesJSON,
		RewardCredits:  int64(m.RewardCredits),
		RewardItemID:   m.RewardItemID,
		RewardItemName: m.RewardItemName,
		RewardItemDesc: m.RewardItemDesc,
		Status:         m.Status,
		StartedAt:      m.StartedAt,
	})
}

// GetActiveArena returns the active arena match, or nil.
func (g *GameDB) GetActiveArena(ctx context.Context) *ArenaMatch {
	if g.pg != nil {
		row, err := g.pg.GetSharedActiveArena(ctx, pgq.GetSharedActiveArenaParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil
		}
		// Unpack extra fields from enemies blob.
		var extra arenaExtra
		if err := json.Unmarshal([]byte(row.Enemies), &extra); err != nil {
			return nil
		}
		arenaID := uuid.UUID(row.ID.Bytes)
		return &ArenaMatch{
			ID:             arenaID.String(),
			GameType:       row.GameType,
			Phase:          extra.Phase,
			Wave:           int(row.Wave),
			EnemiesJSON:    string(extra.Enemies),
			RewardCredits:  extra.RewardCredits,
			RewardItemID:   extra.RewardItemID,
			RewardItemName: extra.RewardItemName,
			RewardItemDesc: extra.RewardItemDesc,
			Status:         extra.Status,
			StartedAt:      row.StartedAt.Time.Unix(),
		}
	}
	row, err := g.sqlite.GetActiveArenaSession(ctx)
	if err != nil {
		return nil
	}
	return &ArenaMatch{
		ID:             row.ID,
		GameType:       row.GameType,
		Phase:          row.Phase,
		Wave:           int(row.Wave),
		EnemiesJSON:    row.EnemiesJson,
		RewardCredits:  int(row.RewardCredits),
		RewardItemID:   row.RewardItemID,
		RewardItemName: row.RewardItemName,
		RewardItemDesc: row.RewardItemDesc,
		Status:         row.Status,
		StartedAt:      row.StartedAt,
	}
}

// UpdateArenaSession saves arena match state.
func (g *GameDB) UpdateArenaSession(ctx context.Context, m ArenaMatch) {
	if g.pg != nil {
		// Re-pack everything into enemies blob.
		extra := arenaExtra{
			Phase:          m.Phase,
			RewardCredits:  m.RewardCredits,
			RewardItemID:   m.RewardItemID,
			RewardItemName: m.RewardItemName,
			RewardItemDesc: m.RewardItemDesc,
			Status:         m.Status,
			Enemies:        json.RawMessage(m.EnemiesJSON),
		}
		blob, _ := json.Marshal(extra)
		id, _ := uuid.Parse(m.ID)
		g.pg.SaveSharedArena(ctx, pgq.SaveSharedArenaParams{ //nolint:errcheck
			Wave:    int32(m.Wave),
			Enemies: string(blob),
			ID:      pgtype.UUID{Bytes: id, Valid: true},
		})
		return
	}
	g.sqlite.UpdateArenaSession(ctx, sqliteq.UpdateArenaSessionParams{ //nolint:errcheck
		Phase:       m.Phase,
		Wave:        int64(m.Wave),
		EnemiesJson: m.EnemiesJSON,
		Status:      m.Status,
		ID:          m.ID,
	})
}

// QuitArenaSession forfeits the active arena match.
func (g *GameDB) QuitArenaSession(ctx context.Context) {
	if g.pg != nil {
		g.pg.QuitSharedArena(ctx, pgq.QuitSharedArenaParams{ //nolint:errcheck
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		return
	}
	g.sqlite.QuitArenaSession(ctx) //nolint:errcheck
}

// ─── Base / Building ───────────────────────────────────────────────────────

// ListBuildIDsInRoom returns build IDs for a room (for defense score calc).
func (g *GameDB) ListBuildIDsInRoom(ctx context.Context, roomID string) ([]string, error) {
	if g.pg != nil {
		return g.pg.ListSharedBuilds(ctx, pgq.ListSharedBuildsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
	}
	return g.sqlite.ListBuildIDsInRoom(ctx, roomID)
}

// BuildInRoomRow is a build record.
type BuildInRoomRow struct {
	BuildID string
	Name    string
}

// ListBuildsInRoom returns build IDs and names for a room.
func (g *GameDB) ListBuildsInRoom(ctx context.Context, roomID string) ([]BuildInRoomRow, error) {
	if g.pg != nil {
		// PG ListSharedBuilds only returns build_id. We'll return with empty names
		// and let the caller resolve names from the recipe catalog.
		ids, err := g.pg.ListSharedBuilds(ctx, pgq.ListSharedBuildsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
		if err != nil {
			return nil, err
		}
		rows := make([]BuildInRoomRow, len(ids))
		for i, id := range ids {
			rows[i] = BuildInRoomRow{BuildID: id, Name: id}
		}
		return rows, nil
	}
	sqlRows, err := g.sqlite.ListBuildsInRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	rows := make([]BuildInRoomRow, len(sqlRows))
	for i, r := range sqlRows {
		rows[i] = BuildInRoomRow{BuildID: r.BuildID, Name: r.Name}
	}
	return rows, nil
}

// CountBuildsInRoom returns the number of builds in a room.
func (g *GameDB) CountBuildsInRoom(ctx context.Context, roomID string) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedBuilds(ctx, pgq.CountSharedBuildsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
	}
	return g.sqlite.CountBuildsInRoom(ctx, roomID)
}

// CountBuildsByType counts builds of a specific type in a room.
func (g *GameDB) CountBuildsByType(ctx context.Context, roomID, buildID string) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedBuildsByType(ctx, pgq.CountSharedBuildsByTypeParams{
			WorldID: g.worldID,
			RoomID:  roomID,
			BuildID: buildID,
		})
	}
	return g.sqlite.CountBuildsByType(ctx, sqliteq.CountBuildsByTypeParams{
		RoomID:  roomID,
		BuildID: buildID,
	})
}

// InsertBuild records a build in a room.
func (g *GameDB) InsertBuild(ctx context.Context, roomID, buildID, name, desc string, placedAt int) error {
	if g.pg != nil {
		return g.pg.InsertSharedBuild(ctx, pgq.InsertSharedBuildParams{
			WorldID:  g.worldID,
			RoomID:   roomID,
			BuildID:  buildID,
			PlacedBy: g.pgUUID(),
		})
	}
	return g.sqlite.InsertBuild(ctx, sqliteq.InsertBuildParams{
		RoomID:   roomID,
		BuildID:  buildID,
		Name:     name,
		Desc:     desc,
		PlacedAt: int64(placedAt),
	})
}

// CountActiveBaseRaids returns the count of active base raids for a room.
func (g *GameDB) CountActiveBaseRaids(ctx context.Context, targetRoom string) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedActiveBaseRaids(ctx, pgq.CountSharedActiveBaseRaidsParams{
			WorldID:    g.worldID,
			TargetRoom: targetRoom,
		})
	}
	return g.sqlite.CountActiveBaseRaids(ctx, targetRoom)
}

// WorldEventParams holds params for inserting a world event.
type WorldEventParams struct {
	ID             string
	Type           string
	Title          string
	Description    string
	TargetRoom     string
	Faction        string
	PayoutCredits  int
	PayoutItemID   string
	PayoutItemName string
	PayoutItemDesc string
	Status         string
	ExpiresActions int
	CreatedActions int
	CreatedAt      int64
}

// InsertWorldEvent creates a world event.
func (g *GameDB) InsertWorldEvent(ctx context.Context, p WorldEventParams) error {
	if g.pg != nil {
		return g.pg.InsertSharedWorldEvent(ctx, pgq.InsertSharedWorldEventParams{
			ID:             p.ID,
			WorldID:        g.worldID,
			Type:           p.Type,
			Title:          p.Title,
			Description:    pgText(p.Description),
			TargetRoom:     p.TargetRoom,
			Faction:        pgText(p.Faction),
			PayoutCredits:  int32(p.PayoutCredits),
			PayoutItemID:   pgText(p.PayoutItemID),
			PayoutItemName: pgText(p.PayoutItemName),
			PayoutItemDesc: pgText(p.PayoutItemDesc),
			Status:         p.Status,
			ExpiresActions: int32(p.ExpiresActions),
			CreatedActions: int32(p.CreatedActions),
			CreatedAt:      int32(p.CreatedAt),
		})
	}
	return g.sqlite.InsertWorldEvent(ctx, sqliteq.InsertWorldEventParams{
		ID:             p.ID,
		Type:           p.Type,
		Title:          p.Title,
		Description:    sql.NullString{String: p.Description, Valid: p.Description != ""},
		TargetRoom:     p.TargetRoom,
		Faction:        sql.NullString{String: p.Faction, Valid: p.Faction != ""},
		PayoutCredits:  int64(p.PayoutCredits),
		PayoutItemID:   sql.NullString{String: p.PayoutItemID, Valid: p.PayoutItemID != ""},
		PayoutItemName: sql.NullString{String: p.PayoutItemName, Valid: p.PayoutItemName != ""},
		PayoutItemDesc: sql.NullString{String: p.PayoutItemDesc, Valid: p.PayoutItemDesc != ""},
		Status:         p.Status,
		ExpiresActions: int64(p.ExpiresActions),
		CreatedActions: int64(p.CreatedActions),
		CreatedAt:      p.CreatedAt,
	})
}

// ListExpiredBaseRaids returns IDs of expired base raid events.
func (g *GameDB) ListExpiredBaseRaids(ctx context.Context, targetRoom string, createdActions int) ([]string, error) {
	if g.pg != nil {
		return g.pg.ListSharedExpiredBaseRaids(ctx, pgq.ListSharedExpiredBaseRaidsParams{
			WorldID:        g.worldID,
			TargetRoom:     targetRoom,
			CreatedActions: int32(createdActions),
		})
	}
	return g.sqlite.ListExpiredBaseRaids(ctx, sqliteq.ListExpiredBaseRaidsParams{
		TargetRoom:     targetRoom,
		CreatedActions: int64(createdActions),
	})
}

// ResolveWorldEvent marks a world event as resolved.
func (g *GameDB) ResolveWorldEvent(ctx context.Context, id string) error {
	if g.pg != nil {
		return g.pg.ResolveSharedWorldEvent(ctx, id)
	}
	return g.sqlite.ResolveWorldEvent(ctx, id)
}

// RandomChestItem is an item from a chest.
type RandomChestItem struct {
	ItemID   string
	ItemName string
}

// ListRandomChestItems returns random items from a chest (for raid loss).
func (g *GameDB) ListRandomChestItems(ctx context.Context, roomID string, limit int) ([]RandomChestItem, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedRandomChestItems(ctx, pgq.ListSharedRandomChestItemsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
			Limit:   int32(limit),
		})
		if err != nil {
			return nil, err
		}
		items := make([]RandomChestItem, len(rows))
		for i, r := range rows {
			items[i] = RandomChestItem{ItemID: r.ItemID, ItemName: r.ItemName}
		}
		return items, nil
	}
	rows, err := g.sqlite.ListRandomChestItems(ctx, sqliteq.ListRandomChestItemsParams{
		RoomID: roomID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]RandomChestItem, len(rows))
	for i, r := range rows {
		items[i] = RandomChestItem{ItemID: r.ItemID, ItemName: r.ItemName}
	}
	return items, nil
}

// DeleteChestItem deletes a specific item from a chest.
func (g *GameDB) DeleteChestItem(ctx context.Context, roomID, itemID string) error {
	if g.pg != nil {
		return g.pg.DeleteSharedChestItem(ctx, pgq.DeleteSharedChestItemParams{
			WorldID: g.worldID,
			RoomID:  roomID,
			ItemID:  itemID,
		})
	}
	return g.sqlite.DeleteChestItemBase(ctx, sqliteq.DeleteChestItemBaseParams{
		RoomID: roomID,
		ItemID: itemID,
	})
}

// CountChestInRoom returns whether a chest build exists in a room.
func (g *GameDB) CountChestInRoom(ctx context.Context, roomID string) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedChestInRoom(ctx, pgq.CountSharedChestInRoomParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
	}
	return g.sqlite.CountChestInRoom(ctx, roomID)
}

// ChestItem is an item stored in a chest.
type ChestItem struct {
	ItemID   string
	ItemName string
	ItemDesc string
}

// ListChestItems returns items in a chest.
func (g *GameDB) ListChestItems(ctx context.Context, roomID string) ([]ChestItem, error) {
	if g.pg != nil {
		rows, err := g.pg.GetSharedChestItems(ctx, pgq.GetSharedChestItemsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
		if err != nil {
			return nil, err
		}
		items := make([]ChestItem, len(rows))
		for i, r := range rows {
			items[i] = ChestItem{ItemID: r.ItemID, ItemName: r.ItemName}
		}
		return items, nil
	}
	rows, err := g.sqlite.ListChestItems(ctx, roomID)
	if err != nil {
		return nil, err
	}
	items := make([]ChestItem, len(rows))
	for i, r := range rows {
		items[i] = ChestItem{ItemID: r.ItemID, ItemName: r.ItemName}
	}
	return items, nil
}

// GetChestItem returns a specific item from a chest.
func (g *GameDB) GetChestItem(ctx context.Context, roomID, itemID string) (*ChestItem, error) {
	if g.pg != nil {
		rows, err := g.pg.GetSharedChestItems(ctx, pgq.GetSharedChestItemsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.ItemID == itemID {
				return &ChestItem{ItemID: r.ItemID, ItemName: r.ItemName}, nil
			}
		}
		return nil, sql.ErrNoRows
	}
	row, err := g.sqlite.GetChestItem(ctx, sqliteq.GetChestItemParams{
		RoomID: roomID,
		ItemID: itemID,
	})
	if err != nil {
		return nil, err
	}
	return &ChestItem{ItemID: itemID, ItemName: row.ItemName, ItemDesc: row.ItemDesc}, nil
}

// InsertChestItem stores an item in a chest.
func (g *GameDB) InsertChestItem(ctx context.Context, roomID, itemID, itemName, itemDesc string) error {
	if g.pg != nil {
		return g.pg.InsertSharedChest(ctx, pgq.InsertSharedChestParams{
			WorldID:  g.worldID,
			RoomID:   roomID,
			ItemID:   itemID,
			ItemName: itemName,
			StoredBy: g.pgUUID(),
		})
	}
	return g.sqlite.InsertChestItem(ctx, sqliteq.InsertChestItemParams{
		RoomID:   roomID,
		ItemID:   itemID,
		ItemName: itemName,
		ItemDesc: itemDesc,
	})
}

// DeleteChestItemByRoomAndID deletes one chest item (building package variant).
func (g *GameDB) DeleteChestItemByRoomAndID(ctx context.Context, roomID, itemID string) error {
	if g.pg != nil {
		return g.pg.DeleteSharedChestItem(ctx, pgq.DeleteSharedChestItemParams{
			WorldID: g.worldID,
			RoomID:  roomID,
			ItemID:  itemID,
		})
	}
	return g.sqlite.DeleteChestItem(ctx, sqliteq.DeleteChestItemParams{
		RoomID: roomID,
		ItemID: itemID,
	})
}

// ─── Factions ──────────────────────────────────────────────────────────────

// FactionRecord holds faction data.
type FactionRecord struct {
	FactionID     string
	FactionName   string
	Agenda        string
	HideoutRoomID string
	Credits       int
	CreatedAt     int64
}

// FactionExists reports whether the player already has a faction.
func (g *GameDB) FactionExists(ctx context.Context) (bool, error) {
	if g.pg != nil {
		_, err := g.pg.SharedFactionExists(ctx, pgq.SharedFactionExistsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return false, nil
		}
		return true, nil
	}
	_, err := g.sqlite.FactionExists(ctx)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateFaction creates the player's faction.
func (g *GameDB) CreateFaction(ctx context.Context, factionID, factionName, agenda string, createdAt int64) error {
	if g.pg != nil {
		return g.pg.CreateSharedFaction(ctx, pgq.CreateSharedFactionParams{
			AccountID:   g.pgUUID(),
			WorldID:     g.worldID,
			FactionID:   factionID,
			FactionName: factionName,
			Agenda:      pgText(agenda),
			CreatedAt:   int32(createdAt),
		})
	}
	return g.sqlite.CreateFaction(ctx, sqliteq.CreateFactionParams{
		FactionID:   factionID,
		FactionName: factionName,
		Agenda:      sql.NullString{String: agenda, Valid: agenda != ""},
		CreatedAt:   createdAt,
	})
}

// GetFaction returns the player's faction.
func (g *GameDB) GetFaction(ctx context.Context) (*FactionRecord, error) {
	if g.pg != nil {
		row, err := g.pg.GetSharedFaction(ctx, pgq.GetSharedFactionParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, normErr(err)
		}
		return &FactionRecord{
			FactionID:     row.FactionID,
			FactionName:   row.FactionName,
			Agenda:        row.Agenda.String,
			HideoutRoomID: row.HideoutRoomID.String,
			Credits:       int(row.Credits),
			CreatedAt:     int64(row.CreatedAt),
		}, nil
	}
	row, err := g.sqlite.GetFaction(ctx)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no faction exists")
	}
	if err != nil {
		return nil, err
	}
	return &FactionRecord{
		FactionID:     row.FactionID,
		FactionName:   row.FactionName,
		Agenda:        row.Agenda.String,
		HideoutRoomID: row.HideoutRoomID.String,
		Credits:       int(row.Credits),
		CreatedAt:     row.CreatedAt,
	}, nil
}

// SetFactionHideout updates the hideout room.
func (g *GameDB) SetFactionHideout(ctx context.Context, roomID string) error {
	if g.pg != nil {
		return g.pg.SetSharedHideout(ctx, pgq.SetSharedHideoutParams{
			HideoutRoomID: pgText(roomID),
			AccountID:     g.pgUUID(),
			WorldID:       g.worldID,
		})
	}
	return g.sqlite.SetFactionHideout(ctx, sql.NullString{String: roomID, Valid: roomID != ""})
}

// FactionMemberRecord holds faction member data.
type FactionMemberRecord struct {
	NPCID         string
	NPCName       string
	NPCDesc       string
	Role          string
	StationedRoom string
	Loyalty       int
	RecruitedAt   int64
}

// ListFactionMembers returns all faction members.
func (g *GameDB) ListFactionMembers(ctx context.Context) ([]FactionMemberRecord, error) {
	if g.pg != nil {
		rows, err := g.pg.GetSharedMembers(ctx, pgq.GetSharedMembersParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]FactionMemberRecord, len(rows))
		for i, r := range rows {
			out[i] = FactionMemberRecord{
				NPCID:         r.NpcID,
				NPCName:       r.NpcName,
				NPCDesc:       r.NpcDesc.String,
				Role:          r.Role,
				StationedRoom: r.StationedRoom.String,
				Loyalty:       int(r.Loyalty),
				RecruitedAt:   int64(r.RecruitedAt),
			}
		}
		return out, nil
	}
	rows, err := g.sqlite.ListFactionMembers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]FactionMemberRecord, len(rows))
	for i, r := range rows {
		out[i] = FactionMemberRecord{
			NPCID:         r.NpcID,
			NPCName:       r.NpcName,
			NPCDesc:       r.NpcDesc.String,
			Role:          r.Role,
			StationedRoom: r.StationedRoom.String,
			Loyalty:       int(r.Loyalty),
			RecruitedAt:   r.RecruitedAt,
		}
	}
	return out, nil
}

// InsertFactionMember adds an NPC to the faction.
func (g *GameDB) InsertFactionMember(ctx context.Context, npcID, npcName, npcDesc, role string, recruitedAt int64) error {
	if g.pg != nil {
		return g.pg.RecruitSharedMember(ctx, pgq.RecruitSharedMemberParams{
			AccountID:   g.pgUUID(),
			WorldID:     g.worldID,
			NpcID:       npcID,
			NpcName:     npcName,
			NpcDesc:     pgText(npcDesc),
			Role:        role,
			RecruitedAt: int32(recruitedAt),
		})
	}
	return g.sqlite.InsertFactionMember(ctx, sqliteq.InsertFactionMemberParams{
		NpcID:       npcID,
		NpcName:     npcName,
		NpcDesc:     sql.NullString{String: npcDesc, Valid: npcDesc != ""},
		Role:        role,
		RecruitedAt: recruitedAt,
	})
}

// IsFactionMember reports whether an NPC is in the faction.
func (g *GameDB) IsFactionMember(ctx context.Context, npcID string) (bool, error) {
	if g.pg != nil {
		_, err := g.pg.IsSharedRecruited(ctx, npcID)
		if err != nil {
			return false, nil
		}
		return true, nil
	}
	_, err := g.sqlite.GetFactionMember(ctx, npcID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CountFactionMembers returns the member count.
func (g *GameDB) CountFactionMembers(ctx context.Context) (int, error) {
	if g.pg != nil {
		n, err := g.pg.SharedMemberCount(ctx, pgq.SharedMemberCountParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		return int(n), err
	}
	n, err := g.sqlite.CountFactionMembers(ctx)
	return int(n), err
}

// ─── Hacking / System State ───────────────────────────────────────────────

// IsSystemHacked reports whether a system was already hacked.
func (g *GameDB) IsSystemHacked(ctx context.Context, roomID, systemID string) bool {
	if g.pg != nil {
		row, err := g.pg.GetSharedSystemState(ctx, pgq.GetSharedSystemStateParams{
			WorldID:  g.worldID,
			SystemID: systemID,
		})
		if err != nil {
			return false
		}
		return row.Hacked
	}
	hacked, err := g.sqlite.GetSystemHacked(ctx, sqliteq.GetSystemHackedParams{
		RoomID:   roomID,
		SystemID: systemID,
	})
	if err != nil {
		return false
	}
	return hacked == 1
}

// GetSystemAlert returns the alert level for a system.
func (g *GameDB) GetSystemAlert(ctx context.Context, roomID, systemID string) int {
	if g.pg != nil {
		row, err := g.pg.GetSharedSystemState(ctx, pgq.GetSharedSystemStateParams{
			WorldID:  g.worldID,
			SystemID: systemID,
		})
		if err != nil {
			return 0
		}
		return int(row.AlertLevel)
	}
	alert, err := g.sqlite.GetSystemAlert(ctx, sqliteq.GetSystemAlertParams{
		RoomID:   roomID,
		SystemID: systemID,
	})
	if err != nil {
		return 0
	}
	return int(alert)
}

// MarkSystemHacked marks a system as hacked.
func (g *GameDB) MarkSystemHacked(ctx context.Context, roomID, systemID string) error {
	if g.pg != nil {
		return g.pg.MarkSharedSystemHacked(ctx, pgq.MarkSharedSystemHackedParams{
			WorldID:  g.worldID,
			SystemID: systemID,
		})
	}
	return g.sqlite.MarkSystemHacked(ctx, sqliteq.MarkSystemHackedParams{
		RoomID:   roomID,
		SystemID: systemID,
	})
}

// IncrementSystemAlert increments the alert level for a system.
func (g *GameDB) IncrementSystemAlert(ctx context.Context, roomID, systemID string) error {
	if g.pg != nil {
		return g.pg.IncrementSharedAlert(ctx, pgq.IncrementSharedAlertParams{
			WorldID:  g.worldID,
			SystemID: systemID,
		})
	}
	return g.sqlite.IncrementSystemAlert(ctx, sqliteq.IncrementSystemAlertParams{
		RoomID:   roomID,
		SystemID: systemID,
	})
}

// UpsertSystemAlert sets the alert level for a system (used by HackMulti).
func (g *GameDB) UpsertSystemAlert(ctx context.Context, roomID, systemID string, alertLevel int) error {
	if g.pg != nil {
		return g.pg.UpsertSharedSystemAlert(ctx, pgq.UpsertSharedSystemAlertParams{
			WorldID:    g.worldID,
			SystemID:   systemID,
			AlertLevel: int32(alertLevel),
		})
	}
	return g.sqlite.UpsertSystemAlert(ctx, sqliteq.UpsertSystemAlertParams{
		RoomID:   roomID,
		SystemID: systemID,
		Alert:    int64(alertLevel),
	})
}

// InsertBounty records a bounty for a failed exfil.
func (g *GameDB) InsertBounty(ctx context.Context, roomID, npcID string, createdAt int64) error {
	if g.pg != nil {
		return g.pg.InsertSharedBounty(ctx, pgq.InsertSharedBountyParams{
			WorldID:   g.worldID,
			RoomID:    roomID,
			NpcID:     npcID,
			CreatedAt: int32(createdAt),
		})
	}
	return g.sqlite.InsertBounty(ctx, sqliteq.InsertBountyParams{
		RoomID:    roomID,
		NpcID:     sql.NullString{String: npcID, Valid: true},
		CreatedAt: sql.NullInt64{Int64: createdAt, Valid: true},
	})
}

// SetVulnWindow sets a vulnerability window for a system.
func (g *GameDB) SetVulnWindow(ctx context.Context, systemID string, bonus, expiresAction int) error {
	if g.pg != nil {
		return g.pg.SetSharedVulnWindow(ctx, pgq.SetSharedVulnWindowParams{
			WorldID:       g.worldID,
			SystemID:      systemID,
			Bonus:         int32(bonus),
			ExpiresAction: int32(expiresAction),
		})
	}
	return g.sqlite.SetVulnWindow(ctx, sqliteq.SetVulnWindowParams{
		SystemID:      systemID,
		Bonus:         sql.NullInt64{Int64: int64(bonus), Valid: true},
		ExpiresAction: sql.NullInt64{Int64: int64(expiresAction), Valid: true},
	})
}

// VulnBonus returns the vulnerability bonus for a system, or 0 if expired.
func (g *GameDB) VulnBonus(ctx context.Context, systemID string, currentAction int) int {
	if g.pg != nil {
		row, err := g.pg.GetSharedVulnWindow(ctx, pgq.GetSharedVulnWindowParams{
			WorldID:  g.worldID,
			SystemID: systemID,
		})
		if err != nil {
			return 0
		}
		if int(row.ExpiresAction) < currentAction {
			g.pg.DeleteSharedVulnWindow(ctx, pgq.DeleteSharedVulnWindowParams{ //nolint:errcheck
				WorldID:  g.worldID,
				SystemID: systemID,
			})
			return 0
		}
		return int(row.Bonus)
	}
	row, err := g.sqlite.GetVulnWindow(ctx, systemID)
	if err != nil {
		return 0
	}
	if !row.Bonus.Valid || !row.ExpiresAction.Valid {
		return 0
	}
	if int64(currentAction) > row.ExpiresAction.Int64 {
		g.sqlite.DeleteVulnWindow(ctx, systemID) //nolint:errcheck
		return 0
	}
	return int(row.Bonus.Int64)
}

// ─── Locking ───────────────────────────────────────────────────────────────

// IsLocked reports whether a lock is currently locked.
func (g *GameDB) IsLocked(ctx context.Context, lockID string) bool {
	if g.pg != nil {
		unlocked, err := g.pg.GetSharedLockState(ctx, pgq.GetSharedLockStateParams{
			WorldID: g.worldID,
			LockID:  lockID,
		})
		if err != nil {
			return true // default: locked
		}
		return !unlocked
	}
	var un int
	err := g.sqliteDB.QueryRow(`SELECT unlocked FROM lock_state WHERE lock_id=?`, lockID).Scan(&un)
	if err != nil {
		return true
	}
	return un == 0
}

// UnlockLock sets a lock to unlocked.
func (g *GameDB) UnlockLock(ctx context.Context, lockID string) error {
	if g.pg != nil {
		return g.pg.SetSharedLockUnlocked(ctx, pgq.SetSharedLockUnlockedParams{
			WorldID: g.worldID,
			LockID:  lockID,
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT INTO lock_state (lock_id, unlocked) VALUES (?,1)
		 ON CONFLICT(lock_id) DO UPDATE SET unlocked=1`,
		lockID,
	)
	return err
}

// ─── Generation / Content Cache ───────────────────────────────────────────

// GetCachedContent returns cached generated content by hash.
func (g *GameDB) GetCachedContent(ctx context.Context, hash string) (string, error) {
	if g.pg != nil {
		row, err := g.pg.GetSharedGeneratedContent(ctx, pgq.GetSharedGeneratedContentParams{
			WorldID:    g.worldID,
			PromptHash: hash,
		})
		if err != nil {
			return "", normErr(err)
		}
		return row.YamlBlob, nil
	}
	var blob string
	err := g.sqliteDB.QueryRow(
		`SELECT yaml_blob FROM generated_content WHERE prompt_hash=? AND type='room'`, hash,
	).Scan(&blob)
	return blob, err
}

// PersistContent stores generated content in the cache.
func (g *GameDB) PersistContent(ctx context.Context, hash, contentType, yamlBlob string, createdAt int64) error {
	if g.pg != nil {
		return g.pg.UpsertSharedGeneratedContent(ctx, pgq.UpsertSharedGeneratedContentParams{
			WorldID:    g.worldID,
			PromptHash: hash,
			Type:       contentType,
			YamlBlob:   yamlBlob,
			CreatedAt:  int32(createdAt),
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT OR IGNORE INTO generated_content (prompt_hash, type, yaml_blob, created_at) VALUES (?,?,?,?)`,
		hash, contentType, yamlBlob, createdAt,
	)
	return err
}

// ─── Item Mods ────────────────────────────────────────────────────────────

// ApplyItemMod applies a mod to an item instance.
func (g *GameDB) ApplyItemMod(ctx context.Context, itemInstance, modID string) error {
	if g.pg != nil {
		return g.pg.UpsertSharedItemMod(ctx, pgq.UpsertSharedItemModParams{
			AccountID:    g.pgUUID(),
			WorldID:      g.worldID,
			ItemInstance: itemInstance,
			ModID:        modID,
			AppliedAt:    int32(time.Now().Unix()),
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT OR REPLACE INTO item_mods (item_instance, mod_id, applied_at) VALUES (?, ?, ?)`,
		itemInstance, modID, time.Now().Unix())
	return err
}

// GetItemMod returns the mod ID for an item instance.
func (g *GameDB) GetItemMod(ctx context.Context, itemInstance string) (string, bool) {
	if g.pg != nil {
		row, err := g.pg.GetSharedItemMod(ctx, pgq.GetSharedItemModParams{
			AccountID:    g.pgUUID(),
			WorldID:      g.worldID,
			ItemInstance: itemInstance,
		})
		if err != nil {
			return "", false
		}
		return row.ModID, true
	}
	var modID string
	err := g.sqliteDB.QueryRow(`SELECT mod_id FROM item_mods WHERE item_instance = ?`, itemInstance).Scan(&modID)
	if err != nil {
		return "", false
	}
	return modID, true
}

// ─── Enchanting ───────────────────────────────────────────────────────────

// ApplyEnchant applies an enchantment to an item.
func (g *GameDB) ApplyEnchant(ctx context.Context, itemID, enchantID string, level, appliedAt int) error {
	if g.pg != nil {
		return g.pg.ApplySharedEnchant(ctx, pgq.ApplySharedEnchantParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
			EnchantID: enchantID,
			Level:     int32(level),
			AppliedAt: int32(appliedAt),
		})
	}
	return g.sqlite.ApplyEnchant(ctx, sqliteq.ApplyEnchantParams{
		ItemID:    itemID,
		EnchantID: enchantID,
		Level:     int64(level),
		AppliedAt: int64(appliedAt),
	})
}

// EnchantRecord is a record of an enchantment.
type EnchantRecord struct {
	ItemID    string
	EnchantID string
	Level     int
}

// ListEnchants returns all enchantments on an item.
func (g *GameDB) ListEnchants(ctx context.Context, itemID string) ([]EnchantRecord, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedEnchants(ctx, pgq.ListSharedEnchantsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ItemID:    itemID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]EnchantRecord, len(rows))
		for i, r := range rows {
			out[i] = EnchantRecord{ItemID: r.ItemID, EnchantID: r.EnchantID, Level: int(r.Level)}
		}
		return out, nil
	}
	rows, err := g.sqlite.ListEnchants(ctx, itemID)
	if err != nil {
		return nil, err
	}
	out := make([]EnchantRecord, len(rows))
	for i, r := range rows {
		out[i] = EnchantRecord{ItemID: r.ItemID, EnchantID: r.EnchantID, Level: int(r.Level)}
	}
	return out, nil
}

// AddEnchantingXP adds enchanting XP.
func (g *GameDB) AddEnchantingXP(ctx context.Context, amount int) error {
	if g.pg != nil {
		return g.pg.AddSharedEnchantingXP(ctx, pgq.AddSharedEnchantingXPParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Xp:        int32(amount),
		})
	}
	return g.sqlite.AddEnchantingXP(ctx, sqliteq.AddEnchantingXPParams{
		Xp:   int64(amount),
		Xp_2: int64(amount),
	})
}

// GetEnchantingXPState returns current enchanting XP and level.
func (g *GameDB) GetEnchantingXPState(ctx context.Context) (xp, level int, err error) {
	if g.pg != nil {
		row, qerr := g.pg.GetSharedEnchantingXP(ctx, pgq.GetSharedEnchantingXPParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if qerr != nil {
			return 0, 0, normErr(qerr)
		}
		return int(row.Xp), int(row.Level), nil
	}
	row, qerr := g.sqlite.GetEnchantingXPState(ctx)
	if qerr != nil {
		return 0, 0, qerr
	}
	return int(row.Xp), int(row.Level), nil
}

// CountEnchantingTable returns the count of enchanting tables in a room.
func (g *GameDB) CountEnchantingTable(ctx context.Context, roomID string) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedEnchantingTable(ctx, pgq.CountSharedEnchantingTableParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
	}
	return g.sqlite.CountEnchantingTable(ctx, roomID)
}

// DeductEnchantingXP deducts enchanting XP.
func (g *GameDB) DeductEnchantingXP(ctx context.Context, amount int) error {
	if g.pg != nil {
		return g.pg.DeductSharedEnchantingXP(ctx, pgq.DeductSharedEnchantingXPParams{
			Xp:        int32(amount),
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
	}
	return g.sqlite.DeductEnchantingXP(ctx, int64(amount))
}

// ─── Augments ─────────────────────────────────────────────────────────────

// AugmentRecord holds augment data.
type AugmentRecord struct {
	Skill string
	Bonus int
}

// InstallAugment installs a player augment.
func (g *GameDB) InstallAugment(ctx context.Context, skill string, bonus int) error {
	if g.pg != nil {
		return g.pg.UpsertSharedAugment(ctx, pgq.UpsertSharedAugmentParams{
			AccountID:   g.pgUUID(),
			WorldID:     g.worldID,
			Skill:       skill,
			Bonus:       int32(bonus),
			InstalledAt: int32(time.Now().Unix()),
		})
	}
	_, err := g.sqliteDB.Exec(`INSERT INTO player_augments (skill, bonus, installed_at) VALUES (?, ?, ?)`,
		skill, bonus, time.Now().Unix())
	return err
}

// AugmentCount returns the number of installed augments.
func (g *GameDB) AugmentCount(ctx context.Context) int {
	if g.pg != nil {
		rows, err := g.pg.ListSharedAugments(ctx, pgq.ListSharedAugmentsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return 0
		}
		return len(rows)
	}
	var count int
	g.sqliteDB.QueryRow(`SELECT COUNT(*) FROM player_augments`).Scan(&count) //nolint:errcheck
	return count
}

// AugmentTotalBonus returns the total bonus for a skill from augments.
func (g *GameDB) AugmentTotalBonus(ctx context.Context, skill string) int {
	if g.pg != nil {
		row, err := g.pg.GetSharedAugment(ctx, pgq.GetSharedAugmentParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Skill:     skill,
		})
		if err != nil {
			return 0
		}
		return int(row.Bonus)
	}
	var total int
	_ = g.sqliteDB.QueryRow(`SELECT COALESCE(SUM(bonus), 0) FROM player_augments WHERE skill = ?`, skill).Scan(&total)
	return total
}

// ListAugments returns all installed augments.
func (g *GameDB) ListAugments(ctx context.Context) ([]AugmentRecord, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedAugments(ctx, pgq.ListSharedAugmentsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]AugmentRecord, len(rows))
		for i, r := range rows {
			out[i] = AugmentRecord{Skill: r.Skill, Bonus: int(r.Bonus)}
		}
		return out, nil
	}
	rows, err := g.sqliteDB.Query(`SELECT skill, bonus FROM player_augments ORDER BY installed_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AugmentRecord
	for rows.Next() {
		var a AugmentRecord
		if err := rows.Scan(&a.Skill, &a.Bonus); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ─── Weather ──────────────────────────────────────────────────────────────

// GetWeatherCondition returns the current weather condition for a biome.
func (g *GameDB) GetWeatherCondition(ctx context.Context, biome string) (string, error) {
	if g.pg != nil {
		cond, err := g.pg.GetSharedWeatherCondition(ctx, pgq.GetSharedWeatherConditionParams{
			WorldID: g.worldID,
			Biome:   biome,
		})
		if err != nil {
			return "clear", normErr(err)
		}
		return cond, nil
	}
	cond, err := g.sqlite.GetWeatherCondition(ctx, biome)
	if err == sql.ErrNoRows {
		return "clear", nil
	}
	return cond, err
}

// WeatherState holds condition and expiry.
type WeatherState struct {
	Condition     string
	ExpiresAction int
}

// GetWeatherState returns the full weather state.
func (g *GameDB) GetWeatherState(ctx context.Context, biome string) (WeatherState, error) {
	if g.pg != nil {
		row, err := g.pg.GetSharedWeather(ctx, pgq.GetSharedWeatherParams{
			WorldID: g.worldID,
			Biome:   biome,
		})
		if err != nil {
			return WeatherState{}, normErr(err)
		}
		return WeatherState{
			Condition:     row.Condition,
			ExpiresAction: int(row.ExpiresAction),
		}, nil
	}
	row, err := g.sqlite.GetWeatherState(ctx, biome)
	if err != nil {
		return WeatherState{}, err
	}
	return WeatherState{
		Condition:     row.Condition,
		ExpiresAction: int(row.ExpiresAction),
	}, nil
}

// UpsertWeatherState sets the weather state for a biome.
func (g *GameDB) UpsertWeatherState(ctx context.Context, biome, condition string, expiresAction int) error {
	if g.pg != nil {
		return g.pg.UpsertSharedWeather(ctx, pgq.UpsertSharedWeatherParams{
			WorldID:       g.worldID,
			Biome:         biome,
			Condition:     condition,
			ExpiresAction: int32(expiresAction),
		})
	}
	return g.sqlite.UpsertWeatherState(ctx, sqliteq.UpsertWeatherStateParams{
		Biome:         biome,
		Condition:     condition,
		ExpiresAction: int64(expiresAction),
	})
}

// ─── Reputation / Trading ─────────────────────────────────────────────────

// GetReputation returns the player's reputation with a faction.
func (g *GameDB) GetReputation(ctx context.Context, faction string) int {
	if g.pg != nil {
		val, err := g.pg.GetSharedReputation(ctx, pgq.GetSharedReputationParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Faction:   faction,
		})
		if err != nil {
			return 0
		}
		return int(val)
	}
	var value int
	g.sqliteDB.QueryRow(`SELECT value FROM player_reputation WHERE faction=?`, faction).Scan(&value) //nolint:errcheck
	return value
}

// AdjustReputation adds an arbitrary signed delta to a faction's reputation.
// Used by character creation, quest rewards, and any future reputation event.
func (g *GameDB) AdjustReputation(ctx context.Context, faction string, delta int) error {
	if g.pg != nil {
		current := g.GetReputation(ctx, faction)
		return g.pg.UpsertSharedReputation(ctx, pgq.UpsertSharedReputationParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Faction:   faction,
			Value:     int32(current + delta),
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT INTO player_reputation (faction, value) VALUES (?, ?)
		 ON CONFLICT(faction) DO UPDATE SET value = value + excluded.value`,
		faction, delta,
	)
	return err
}

// IncrementReputation adds 1 to a faction's reputation.
func (g *GameDB) IncrementReputation(ctx context.Context, faction string) error {
	if g.pg != nil {
		current := g.GetReputation(ctx, faction)
		return g.pg.UpsertSharedReputation(ctx, pgq.UpsertSharedReputationParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Faction:   faction,
			Value:     int32(current + 1),
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT INTO player_reputation (faction, value) VALUES (?,1)
		 ON CONFLICT(faction) DO UPDATE SET value=value+1`,
		faction,
	)
	return err
}

// ReputationRecord holds reputation data.
type ReputationRecord struct {
	Faction string
	Value   int
}

// ListReputations returns all faction reputations.
func (g *GameDB) ListReputations(ctx context.Context) ([]ReputationRecord, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedReputations(ctx, pgq.ListSharedReputationsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]ReputationRecord, len(rows))
		for i, r := range rows {
			out[i] = ReputationRecord{Faction: r.Faction, Value: int(r.Value)}
		}
		return out, nil
	}
	rows, err := g.sqlite.ListReputations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ReputationRecord, len(rows))
	for i, r := range rows {
		out[i] = ReputationRecord{Faction: r.Faction, Value: int(r.Value)}
	}
	return out, nil
}

// ListHighRepFactions returns factions where reputation >= 3.
func (g *GameDB) ListHighRepFactions(ctx context.Context) ([]ReputationRecord, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedHighRepFactions(ctx, pgq.ListSharedHighRepFactionsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]ReputationRecord, len(rows))
		for i, r := range rows {
			out[i] = ReputationRecord{Faction: r.Faction, Value: int(r.Value)}
		}
		return out, nil
	}
	rows, err := g.sqlite.ListHighRepFactions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ReputationRecord, len(rows))
	for i, r := range rows {
		out[i] = ReputationRecord{Faction: r.Faction, Value: int(r.Value)}
	}
	return out, nil
}

// ─── Espionage / Stealth ──────────────────────────────────────────────────

// StealthState holds stealth info.
type StealthState struct {
	Level    int
	Disguise string
}

// LoadStealth returns the stealth state.
func (g *GameDB) LoadStealth(ctx context.Context) StealthState {
	if g.pg != nil {
		row, err := g.pg.GetSharedStealth(ctx, pgq.GetSharedStealthParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return StealthState{Level: 50, Disguise: "none"}
		}
		return StealthState{Level: int(row.Level), Disguise: row.Disguise}
	}
	var s StealthState
	err := g.sqliteDB.QueryRow(`SELECT level, disguise FROM player_stealth WHERE id=1`).
		Scan(&s.Level, &s.Disguise)
	if err != nil {
		return StealthState{Level: 50, Disguise: "none"}
	}
	return s
}

// SaveStealth persists stealth state.
func (g *GameDB) SaveStealth(ctx context.Context, s StealthState) error {
	if g.pg != nil {
		return g.pg.UpsertSharedStealth(ctx, pgq.UpsertSharedStealthParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Level:     int32(s.Level),
			Disguise:  s.Disguise,
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT INTO player_stealth (id, level, disguise) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET level=excluded.level, disguise=excluded.disguise`,
		s.Level, s.Disguise,
	)
	return err
}

// RecordNPCMemory records an NPC interaction.
func (g *GameDB) RecordNPCMemory(ctx context.Context, npcID, action string) error {
	if g.pg != nil {
		return g.pg.UpsertSharedNPCMemory(ctx, pgq.UpsertSharedNPCMemoryParams{
			WorldID: g.worldID,
			NpcID:   npcID,
			Action:  action,
			Ts:      int32(time.Now().Unix()),
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT INTO npc_memory (npc_id, action, ts) VALUES (?,?,?)
		 ON CONFLICT(npc_id, action) DO UPDATE SET ts=excluded.ts`,
		npcID, action, time.Now().Unix(),
	)
	return err
}

// ─── Hideout ──────────────────────────────────────────────────────────────

// ListHideoutUpgrades returns installed upgrade IDs.
func (g *GameDB) ListHideoutUpgrades(ctx context.Context) ([]string, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedHideoutUpgrades(ctx, pgq.ListSharedHideoutUpgradesParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(rows))
		for i, r := range rows {
			ids[i] = r.UpgradeID
		}
		return ids, nil
	}
	rows, err := g.sqliteDB.Query(`SELECT upgrade_id FROM hideout_upgrades`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// HasHideoutUpgrade reports whether an upgrade is installed.
func (g *GameDB) HasHideoutUpgrade(ctx context.Context, id string) (bool, error) {
	ids, err := g.ListHideoutUpgrades(ctx)
	if err != nil {
		return false, err
	}
	for _, uid := range ids {
		if uid == id {
			return true, nil
		}
	}
	return false, nil
}

// InstallHideoutUpgrade records a hideout upgrade.
func (g *GameDB) InstallHideoutUpgrade(ctx context.Context, id string) error {
	if g.pg != nil {
		return g.pg.InsertSharedHideoutUpgrade(ctx, pgq.InsertSharedHideoutUpgradeParams{
			AccountID:   g.pgUUID(),
			WorldID:     g.worldID,
			UpgradeID:   id,
			InstalledAt: int32(time.Now().Unix()),
		})
	}
	_, err := g.sqliteDB.Exec(
		`INSERT OR IGNORE INTO hideout_upgrades (upgrade_id, installed_at) VALUES (?, ?)`,
		id, time.Now().Unix(),
	)
	return err
}

// ─── Quests ───────────────────────────────────────────────────────────────

// QuestRecord holds quest data.
type QuestRecord struct {
	ID             string
	Title          string
	Description    string
	Status         string
	ObjType        string
	ObjTarget      string
	ObjRoom        string
	ObjCount       int
	ObjProgress    int
	RewardCredits  int
	RewardXPSkill  string
	RewardXPAmount int
	RewardItemID   string
	RewardItemName string
	RewardItemDesc string
	GiverNPCID     string
	AcceptedAt     int64
	NextQuestID    string
}

// AcceptQuest inserts a new quest.
func (g *GameDB) AcceptQuest(ctx context.Context, q QuestRecord) error {
	if g.pg != nil {
		return g.pg.AcceptSharedQuest(ctx, pgq.AcceptSharedQuestParams{
			ID:             q.ID,
			AccountID:      g.pgUUID(),
			WorldID:        g.worldID,
			Title:          q.Title,
			Description:    pgText(q.Description),
			Status:         "active",
			ObjType:        q.ObjType,
			ObjTarget:      q.ObjTarget,
			ObjRoom:        pgText(q.ObjRoom),
			ObjCount:       int32(q.ObjCount),
			ObjProgress:    0,
			RewardCredits:  int32(q.RewardCredits),
			RewardXpSkill:  pgText(q.RewardXPSkill),
			RewardXpAmount: int32(q.RewardXPAmount),
			RewardItemID:   pgText(q.RewardItemID),
			RewardItemName: pgText(q.RewardItemName),
			RewardItemDesc: pgText(q.RewardItemDesc),
			GiverNpcID:     pgText(q.GiverNPCID),
			AcceptedAt:     int32(q.AcceptedAt),
			NextQuestID:    q.NextQuestID,
		})
	}
	return g.sqlite.AcceptQuest(ctx, sqliteq.AcceptQuestParams{
		ID:             q.ID,
		Title:          q.Title,
		Description:    sql.NullString{String: q.Description, Valid: q.Description != ""},
		Status:         "active",
		ObjType:        q.ObjType,
		ObjTarget:      q.ObjTarget,
		ObjRoom:        sql.NullString{String: q.ObjRoom, Valid: q.ObjRoom != ""},
		ObjCount:       int64(q.ObjCount),
		ObjProgress:    0,
		RewardCredits:  int64(q.RewardCredits),
		RewardXpSkill:  sql.NullString{String: q.RewardXPSkill, Valid: q.RewardXPSkill != ""},
		RewardXpAmount: int64(q.RewardXPAmount),
		RewardItemID:   sql.NullString{String: q.RewardItemID, Valid: q.RewardItemID != ""},
		RewardItemName: sql.NullString{String: q.RewardItemName, Valid: q.RewardItemName != ""},
		RewardItemDesc: sql.NullString{String: q.RewardItemDesc, Valid: q.RewardItemDesc != ""},
		GiverNpcID:     sql.NullString{String: q.GiverNPCID, Valid: q.GiverNPCID != ""},
		AcceptedAt:     q.AcceptedAt,
		NextQuestID:    q.NextQuestID,
	})
}

// ListActiveQuests returns active quests.
func (g *GameDB) ListActiveQuests(ctx context.Context) ([]QuestRecord, error) {
	if g.pg != nil {
		rows, err := g.pg.GetSharedActiveQuests(ctx, pgq.GetSharedActiveQuestsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]QuestRecord, len(rows))
		for i, r := range rows {
			out[i] = QuestRecord{
				ID:             r.ID,
				Title:          r.Title,
				Description:    r.Description.String,
				Status:         r.Status,
				ObjType:        r.ObjType,
				ObjTarget:      r.ObjTarget,
				ObjRoom:        r.ObjRoom.String,
				ObjCount:       int(r.ObjCount),
				ObjProgress:    int(r.ObjProgress),
				RewardCredits:  int(r.RewardCredits),
				RewardXPSkill:  r.RewardXpSkill.String,
				RewardXPAmount: int(r.RewardXpAmount),
				RewardItemID:   r.RewardItemID.String,
				RewardItemName: r.RewardItemName.String,
				RewardItemDesc: r.RewardItemDesc.String,
				GiverNPCID:     r.GiverNpcID.String,
				AcceptedAt:     int64(r.AcceptedAt),
				NextQuestID:    r.NextQuestID,
			}
		}
		return out, nil
	}
	rows, err := g.sqlite.ListActiveQuests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]QuestRecord, len(rows))
	for i, r := range rows {
		out[i] = questFromSqlc(r)
	}
	return out, nil
}

// ProgressQuest increments quest progress by n.
func (g *GameDB) ProgressQuest(ctx context.Context, id string, n int) error {
	if g.pg != nil {
		return g.pg.ProgressSharedQuest(ctx, pgq.ProgressSharedQuestParams{
			ObjProgress: int32(n),
			ID:          id,
			AccountID:   g.pgUUID(),
			WorldID:     g.worldID,
		})
	}
	return g.sqlite.ProgressQuest(ctx, sqliteq.ProgressQuestParams{
		ObjProgress: int64(n),
		ID:          id,
	})
}

// CompleteQuest marks a quest as completed.
func (g *GameDB) CompleteQuest(ctx context.Context, id string) error {
	if g.pg != nil {
		return g.pg.CompleteSharedQuest(ctx, pgq.CompleteSharedQuestParams{
			ID:        id,
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
	}
	return g.sqlite.CompleteQuest(ctx, id)
}

// FailQuest marks a quest as failed.
func (g *GameDB) FailQuest(ctx context.Context, id string) error {
	if g.pg != nil {
		return g.pg.FailSharedQuest(ctx, pgq.FailSharedQuestParams{
			ID:        id,
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
	}
	return g.sqlite.FailQuest(ctx, id)
}

// GetQuest fetches a single quest by ID.
func (g *GameDB) GetQuest(ctx context.Context, id string) (*QuestRecord, error) {
	if g.pg != nil {
		row, err := g.pg.GetSharedQuest(ctx, pgq.GetSharedQuestParams{
			ID:        id,
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, normErr(err)
		}
		q := QuestRecord{
			ID:             row.ID,
			Title:          row.Title,
			Description:    row.Description.String,
			Status:         row.Status,
			ObjType:        row.ObjType,
			ObjTarget:      row.ObjTarget,
			ObjRoom:        row.ObjRoom.String,
			ObjCount:       int(row.ObjCount),
			ObjProgress:    int(row.ObjProgress),
			RewardCredits:  int(row.RewardCredits),
			RewardXPSkill:  row.RewardXpSkill.String,
			RewardXPAmount: int(row.RewardXpAmount),
			RewardItemID:   row.RewardItemID.String,
			RewardItemName: row.RewardItemName.String,
			RewardItemDesc: row.RewardItemDesc.String,
			GiverNPCID:     row.GiverNpcID.String,
			AcceptedAt:     int64(row.AcceptedAt),
			NextQuestID:    row.NextQuestID,
		}
		return &q, nil
	}
	row, err := g.sqlite.GetQuest(ctx, id)
	if err != nil {
		return nil, normErr(err)
	}
	q := questFromSqlc(row)
	return &q, nil
}

// ListActiveQuestIDs returns a set of active quest IDs.
func (g *GameDB) ListActiveQuestIDs(ctx context.Context) (map[string]bool, error) {
	if g.pg != nil {
		idList, err := g.pg.ListSharedActiveQuestIDs(ctx, pgq.ListSharedActiveQuestIDsParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if err != nil {
			return nil, err
		}
		ids := make(map[string]bool, len(idList))
		for _, id := range idList {
			ids[id] = true
		}
		return ids, nil
	}
	idList, err := g.sqlite.ListActiveQuestIDs(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(idList))
	for _, id := range idList {
		ids[id] = true
	}
	return ids, nil
}

// ListActiveQuestsByTypeTarget returns active quests matching type and target.
func (g *GameDB) ListActiveQuestsByTypeTarget(ctx context.Context, objType, target string) ([]QuestRecord, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedActiveQuestsByTypeTarget(ctx, pgq.ListSharedActiveQuestsByTypeTargetParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			ObjType:   objType,
			ObjTarget: target,
		})
		if err != nil {
			return nil, err
		}
		out := make([]QuestRecord, len(rows))
		for i, r := range rows {
			out[i] = QuestRecord{
				ID:             r.ID,
				Title:          r.Title,
				Description:    r.Description.String,
				Status:         r.Status,
				ObjType:        r.ObjType,
				ObjTarget:      r.ObjTarget,
				ObjRoom:        r.ObjRoom.String,
				ObjCount:       int(r.ObjCount),
				ObjProgress:    int(r.ObjProgress),
				RewardCredits:  int(r.RewardCredits),
				RewardXPSkill:  r.RewardXpSkill.String,
				RewardXPAmount: int(r.RewardXpAmount),
				RewardItemID:   r.RewardItemID.String,
				RewardItemName: r.RewardItemName.String,
				RewardItemDesc: r.RewardItemDesc.String,
				GiverNPCID:     r.GiverNpcID.String,
				AcceptedAt:     int64(r.AcceptedAt),
				NextQuestID:    r.NextQuestID,
			}
		}
		return out, nil
	}
	rows, err := g.sqlite.ListActiveQuestsByTypeTarget(ctx, sqliteq.ListActiveQuestsByTypeTargetParams{
		ObjType:   objType,
		ObjTarget: target,
	})
	if err != nil {
		return nil, err
	}
	out := make([]QuestRecord, len(rows))
	for i, r := range rows {
		out[i] = questFromSqlc(r)
	}
	return out, nil
}

// questFromSqlc converts a sqliteq.Quest to QuestRecord.
func questFromSqlc(q sqliteq.Quest) QuestRecord {
	return QuestRecord{
		ID:             q.ID,
		Title:          q.Title,
		Description:    q.Description.String,
		Status:         q.Status,
		ObjType:        q.ObjType,
		ObjTarget:      q.ObjTarget,
		ObjRoom:        q.ObjRoom.String,
		ObjCount:       int(q.ObjCount),
		ObjProgress:    int(q.ObjProgress),
		RewardCredits:  int(q.RewardCredits),
		RewardXPSkill:  q.RewardXpSkill.String,
		RewardXPAmount: int(q.RewardXpAmount),
		RewardItemID:   q.RewardItemID.String,
		RewardItemName: q.RewardItemName.String,
		RewardItemDesc: q.RewardItemDesc.String,
		GiverNPCID:     q.GiverNpcID.String,
		AcceptedAt:     q.AcceptedAt,
		NextQuestID:    q.NextQuestID,
	}
}

// ─── Events ───────────────────────────────────────────────────────────────

// WorldEvent mirrors the world_events table.
type WorldEvent struct {
	ID             string
	Type           string
	Title          string
	Description    string
	TargetRoom     string
	Faction        string
	PayoutCredits  int
	PayoutItemID   string
	PayoutItemName string
	PayoutItemDesc string
	Status         string
	ExpiresActions int
	CreatedActions int
	CreatedAt      int64
}

// ListActiveEvents returns all active world events.
func (g *GameDB) ListActiveEvents(ctx context.Context) ([]WorldEvent, error) {
	if g.pg != nil {
		rows, err := g.pg.ListSharedActiveEvents(ctx, g.worldID)
		if err != nil {
			return nil, err
		}
		events := make([]WorldEvent, 0, len(rows))
		for _, r := range rows {
			events = append(events, WorldEvent{
				ID:             r.ID,
				Type:           r.Type,
				Title:          r.Title,
				Description:    r.Description.String,
				TargetRoom:     r.TargetRoom,
				Faction:        r.Faction.String,
				PayoutCredits:  int(r.PayoutCredits),
				PayoutItemID:   r.PayoutItemID.String,
				PayoutItemName: r.PayoutItemName.String,
				PayoutItemDesc: r.PayoutItemDesc.String,
				Status:         r.Status,
				ExpiresActions: int(r.ExpiresActions),
				CreatedActions: int(r.CreatedActions),
				CreatedAt:      int64(r.CreatedAt),
			})
		}
		return events, nil
	}
	sqlRows, err := g.sqliteDB.Query(
		`SELECT id, type, title, description, target_room, faction,
		        payout_credits, payout_item_id, payout_item_name, payout_item_desc,
		        status, expires_actions, created_actions, created_at
		 FROM world_events WHERE status='active'`,
	)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()
	return scanSQLiteEvents(sqlRows)
}

// GetEvent returns a single world event by ID.
func (g *GameDB) GetEvent(ctx context.Context, id string) (*WorldEvent, error) {
	if g.pg != nil {
		r, err := g.pg.GetSharedEvent(ctx, pgq.GetSharedEventParams{
			ID:      id,
			WorldID: g.worldID,
		})
		if err != nil {
			return nil, normErr(err)
		}
		return &WorldEvent{
			ID:             r.ID,
			Type:           r.Type,
			Title:          r.Title,
			Description:    r.Description.String,
			TargetRoom:     r.TargetRoom,
			Faction:        r.Faction.String,
			PayoutCredits:  int(r.PayoutCredits),
			PayoutItemID:   r.PayoutItemID.String,
			PayoutItemName: r.PayoutItemName.String,
			PayoutItemDesc: r.PayoutItemDesc.String,
			Status:         r.Status,
			ExpiresActions: int(r.ExpiresActions),
			CreatedActions: int(r.CreatedActions),
			CreatedAt:      int64(r.CreatedAt),
		}, nil
	}
	row := g.sqliteDB.QueryRow(
		`SELECT id, type, title, description, target_room, faction,
		        payout_credits, payout_item_id, payout_item_name, payout_item_desc,
		        status, expires_actions, created_actions, created_at
		 FROM world_events WHERE id=?`, id,
	)
	var e WorldEvent
	err := row.Scan(
		&e.ID, &e.Type, &e.Title, &e.Description, &e.TargetRoom, &e.Faction,
		&e.PayoutCredits, &e.PayoutItemID, &e.PayoutItemName, &e.PayoutItemDesc,
		&e.Status, &e.ExpiresActions, &e.CreatedActions, &e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// CreateEvent inserts a new world event.
func (g *GameDB) CreateEvent(ctx context.Context, e WorldEvent) error {
	if e.CreatedAt == 0 {
		e.CreatedAt = time.Now().Unix()
	}
	return g.InsertWorldEvent(ctx, WorldEventParams{
		ID:             e.ID,
		Type:           e.Type,
		Title:          e.Title,
		Description:    e.Description,
		TargetRoom:     e.TargetRoom,
		Faction:        e.Faction,
		PayoutCredits:  e.PayoutCredits,
		PayoutItemID:   e.PayoutItemID,
		PayoutItemName: e.PayoutItemName,
		PayoutItemDesc: e.PayoutItemDesc,
		Status:         "active",
		ExpiresActions: e.ExpiresActions,
		CreatedActions: e.CreatedActions,
		CreatedAt:      e.CreatedAt,
	})
}

// CompleteEvent sets an event status to 'completed'.
func (g *GameDB) CompleteEvent(ctx context.Context, id string) error {
	if g.pg != nil {
		return g.pg.CompleteSharedEvent(ctx, pgq.CompleteSharedEventParams{
			ID:      id,
			WorldID: g.worldID,
		})
	}
	_, err := g.sqliteDB.Exec(`UPDATE world_events SET status='completed' WHERE id=?`, id)
	return err
}

// ExpireOldEvents expires events whose lifetime has elapsed.
func (g *GameDB) ExpireOldEvents(ctx context.Context, currentActions int) (int, error) {
	if g.pg != nil {
		tag, err := g.pg.ExpireSharedOldEvents(ctx, pgq.ExpireSharedOldEventsParams{
			WorldID:        g.worldID,
			CreatedActions: int32(currentActions),
		})
		if err != nil {
			return 0, err
		}
		return int(tag.RowsAffected()), nil
	}
	res, err := g.sqliteDB.Exec(
		`UPDATE world_events SET status='expired'
		 WHERE status='active' AND (created_actions + expires_actions) <= ?`,
		currentActions,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// scanSQLiteEvents scans world_events rows.
func scanSQLiteEvents(rows *sql.Rows) ([]WorldEvent, error) {
	var events []WorldEvent
	for rows.Next() {
		var e WorldEvent
		err := rows.Scan(
			&e.ID, &e.Type, &e.Title, &e.Description, &e.TargetRoom, &e.Faction,
			&e.PayoutCredits, &e.PayoutItemID, &e.PayoutItemName, &e.PayoutItemDesc,
			&e.Status, &e.ExpiresActions, &e.CreatedActions, &e.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ─── Resources / Mining ───────────────────────────────────────────────────

// ResourceState holds resource depletion state.
type ResourceState struct {
	Depleted        bool
	DepletedAtAction int
}

// GetResourceState returns the depletion state of a resource.
func (g *GameDB) GetResourceState(ctx context.Context, roomID, resourceID string) (ResourceState, error) {
	if g.pg != nil {
		row, err := g.pg.GetSharedResourceState(ctx, pgq.GetSharedResourceStateParams{
			WorldID:    g.worldID,
			ResourceID: resourceID,
		})
		if err != nil {
			return ResourceState{}, normErr(err)
		}
		var deplAt int
		if row.RespawnAt.Valid {
			deplAt = int(row.RespawnAt.Time.Unix())
		}
		return ResourceState{Depleted: row.Depleted, DepletedAtAction: deplAt}, nil
	}
	row, err := g.sqlite.GetResourceState(ctx, sqliteq.GetResourceStateParams{
		RoomID:     roomID,
		ResourceID: resourceID,
	})
	if err != nil {
		return ResourceState{}, err
	}
	return ResourceState{Depleted: row.Depleted == 1, DepletedAtAction: int(row.DepletedAtAction)}, nil
}

// DepleteResource marks a resource as depleted.
func (g *GameDB) DepleteResource(ctx context.Context, roomID, resourceID string, depletedAtAction int) error {
	if g.pg != nil {
		return g.pg.DepleteSharedResource(ctx, pgq.DepleteSharedResourceParams{
			WorldID:    g.worldID,
			ResourceID: resourceID,
			RoomID:     roomID,
			RespawnAt: pgtype.Timestamptz{
				Time:  time.Unix(int64(depletedAtAction), 0),
				Valid: true,
			},
		})
	}
	return g.sqlite.DepleteResource(ctx, sqliteq.DepleteResourceParams{
		RoomID:           roomID,
		ResourceID:       resourceID,
		DepletedAtAction: int64(depletedAtAction),
	})
}

// ClearResourceDepletion clears resource depletion.
func (g *GameDB) ClearResourceDepletion(ctx context.Context, roomID, resourceID string) error {
	if g.pg != nil {
		return g.pg.UndepleteSharedResource(ctx, pgq.UndepleteSharedResourceParams{
			WorldID:    g.worldID,
			ResourceID: resourceID,
		})
	}
	return g.sqlite.ClearResourceDepletion(ctx, sqliteq.ClearResourceDepletionParams{
		RoomID:     roomID,
		ResourceID: resourceID,
	})
}

// ─── Crops / Farming ──────────────────────────────────────────────────────

// ListReadyCrops returns seed IDs of ready crops.
func (g *GameDB) ListReadyCrops(ctx context.Context, roomID string, readyAtAction int) ([]string, error) {
	if g.pg != nil {
		return g.pg.GetSharedReadyCrops(ctx, pgq.GetSharedReadyCropsParams{
			WorldID:       g.worldID,
			RoomID:        roomID,
			ReadyAtAction: int32(readyAtAction),
		})
	}
	return g.sqlite.ListReadyCrops(ctx, sqliteq.ListReadyCropsParams{
		RoomID:        roomID,
		ReadyAtAction: int64(readyAtAction),
	})
}

// CountReadyCrops returns the count of ready crops of a type.
func (g *GameDB) CountReadyCrops(ctx context.Context, roomID, seedID string, readyAtAction int) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedReadyCrops(ctx, pgq.CountSharedReadyCropsParams{
			WorldID:       g.worldID,
			RoomID:        roomID,
			SeedID:        seedID,
			ReadyAtAction: int32(readyAtAction),
		})
	}
	return g.sqlite.CountReadyCrops(ctx, sqliteq.CountReadyCropsParams{
		RoomID:        roomID,
		SeedID:        seedID,
		ReadyAtAction: int64(readyAtAction),
	})
}

// HarvestCrops marks crops as harvested.
func (g *GameDB) HarvestCrops(ctx context.Context, roomID, seedID string, readyAtAction int) error {
	if g.pg != nil {
		return g.pg.HarvestSharedCrops(ctx, pgq.HarvestSharedCropsParams{
			WorldID:       g.worldID,
			RoomID:        roomID,
			SeedID:        seedID,
			ReadyAtAction: int32(readyAtAction),
		})
	}
	return g.sqlite.HarvestCrops(ctx, sqliteq.HarvestCropsParams{
		RoomID:        roomID,
		SeedID:        seedID,
		ReadyAtAction: int64(readyAtAction),
	})
}

// ListActiveCropSlots returns occupied slot numbers.
func (g *GameDB) ListActiveCropSlots(ctx context.Context, roomID string) ([]int, error) {
	if g.pg != nil {
		slots, err := g.pg.GetSharedUsedSlots(ctx, pgq.GetSharedUsedSlotsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]int, len(slots))
		for i, s := range slots {
			out[i] = int(s)
		}
		return out, nil
	}
	slots, err := g.sqlite.ListActiveCropSlots(ctx, roomID)
	if err != nil {
		return nil, err
	}
	out := make([]int, len(slots))
	for i, s := range slots {
		out[i] = int(s)
	}
	return out, nil
}

// InsertCrop plants a crop.
func (g *GameDB) InsertCrop(ctx context.Context, roomID string, slot int, seedID string, plantedAtAction, readyAtAction int) error {
	if g.pg != nil {
		return g.pg.PlantSharedCrop(ctx, pgq.PlantSharedCropParams{
			WorldID:         g.worldID,
			RoomID:          roomID,
			Slot:            int32(slot),
			SeedID:          seedID,
			PlantedAtAction: int32(plantedAtAction),
			ReadyAtAction:   int32(readyAtAction),
		})
	}
	return g.sqlite.InsertCrop(ctx, sqliteq.InsertCropParams{
		RoomID:          roomID,
		Slot:            int64(slot),
		SeedID:          seedID,
		PlantedAtAction: int64(plantedAtAction),
		ReadyAtAction:   int64(readyAtAction),
	})
}

// ─── Crafting Helpers ─────────────────────────────────────────────────────

// CountUnlockedRecipe returns whether a recipe is unlocked (via blueprint).
func (g *GameDB) CountUnlockedRecipe(ctx context.Context, recipeID string) (int64, error) {
	if g.pg != nil {
		return g.pg.IsSharedRecipeUnlocked(ctx, pgq.IsSharedRecipeUnlockedParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			RecipeID:  recipeID,
		})
	}
	return g.sqlite.CountUnlockedRecipe(ctx, recipeID)
}

// UnlockRecipe records that a recipe has been unlocked.
func (g *GameDB) UnlockRecipe(ctx context.Context, recipeID string) error {
	if g.pg != nil {
		return g.pg.UnlockSharedRecipe(ctx, pgq.UnlockSharedRecipeParams{
			AccountID:  g.pgUUID(),
			WorldID:    g.worldID,
			RecipeID:   recipeID,
			UnlockedAt: int32(time.Now().Unix()),
		})
	}
	return g.sqlite.UnlockRecipe(ctx, sqliteq.UnlockRecipeParams{
		RecipeID:   recipeID,
		UnlockedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
	})
}

// IsRecipeUnlocked reports whether a recipe is unlocked.
func (g *GameDB) IsRecipeUnlocked(ctx context.Context, recipeID string) (bool, error) {
	count, err := g.CountUnlockedRecipe(ctx, recipeID)
	return count > 0, err
}

// SetPlayerFlag sets a boolean flag.
func (g *GameDB) SetPlayerFlag(ctx context.Context, flag string) error {
	if g.pg != nil {
		return g.pg.SetSharedPlayerFlag(ctx, pgq.SetSharedPlayerFlagParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Flag:      flag,
		})
	}
	return g.sqlite.SetPlayerFlag(ctx, flag)
}

// IsPlayerFlagSet returns true if a flag is set.
func (g *GameDB) IsPlayerFlagSet(ctx context.Context, flag string) bool {
	if g.pg != nil {
		count, _ := g.pg.HasSharedPlayerFlag(ctx, pgq.HasSharedPlayerFlagParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			Flag:      flag,
		})
		return count > 0
	}
	count, _ := g.sqlite.CountPlayerFlag(ctx, flag)
	return count > 0
}

// CountChestItemsInRoom returns count of chest items in a room.
func (g *GameDB) CountChestItemsInRoom(ctx context.Context, roomID string) (int64, error) {
	if g.pg != nil {
		return g.pg.CountSharedChestItems(ctx, pgq.CountSharedChestItemsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
	}
	return g.sqlite.CountChestItemsInRoom(ctx, roomID)
}

// UpdateFactionMemberStation updates a faction member's stationed room.
func (g *GameDB) UpdateFactionMemberStation(ctx context.Context, npcID, stationedRoom string) error {
	if g.pg != nil {
		return g.pg.UpdateSharedFactionMemberStation(ctx, pgq.UpdateSharedFactionMemberStationParams{
			StationedRoom: pgText(stationedRoom),
			NpcID:         npcID,
		})
	}
	return g.sqlite.UpdateFactionMemberStation(ctx, sqliteq.UpdateFactionMemberStationParams{
		StationedRoom: sql.NullString{String: stationedRoom, Valid: stationedRoom != ""},
		NpcID:         npcID,
	})
}

// ─── Taken Room Items ──────────────────────────────────────────────────────
//
// In shared (Postgres) worlds, when a YAML-defined room item is picked up by
// any player, it is recorded in shared_taken_room_items so it disappears for
// everyone. In solo (SQLite) worlds we don't persist this — display-time
// filtering against the player's own inventory handles it.

// TakeRoomItem marks a YAML room item as taken. No-op for solo worlds.
func (g *GameDB) TakeRoomItem(ctx context.Context, roomID, itemID string) error {
	if g.pg != nil {
		return g.pg.TakeSharedRoomItem(ctx, pgq.TakeSharedRoomItemParams{
			WorldID: g.worldID,
			RoomID:  roomID,
			ItemID:  itemID,
			TakenBy: g.pgUUID(),
		})
	}
	return nil
}

// IsRoomItemTaken reports whether a YAML room item has been taken.
// For shared worlds: checks shared_taken_room_items.
// For solo worlds: checks the player's own inventory.
func (g *GameDB) IsRoomItemTaken(ctx context.Context, roomID, itemID string) bool {
	if g.pg != nil {
		_, err := g.pg.IsSharedRoomItemTaken(ctx, pgq.IsSharedRoomItemTakenParams{
			WorldID: g.worldID,
			RoomID:  roomID,
			ItemID:  itemID,
		})
		return err == nil
	}
	inv, err := g.ListInventory(ctx)
	if err != nil {
		return false
	}
	for _, it := range inv {
		if it.ID == itemID {
			return true
		}
	}
	return false
}

// ListTakenRoomItems returns the IDs of YAML-defined room items that should
// be hidden from this player in the given room. For shared worlds: every
// item recorded in shared_taken_room_items. For solo worlds: every item the
// player already carries (so the room can't show duplicates).
func (g *GameDB) ListTakenRoomItems(ctx context.Context, roomID string) []string {
	if g.pg != nil {
		ids, err := g.pg.ListSharedTakenRoomItems(ctx, pgq.ListSharedTakenRoomItemsParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		})
		if err != nil {
			return nil
		}
		return ids
	}
	inv, err := g.ListInventory(ctx)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(inv))
	for _, it := range inv {
		ids = append(ids, it.ID)
	}
	return ids
}
