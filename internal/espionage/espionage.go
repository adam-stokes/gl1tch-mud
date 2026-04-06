// Package espionage implements stealth, disguise, NPC dialogue, and NPC memory.
package espionage

import (
	"context"
	"math/rand"
	"strconv"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// StealthState holds the player's session-scoped stealth information.
// The values are read/written from the player_stealth table.
type StealthState struct {
	Level    int
	Disguise string
}

// LoadStealth reads stealth state from DB; defaults to level=50, disguise="none".
func LoadStealth(gdb *gamedb.GameDB) StealthState {
	s := gdb.LoadStealth(context.Background())
	return StealthState{Level: s.Level, Disguise: s.Disguise}
}

// SaveStealth persists stealth state to DB.
func SaveStealth(gdb *gamedb.GameDB, s StealthState) error {
	return gdb.SaveStealth(context.Background(), gamedb.StealthState{Level: s.Level, Disguise: s.Disguise})
}

// Hide attempts to raise stealth by a random amount between 5 and 15 (capped at 100).
func Hide(gdb *gamedb.GameDB) (StealthState, bool) {
	s := LoadStealth(gdb)
	gain := 5 + rand.Intn(11) // 5..15
	s.Level += gain
	if s.Level > 100 {
		s.Level = 100
	}
	SaveStealth(gdb, s) //nolint:errcheck
	return s, s.Level > 70
}

// Disguise applies a disguise item to the player's stealth state.
// Returns false if the item is not in inventory or not a disguise item.
func Disguise(gdb *gamedb.GameDB, w *world.World, itemID string, inventoryIDs []string) (StealthState, bool, string) {
	// Check item is in inventory
	hasItem := false
	for _, id := range inventoryIDs {
		if id == itemID {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return LoadStealth(gdb), false, "you don't have \"" + itemID + "\"."
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
	if !isDisguise {
		for _, recipe := range w.CraftingRecipes {
			if recipe.Output.ID == itemID && recipe.Output.IsDisguise {
				isDisguise = true
				break
			}
		}
	}

	if !isDisguise {
		return LoadStealth(gdb), false, "\"" + itemID + "\" is not a disguise item."
	}

	s := LoadStealth(gdb)
	s.Disguise = itemID
	SaveStealth(gdb, s) //nolint:errcheck
	return s, true, "you put on " + itemID + ". you look the part."
}

// PlayerContext holds the info needed to evaluate dialogue triggers.
type PlayerContext struct {
	InventoryIDs       []string
	Reputation         map[string]int // faction → rep value
	Skills             map[string]int // skill → level
	Disguise           string
	AllShardsCollected bool            // true when all crystal_shards rows have collected=1
	ActiveQuestIDs     map[string]bool // set of quest IDs currently active for the player
	Class              string          // mudout character class id, or "" for none
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

	case strings.HasPrefix(trigger, "quest_not_active:"):
		questID := strings.TrimPrefix(trigger, "quest_not_active:")
		return !ctx.ActiveQuestIDs[questID]

	case strings.HasPrefix(trigger, "class:"):
		want := strings.TrimPrefix(trigger, "class:")
		return ctx.Class == want

	case strings.HasPrefix(trigger, "class_not:"):
		want := strings.TrimPrefix(trigger, "class_not:")
		return ctx.Class != want
	}

	return false
}

// RecordMemory records an NPC interaction in the npc_memory table.
func RecordMemory(gdb *gamedb.GameDB, npcID, action string) error {
	return gdb.RecordNPCMemory(context.Background(), npcID, action)
}
