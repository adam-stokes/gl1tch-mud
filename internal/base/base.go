// Package base manages the player's permanent base in the mudout world.
package base

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

const baseRoomID = "dusthaven-4"

// actionCount reads the player's action count from player_actions.
func actionCount(gdb *gamedb.GameDB) int {
	return gdb.GetActionCount(context.Background())
}

// DefenseScore sums the defense stats of all structures built in dusthaven-4.
func DefenseScore(gdb *gamedb.GameDB, w *world.World) int {
	buildIDs, err := gdb.ListBuildIDsInRoom(context.Background(), baseRoomID)
	if err != nil {
		return 0
	}
	score := 0
	for _, buildID := range buildIDs {
		if r := w.FindRecipe(buildID); r != nil {
			score += r.Output.Stats["defense"]
		}
	}
	return score
}

// MaybeSpawnRaid spawns a base-raid world event if all conditions are met:
// action count is a multiple of 30, at least one structure is built in
// dusthaven-4, and no active base-raid event already exists.
func MaybeSpawnRaid(gdb *gamedb.GameDB) {
	current := actionCount(gdb)
	if current == 0 || current%30 != 0 {
		return
	}

	ctx := context.Background()

	structCount, _ := gdb.CountBuildsInRoom(ctx, baseRoomID)
	if structCount == 0 {
		return
	}

	activeRaids, _ := gdb.CountActiveBaseRaids(ctx, baseRoomID)
	if activeRaids > 0 {
		return
	}

	id := fmt.Sprintf("base-raid-%d", time.Now().UnixNano())
	gdb.InsertWorldEvent(ctx, gamedb.WorldEventParams{ //nolint:errcheck
		ID:             id,
		Type:           "base-raid",
		Title:          "Ash Raider Attack",
		Description:    "Ash Raiders are moving on your base.",
		TargetRoom:     baseRoomID,
		Faction:        "ash-raiders",
		PayoutCredits:  0,
		Status:         "active",
		ExpiresActions: 30,
		CreatedActions: current,
		CreatedAt:      time.Now().Unix(),
	})
}

// ResolvePendingRaids checks for expired base-raid events, resolves them,
// and returns a narrative report string. Returns empty string if no raids pending.
func ResolvePendingRaids(gdb *gamedb.GameDB, w *world.World) string {
	current := actionCount(gdb)
	ctx := context.Background()

	raidIDs, err := gdb.ListExpiredBaseRaids(ctx, baseRoomID, current)
	if err != nil || len(raidIDs) == 0 {
		return ""
	}

	defense := DefenseScore(gdb, w)
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
			lost := loseChestItems(gdb, 3)
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
		gdb.ResolveWorldEvent(ctx, id) //nolint:errcheck
	}

	return strings.Join(reports, "\n\n")
}

// loseChestItems deletes up to max random items from the base chest and
// returns the names of lost items.
func loseChestItems(gdb *gamedb.GameDB, max int) []string {
	ctx := context.Background()

	items, err := gdb.ListRandomChestItems(ctx, baseRoomID, max)
	if err != nil {
		return nil
	}

	var names []string
	for _, item := range items {
		names = append(names, item.ItemName)
		gdb.DeleteChestItem(ctx, baseRoomID, item.ItemID) //nolint:errcheck
	}
	return names
}
