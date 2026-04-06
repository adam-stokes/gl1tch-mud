package player

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
)

// State holds the player's current state.
type State struct {
	PlayerID string // authenticated player ID (set by session, not DB)
	Name     string
	RoomID   string
	HP       int
	MaxHP    int
	World    string
	Defense  int
}

// Load reads the player state from the database, seeding defaults on first run.
// Uses cyberspace defaults for a fresh database.
func Load(db *sql.DB) (*State, error) {
	return LoadForWorld(db, "cyberspace", "net-0")
}

// LoadForWorld reads the player state from the database, seeding with the given
// world name and start room when no record exists yet.
func LoadForWorld(db *sql.DB, worldName, startRoom string) (*State, error) {
	q := sqliteq.New(db)
	ctx := context.Background()
	row, err := q.GetPlayer(ctx)
	if err == sql.ErrNoRows {
		s := &State{Name: "hacker", RoomID: startRoom, HP: 100, MaxHP: 100, World: worldName}
		if err := q.SeedPlayer(ctx, sqliteq.SeedPlayerParams{
			Name:   s.Name,
			RoomID: s.RoomID,
			Hp:     int64(s.HP),
			MaxHp:  int64(s.MaxHP),
			World:  s.World,
		}); err != nil {
			return nil, fmt.Errorf("player: seed: %w", err)
		}
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("player: load: %w", err)
	}
	return &State{
		Name:  row.Name,
		RoomID: row.RoomID,
		HP:    int(row.Hp),
		MaxHP: int(row.MaxHp),
		World: row.World,
	}, nil
}

// Save persists the player state to the database.
func Save(db *sql.DB, s *State) error {
	q := sqliteq.New(db)
	return q.SavePlayer(context.Background(), sqliteq.SavePlayerParams{
		RoomID: s.RoomID,
		Hp:     int64(s.HP),
		MaxHp:  int64(s.MaxHP),
		World:  s.World,
	})
}

