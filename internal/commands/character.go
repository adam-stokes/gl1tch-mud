package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/classes"
	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/skills"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Character creation state lives entirely in the DB so the wizard survives
// disconnects:
//
//	flag "char_named"        — name has been set
//	s.Class != ""            — class chosen, skill bonuses + rep applied
//	flag "kit:<itemID>"      — this kit item has been selected
//	flag "kit_done"          — kit confirmed and materialized into inventory
//	flag "tutorial_complete" — mentor handoff finished (set elsewhere)
//
// CharacterWizardActive returns true while the player has not finished creation.
// Pre-existing players (those who logged in before the wizard existed) won't
// be in wakeup-camp, so we skip the wizard for them entirely — they keep their
// progress and can earn class verbs the long way via skill levels.
func CharacterWizardActive(gdb *gamedb.GameDB, s *player.State) bool {
	if s.RoomID != "wakeup-camp" {
		return false
	}
	if s.Class == "" {
		return true
	}
	return !gdb.IsPlayerFlagSet(context.Background(), "kit_done")
}

// CharacterIntercept handles ALL input while the wizard is active.
// Returns the wizard's response and a bool indicating it consumed the command.
func CharacterIntercept(gdb *gamedb.GameDB, s *player.State, w *world.World, raw string) (Result, bool) {
	if !CharacterWizardActive(gdb, s) {
		return Result{}, false
	}
	ctx := context.Background()

	fields := strings.Fields(strings.TrimSpace(raw))
	cmd := ""
	if len(fields) > 0 {
		cmd = strings.ToLower(fields[0])
	}
	args := fields[1:]

	// Step 1: name
	if !gdb.IsPlayerFlagSet(ctx, "char_named") {
		if cmd == "name" && len(args) >= 1 {
			name := strings.Join(args, " ")
			s.Name = name
			_ = player.Save(gdb, s)
			_ = gdb.SetPlayerFlag(ctx, "char_named")
			return Result{Output: fmt.Sprintf(
				"name set: %s\n\nNext: pick a class. type 'class list' to see your options.",
				name,
			)}, true
		}
		return Result{Output: "Welcome to the wasteland.\n\nFirst: what should we call you?\nType 'name <yourname>' to continue."}, true
	}

	// Step 2: class
	if s.Class == "" {
		if cmd == "class" {
			if len(args) == 0 || args[0] == "list" {
				return Result{Output: renderClassList()}, true
			}
			if args[0] == "pick" && len(args) >= 2 {
				c := classes.ByID(strings.ToLower(args[1]))
				if c == nil {
					return Result{Output: "no such class. try 'class list'."}, true
				}
				s.Class = c.ID
				_ = player.Save(gdb, s)
				// Skill bonuses: award threshold XP so the level actually increments.
				for _, sb := range c.SkillBonuses {
					_, _ = skills.Award(gdb, sb.Skill, 50*sb.Level)
				}
				// Starting faction rep deltas.
				for _, rd := range c.StartingRep {
					_ = gdb.AdjustReputation(ctx, rd.Faction, rd.Delta)
				}
				return Result{Output: fmt.Sprintf(
					"class locked: %s.\n\nyou get %d kit points to spend.\ntype 'kit list' to see what's available.",
					c.Name, classes.KitBudget,
				)}, true
			}
		}
		return Result{Output: "Pick your class.\nType 'class list' to see your options, then 'class pick <id>' to commit."}, true
	}

	// Step 3: kit
	if !gdb.IsPlayerFlagSet(ctx, "kit_done") {
		return handleKitCommand(gdb, s, cmd, args), true
	}

	return Result{}, false
}

// renderClassList writes a player-facing summary of all classes.
func renderClassList() string {
	var b strings.Builder
	b.WriteString("CLASSES:\n\n")
	for _, c := range classes.All() {
		b.WriteString(fmt.Sprintf("  %-12s — %s\n", c.ID, c.Tagline))
		b.WriteString(fmt.Sprintf("    %s\n", c.Flavor))
		b.WriteString(fmt.Sprintf("    signature: %s | bonus: ", c.SignatureVerb))
		for i, sb := range c.SkillBonuses {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s +%d", sb.Skill, sb.Level))
		}
		b.WriteString("\n\n")
	}
	b.WriteString("type 'class pick <id>' to commit.\n")
	return b.String()
}

