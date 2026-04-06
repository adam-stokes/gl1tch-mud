// Package events manages world events backed by SQLite.
package events

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// WorldEvent mirrors the world_events table.
type WorldEvent struct {
	ID              string
	Type            string
	Title           string
	Description     string
	TargetRoom      string
	Faction         string
	PayoutCredits   int
	PayoutItemID    string
	PayoutItemName  string
	PayoutItemDesc  string
	Status          string
	ExpiresActions  int
	CreatedActions  int
	CreatedAt       int64
}

// Active returns all events with status='active'.
func Active(gdb *gamedb.GameDB) ([]WorldEvent, error) {
	db := gdb.SQLiteDB()
	rows, err := db.Query(
		`SELECT id, type, title, description, target_room, faction,
		        payout_credits, payout_item_id, payout_item_name, payout_item_desc,
		        status, expires_actions, created_actions, created_at
		 FROM world_events WHERE status='active'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// Get fetches a single world event by ID.
func Get(gdb *gamedb.GameDB, id string) (*WorldEvent, error) {
	db := gdb.SQLiteDB()
	row := db.QueryRow(
		`SELECT id, type, title, description, target_room, faction,
		        payout_credits, payout_item_id, payout_item_name, payout_item_desc,
		        status, expires_actions, created_actions, created_at
		 FROM world_events WHERE id=?`, id,
	)
	var e WorldEvent
	err := scanEvent(row, &e)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Create inserts a new world event.
func Create(gdb *gamedb.GameDB, e WorldEvent) error {
	db := gdb.SQLiteDB()
	if e.CreatedAt == 0 {
		e.CreatedAt = time.Now().Unix()
	}
	_, err := db.Exec(
		`INSERT OR REPLACE INTO world_events
		 (id, type, title, description, target_room, faction,
		  payout_credits, payout_item_id, payout_item_name, payout_item_desc,
		  status, expires_actions, created_actions, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.Type, e.Title, e.Description, e.TargetRoom, e.Faction,
		e.PayoutCredits, e.PayoutItemID, e.PayoutItemName, e.PayoutItemDesc,
		"active", e.ExpiresActions, e.CreatedActions, e.CreatedAt,
	)
	return err
}

// Complete sets an event status to 'completed'.
func Complete(gdb *gamedb.GameDB, id string) error {
	db := gdb.SQLiteDB()
	_, err := db.Exec(`UPDATE world_events SET status='completed' WHERE id=?`, id)
	return err
}

// ExpireOld expires events whose lifetime has elapsed. Returns count of expired events.
func ExpireOld(gdb *gamedb.GameDB, currentActions int) (int, error) {
	db := gdb.SQLiteDB()
	res, err := db.Exec(
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

// eventTypes lists the possible event types.
var eventTypes = []string{"raid", "lockdown", "signal_surge", "black_market"}

// knownFactions lists faction IDs to randomly assign.
var knownFactions = []string{
	"ghost-protocol", "null-collective", "axiom-security",
	"phantom-collective", "signal-collective",
}

// payoutItems is a small catalog of possible payout items.
var payoutItems = []struct{ id, name, desc string }{
	{"data-chip", "Data Chip", "A fragment of stolen data."},
	{"raw-silicon", "Raw Silicon", "Unprocessed chip substrate."},
	{"decryption-key", "Decryption Key", "Cryptographic override material."},
	{"server-core", "Crystalline Server Core", "Dense storage matrix, still warm."},
	{"rf-dampener-core", "RF Dampener Core", "Absorbs EM radiation across frequencies."},
	{"copper-wire", "Copper Wire", "High-purity conductor salvage."},
}

// eventTemplates maps type → (title template, desc template).
var eventTemplates = map[string][2]string{
	"raid": {
		"Raid Alert: %s",
		"A coordinated strike force is moving through %s. High-value targets and loot on the floor. Window is short.",
	},
	"lockdown": {
		"Lockdown: %s",
		"Security protocols have sealed %s. Axiom's grid is hot. Get in, get what you need, get out before the sweep.",
	},
	"signal_surge": {
		"Signal Surge: %s",
		"An anomalous signal burst is overloading receivers in %s. Hack windows are wide open — countermeasures are blind.",
	},
	"black_market": {
		"Black Market: %s",
		"A temporary market has materialized in %s. Rare goods, no questions, gone before dawn.",
	},
}

// SeedRandom generates a random world event for a random room and inserts it.
func SeedRandom(gdb *gamedb.GameDB, w *world.World) (*WorldEvent, error) {
	if len(w.Rooms) == 0 {
		return nil, fmt.Errorf("world has no rooms")
	}

	room := w.Rooms[rand.Intn(len(w.Rooms))]
	evType := eventTypes[rand.Intn(len(eventTypes))]
	faction := knownFactions[rand.Intn(len(knownFactions))]
	payout := 500 + rand.Intn(1501) // 500-2000
	item := payoutItems[rand.Intn(len(payoutItems))]

	tmpl := eventTemplates[evType]
	title := fmt.Sprintf(tmpl[0], room.Name)
	desc := fmt.Sprintf(tmpl[1], room.Name)

	id := fmt.Sprintf("event-%s-%d", evType, time.Now().UnixNano())
	id = strings.ReplaceAll(id, "_", "-")

	currentActions := actionCount(gdb)

	e := WorldEvent{
		ID:             id,
		Type:           evType,
		Title:          title,
		Description:    desc,
		TargetRoom:     room.ID,
		Faction:        faction,
		PayoutCredits:  payout,
		PayoutItemID:   item.id,
		PayoutItemName: item.name,
		PayoutItemDesc: item.desc,
		ExpiresActions: 15,
		CreatedActions: currentActions,
		CreatedAt:      time.Now().Unix(),
	}

	if err := Create(gdb, e); err != nil {
		return nil, err
	}
	return &e, nil
}

// actionCount reads the player's action count from player_actions.
func actionCount(gdb *gamedb.GameDB) int {
	db := gdb.SQLiteDB()
	var c int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&c) //nolint:errcheck
	return c
}

func scanEvents(rows *sql.Rows) ([]WorldEvent, error) {
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner, e *WorldEvent) error {
	return row.Scan(
		&e.ID, &e.Type, &e.Title, &e.Description, &e.TargetRoom, &e.Faction,
		&e.PayoutCredits, &e.PayoutItemID, &e.PayoutItemName, &e.PayoutItemDesc,
		&e.Status, &e.ExpiresActions, &e.CreatedActions, &e.CreatedAt,
	)
}
