// Package enchanting manages item enchantments and enchanting XP.
package enchanting

import (
	"context"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
)

// Enchant is a record of an enchantment applied to an item.
type Enchant struct {
	ItemID    string
	EnchantID string
	Level     int
}

// Apply adds an enchantment to an item (or upgrades level if already present).
func Apply(gdb *gamedb.GameDB, itemID, enchantID string, level, appliedAt int) error {
	q := sqliteq.New(gdb.SQLiteDB())
	return q.ApplyEnchant(context.Background(), sqliteq.ApplyEnchantParams{
		ItemID:    itemID,
		EnchantID: enchantID,
		Level:     int64(level),
		AppliedAt: int64(appliedAt),
	})
}

// List returns all enchantments on an item.
func List(gdb *gamedb.GameDB, itemID string) ([]Enchant, error) {
	q := sqliteq.New(gdb.SQLiteDB())
	rows, err := q.ListEnchants(context.Background(), itemID)
	if err != nil {
		return nil, err
	}
	out := make([]Enchant, 0, len(rows))
	for _, r := range rows {
		out = append(out, Enchant{
			ItemID:    r.ItemID,
			EnchantID: r.EnchantID,
			Level:     int(r.Level),
		})
	}
	return out, nil
}

// AddXP adds enchanting experience points and recalculates level (100 XP per level, cap 30).
func AddXP(gdb *gamedb.GameDB, amount int) error {
	q := sqliteq.New(gdb.SQLiteDB())
	return q.AddEnchantingXP(context.Background(), sqliteq.AddEnchantingXPParams{
		Xp:   int64(amount),
		Xp_2: int64(amount),
	})
}

// XPState returns current enchanting XP and level.
func XPState(gdb *gamedb.GameDB) (xp, level int, err error) {
	q := sqliteq.New(gdb.SQLiteDB())
	row, err := q.GetEnchantingXPState(context.Background())
	if err != nil {
		return 0, 0, err
	}
	return int(row.Xp), int(row.Level), nil
}

// AttackBonus returns the attack bonus granted by an enchantment at a given level.
func AttackBonus(enchantID string, level int) int {
	switch enchantID {
	case "sharpness":
		return level * 5
	case "flame-touch":
		return 5
	case "frost-edge":
		return 8
	default:
		return 0
	}
}

// YieldBonus returns the extra yield count granted by an enchantment at a given level.
func YieldBonus(enchantID string, level int) int {
	switch enchantID {
	case "fortune":
		return level
	default:
		return 0
	}
}

// AvailableForItemType returns enchantment IDs applicable to a category.
// Categories: "sword", "pickaxe", "axe", "boots", "any"
func AvailableForItemType(category string) []string {
	switch category {
	case "sword":
		return []string{"sharpness", "flame-touch", "frost-edge", "diamond-luck"}
	case "pickaxe":
		return []string{"fortune", "silk-touch", "diamond-luck"}
	case "axe":
		return []string{"fortune", "sharpness", "diamond-luck"}
	case "boots":
		return []string{"swift-feet", "feather-fall", "diamond-luck"}
	default:
		return []string{"diamond-luck"}
	}
}

// EnchantName returns the display name for an enchantment ID and level.
func EnchantName(id string, level int) string {
	lv := ""
	if level >= 1 {
		lv = " " + levelRoman(level)
	}
	names := map[string]string{
		"sharpness":    "Sharpness",
		"fortune":      "Fortune",
		"swift-feet":   "Swift Feet",
		"flame-touch":  "Flame Touch",
		"silk-touch":   "Silk Touch",
		"feather-fall": "Feather Fall",
		"frost-edge":   "Frost Edge",
		"diamond-luck": "Diamond Luck",
	}
	if n, ok := names[id]; ok {
		return n + lv
	}
	return id + lv
}

// levelRoman converts a level integer to a Roman numeral string (I–XXX).
func levelRoman(n int) string {
	vals := []int{10, 9, 5, 4, 1}
	syms := []string{"X", "IX", "V", "IV", "I"}
	out := ""
	for i, v := range vals {
		for n >= v {
			out += syms[i]
			n -= v
		}
	}
	return out
}