// handleKitCommand processes the kit subcommands during creation.
func handleKitCommand(gdb *gamedb.GameDB, s *player.State, cmd string, args []string) Result {
	ctx := context.Background()
	c := classes.ByID(s.Class)
	if c == nil {
		return Result{Output: "internal error: class missing"}
	}

	isPicked := func(itemID string) bool { return gdb.IsPlayerFlagSet(ctx, "kit:"+itemID) }
	spent := func() int {
		total := 0
		for _, k := range c.Kit {
			if isPicked(k.ID) {
				total += k.Cost
			}
		}
		return total
	}

	if cmd != "kit" {
		return Result{Output: "type 'kit list', 'kit add <id>', 'kit remove <id>', or 'kit done'."}
	}

	// matchKit returns the KitItem matching a user-typed ID or name.
	// Accepts exact id ("worn-revolver"), name with spaces ("worn revolver"),
	// or a unique prefix match. Case-insensitive.
	matchKit := func(input string) *classes.KitItem {
		norm := strings.ToLower(strings.TrimSpace(input))
		normDash := strings.ReplaceAll(norm, " ", "-")
		// Exact match first.
		for i := range c.Kit {
			k := &c.Kit[i]
			if strings.ToLower(k.ID) == normDash || strings.ToLower(k.Name) == norm {
				return k
			}
		}
		// Fall back to prefix match on id or name.
		for i := range c.Kit {
			k := &c.Kit[i]
			if strings.HasPrefix(strings.ToLower(k.ID), normDash) ||
				strings.HasPrefix(strings.ToLower(k.Name), norm) {
				return k
			}
		}
		return nil
	}

	if len(args) == 0 || args[0] == "list" {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("KIT — %d/%d points spent\n\n", spent(), classes.KitBudget))
		for _, k := range c.Kit {
			marker := "[ ]"
			if isPicked(k.ID) {
				marker = "[x]"
			}
			b.WriteString(fmt.Sprintf("  %s %dpt  %-22s  %-25s — %s\n", marker, k.Cost, k.ID, k.Name, k.Desc))
		}
		b.WriteString("\ntype 'kit add <id-or-name>', 'kit remove <id-or-name>', or 'kit done'.\n")
		return Result{Output: b.String()}
	}

	if args[0] == "add" && len(args) >= 2 {
		k := matchKit(strings.Join(args[1:], " "))
		if k == nil {
			return Result{Output: "no such kit item. type 'kit list' to see options."}
		}
		if isPicked(k.ID) {
			return Result{Output: "already picked."}
		}
		if spent()+k.Cost > classes.KitBudget {
			return Result{Output: fmt.Sprintf("not enough kit points (%d/%d).", spent(), classes.KitBudget)}
		}
		_ = gdb.SetPlayerFlag(ctx, "kit:"+k.ID)
		return Result{Output: fmt.Sprintf("added %s. %d/%d points spent.", k.Name, spent(), classes.KitBudget)}
	}

	if args[0] == "remove" && len(args) >= 2 {
		k := matchKit(strings.Join(args[1:], " "))
		if k == nil || !isPicked(k.ID) {
			return Result{Output: "not in your kit."}
		}
		_ = gdb.DeletePlayerFlag(ctx, "kit:"+k.ID)
		return Result{Output: fmt.Sprintf("removed %s. %d/%d points spent.", k.Name, spent(), classes.KitBudget)}
	}

	if args[0] == "done" {
		// Always grant the free hook items (cost 0) automatically.
		for _, k := range c.Kit {
			if k.Cost == 0 {
				_ = gdb.SetPlayerFlag(ctx, "kit:"+k.ID)
			}
		}
		// Materialize chosen kit into inventory.
		for _, k := range c.Kit {
			if isPicked(k.ID) {
				_ = player.AddItem(gdb, k.ID, k.Name, k.Desc)
			}
		}
		_ = gdb.SetPlayerFlag(ctx, "kit_done")
		return Result{Output: "kit confirmed. items added to inventory.\n\nlook around — your mentor is here. when you're ready, head south to the Gate."}
	}

	return Result{Output: "type 'kit list', 'kit add <id>', 'kit remove <id>', or 'kit done'."}
}
