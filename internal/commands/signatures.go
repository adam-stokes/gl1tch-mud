package commands

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/classes"
	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/locking"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/skills"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Player flag names used by signature verbs to communicate with downstream
// systems (Attack, Trade, etc.). Each is one-shot — set by the signature verb,
// consumed by the next matching action.
const (
	flagFanPrimed    = "sig:fan_primed"    // double damage on next attack
	flagBarterPrimed = "sig:barter_primed" // double output on next trade
)

// canUseSignature returns true if the player's class owns the verb,
// or if their gating skill is at or above SignatureVerbUnlockLevel.
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

// Fan: Gunslinger multi-shot. Primes the next attack to deal double damage.
func Fan(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "fan")
	if !ok {
		return Result{Output: msg}
	}
	ctx := context.Background()
	if gdb.IsPlayerFlagSet(ctx, flagFanPrimed) {
		return Result{Output: "you're already primed. take the shot."}
	}
	_ = gdb.SetPlayerFlag(ctx, flagFanPrimed)
	return Result{Output: "you fan the hammer — two shots queued, fast as breathing. (your next attack hits twice.)"}
}

// Hotwire: Mechanic system bypass. Unlocks the first locked exit in the room
// without a skill check or key. Awards crafting XP.
func Hotwire(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "hotwire")
	if !ok {
		return Result{Output: msg}
	}
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nothing here to hotwire."}
	}
	// Find first locked lock in this room.
	for i := range room.Locks {
		l := &room.Locks[i]
		if locking.IsLocked(gdb, l.ID) {
			if err := gdb.UnlockLock(context.Background(), l.ID); err != nil {
				return Result{Output: "the wires fight you. nothing happens."}
			}
			awardRes, _ := skills.Award(gdb, "crafting", 15)
			out := fmt.Sprintf("you crack the housing and cross the wires. *click.* %s is open.", l.ID)
			if awardRes != nil && awardRes.LeveledUp {
				out += "\n" + awardRes.LevelUpMsg
			}
			return Result{Output: out}
		}
	}
	return Result{Output: "nothing locked here. save it for the next door."}
}

// Barter: Scavver vendor advantage. Primes the next trade to grant double output.
func Barter(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "barter")
	if !ok {
		return Result{Output: msg}
	}
	ctx := context.Background()
	if gdb.IsPlayerFlagSet(ctx, flagBarterPrimed) {
		return Result{Output: "you're already in mid-pitch. close the deal first."}
	}
	_ = gdb.SetPlayerFlag(ctx, flagBarterPrimed)
	return Result{Output: "you start the dance. your next trade pays double."}
}

// Lift: Scavver pickpocket. Steals one item from a target NPC's trade offers
// (no payment) and turns the NPC hostile. Awards scavenging XP on success.
func Lift(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	ok, msg := canUseSignature(gdb, s, "lift")
	if !ok {
		return Result{Output: msg}
	}
	if len(args) == 0 {
		return Result{Output: "lift who? (lift <npc-id>)"}
	}
	target := strings.ToLower(strings.Join(args, " "))
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "no one here."}
	}
	for i := range room.NPCs {
		npc := &room.NPCs[i]
		if !strings.Contains(strings.ToLower(npc.Name), target) && npc.ID != target {
			continue
		}
		if !player.NPCAlive(gdb, s.RoomID, npc.ID) {
			return Result{Output: fmt.Sprintf("%s is dead. nothing left to lift.", npc.Name)}
		}
		// Collect candidate items from the NPC's trade offers.
		var pool []world.TradeIngredient
		for _, t := range npc.Trades {
			pool = append(pool, t.Offers...)
		}
		if len(pool) == 0 {
			return Result{Output: fmt.Sprintf("%s has nothing worth lifting.", npc.Name)}
		}
		pick := pool[rand.Intn(len(pool))]
		name := pick.Name
		if name == "" {
			name = pick.ID
		}
		desc := pick.Desc
		if desc == "" {
			desc = "lifted from " + npc.Name
		}
		if err := player.AddItem(gdb, pick.ID, name, desc); err != nil {
			return Result{Output: "your fingers slip. nothing."}
		}
		awardRes, _ := skills.Award(gdb, "scavenging", 10)
		out := fmt.Sprintf("you slip closer, fingers light. you lifted: %s.", name)
		if awardRes != nil && awardRes.LeveledUp {
			out += "\n" + awardRes.LevelUpMsg
		}
		out += fmt.Sprintf("\n%s notices and takes a swing. you'd better run.", npc.Name)
		// NPC retaliates: take one swing of damage now.
		dmg := npc.Attack - s.Defense
		if dmg < 1 {
			dmg = 1
		}
		s.HP -= dmg
		_ = player.Save(gdb, s)
		out += fmt.Sprintf(" (-%d HP, %d/%d)", dmg, s.HP, s.MaxHP)
		return Result{Output: out}
	}
	return Result{Output: fmt.Sprintf("no one called %q here.", target)}
}

// Stim: Medic heal + temporary buff.
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
	return Result{Output: fmt.Sprintf("you jam the stim home. +%d HP. you feel sharper. (now %d/%d)", heal, s.HP, s.MaxHP)}
}

// RadFeed: Ghoul consume rad-junk for HP.
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
			return Result{Output: fmt.Sprintf("you tear into the irradiated junk. +%d HP. it tastes like home. (now %d/%d)", heal, s.HP, s.MaxHP)}
		}
	}
	return Result{Output: "nothing irradiated to feed on."}
}
