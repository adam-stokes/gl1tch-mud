// Package crafting implements the craft command and recipe processing.
package crafting

import (
	"database/sql"
	"fmt"
	"strings"

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
// hackingSkill is the player's current hacking skill level (used for skill gate).
// inventoryIDs is a list of item IDs the player currently carries.
func Craft(db *sql.DB, w *world.World, recipeID string, inventoryIDs []string, hackingSkill int) Result {
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

	// Consume ingredients
	for _, ing := range recipe.Ingredients {
		for i := 0; i < ing.Count; i++ {
			db.Exec(`DELETE FROM inventory WHERE item_id=? LIMIT 1`, ing.ID) //nolint:errcheck
		}
	}

	// Add output item
	out := recipe.Output
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
