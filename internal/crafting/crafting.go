// Package crafting implements the craft command and recipe processing.
package crafting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Sentinel errors for assembly crafting.
var (
	ErrMissingSlot    = errors.New("required slot not filled")
	ErrWrongComponent = errors.New("item does not fit this slot")
)

// Result is the outcome of a crafting attempt.
type Result struct {
	OK           bool
	OutputItem   world.Item
	MissingItems []string
	Message      string
	UnlocksFlag  string // non-empty if OutputItem.UnlocksFlag is set
}

// Craft attempts to craft the recipe with the given ID.
// hackingSkill is the player's current hacking skill level.
// inventoryIDs is a list of item IDs the player currently carries.
// room is the player's current room (used for workbench check).
// slots maps slotID → itemID for assembly recipes; nil for ingredient recipes.
func Craft(db *sql.DB, w *world.World, room *world.Room, recipeID string, inventoryIDs []string, hackingSkill int, slots map[string]string) Result {
	recipe := w.FindRecipe(recipeID)
	if recipe == nil {
		var names []string
		for _, r := range w.CraftingRecipes {
			names = append(names, r.ID)
		}
		if len(names) == 0 {
			return Result{Message: "no recipes known."}
		}
		return Result{
			Message: fmt.Sprintf("unknown recipe %q. known: %s", recipeID, strings.Join(names, ", ")),
		}
	}

	switch recipe.Type {
	case world.RecipeTypeAssembly:
		return craftAssemble(db, w, room, recipe, inventoryIDs, hackingSkill, slots)
	default:
		return craftIngredient(db, w, room, recipe, inventoryIDs, hackingSkill)
	}
}

// craftIngredient is the existing ingredient-list crafting path, unchanged in behaviour.
func craftIngredient(db *sql.DB, w *world.World, room *world.Room, recipe *world.CraftingRecipe, inventoryIDs []string, hackingSkill int) Result {
	q := sqliteq.New(db)
	ctx := context.Background()

	// Blueprint/unlock check
	if len(recipe.TierThresholds) > 0 {
		count, _ := q.CountUnlockedRecipe(ctx, recipe.ID)
		if count == 0 {
			return Result{Message: "You need a blueprint to craft this."}
		}
	}

	// Skill gate
	if recipe.SkillReq > 0 && hackingSkill < recipe.SkillReq {
		return Result{
			Message: fmt.Sprintf(
				"skill too low: %s requires hacking level %d (you have %d).",
				recipe.Name, recipe.SkillReq, hackingSkill,
			),
		}
	}

	// Workbench check
	if recipe.Workbench != "" && !roomHasWorkbench(room, recipe.Workbench) {
		return Result{Message: fmt.Sprintf("This recipe requires a %s.", recipe.Workbench)}
	}

	// Build inventory count map
	invCount := make(map[string]int)
	for _, id := range inventoryIDs {
		invCount[id]++
	}

	// Check ingredients
	var missing []string
	for _, ing := range recipe.Ingredients {
		if invCount[ing.ID] < ing.Count {
			missing = append(missing, fmt.Sprintf("%s x%d", ing.ID, ing.Count))
		}
	}
	if len(missing) > 0 {
		return Result{
			MissingItems: missing,
			Message:      fmt.Sprintf("missing ingredients: %s", strings.Join(missing, ", ")),
		}
	}

	// Consume ingredients
	for _, ing := range recipe.Ingredients {
		for i := 0; i < ing.Count; i++ {
			q.DeleteOneInventoryItem(ctx, ing.ID) //nolint:errcheck
		}
	}

	// Apply tier
	out := recipe.Output
	tier := ""
	if len(recipe.TierThresholds) > 0 && len(recipe.TierNames) == len(recipe.TierThresholds) {
		for i := len(recipe.TierThresholds) - 1; i >= 0; i-- {
			if hackingSkill >= recipe.TierThresholds[i] {
				tier = recipe.TierNames[i]
				break
			}
		}
		if tier != "" {
			out.Name = tier + " " + out.Name
			out.ID = out.ID + "_" + strings.ToLower(tier)
		}
	}

	q.InsertInventoryItemCraft(ctx, sqliteq.InsertInventoryItemCraftParams{ //nolint:errcheck
		ItemID:   out.ID,
		ItemName: out.Name,
		ItemDesc: out.Desc,
	})

	return Result{
		OK:          true,
		OutputItem:  out,
		UnlocksFlag: out.UnlocksFlag,
		Message:     fmt.Sprintf("you craft %s.", out.Name),
	}
}

