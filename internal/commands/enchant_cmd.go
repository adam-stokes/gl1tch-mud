package commands

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
	"github.com/adam-stokes/gl1tch-mud/internal/enchanting"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["enchant"] = Enchant
}

// itemCategory guesses the enchant category of an item by ID conventions.
func itemCategory(itemID string) string {
	switch {
	case strings.Contains(itemID, "sword"):
		return "sword"
	case strings.Contains(itemID, "pickaxe"):
		return "pickaxe"
	case strings.Contains(itemID, "axe"):
		return "axe"
	case strings.Contains(itemID, "boots"):
		return "boots"
	default:
		return "any"
	}
}

// Enchant enchants an item using the enchanting table.
func Enchant(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	q := sqliteq.New(db)
	ctx := context.Background()
	cnt, err := q.CountEnchantingTable(ctx, s.RoomID)
	if err != nil {
		return Result{Output: "unable to check for enchanting table."}
	}
	if cnt == 0 {
		return Result{Output: "you need an enchanting table. build one with 'build enchanting-table'."}
	}

	xp, level, _ := enchanting.XPState(db)

	if len(args) == 0 {
		return Result{Output: fmt.Sprintf(
			"enchanting level: %d (XP: %d)\nusage: enchant <item-id>", level, xp,
		)}
	}

	itemID := strings.ToLower(args[0])
	invIDs := inventoryIDs(db)
	hasItem := false
	for _, id := range invIDs {
		if id == itemID {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return Result{Output: fmt.Sprintf("you don't have %q.", itemID)}
	}

	if level < 1 {
		return Result{Output: "you need at least level 1 enchanting XP. earn XP by mining, fighting, and questing."}
	}

	category := itemCategory(itemID)
	available := enchanting.AvailableForItemType(category)
	if len(available) == 0 {
		return Result{Output: fmt.Sprintf("no enchantments available for %s.", itemID)}
	}

	enchantLevel := 1
	if level >= 20 {
		enchantLevel = 3
	} else if level >= 10 {
		enchantLevel = 2
	}

	chosenID := available[rand.Intn(len(available))]

	xpCost := enchantLevel * 10
	if xp < xpCost {
		return Result{Output: fmt.Sprintf(
			"not enough enchanting XP. need %d, have %d.", xpCost, xp,
		)}
	}

	current := actionCount(db)
	if err := enchanting.Apply(db, itemID, chosenID, enchantLevel, current); err != nil {
		return Result{Output: "enchantment failed — try again."}
	}
	if err := q.DeductEnchantingXP(ctx, int64(xpCost)); err != nil {
		return Result{Output: "enchantment applied but XP deduction failed."}
	}

	name := enchanting.EnchantName(chosenID, enchantLevel)
	return Result{Output: fmt.Sprintf(
		"the enchanting table glows...\nyour %s has been enchanted with %s! (-%d XP)",
		itemID, name, xpCost,
	)}
}
