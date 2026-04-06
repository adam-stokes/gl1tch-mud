// Package base manages the player's permanent base in the mudout world.
package base

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

const baseRoomID = "dusthaven-4"

// actionCount reads the player's action count from player_actions.
func actionCount(db *sql.DB) int {
	var c int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&c) //nolint:errcheck
	return c
}

// DefenseScore sums the defense stats of all structures built in dusthaven-4.
func DefenseScore(db *sql.DB, w *world.World) int {
	rows, err := db.Query(`SELECT build_id FROM builds WHERE room_id=?`, baseRoomID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	score := 0
	for rows.Next() {
		var buildID string
		rows.Scan(&buildID) //nolint:errcheck
		if r := w.FindRecipe(buildID); r != nil {
			score += r.Output.Stats["defense"]
		}
	}
	return score
}

// MaybeSpawnRaid spawns a base-raid world event if all conditions are met:
// action count is a multiple of 30, at least one structure is built in
// dusthaven-4, and no active base-raid event already exists.
func MaybeSpawnRaid(db *sql.DB) {
	current := actionCount(db)
	if current == 0 || current%30 != 0 {
		return
	}

	var structCount int
	db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=?`, baseRoomID).Scan(&structCount) //nolint:errcheck
	if structCount == 0 {
		return
	}

	var activeRaids int
	db.QueryRow(
		`SELECT COUNT(*) FROM world_events WHERE type='base-raid' AND target_room=? AND status='active'`,
		baseRoomID,
	).Scan(&activeRaids) //nolint:errcheck
	if activeRaids > 0 {
		return
	}

	id := fmt.Sprintf("base-raid-%d", time.Now().UnixNano())
	db.Exec( //nolint:errcheck
		`INSERT INTO world_events
		 (id, type, title, description, target_room, faction,
		  payout_credits, payout_item_id, payout_item_name, payout_item_desc,
		  status, expires_actions, created_actions, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, "base-raid", "Ash Raider Attack",
		"Ash Raiders are moving on your base.",
		baseRoomID, "ash-raiders",
		0, "", "", "",
		"active", 30, current, time.Now().Unix(),
	)
}

// ResolvePendingRaids checks for expired base-raid events, resolves them,
// and returns a narrative report string. Returns empty string if no raids pending.
func ResolvePendingRaids(db *sql.DB, w *world.World) string {
	current := actionCount(db)

	rows, err := db.Query(
		`SELECT id FROM world_events
		 WHERE type='base-raid' AND target_room=? AND status='active'
		 AND (created_actions + expires_actions) <= ?`,
		baseRoomID, current,
	)
	if err != nil {
		return ""
	}

	var raidIDs []string
	for rows.Next() {
		var id string
		rows.Scan(&id) //nolint:errcheck
		raidIDs = append(raidIDs, id)
	}
	rows.Close()

	if len(raidIDs) == 0 {
		return ""
	}

	defense := DefenseScore(db, w)
	var reports []string

	for _, id := range raidIDs {
		strength := rand.Intn(15) + 1 //nolint:gosec
		var report string
		if defense >= strength {
			report = fmt.Sprintf(
				"RAID REPORT: Ash Raiders hit your base while you were gone.\nRaid strength: %d  |  Your defense: %d\nYour defenses held. Nothing was taken.",
				strength, defense,
			)
		} else {
			lost := loseChestItems(db, 3)
			if len(lost) == 0 {
				report = fmt.Sprintf(
					"RAID REPORT: Ash Raiders hit your base while you were gone.\nRaid strength: %d  |  Your defense: %d\nRaiders broke through. Your storage was empty — nothing lost.",
					strength, defense,
				)
			} else {
				report = fmt.Sprintf(
					"RAID REPORT: Ash Raiders hit your base while you were gone.\nRaid strength: %d  |  Your defense: %d\nRaiders broke through. Lost: %s.",
					strength, defense, strings.Join(lost, ", "),
				)
			}
		}
		reports = append(reports, report)
		db.Exec(`UPDATE world_events SET status='resolved' WHERE id=?`, id) //nolint:errcheck
	}

	return strings.Join(reports, "\n\n")
}

// loseChestItems deletes up to max random items from the base chest and
// returns the names of lost items.
func loseChestItems(db *sql.DB, max int) []string {
	rows, err := db.Query(
		`SELECT item_id, item_name FROM chests WHERE room_id=? ORDER BY RANDOM() LIMIT ?`,
		baseRoomID, max,
	)
	if err != nil {
		return nil
	}
	var ids, names []string
	for rows.Next() {
		var id, name string
		rows.Scan(&id, &name) //nolint:errcheck
		ids = append(ids, id)
		names = append(names, name)
	}
	rows.Close()

	for _, id := range ids {
		db.Exec(`DELETE FROM chests WHERE room_id=? AND item_id=?`, baseRoomID, id) //nolint:errcheck
	}
	return names
}