// craftAssemble is the slot-based assembly path.
func craftAssemble(db *sql.DB, w *world.World, room *world.Room, recipe *world.CraftingRecipe, inventoryIDs []string, hackingSkill int, slots map[string]string) Result {
	q := sqliteq.New(db)
	ctx := context.Background()

	// Skill gate
	if recipe.SkillReq > 0 && hackingSkill < recipe.SkillReq {
		return Result{
			Message: fmt.Sprintf(
				"skill too low: %s requires hacking level %d (you have %d).",
				recipe.Name, recipe.SkillReq, hackingSkill,
			),
		}
	}

	// Workbench check
	if recipe.Workbench != "" && !roomHasWorkbench(room, recipe.Workbench) {
		return Result{Message: fmt.Sprintf("This recipe requires a %s.", recipe.Workbench)}
	}

	// Build inventory set for fast lookup
	invSet := make(map[string]bool)
	for _, id := range inventoryIDs {
		invSet[id] = true
	}

	// Validate all required slots are filled
	for _, slot := range recipe.Slots {
		if slot.Required {
			if _, ok := slots[slot.ID]; !ok {
				return Result{Message: fmt.Sprintf("%s: required slot '%s' not filled.", ErrMissingSlot, slot.Name)}
			}
		}
	}

	// Validate each filled slot — item must be in inventory and have the right tag
	for _, slot := range recipe.Slots {
		itemID, filled := slots[slot.ID]
		if !filled {
			continue
		}
		if !invSet[itemID] {
			return Result{Message: fmt.Sprintf("you don't have %s in your inventory.", itemID)}
		}
		item := w.FindItem(itemID)
		if item == nil {
			return Result{Message: fmt.Sprintf("unknown item: %s.", itemID)}
		}
		if !hasTag(item.Tags, slot.AcceptsTag) {
			return Result{Message: fmt.Sprintf("%s: %s doesn't fit the %s slot.", ErrWrongComponent, item.Name, slot.Name)}
		}
	}

	// Consume all slot items from inventory
	for _, itemID := range slots {
		q.DeleteOneInventoryItem(ctx, itemID) //nolint:errcheck
	}

	// Build output: start from base output, accumulate stats from slot item StatMods
	out := recipe.Output
	if out.Stats == nil {
		out.Stats = make(map[string]int)
	}
	for _, slot := range recipe.Slots {
		itemID, filled := slots[slot.ID]
		if !filled {
			continue
		}
		item := w.FindItem(itemID)
		if item == nil {
			continue
		}
		for stat, val := range item.StatMods {
			out.Stats[stat] += val
		}
	}

	q.InsertInventoryItemCraft(ctx, sqliteq.InsertInventoryItemCraftParams{ //nolint:errcheck
		ItemID:   out.ID,
		ItemName: out.Name,
		ItemDesc: out.Desc,
	})

	return Result{
		OK:          true,
		OutputItem:  out,
		UnlocksFlag: out.UnlocksFlag,
		Message:     fmt.Sprintf("you forge %s.", out.Name),
	}
}

// roomHasWorkbench returns true if the room has the given workbench type in its WorkbenchTypes list.
func roomHasWorkbench(room *world.Room, workbench string) bool {
	if room == nil {
		return false
	}
	for _, wt := range room.WorkbenchTypes {
		if wt == workbench {
			return true
		}
	}
	return false
}

// hasTag returns true if the tag slice contains the target tag.
func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

// UnlockRecipe records that the given recipe has been unlocked via a blueprint.
func UnlockRecipe(db *sql.DB, recipeID string) error {
	q := sqliteq.New(db)
	return q.UnlockRecipe(context.Background(), sqliteq.UnlockRecipeParams{
		RecipeID:   recipeID,
		UnlockedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
	})
}

// IsUnlocked reports whether the given recipe has been unlocked.
func IsUnlocked(db *sql.DB, recipeID string) (bool, error) {
	q := sqliteq.New(db)
	count, err := q.IsRecipeUnlocked(context.Background(), recipeID)
	return count > 0, err
}

// SetPlayerFlag sets a boolean flag in the player_flags table.
func SetPlayerFlag(db *sql.DB, flag string) error {
	q := sqliteq.New(db)
	return q.SetPlayerFlag(context.Background(), flag)
}

// IsPlayerFlagSet returns true if the flag exists in player_flags.
func IsPlayerFlagSet(db *sql.DB, flag string) bool {
	q := sqliteq.New(db)
	count, _ := q.CountPlayerFlag(context.Background(), flag)
	return count > 0
}
