// Package base manages the player's permanent base in the mudout world.
package base

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

const baseRoomID = "dusthaven-4"

// actionCount reads the player's action count from player_actions.
func actionCount(db *sql.DB) int {
	q := sqliteq.New(db)
	c, err := q.GetActionCountBase(context.Background())
	if err != nil || !c.Valid {
		return 0
	}
	return int(c.Int64)
}

// DefenseScore sums the defense stats of all structures built in dusthaven-4.
func DefenseScore(db *sql.DB, w *world.World) int {
	q := sqliteq.New(db)
	buildIDs, err := q.ListBuildIDsInRoom(context.Background(), baseRoomID)
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
func MaybeSpawnRaid(db *sql.DB) {
	current := actionCount(db)
	if current == 0 || current%30 != 0 {
		return
	}

	q := sqliteq.New(db)
	ctx := context.Background()

	structCount, _ := q.CountBuildsInRoom(ctx, baseRoomID)
	if structCount == 0 {
		return
	}

	activeRaids, _ := q.CountActiveBaseRaids(ctx, baseRoomID)
	if activeRaids > 0 {
		return
	}

	id := fmt.Sprintf("base-raid-%d", time.Now().UnixNano())
	q.InsertWorldEvent(ctx, sqliteq.InsertWorldEventParams{ //nolint:errcheck
		ID:             id,
		Type:           "base-raid",
		Title:          "Ash Raider Attack",
		Description:    sql.NullString{String: "Ash Raiders are moving on your base.", Valid: true},
		TargetRoom:     baseRoomID,
		Faction:        sql.NullString{String: "ash-raiders", Valid: true},
		PayoutCredits:  0,
		PayoutItemID:   sql.NullString{},
		PayoutItemName: sql.NullString{},
		PayoutItemDesc: sql.NullString{},
		Status:         "active",
		ExpiresActions: 30,
		CreatedActions: int64(current),
		CreatedAt:      time.Now().Unix(),
	})
}

// ResolvePendingRaids checks for expired base-raid events, resolves them,
// and returns a narrative report string. Returns empty string if no raids pending.
func ResolvePendingRaids(db *sql.DB, w *world.World) string {
	current := actionCount(db)

	q := sqliteq.New(db)
	ctx := context.Background()

	raidIDs, err := q.ListExpiredBaseRaids(ctx, sqliteq.ListExpiredBaseRaidsParams{
		TargetRoom:     baseRoomID,
		CreatedActions: int64(current),
	})
	if err != nil || len(raidIDs) == 0 {
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
		q.ResolveWorldEvent(ctx, id) //nolint:errcheck
	}

	return strings.Join(reports, "\n\n")
}

// loseChestItems deletes up to max random items from the base chest and
// returns the names of lost items.
func loseChestItems(db *sql.DB, max int) []string {
	q := sqliteq.New(db)
	ctx := context.Background()

	items, err := q.ListRandomChestItems(ctx, sqliteq.ListRandomChestItemsParams{
		RoomID: baseRoomID,
		Limit:  int64(max),
	})
	if err != nil {
		return nil
	}

	var names []string
	for _, item := range items {
		names = append(names, item.ItemName)
		q.DeleteChestItemBase(ctx, sqliteq.DeleteChestItemBaseParams{ //nolint:errcheck
			RoomID: baseRoomID,
			ItemID: item.ItemID,
		})
	}
	return names
}
