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
