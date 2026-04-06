// Package gamedb provides a unified query interface that wraps either SQLite
// (solo worlds) or Postgres (shared worlds) behind a single concrete struct.
// Command handlers use *GameDB instead of raw *sql.DB.
package gamedb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
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

// --- Escape-hatch accessors for queries not yet in the delegation layer ---

// SQLite returns the raw sqliteq.Queries. Nil for shared worlds.
func (g *GameDB) SQLite() *sqliteq.Queries { return g.sqlite }

// Postgres returns the raw pgq.Queries. Nil for solo worlds.
func (g *GameDB) Postgres() *pgq.Queries { return g.pg }

// SQLiteDB returns the underlying *sql.DB. Nil for shared worlds.
func (g *GameDB) SQLiteDB() *sql.DB { return g.sqliteDB }

// PgPool returns the underlying pgxpool.Pool. Nil for solo worlds.
func (g *GameDB) PgPool() *pgxpool.Pool { return g.pgPool }

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

// ─── Player State ──────────────────────────────────────────────────────────

// GetPlayer returns core player state. For solo: reads from the single-row
// player table. For shared: reads from shared_player_state.
func (g *GameDB) GetPlayer(ctx context.Context) (roomID string, hp, maxHP int, worldName string, err error) {
	if g.pg != nil {
		row, qerr := g.pg.GetSharedPlayerState(ctx, pgq.GetSharedPlayerStateParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		})
		if qerr != nil {
			return "", 0, 0, "", qerr
		}
		return row.RoomID, int(row.Hp), int(row.MaxHp), g.worldID, nil
	}
	row, qerr := g.sqlite.GetPlayer(ctx)
	if qerr != nil {
		return "", 0, 0, "", qerr
	}
	return row.RoomID, int(row.Hp), int(row.MaxHp), row.World, nil
}

// SavePlayer persists current player state.
func (g *GameDB) SavePlayer(ctx context.Context, roomID string, hp, maxHP int, worldName string) error {
	if g.pg != nil {
		return g.pg.UpsertSharedPlayerState(ctx, pgq.UpsertSharedPlayerStateParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			RoomID:    roomID,
			Hp:        int32(hp),
			MaxHp:     int32(maxHP),
			Credits:   0, // credits managed separately
		})
	}
	return g.sqlite.SavePlayer(ctx, sqliteq.SavePlayerParams{
		RoomID: roomID,
		Hp:     int64(hp),
		MaxHp:  int64(maxHP),
		World:  worldName,
	})
}

// SeedPlayer creates the initial player record (SQLite only — Postgres uses upsert).
func (g *GameDB) SeedPlayer(ctx context.Context, name, roomID string, hp, maxHP int, worldName string) error {
	if g.pg != nil {
		return g.pg.UpsertSharedPlayerState(ctx, pgq.UpsertSharedPlayerStateParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
			RoomID:    roomID,
			Hp:        int32(hp),
			MaxHp:     int32(maxHP),
		})
	}
	return g.sqlite.SeedPlayer(ctx, sqliteq.SeedPlayerParams{
		Name:   name,
		RoomID: roomID,
		Hp:     int64(hp),
		MaxHp:  int64(maxHP),
		World:  worldName,
	})
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
// Returns ("", 0, nil) if none. Postgres path returns ("", 0, nil) — not yet mapped.
func (g *GameDB) AnyDeathPile(ctx context.Context) (roomID string, count int, err error) {
	if g.pg != nil {
		// No AnySharedDeathPile in pgq — return empty for now.
		return "", 0, nil
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

	if g.pg != nil {
		tx, err := g.pgPool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		qtx := pgq.New(tx)
		for _, it := range items {
			if err := qtx.InsertSharedDeathPile(ctx, pgq.InsertSharedDeathPileParams{
				WorldID:  g.worldID,
				RoomID:   roomID,
				ItemID:   it.ID,
				ItemName: it.Name,
				ItemDesc: it.Desc,
			}); err != nil {
				return err
			}
		}
		if err := qtx.ClearSharedInventory(ctx, pgq.ClearSharedInventoryParams{
			AccountID: g.pgUUID(),
			WorldID:   g.worldID,
		}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// SQLite: use a transaction.
	tx, err := g.sqliteDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	q := sqliteq.New(tx)
	for _, it := range items {
		if err := q.InsertDeathPile(ctx, sqliteq.InsertDeathPileParams{
			RoomID:   roomID,
			ItemID:   it.ID,
			ItemName: it.Name,
			ItemDesc: it.Desc,
			DiedAt:   int64(actionCount),
		}); err != nil {
			return err
		}
	}
	if err := q.ClearInventory(ctx); err != nil {
		return err
	}
	return tx.Commit()
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

	if g.pg != nil {
		tx, err := g.pgPool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		qtx := pgq.New(tx)
		for _, it := range items {
			if err := qtx.AddSharedItem(ctx, pgq.AddSharedItemParams{
				AccountID: g.pgUUID(),
				WorldID:   g.worldID,
				ItemID:    it.ID,
				ItemName:  it.Name,
				ItemDesc:  it.Desc,
			}); err != nil {
				return err
			}
		}
		if err := qtx.DeleteSharedDeathPile(ctx, pgq.DeleteSharedDeathPileParams{
			WorldID: g.worldID,
			RoomID:  roomID,
		}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	tx, err := g.sqliteDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	q := sqliteq.New(tx)
	for _, it := range items {
		if err := q.AddItem(ctx, sqliteq.AddItemParams{
			ItemID:   it.ID,
			ItemName: it.Name,
			ItemDesc: it.Desc,
		}); err != nil {
			return err
		}
	}
	if err := q.DeleteDeathPile(ctx, roomID); err != nil {
		return err
	}
	return tx.Commit()
}

// ─── Crystal Shards ────────────────────────────────────────────────────────

// MarkShardCollected marks a crystal shard as collected.
func (g *GameDB) MarkShardCollected(ctx context.Context, shardID string) error {
	if g.pg != nil {
		// Shared worlds use a different shard mechanism; skip for now.
		return nil
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
