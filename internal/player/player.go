package player

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

// State holds the player's current state.
type State struct {
	PlayerID string // authenticated player ID (set by session, not DB)
	Role     string // "admin" or "player"
	Name     string
	RoomID   string
	HP       int
	MaxHP    int
	World    string
	Defense  int
	Class    string
}

// Load reads the player state from the database, seeding defaults on first run.
// Uses cyberspace defaults for a fresh database.
func Load(gdb *gamedb.GameDB) (*State, error) {
	return LoadForWorld(gdb, "cyberspace", "net-0")
}

// LoadForWorld reads the player state from the database, seeding with the given
// world name and start room when no record exists yet.
func LoadForWorld(gdb *gamedb.GameDB, worldName, startRoom string) (*State, error) {
	ctx := context.Background()
	roomID, hp, maxHP, world, class, err := gdb.GetPlayer(ctx)
	if err == sql.ErrNoRows {
		s := &State{Name: "hacker", RoomID: startRoom, HP: 100, MaxHP: 100, World: worldName}
		if err := gdb.SeedPlayer(ctx, s.Name, s.RoomID, s.HP, s.MaxHP, s.World, s.Class); err != nil {
			return nil, fmt.Errorf("player: seed: %w", err)
		}
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("player: load: %w", err)
	}
	return &State{
		Name:   "hacker",
		RoomID: roomID,
		HP:     hp,
		MaxHP:  maxHP,
		World:  world,
		Class:  class,
	}, nil
}

// Save persists the player state to the database.
func Save(gdb *gamedb.GameDB, s *State) error {
	return gdb.SavePlayer(context.Background(), s.RoomID, s.HP, s.MaxHP, s.World, s.Class)
}

// InventoryItem is a carried item.
type InventoryItem struct {
	ID   string
	Name string
	Desc string
}

// Inventory returns the player's current inventory items.
func Inventory(gdb *gamedb.GameDB) ([]InventoryItem, error) {
	items, err := gdb.ListInventory(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]InventoryItem, len(items))
	for i, it := range items {
		result[i] = InventoryItem{ID: it.ID, Name: it.Name, Desc: it.Desc}
	}
	return result, nil
}

// AddItem adds an item to inventory.
func AddItem(gdb *gamedb.GameDB, id, name, desc string) error {
	return gdb.AddItem(context.Background(), id, name, desc)
}

// NPCAlive reports whether an NPC in a room is still alive.
// Returns true if no record exists (default alive).
func NPCAlive(gdb *gamedb.GameDB, roomID, npcID string) bool {
	return gdb.NPCAlive(context.Background(), roomID, npcID)
}

// KillNPC marks an NPC as dead.
func KillNPC(gdb *gamedb.GameDB, roomID, npcID string, finalHP int) error {
	return gdb.KillNPC(context.Background(), roomID, npcID, finalHP)
}

// NPCCurrentHP returns an NPC's current HP (default from world if no record).
func NPCCurrentHP(gdb *gamedb.GameDB, roomID, npcID string, defaultHP int) int {
	return gdb.NPCCurrentHP(context.Background(), roomID, npcID, defaultHP)
}

// SetNPCHP updates an NPC's HP without killing it.
func SetNPCHP(gdb *gamedb.GameDB, roomID, npcID string, hp int) error {
	return gdb.SetNPCHP(context.Background(), roomID, npcID, hp)
}

// MarkVisited records that the player has visited a room.
func MarkVisited(gdb *gamedb.GameDB, roomID string) {
	gdb.MarkVisited(context.Background(), roomID)
}

// HasVisited reports whether the player has previously visited a room.
func HasVisited(gdb *gamedb.GameDB, roomID string) bool {
	return gdb.HasVisited(context.Background(), roomID)
}

// RemoveItem removes an item from inventory by ID.
func RemoveItem(gdb *gamedb.GameDB, itemID string) error {
	return gdb.RemoveItem(context.Background(), itemID)
}

// DumpToDeathPile moves all inventory items to the death_pile table for roomID.
// actionCount is the current player action counter (for expiry tracking).
func DumpToDeathPile(gdb *gamedb.GameDB, roomID string, actionCount int) error {
	return gdb.DumpToDeathPile(context.Background(), roomID, actionCount)
}

// GetDeathPile returns death pile items for a given room.
func GetDeathPile(gdb *gamedb.GameDB, roomID string) ([]InventoryItem, error) {
	items, err := gdb.GetDeathPile(context.Background(), roomID)
	if err != nil {
		return nil, err
	}
	result := make([]InventoryItem, len(items))
	for i, it := range items {
		result[i] = InventoryItem{ID: it.ID, Name: it.Name, Desc: it.Desc}
	}
	return result, nil
}

// ClaimDeathPile moves all death pile items for roomID back to inventory and deletes them.
func ClaimDeathPile(gdb *gamedb.GameDB, roomID string) error {
	return gdb.ClaimDeathPile(context.Background(), roomID)
}

// AnyDeathPile returns the room_id and item count of the most recent death pile.
// Returns ("", 0, nil) if no pile exists.
func AnyDeathPile(gdb *gamedb.GameDB) (roomID string, count int, err error) {
	return gdb.AnyDeathPile(context.Background())
}

// MarkShardCollected marks a Crystal Shard as collected.
func MarkShardCollected(gdb *gamedb.GameDB, shardID string) error {
	return gdb.MarkShardCollected(context.Background(), shardID)
}

// EquippedArmorRecord holds data about the currently equipped armor.
type EquippedArmorRecord struct {
	ItemID   string
	ItemName string
	Defense  int
}

// EquipArmor upserts the equipped armor record (single-row table, id always 1).
func EquipArmor(gdb *gamedb.GameDB, itemID, itemName string, defense int) error {
	return gdb.EquipArmor(context.Background(), itemID, itemName, defense)
}

// UnequipArmor removes the equipped armor record.
func UnequipArmor(gdb *gamedb.GameDB) error {
	return gdb.UnequipArmor(context.Background())
}

// GetEquippedArmor returns the current equipped armor, or nil if nothing is equipped.
func GetEquippedArmor(gdb *gamedb.GameDB) (*EquippedArmorRecord, error) {
	rec, err := gdb.GetEquippedArmor(context.Background())
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	return &EquippedArmorRecord{
		ItemID:   rec.ItemID,
		ItemName: rec.ItemName,
		Defense:  rec.Defense,
	}, nil
}

// LoadDefense reads the equipped armor defense value into s.Defense.
// Call this after LoadForWorld to populate the defense stat.
func LoadDefense(gdb *gamedb.GameDB, s *State) {
	rec, err := GetEquippedArmor(gdb)
	if err != nil || rec == nil {
		s.Defense = 0
		return
	}
	s.Defense = rec.Defense
}

