// Package enchanting manages item enchantments and enchanting XP.
package enchanting

import (
	"database/sql"
)

// Enchant is a record of an enchantment applied to an item.
type Enchant struct {
	ItemID    string
	EnchantID string
	Level     int
}

// Apply adds an enchantment to an item (or upgrades level if already present).
func Apply(db *sql.DB, itemID, enchantID string, level, actionCount int) error {
	_, err := db.Exec(
		`INSERT INTO enchants (item_id, enchant_id, level, applied_at) VALUES (?,?,?,?)
		 ON CONFLICT(item_id, enchant_id) DO UPDATE SET level=excluded.level, applied_at=excluded.applied_at`,
		itemID, enchantID, level, actionCount,
	)
	return err
}

// List returns all enchantments on an item.
func List(db *sql.DB, itemID string) ([]Enchant, error) {
	rows, err := db.Query(`SELECT item_id, enchant_id, level FROM enchants WHERE item_id=?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Enchant
	for rows.Next() {
		var e Enchant
		if err := rows.Scan(&e.ItemID, &e.EnchantID, &e.Level); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AddXP adds enchanting experience points and recalculates level (100 XP per level, cap 30).
func AddXP(db *sql.DB, amount int) error {
	_, err := db.Exec(`UPDATE enchanting_xp SET xp = xp + ? WHERE id = 1`, amount)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		UPDATE enchanting_xp
		SET level = MIN(MAX(1, xp / 100), 30)
		WHERE id = 1
	`)
	return err
}

// XPState returns current enchanting XP and level.
func XPState(db *sql.DB) (xp, level int, err error) {
	err = db.QueryRow(`SELECT xp, level FROM enchanting_xp WHERE id=1`).Scan(&xp, &level)
	return
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
	levelNames := []string{"", "I", "II", "III"}
	lv := ""
	if level >= 1 && level <= 3 {
		lv = " " + levelNames[level]
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
