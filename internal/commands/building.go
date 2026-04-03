package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["build"]   = Build
	Registry["stash"]   = Stash
	Registry["unstash"] = Unstash
}

// Build constructs a structure in the current room using world crafting recipes
// tagged with workbench type "build".
func Build(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		invIDs := inventoryIDs(db)
		invSet := make(map[string]bool, len(invIDs))
		for _, id := range invIDs {
			invSet[id] = true
		}
		var b strings.Builder
		b.WriteString("build recipes:\n")
		found := false
		for _, r := range w.CraftingRecipes {
			if r.Workbench != "build" {
				continue
			}
			found = true
			affordable := true
			for _, ing := range r.Ingredients {
				if !invSet[ing.ID] {
					affordable = false
					break
				}
			}
			suffix := ""
			if !affordable {
				suffix = " (need more materials)"
			}
			fmt.Fprintf(&b, "  %s — %s%s\n", r.ID, r.Name, suffix)
		}
		if !found {
			return Result{Output: "no build recipes available in this world."}
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	recipeID := strings.ToLower(args[0])
	var recipe *world.CraftingRecipe
	for i := range w.CraftingRecipes {
		if w.CraftingRecipes[i].ID == recipeID && w.CraftingRecipes[i].Workbench == "build" {
			recipe = &w.CraftingRecipes[i]
			break
		}
	}
	if recipe == nil {
		return Result{Output: fmt.Sprintf("no build recipe %q.", recipeID)}
	}

	invIDs := inventoryIDs(db)
	invSet := make(map[string]bool, len(invIDs))
	for _, id := range invIDs {
		invSet[id] = true
	}
	for _, ing := range recipe.Ingredients {
		if !invSet[ing.ID] {
			return Result{Output: fmt.Sprintf("you need %dx %s.", ing.Count, ing.ID)}
		}
	}

	for _, ing := range recipe.Ingredients {
		for i := 0; i < ing.Count; i++ {
			player.RemoveItem(db, ing.ID) //nolint:errcheck
		}
	}

	current := actionCount(db)
	db.Exec( //nolint:errcheck
		`INSERT INTO builds (room_id, build_id, name, desc, placed_at) VALUES (?,?,?,?,?)`,
		s.RoomID, recipe.ID, recipe.Name, recipe.Output.Desc, current,
	)
	bumpActions(db)

	unlocks := buildUnlockMessage(recipe.ID)
	return Result{Output: fmt.Sprintf("you build a %s.%s", recipe.Name, unlocks)}
}

func buildUnlockMessage(buildID string) string {
	switch buildID {
	case "workbench":
		return "\nthe workbench unlocks advanced crafting recipes."
	case "furnace":
		return "\nthe furnace is ready. use 'smelt <ore>' to process materials."
	case "enchanting-table":
		return "\nthe enchanting table glows softly. use 'enchant <item>' to enchant your gear."
	case "chest":
		return "\na chest sits in the corner. use 'stash <item>' to store items."
	case "garden-plot":
		return "\nfertile soil is ready. use 'plant <seed>' to grow crops."
	}
	return ""
}

// Stash puts an item from inventory into a chest in the current room.
func Stash(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "stash <item-id> — store an item in the room's chest"}
	}

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=? AND build_id='chest'`, s.RoomID).Scan(&cnt) //nolint:errcheck
	if cnt == 0 {
		return Result{Output: "there is no chest here. build one first."}
	}

	itemID := strings.ToLower(args[0])
	items, _ := player.Inventory(db)
	var found player.InventoryItem
	for _, it := range items {
		if it.ID == itemID {
			found = it
			break
		}
	}
	if found.ID == "" {
		return Result{Output: fmt.Sprintf("you don't have %q.", itemID)}
	}

	if err := player.RemoveItem(db, itemID); err != nil {
		return Result{Output: fmt.Sprintf("could not remove %s: %v", itemID, err)}
	}
	db.Exec( //nolint:errcheck
		`INSERT OR IGNORE INTO chests (room_id, item_id, item_name, item_desc) VALUES (?,?,?,?)`,
		s.RoomID, found.ID, found.Name, found.Desc,
	)
	return Result{Output: fmt.Sprintf("you store %s in the chest.", found.Name)}
}

// Unstash retrieves an item from the chest in the current room.
func Unstash(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=? AND build_id='chest'`, s.RoomID).Scan(&cnt) //nolint:errcheck
	if cnt == 0 {
		return Result{Output: "there is no chest here."}
	}

	if len(args) == 0 {
		rows, err := db.Query(`SELECT item_id, item_name FROM chests WHERE room_id=?`, s.RoomID)
		if err != nil || rows == nil {
			return Result{Output: "the chest is empty."}
		}
		defer rows.Close()
		var b strings.Builder
		b.WriteString("chest contents:\n")
		found := false
		for rows.Next() {
			var id, name string
			rows.Scan(&id, &name) //nolint:errcheck
			fmt.Fprintf(&b, "  %s (%s)\n", name, id)
			found = true
		}
		if !found {
			return Result{Output: "the chest is empty."}
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	itemID := strings.ToLower(args[0])
	var name, desc string
	err := db.QueryRow(`SELECT item_name, item_desc FROM chests WHERE room_id=? AND item_id=?`, s.RoomID, itemID).
		Scan(&name, &desc)
	if err != nil {
		return Result{Output: fmt.Sprintf("no %q in the chest.", itemID)}
	}

	db.Exec(`DELETE FROM chests WHERE room_id=? AND item_id=?`, s.RoomID, itemID) //nolint:errcheck
	player.AddItem(db, itemID, name, desc)                                         //nolint:errcheck
	return Result{Output: fmt.Sprintf("you take %s from the chest.", name)}
}
