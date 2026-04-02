// Package crafting implements the craft command and recipe processing.
package crafting

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Result is the outcome of a crafting attempt.
type Result struct {
	OK          bool
	OutputItem  world.Item
	MissingItems []string
	Message     string
}

// Craft attempts to craft the recipe with the given ID.
// hackingSkill is the player's current hacking skill level (used for skill gate and tier selection).
// inventoryIDs is a list of item IDs the player currently carries.
// room is the player's current room (used for workbench check).
func Craft(db *sql.DB, w *world.World, room *world.Room, recipeID string, inventoryIDs []string, hackingSkill int) Result {
	recipe := w.FindRecipe(recipeID)
	if recipe == nil {
		// List available recipes
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

	// Blueprint/unlock check: if recipe has TierThresholds, a blueprint must have been decoded.
	if len(recipe.TierThresholds) > 0 {
		var count int
		_ = db.QueryRow(`SELECT COUNT(*) FROM unlocked_recipes WHERE recipe_id = ?`, recipe.ID).Scan(&count)
		if count == 0 {
			return Result{Message: "You need a blueprint to craft this."}
		}
	}

	// Skill gate check
	if recipe.SkillReq > 0 && hackingSkill < recipe.SkillReq {
		return Result{
			Message: fmt.Sprintf(
				"skill too low: %s requires hacking level %d (you have %d).",
				recipe.Name, recipe.SkillReq, hackingSkill,
			),
		}
	}

	// Build inventory count map
	invCount := make(map[string]int)
	for _, id := range inventoryIDs {
		invCount[id]++
	}

	// Check all ingredients
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

	// Workbench check
	if recipe.Workbench != "" && (room == nil || room.ID != recipe.Workbench) {
		return Result{Message: fmt.Sprintf("This recipe requires a workbench in %s.", recipe.Workbench)}
	}

	// Consume ingredients
	for _, ing := range recipe.Ingredients {
		for i := 0; i < ing.Count; i++ {
			db.Exec(`DELETE FROM inventory WHERE item_id=? LIMIT 1`, ing.ID) //nolint:errcheck
		}
	}

	// Add output item, applying tier if configured
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

	db.Exec( //nolint:errcheck
		`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`,
		out.ID, out.Name, out.Desc,
	)

	return Result{
		OK:         true,
		OutputItem: out,
		Message:    fmt.Sprintf("you craft %s.", out.Name),
	}
}

// UnlockRecipe records that the given recipe has been unlocked via a blueprint.
func UnlockRecipe(db *sql.DB, recipeID string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO unlocked_recipes (recipe_id, unlocked_at) VALUES (?, ?)`,
		recipeID, time.Now().Unix())
	return err
}

// IsUnlocked reports whether the given recipe has been unlocked.
func IsUnlocked(db *sql.DB, recipeID string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM unlocked_recipes WHERE recipe_id = ?`, recipeID).Scan(&count)
	return count > 0, err
}
