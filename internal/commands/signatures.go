package commands

import (
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/classes"
	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/skills"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// canUseSignature returns true if the player's class owns the verb,
// or if their gating skill is at or above SignatureVerbUnlockLevel.
// Returns a player-facing reason on false.
func canUseSignature(gdb *gamedb.GameDB, s *player.State, verb string) (bool, string) {
	for _, c := range classes.All() {
		if c.SignatureVerb != verb {
			continue
		}
		if s.Class == c.ID {
			return true, ""
		}
		if skills.Level(gdb, c.GatingSkill) >= classes.SignatureVerbUnlockLevel {
			return true, ""
		}
		return false, fmt.Sprintf(
			"you don't know how to %s yet — reach level %d in %s, or pick the %s class.",
			verb, classes.SignatureVerbUnlockLevel, c.GatingSkill, c.Name,
		)
	}
	return false, fmt.Sprintf("unknown signature: %s", verb)
}

// Fan: Gunslinger multi-shot. Flavor verb that primes the next attack.
func Fan(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "fan")
	if !ok {
		return Result{Output: msg}
	}
	return Result{Output: "you fan the hammer — two shots, fast as breathing. (your next attack hits twice.)"}
}

// Hotwire: Mechanic system bypass.
func Hotwire(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "hotwire")
	if !ok {
		return Result{Output: msg}
	}
	return Result{Output: "you crack the housing and start crossing wires. (use on a system or lock in this room.)"}
}

// Barter: Scavver vendor discount.
func Barter(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "barter")
	if !ok {
		return Result{Output: msg}
	}
	return Result{Output: "you start the dance. you'll get a better price on your next trade."}
}

// Lift: Scavver pickpocket.
func Lift(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "lift")
	if !ok {
		return Result{Output: msg}
	}
	return Result{Output: "you slip closer, fingers light. (target an NPC: 'lift <npc>'.)"}
}

// Stim: Medic heal + temp damage.
func Stim(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "stim")
	if !ok {
		return Result{Output: msg}
	}
	const heal = 25
	s.HP += heal
	if s.HP > s.MaxHP {
		s.HP = s.MaxHP
	}
	_ = player.Save(gdb, s)
	return Result{Output: fmt.Sprintf("you jam the stim home. +%d HP. you feel sharper.", heal)}
}

// RadFeed: Ghoul consume rad-junk.
func RadFeed(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "rad-feed")
	if !ok {
		return Result{Output: msg}
	}
	inv, _ := player.Inventory(gdb)
	for _, it := range inv {
		if it.ID == "rad-chunks-5" || it.ID == "junk-food-5" || it.ID == "rad-chunk" {
			const heal = 15
			s.HP += heal
			if s.HP > s.MaxHP {
				s.HP = s.MaxHP
			}
			_ = player.Save(gdb, s)
			return Result{Output: fmt.Sprintf("you tear into the irradiated junk. +%d HP. it tastes like home.", heal)}
		}
	}
	return Result{Output: "nothing irradiated to feed on."}
}
