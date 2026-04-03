package player

import (
	"database/sql"
	"fmt"
)

// State holds the player's current state.
type State struct {
	Name   string
	RoomID string
	HP     int
	MaxHP  int
	World  string
}

// Load reads the player state from the database, seeding defaults on first run.
func Load(db *sql.DB) (*State, error) {
	s := &State{}
	err := db.QueryRow(`SELECT name, room_id, hp, max_hp, world FROM player WHERE id = 1`).
		Scan(&s.Name, &s.RoomID, &s.HP, &s.MaxHP, &s.World)
	if err == sql.ErrNoRows {
		s = &State{Name: "hacker", RoomID: "net-0", HP: 100, MaxHP: 100, World: "cyberspace"}
		_, err = db.Exec(`INSERT INTO player (id, name, room_id, hp, max_hp, world) VALUES (1,?,?,?,?,?)`,
			s.Name, s.RoomID, s.HP, s.MaxHP, s.World)
		if err != nil {
			return nil, fmt.Errorf("player: seed: %w", err)
		}
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("player: load: %w", err)
	}
	return s, nil
}

// Save persists the player state to the database.
func Save(db *sql.DB, s *State) error {
	_, err := db.Exec(`UPDATE player SET room_id=?, hp=?, max_hp=?, world=? WHERE id=1`,
		s.RoomID, s.HP, s.MaxHP, s.World)
	return err
}

// Inventory returns the player's current inventory items.
func Inventory(db *sql.DB) ([]InventoryItem, error) {
	rows, err := db.Query(`SELECT item_id, item_name, item_desc FROM inventory`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []InventoryItem
	for rows.Next() {
		var it InventoryItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Desc); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// AddItem adds an item to inventory.
func AddItem(db *sql.DB, id, name, desc string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`, id, name, desc)
	return err
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
	var alive int
	err := db.QueryRow(`SELECT alive FROM npc_state WHERE room_id=? AND npc_id=?`, roomID, npcID).Scan(&alive)
	if err == sql.ErrNoRows {
		return true
	}
	return alive == 1
}

// KillNPC marks an NPC as dead.
func KillNPC(db *sql.DB, roomID, npcID string, finalHP int) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO npc_state (room_id, npc_id, hp, alive) VALUES (?,?,?,0)`,
		roomID, npcID, finalHP)
	return err
}

// NPCCurrentHP returns an NPC's current HP (default from world if no record).
func NPCCurrentHP(db *sql.DB, roomID, npcID string, defaultHP int) int {
	var hp int
	err := db.QueryRow(`SELECT hp FROM npc_state WHERE room_id=? AND npc_id=?`, roomID, npcID).Scan(&hp)
	if err == sql.ErrNoRows {
		return defaultHP
	}
	return hp
}

// SetNPCHP updates an NPC's HP without killing it.
func SetNPCHP(db *sql.DB, roomID, npcID string, hp int) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO npc_state (room_id, npc_id, hp, alive) VALUES (?,?,?,1)`,
		roomID, npcID, hp)
	return err
}

// MarkVisited records that the player has visited a room.
func MarkVisited(db *sql.DB, roomID string) {
	db.Exec(`INSERT OR IGNORE INTO visited (room_id) VALUES (?)`, roomID) //nolint:errcheck
}

// HasVisited reports whether the player has previously visited a room.
func HasVisited(db *sql.DB, roomID string) bool {
	var id string
	return db.QueryRow(`SELECT room_id FROM visited WHERE room_id=?`, roomID).Scan(&id) == nil
}

// RemoveItem removes an item from inventory by ID.
func RemoveItem(db *sql.DB, itemID string) error {
	res, err := db.Exec(`DELETE FROM inventory WHERE item_id=?`, itemID)
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
	for _, it := range items {
		if _, err := tx.Exec(
			`INSERT INTO death_pile (room_id, item_id, item_name, item_desc, died_at) VALUES (?,?,?,?,?)`,
			roomID, it.ID, it.Name, it.Desc, actionCount,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM inventory`); err != nil {
		return err
	}
	return tx.Commit()
}

// GetDeathPile returns death pile items for a given room.
func GetDeathPile(db *sql.DB, roomID string) ([]InventoryItem, error) {
	rows, err := db.Query(`SELECT item_id, item_name, item_desc FROM death_pile WHERE room_id=?`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []InventoryItem
	for rows.Next() {
		var it InventoryItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Desc); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
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
	for _, it := range items {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`,
			it.ID, it.Name, it.Desc,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM death_pile WHERE room_id=?`, roomID); err != nil {
		return err
	}
	return tx.Commit()
}

// AnyDeathPile returns the room_id and item count of the most recent death pile.
// Returns ("", 0, nil) if no pile exists.
func AnyDeathPile(db *sql.DB) (roomID string, count int, err error) {
	err = db.QueryRow(
		`SELECT room_id, COUNT(*) FROM death_pile GROUP BY room_id ORDER BY MAX(died_at) DESC, MAX(id) DESC LIMIT 1`,
	).Scan(&roomID, &count)
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	return
}

// MarkShardCollected marks a Crystal Shard as collected.
func MarkShardCollected(db *sql.DB, shardID string) error {
	var actionCnt int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&actionCnt) //nolint:errcheck
	_, err := db.Exec(
		`UPDATE crystal_shards SET collected=1, collected_at=? WHERE shard_id=?`,
		actionCnt, shardID,
	)
	return err
}
