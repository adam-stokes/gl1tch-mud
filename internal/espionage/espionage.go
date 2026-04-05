// Package espionage implements stealth, disguise, NPC dialogue, and NPC memory.
package espionage

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// StealthState holds the player's session-scoped stealth information.
// The values are read/written from the player_stealth table.
type StealthState struct {
	Level   int
	Disguise string
}

// LoadStealth reads stealth state from DB; defaults to level=50, disguise="none".
func LoadStealth(db *sql.DB) StealthState {
	var s StealthState
	err := db.QueryRow(`SELECT level, disguise FROM player_stealth WHERE id=1`).
		Scan(&s.Level, &s.Disguise)
	if err != nil {
		return StealthState{Level: 50, Disguise: "none"}
	}
	return s
}

// SaveStealth persists stealth state to DB.
func SaveStealth(db *sql.DB, s StealthState) error {
	_, err := db.Exec(
		`INSERT INTO player_stealth (id, level, disguise) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET level=excluded.level, disguise=excluded.disguise`,
		s.Level, s.Disguise,
	)
	return err
}

// Hide attempts to raise stealth by a random amount between 5 and 15 (capped at 100).
func Hide(db *sql.DB) (StealthState, bool) {
	s := LoadStealth(db)
	gain := 5 + rand.Intn(11) // 5..15
	s.Level += gain
	if s.Level > 100 {
		s.Level = 100
	}
	SaveStealth(db, s) //nolint:errcheck
	return s, s.Level > 70
}

// Disguise applies a disguise item to the player's stealth state.
// Returns false if the item is not in inventory or not a disguise item.
func Disguise(db *sql.DB, w *world.World, itemID string, inventoryIDs []string) (StealthState, bool, string) {
	// Check item is in inventory
	hasItem := false
	for _, id := range inventoryIDs {
		if id == itemID {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return LoadStealth(db), false, fmt.Sprintf("you don't have %q.", itemID)
	}

	// Check item is tagged as_disguise in any room
	isDisguise := false
	for _, room := range w.Rooms {
		for _, item := range room.Items {
			if item.ID == itemID && item.IsDisguise {
				isDisguise = true
				break
			}
		}
	}
	// Also check if the item is in the world's crafting output (disguise flag)
	// For now, only check room items as per spec.
	if !isDisguise {
		// Check world items by inspecting all rooms and recipe outputs
		for _, recipe := range w.CraftingRecipes {
			if recipe.Output.ID == itemID && recipe.Output.IsDisguise {
				isDisguise = true
				break
			}
		}
	}

	if !isDisguise {
		return LoadStealth(db), false, fmt.Sprintf("%q is not a disguise item.", itemID)
	}

	s := LoadStealth(db)
	s.Disguise = itemID
	SaveStealth(db, s) //nolint:errcheck
	return s, true, fmt.Sprintf("you put on %s. you look the part.", itemID)
}

// PlayerContext holds the info needed to evaluate dialogue triggers.
type PlayerContext struct {
	InventoryIDs        []string
	Reputation          map[string]int // faction → rep value
	Skills              map[string]int // skill → level
	Disguise            string
	AllShardsCollected  bool            // true when all crystal_shards rows have collected=1
	ActiveQuestIDs      map[string]bool // set of quest IDs currently active for the player
}

// EvalDialogue evaluates NPC dialogue triggers in order and returns
// the text of the first matching trigger, or a default if none match.
func EvalDialogue(lines []world.DialogueLine, ctx PlayerContext) string {
	for _, line := range lines {
		if matchTrigger(line.Trigger, ctx) {
			return line.Text
		}
	}
	return "they don't seem interested in talking."
}

func matchTrigger(trigger string, ctx PlayerContext) bool {
	switch {
	case trigger == "always":
		return true

	case strings.HasPrefix(trigger, "has_item:"):
		itemID := strings.TrimPrefix(trigger, "has_item:")
		for _, id := range ctx.InventoryIDs {
			if id == itemID {
				return true
			}
		}
		return false

	case strings.HasPrefix(trigger, "rep_gte:"):
		// format: rep_gte:<faction>:<n>
		rest := strings.TrimPrefix(trigger, "rep_gte:")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return false
		}
		faction := parts[0]
		n, _ := strconv.Atoi(parts[1])
		return ctx.Reputation[faction] >= n

	case strings.HasPrefix(trigger, "skill_gte:"):
		// format: skill_gte:<skill>:<n>
		rest := strings.TrimPrefix(trigger, "skill_gte:")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return false
		}
		skill := parts[0]
		n, _ := strconv.Atoi(parts[1])
		return ctx.Skills[skill] >= n

	case strings.HasPrefix(trigger, "disguise:"):
		want := strings.TrimPrefix(trigger, "disguise:")
		return ctx.Disguise == want

	case trigger == "has_all_shards":
		return ctx.AllShardsCollected

	case strings.HasPrefix(trigger, "quest_active:"):
		questID := strings.TrimPrefix(trigger, "quest_active:")
		return ctx.ActiveQuestIDs[questID]
	}

	return false
}

// RecordMemory records an NPC interaction in the npc_memory table.
func RecordMemory(db *sql.DB, npcID, action string) error {
	_, err := db.Exec(
		`INSERT INTO npc_memory (npc_id, action, ts) VALUES (?,?,?)
		 ON CONFLICT(npc_id, action) DO UPDATE SET ts=excluded.ts`,
		npcID, action, time.Now().Unix(),
	)
	return err
}