// Inventory returns the player's current inventory items.
func Inventory(db *sql.DB) ([]InventoryItem, error) {
	q := sqliteq.New(db)
	rows, err := q.ListInventory(context.Background())
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
func AddItem(db *sql.DB, id, name, desc string) error {
	q := sqliteq.New(db)
	return q.AddItem(context.Background(), sqliteq.AddItemParams{
		ItemID:   id,
		ItemName: name,
		ItemDesc: desc,
	})
}

// InventoryItem is a carried item.
type InventoryItem struct {
	ID   string
	Name string
	Desc string
}

// NPCAlive reports whether an NPC in a room is still alive.
// Returns true if no record exists (default alive).
func NPCAlive(db *sql.DB, roomID, npcID string) bool {
	q := sqliteq.New(db)
	row, err := q.GetNPCState(context.Background(), sqliteq.GetNPCStateParams{
		RoomID: roomID,
		NpcID:  npcID,
	})
	if err == sql.ErrNoRows {
		return true
	}
	return row.Alive == 1
}

// KillNPC marks an NPC as dead.
func KillNPC(db *sql.DB, roomID, npcID string, finalHP int) error {
	q := sqliteq.New(db)
	return q.UpsertNPCDead(context.Background(), sqliteq.UpsertNPCDeadParams{
		RoomID: roomID,
		NpcID:  npcID,
		Hp:     int64(finalHP),
	})
}

// NPCCurrentHP returns an NPC's current HP (default from world if no record).
func NPCCurrentHP(db *sql.DB, roomID, npcID string, defaultHP int) int {
	q := sqliteq.New(db)
	row, err := q.GetNPCState(context.Background(), sqliteq.GetNPCStateParams{
		RoomID: roomID,
		NpcID:  npcID,
	})
	if err == sql.ErrNoRows {
		return defaultHP
	}
	return int(row.Hp)
}

// SetNPCHP updates an NPC's HP without killing it.
func SetNPCHP(db *sql.DB, roomID, npcID string, hp int) error {
	q := sqliteq.New(db)
	return q.UpsertNPCAlive(context.Background(), sqliteq.UpsertNPCAliveParams{
		RoomID: roomID,
		NpcID:  npcID,
		Hp:     int64(hp),
	})
}

// MarkVisited records that the player has visited a room.
func MarkVisited(db *sql.DB, roomID string) {
	q := sqliteq.New(db)
	q.MarkVisited(context.Background(), roomID) //nolint:errcheck
}

// HasVisited reports whether the player has previously visited a room.
func HasVisited(db *sql.DB, roomID string) bool {
	q := sqliteq.New(db)
	_, err := q.HasVisited(context.Background(), roomID)
	return err == nil
}

// RemoveItem removes an item from inventory by ID.
func RemoveItem(db *sql.DB, itemID string) error {
	q := sqliteq.New(db)
	res, err := q.RemoveItem(context.Background(), itemID)
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

// DumpToDeathPile moves all inventory items to the death_pile table for roomID.
// actionCount is the current player action counter (for expiry tracking).
func DumpToDeathPile(db *sql.DB, roomID string, actionCount int) error {
	items, err := Inventory(db)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	q := sqliteq.New(tx)
	ctx := context.Background()
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

// GetDeathPile returns death pile items for a given room.
func GetDeathPile(db *sql.DB, roomID string) ([]InventoryItem, error) {
	q := sqliteq.New(db)
	rows, err := q.GetDeathPile(context.Background(), roomID)
	if err != nil {
		return nil, err
	}
	items := make([]InventoryItem, len(rows))
	for i, r := range rows {
		items[i] = InventoryItem{ID: r.ItemID, Name: r.ItemName, Desc: r.ItemDesc}
	}
	return items, nil
}

// ClaimDeathPile moves all death pile items for roomID back to inventory and deletes them.
func ClaimDeathPile(db *sql.DB, roomID string) error {
	items, err := GetDeathPile(db, roomID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	q := sqliteq.New(tx)
	ctx := context.Background()
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

// AnyDeathPile returns the room_id and item count of the most recent death pile.
// Returns ("", 0, nil) if no pile exists.
func AnyDeathPile(db *sql.DB) (roomID string, count int, err error) {
	q := sqliteq.New(db)
	row, err := q.AnyDeathPile(context.Background())
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return row.RoomID, int(row.Count), nil
}

// MarkShardCollected marks a Crystal Shard as collected.
func MarkShardCollected(db *sql.DB, shardID string) error {
	q := sqliteq.New(db)
	ctx := context.Background()
	actionCnt, _ := q.GetActionCount(ctx) //nolint:errcheck
	var cnt int64
	if actionCnt.Valid {
		cnt = actionCnt.Int64
	}
	return q.MarkShardCollected(ctx, sqliteq.MarkShardCollectedParams{
		CollectedAt: cnt,
		ShardID:     shardID,
	})
}

// EquippedArmorRecord holds data about the currently equipped armor.
type EquippedArmorRecord struct {
	ItemID   string
	ItemName string
	Defense  int
}

// EquipArmor upserts the equipped armor record (single-row table, id always 1).
func EquipArmor(db *sql.DB, itemID, itemName string, defense int) error {
	q := sqliteq.New(db)
	return q.EquipArmor(context.Background(), sqliteq.EquipArmorParams{
		ItemID:   itemID,
		ItemName: itemName,
		Defense:  int64(defense),
	})
}

// UnequipArmor removes the equipped armor record.
func UnequipArmor(db *sql.DB) error {
	q := sqliteq.New(db)
	return q.UnequipArmor(context.Background())
}

// GetEquippedArmor returns the current equipped armor, or nil if nothing is equipped.
func GetEquippedArmor(db *sql.DB) (*EquippedArmorRecord, error) {
	q := sqliteq.New(db)
	row, err := q.GetEquippedArmor(context.Background())
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

// LoadDefense reads the equipped armor defense value into s.Defense.
// Call this after LoadForWorld to populate the defense stat.
func LoadDefense(db *sql.DB, s *State) {
	rec, err := GetEquippedArmor(db)
	if err != nil || rec == nil {
		s.Defense = 0
		return
	}
	s.Defense = rec.Defense
}
