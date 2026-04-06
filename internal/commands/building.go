package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
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
func Build(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		items, _ := player.Inventory(gdb)
		invCount := make(map[string]int, len(items))
		for _, it := range items {
			invCount[it.ID]++
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
				if invCount[ing.ID] < ing.Count {
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

	// Build an item count map from inventory.
	items, _ := player.Inventory(gdb)
	invCount := make(map[string]int, len(items))
	for _, it := range items {
		invCount[it.ID]++
	}
	for _, ing := range recipe.Ingredients {
		if invCount[ing.ID] < ing.Count {
			return Result{Output: fmt.Sprintf("you need %dx %s.", ing.Count, ing.ID)}
		}
	}

	for _, ing := range recipe.Ingredients {
		for i := 0; i < ing.Count; i++ {
			player.RemoveItem(gdb, ing.ID) //nolint:errcheck
		}
	}

	current := actionCount(gdb)
	gdb.InsertBuild(context.Background(), s.RoomID, recipe.ID, recipe.Name, recipe.Output.Desc, current) //nolint:errcheck
	bumpActions(gdb)

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
func Stash(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "stash <item-id> — store an item in the room's chest"}
	}

	ctx := context.Background()
	cnt, _ := gdb.CountChestInRoom(ctx, s.RoomID)
	if cnt == 0 {
		return Result{Output: "there is no chest here. build one first."}
	}

	itemID := strings.ToLower(args[0])
	items, err := player.Inventory(gdb)
	if err != nil {
		return Result{Output: "could not read inventory."}
	}
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

	err = gdb.RunTx(ctx, func(txGDB *gamedb.GameDB) error {
		if err := txGDB.DeleteInventoryItem(ctx, found.ID); err != nil {
			return err
		}
		return txGDB.InsertChestItem(ctx, s.RoomID, found.ID, found.Name, found.Desc)
	})
	if err != nil {
		return Result{Output: "could not store item in chest."}
	}
	return Result{Output: fmt.Sprintf("you store %s in the chest.", found.Name)}
}

// Unstash retrieves an item from the chest in the current room.
func Unstash(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ctx := context.Background()
	cnt, _ := gdb.CountChestInRoom(ctx, s.RoomID)
	if cnt == 0 {
		return Result{Output: "there is no chest here."}
	}

	if len(args) == 0 {
		items, err := gdb.ListChestItems(ctx, s.RoomID)
		if err != nil || len(items) == 0 {
			return Result{Output: "the chest is empty."}
		}
		var b strings.Builder
		b.WriteString("chest contents:\n")
		for _, item := range items {
			fmt.Fprintf(&b, "  %s (%s)\n", item.ItemName, item.ItemID)
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	itemID := strings.ToLower(args[0])
	chestItem, err := gdb.GetChestItem(ctx, s.RoomID, itemID)
	if err != nil {
		return Result{Output: fmt.Sprintf("no %q in the chest.", itemID)}
	}

	err = gdb.RunTx(ctx, func(txGDB *gamedb.GameDB) error {
		if err := txGDB.DeleteChestItemByRoomAndID(ctx, s.RoomID, itemID); err != nil {
			return err
		}
		return txGDB.InsertInventoryItem(ctx, itemID, chestItem.ItemName, chestItem.ItemDesc)
	})
	if err != nil {
		return Result{Output: "could not retrieve item."}
	}
	return Result{Output: fmt.Sprintf("you take %s from the chest.", chestItem.ItemName)}
}
