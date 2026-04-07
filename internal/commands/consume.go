package commands

import (
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// consumable defines the effect of a single-use item.
type consumable struct {
	HealHP int
	Verb   string // "drink", "eat", "pop", "chew" — flavor for the output message
}

// consumables maps item ID (as stored in inventory) to its effect.
// Single-use: the item row is removed on consume. Multi-charge flavor (e.g.
// "whiskey-flask" with "3 charges" in the description) is not currently
// tracked — one use, gone.
var consumables = map[string]consumable{
	// Gunslinger
	"whiskey-flask": {HealHP: 10, Verb: "drink"},
	"trail-rations": {HealHP: 5, Verb: "eat"},
	// Medic
	"stimpak-3":  {HealHP: 25, Verb: "jab"},
	"bandages-5": {HealHP: 10, Verb: "wrap"},
	// Ghoul (also usable via rad-feed signature verb)
	"rad-chunks-5": {HealHP: 15, Verb: "chew"},
	"junk-food-5":  {HealHP: 5, Verb: "eat"},
	// Generic canon
	"canned-food": {HealHP: 5, Verb: "eat"},
	"stimpak":     {HealHP: 25, Verb: "jab"},
	"bandage":     {HealHP: 10, Verb: "wrap"},
	"buffout":     {HealHP: 20, Verb: "pop"},
	"med-x":       {HealHP: 15, Verb: "inject"},
}

// Use consumes an inventory item and applies its effect.
// Accepts the item id, name, or a unique prefix (case-insensitive).
func Use(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "use what? (use <item-id-or-name>)"}
	}
	query := strings.ToLower(strings.Join(args, " "))
	queryDash := strings.ReplaceAll(query, " ", "-")

	inv, err := player.Inventory(gdb)
	if err != nil || len(inv) == 0 {
		return Result{Output: "you're empty-handed."}
	}

	// Find the inventory row matching the query (exact id, exact name, or prefix).
	var match *player.InventoryItem
	for i := range inv {
		it := &inv[i]
		lid := strings.ToLower(it.ID)
		lname := strings.ToLower(it.Name)
		if lid == queryDash || lname == query {
			match = it
			break
		}
	}
	if match == nil {
		for i := range inv {
			it := &inv[i]
			lid := strings.ToLower(it.ID)
			lname := strings.ToLower(it.Name)
			if strings.HasPrefix(lid, queryDash) || strings.HasPrefix(lname, query) {
				match = it
				break
			}
		}
	}
	if match == nil {
		return Result{Output: fmt.Sprintf("you don't have %q.", strings.Join(args, " "))}
	}

	effect, ok := consumables[match.ID]
	if !ok {
		return Result{Output: fmt.Sprintf("you can't use %s like that.", match.Name)}
	}

	// Apply the heal.
	before := s.HP
	s.HP += effect.HealHP
	if s.HP > s.MaxHP {
		s.HP = s.MaxHP
	}
	healed := s.HP - before
	_ = player.Save(gdb, s)

	// Consume the item.
	_ = player.RemoveItem(gdb, match.ID)

	verb := effect.Verb
	if verb == "" {
		verb = "use"
	}
	return Result{Output: fmt.Sprintf("you %s the %s. +%d HP. (now %d/%d)", verb, match.Name, healed, s.HP, s.MaxHP)}
}
