// Package events manages world events backed by the database.
package events

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// WorldEvent mirrors the world_events table.
type WorldEvent = gamedb.WorldEvent

// Active returns all events with status='active'.
func Active(gdb *gamedb.GameDB) ([]WorldEvent, error) {
	return gdb.ListActiveEvents(context.Background())
}

// Get fetches a single world event by ID.
func Get(gdb *gamedb.GameDB, id string) (*WorldEvent, error) {
	return gdb.GetEvent(context.Background(), id)
}

// Create inserts a new world event.
func Create(gdb *gamedb.GameDB, e WorldEvent) error {
	return gdb.CreateEvent(context.Background(), e)
}

// Complete sets an event status to 'completed'.
func Complete(gdb *gamedb.GameDB, id string) error {
	return gdb.CompleteEvent(context.Background(), id)
}

// ExpireOld expires events whose lifetime has elapsed. Returns count of expired events.
func ExpireOld(gdb *gamedb.GameDB, currentActions int) (int, error) {
	return gdb.ExpireOldEvents(context.Background(), currentActions)
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

	currentActions := gdb.GetActionCount(context.Background())

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
