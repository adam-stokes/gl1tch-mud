// Package commands implements the MUD verb dispatch table.
package commands

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/augments"
	"github.com/adam-stokes/gl1tch-mud/internal/crafting"
	"github.com/adam-stokes/gl1tch-mud/internal/credits"
	"github.com/adam-stokes/gl1tch-mud/internal/espionage"
	"github.com/adam-stokes/gl1tch-mud/internal/events"
	"github.com/adam-stokes/gl1tch-mud/internal/factions"
	"github.com/adam-stokes/gl1tch-mud/internal/generation"
	"github.com/adam-stokes/gl1tch-mud/internal/hacking"
	"github.com/adam-stokes/gl1tch-mud/internal/hideout"
	"github.com/adam-stokes/gl1tch-mud/internal/locking"
	"github.com/adam-stokes/gl1tch-mud/internal/looting"
	"github.com/adam-stokes/gl1tch-mud/internal/mods"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/quests"
	"github.com/adam-stokes/gl1tch-mud/internal/skills"
	"github.com/adam-stokes/gl1tch-mud/internal/trading"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Event is an optional BUSD event emitted after a command runs.
type Event struct {
	Topic   string
	Payload map[string]any
}

// Result is returned by every command handler.
type Result struct {
	Output      string
	Event       *Event
	SwitchWorld string // non-empty triggers a world switch in main.go
}

// HandlerFunc is a MUD command handler.
type HandlerFunc func(db *sql.DB, s *player.State, w *world.World, args []string) Result

// Registry maps verb → handler.
var Registry = map[string]HandlerFunc{
	"look":      Look,
	"l":         Look,
	"go":        Go,
	"move":      Go,
	"north":     dirHandler("north"),
	"south":     dirHandler("south"),
	"east":      dirHandler("east"),
	"west":      dirHandler("west"),
	"n":         dirHandler("north"),
	"s":         dirHandler("south"),
	"e":         dirHandler("east"),
	"w":         dirHandler("west"),
	"examine":   Examine,
	"x":         Examine,
	"take":      Take,
	"get":       Take,
	"attack":    Attack,
	"kill":      Attack,
	"inventory": Inventory,
	"i":         Inventory,
	"inv":       Inventory,
	"help":      Help,
	"?":         Help,
	// New commands
	"skills":   Skills,
	"hack":     Hack,
	"pick":     Pick,
	"unlock":   Unlock,
	"offers":   Offers,
	"trade":    Trade,
	"craft":    Craft,
	"hide":     Hide,
	"disguise": Disguise,
	"talk":     Talk,
	"explore":   Explore,
	"read":      Read,
	"search":    Search,
	"install":   Install,
	"mod":       Mod,
	"blueprint": Blueprint,
	// New systems
	"quests":  Quests,
	"quest":   Quests,
	"events":  Events,
	"faction": Faction,
	"recruit": Recruit,
	"hideout": Hideout,
	"upgrade": Upgrade,
	"credits": Credits,
}

// Parse splits raw input into verb + args. Lowercases the verb.
func Parse(input string) (verb string, args []string) {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(input)))
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

// dirHandler returns a handler for a bare direction word (north, s, etc.)
func dirHandler(dir string) HandlerFunc {
	return func(db *sql.DB, s *player.State, w *world.World, args []string) Result {
		return Go(db, s, w, []string{dir})
	}
}

// inventoryIDs returns a list of item IDs from the player's inventory.
func inventoryIDs(db *sql.DB) []string {
	items, err := player.Inventory(db)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

// Look describes the current room.
func Look(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "you are nowhere. that's unsettling."}
	}
	visited := player.HasVisited(db, s.RoomID)
	player.MarkVisited(db, s.RoomID)

	// Expire stale world events
	var actionCnt int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&actionCnt) //nolint:errcheck
	events.ExpireOld(db, actionCnt)                                             //nolint:errcheck

	// Check for low-stealth auto-detection
	detection := checkStealthDetection(db, s, room)

	output := room.Render(visited)

	// Show death pile if player died in this room.
	pile, _ := player.GetDeathPile(db, s.RoomID)
	if len(pile) > 0 {
		output += "\n[your death pile is here — use 'take death-pile' to recover your items]"
	}

	if detection != "" {
		output += "\n" + detection
	}

	// Active world event warning for this room
	activeEvs, _ := events.Active(db)
	for _, ev := range activeEvs {
		if ev.TargetRoom == room.ID {
			output += fmt.Sprintf("\n[EVENT] %s — %s", ev.Title, ev.Description)
		}
	}

	return Result{
		Output: output,
		Event: &Event{
			Topic: "mud.room.entered",
			Payload: map[string]any{
				"room_id":   room.ID,
				"room_name": room.Name,
				"first":     !visited,
			},
		},
	}
}

// checkStealthDetection checks if the player is detected by NPCs due to low stealth.
// Returns a detection message if detected, empty string otherwise.
func checkStealthDetection(db *sql.DB, s *player.State, room *world.Room) string {
	st := espionage.LoadStealth(db)
	if st.Level >= 30 {
		return ""
	}
	// Check for any alive NPCs in the room
	for _, npc := range room.NPCs {
		if !player.NPCAlive(db, room.ID, npc.ID) {
			continue
		}
		return fmt.Sprintf(
			"[ALERT] %s spots you — your stealth is too low! (%d/100)",
			npc.Name, st.Level,
		)
	}
	return ""
}

// Go moves the player in a direction.
func Go(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "go where? (north, south, east, west)"}
	}
	dir := strings.ToLower(args[0])
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "you are in the void."}
	}
	dest, ok := room.Exits[dir]
	if !ok {
		return Result{Output: fmt.Sprintf("no exit to the %s.", dir)}
	}

	// Check for lock on this exit
	lock := room.FindLock(dir)
	if lock != nil && locking.IsLocked(db, lock.ID) {
		return Result{Output: fmt.Sprintf(
			"the passage to the %s is locked. (lock: %s, difficulty: %d)",
			dir, lock.ID, lock.Difficulty,
		)}
	}

	s.RoomID = dest
	if err := player.Save(db, s); err != nil {
		return Result{Output: "system error: failed to save position."}
	}

	res := Look(db, s, w, nil)

	// Check stealth detection in new room
	newRoom := w.Room(s.RoomID)
	if newRoom != nil {
		st := espionage.LoadStealth(db)
		if st.Level < 30 {
			for _, npc := range newRoom.NPCs {
				if !player.NPCAlive(db, newRoom.ID, npc.ID) {
					continue
				}
				res.Output += fmt.Sprintf(
					"\n[STEALTH BROKEN] %s detects you! Your cover is blown.",
					npc.Name,
				)
				res.Event = &Event{
					Topic: "mud.stealth.broken",
					Payload: map[string]any{
						"room_id": newRoom.ID,
						"by_npc":  npc.ID,
					},
				}
				break
			}
		}
	}

	return res
}

// Examine describes an item or NPC in the room.
func Examine(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "examine what?"}
	}
	target := strings.Join(args, " ")
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nothing to examine here."}
	}
	for _, npc := range room.NPCs {
		if strings.Contains(strings.ToLower(npc.Name), target) {
			if !player.NPCAlive(db, s.RoomID, npc.ID) {
				return Result{Output: fmt.Sprintf("%s is dead.", npc.Name)}
			}
			hp := player.NPCCurrentHP(db, s.RoomID, npc.ID, npc.HP)
			return Result{Output: fmt.Sprintf("%s\nHP: %d", npc.Desc, hp)}
		}
	}
	for _, item := range room.Items {
		if strings.Contains(strings.ToLower(item.Name), target) {
			out := item.Desc
			if item.SignalTier != "" {
				out = "[" + strings.ToUpper(item.SignalTier) + "] " + out
			}
			return Result{Output: out}
		}
	}
	return Result{Output: fmt.Sprintf("you don't see any %s here.", target)}
}

// Take picks up an item from the room.
func Take(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "take what?"}
	}
	target := strings.Join(args, " ")
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nothing here."}
	}

	// Special: claim death pile.
	if strings.Join(args, "-") == "death-pile" {
		pile, _ := player.GetDeathPile(db, s.RoomID)
		if len(pile) == 0 {
			return Result{Output: "there is no death pile here."}
		}
		if err := player.ClaimDeathPile(db, s.RoomID); err != nil {
			return Result{Output: "failed to recover items — try again."}
		}
		names := make([]string, len(pile))
		for i, it := range pile {
			names[i] = it.Name
		}
		return Result{Output: fmt.Sprintf("you recover your items: %s.", strings.Join(names, ", "))}
	}

	for _, item := range room.Items {
		if strings.Contains(strings.ToLower(item.Name), target) {
			if err := player.AddItem(db, item.ID, item.Name, item.Desc); err != nil {
				return Result{Output: fmt.Sprintf("can't take %s — already carrying it.", item.Name)}
			}
			// Remove the item from the room's item list.
			for i, ri := range room.Items {
				if ri.ID == item.ID {
					room.Items = append(room.Items[:i], room.Items[i+1:]...)
					break
				}
			}
			out := fmt.Sprintf("you pick up %s.", item.Name)
			// Quest retrieve check
			readyQuests, _ := quests.CheckRetrieve(db, item.ID)
			for _, q := range readyQuests {
				out += fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID)
			}
			return Result{
				Output: out,
				Event: &Event{
					Topic: "mud.item.found",
					Payload: map[string]any{
						"item_id":   item.ID,
						"item_name": item.Name,
						"room_id":   s.RoomID,
					},
				},
			}
		}
	}
	return Result{Output: fmt.Sprintf("no %s here.", target)}
}

// Attack initiates combat with an NPC.
func Attack(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "attack what?"}
	}
	target := strings.Join(args, " ")
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nothing to attack."}
	}
	for _, npc := range room.NPCs {
		if !strings.Contains(strings.ToLower(npc.Name), target) {
			continue
		}
		if !player.NPCAlive(db, s.RoomID, npc.ID) {
			return Result{Output: fmt.Sprintf("%s is already dead.", npc.Name)}
		}

		// Simple combat round: player deals 10-20, NPC retaliates.
		playerDmg := 15
		npcHP := player.NPCCurrentHP(db, s.RoomID, npc.ID, npc.HP) - playerDmg
		s.HP -= npc.Attack

		var out strings.Builder
		out.WriteString(fmt.Sprintf("you hit %s for %d damage.", npc.Name, playerDmg))

		var ev *Event
		if npcHP <= 0 {
			player.KillNPC(db, s.RoomID, npc.ID, 0) //nolint:errcheck
			out.WriteString(fmt.Sprintf("\n%s is dead.", npc.Name))

			// Roll loot
			lootItems := looting.Roll(w, npc.ID, "")
			var lootNames []string
			for _, item := range lootItems {
				room.Items = append(room.Items, item)
				lootNames = append(lootNames, item.Name)
			}
			if len(lootNames) > 0 {
				out.WriteString(fmt.Sprintf("\n%s drops: %s", npc.Name, strings.Join(lootNames, ", ")))
			}

			ev = &Event{
				Topic: "mud.combat.ended",
				Payload: map[string]any{
					"npc_id":   npc.ID,
					"npc_name": npc.Name,
					"outcome":  "victory",
					"room_id":  s.RoomID,
				},
			}

			// Emit loot event if items were dropped
			if len(lootItems) > 0 {
				// We'll include loot info in the combat ended event
				ev = &Event{
					Topic: "mud.loot.dropped",
					Payload: map[string]any{
						"npc_id": npc.ID,
						"items":  lootNames,
					},
				}
			}

			// Quest kill check
			readyQuests, _ := quests.CheckKill(db, npc.ID)
			for _, q := range readyQuests {
				out.WriteString(fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID))
			}
		} else {
			player.SetNPCHP(db, s.RoomID, npc.ID, npcHP) //nolint:errcheck
			out.WriteString(fmt.Sprintf("\n%s retaliates for %d. your HP: %d/%d.", npc.Name, npc.Attack, s.HP, s.MaxHP))
			ev = &Event{
				Topic: "mud.combat.started",
				Payload: map[string]any{
					"npc_id":   npc.ID,
					"npc_name": npc.Name,
					"npc_hp":   npcHP,
					"room_id":  s.RoomID,
				},
			}
		}

		if s.HP <= 0 {
			// Get action count for death pile timestamp.
			var actionCnt int
			db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&actionCnt) //nolint:errcheck

			deathRoom := s.RoomID
			player.DumpToDeathPile(db, deathRoom, actionCnt) //nolint:errcheck

			s.HP = s.MaxHP
			s.RoomID = w.StartRoom
			player.Save(db, s) //nolint:errcheck

			deathRoomName := deathRoom
			if r := w.Room(deathRoom); r != nil {
				deathRoomName = r.Name
			}
			spawnRoomName := w.StartRoom
			if r := w.Room(w.StartRoom); r != nil {
				spawnRoomName = r.Name
			}
			out.WriteString(fmt.Sprintf(
				"\nyou were defeated! your items lie at %s.\nyou wake up at %s.",
				deathRoomName, spawnRoomName,
			))
			ev = &Event{
				Topic: "mud.player.died",
				Payload: map[string]any{
					"killer":  npc.Name,
					"room_id": room.ID,
				},
			}
		} else {
			player.Save(db, s) //nolint:errcheck
		}

		return Result{Output: out.String(), Event: ev}
	}
	return Result{Output: fmt.Sprintf("no %s here to attack.", target)}
}

// Inventory lists carried items.
func Inventory(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	items, err := player.Inventory(db)
	if err != nil || len(items) == 0 {
		return Result{Output: "you are carrying nothing."}
	}
	var b strings.Builder
	b.WriteString("carrying:\n")
	for _, it := range items {
		tier := ""
		if wi := w.FindItem(it.ID); wi != nil && wi.SignalTier != "" {
			tier = " [" + strings.ToUpper(wi.SignalTier) + "]"
		}
		b.WriteString(fmt.Sprintf("  - %s%s\n", it.Name, tier))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Help lists available commands.
func Help(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	return Result{Output: `commands:
  look / l              — describe current room
  go <dir>              — move (north/south/east/west, or just: n/s/e/w)
  examine <thing>       — examine an NPC or item
  take <item>           — pick up an item
  attack <npc>          — attack an enemy
  inventory / i         — list carried items
  skills                — show skill levels and XP
  credits               — show credit balance
  hack <system>         — attempt to hack a system in the room
  pick <lock>           — attempt to pick a lock
  unlock <lock>         — use a key to unlock a lock
  offers <npc>          — list what an NPC will trade
  trade <trade-id>      — execute a trade with an NPC in the room
  craft <recipe>        — craft an item from ingredients
  hide                  — attempt to increase stealth
  disguise <item>       — equip a disguise item
  talk <npc>            — speak to an NPC (may auto-accept quests)
  explore <dir>         — explore an unmapped direction
  read <item>           — read a readable item in the room
  search <item>         — search a container item in the room
  install <item>        — install a neural augment
  mod <item> with <mod> — apply a mod to an item
  blueprint <item>      — decode a blueprint to unlock a recipe
  quests / quest        — list active quests; 'quest complete <id>' to turn in
  events                — list active world events; 'events join <id>' for briefing
  faction               — show faction status; 'faction create <name> [agenda]'
  recruit <npc-id>      — recruit an NPC into your faction
  hideout               — teleport to your faction hideout
  upgrade list          — list hideout upgrades; 'upgrade buy <id>' to purchase
  weather               — show current weather conditions
  gather                — gather natural resources from the current room
  mine                  — mine ore or stone from the current room
  build <recipe>        — build a structure from materials
  stash <item>          — stash an item in the current room
  unstash               — retrieve a stashed item from the current room
  world list            — list available worlds
  world switch <name>   — switch to a different world
  enchant <item>        — enchant an item using a rune stone
  help / ?              — this list
  quit                  — disconnect`}
}

// Read reads a readable item in the current room.
func Read(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "read what?"}
	}
	target := strings.Join(args, " ")
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nothing to read here."}
	}
	for _, item := range room.Items {
		if strings.Contains(strings.ToLower(item.Name), target) || strings.EqualFold(item.ID, target) {
			if !item.Readable || item.Content == "" {
				return Result{Output: "Nothing readable on it."}
			}
			return Result{Output: fmt.Sprintf("--- %s ---\n%s", item.Name, item.Content)}
		}
	}
	return Result{Output: "You don't see that here."}
}

// Skills prints the player's skill levels and XP.
func Skills(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	all, err := skills.All(db)
	if err != nil || len(all) == 0 {
		return Result{Output: "no skills learned yet. use hack, pick, etc. to gain XP."}
	}
	var b strings.Builder
	b.WriteString("skills:\n")
	for skill, lv := range all {
		b.WriteString(fmt.Sprintf("  %-12s level %d  (%d XP)\n", skill, lv[0], lv[1]))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Hack attempts to compromise a hackable system in the current room using multi-phase logic.
func Hack(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "hack what? (hack <system-id>)"}
	}
	systemID := args[0]
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "no hackable systems here."}
	}

	hackSkill := skills.Level(db, "hacking")

	// Check for exploit fragment items in the room targeting this system.
	exploitBonus := 0
	for _, item := range room.Items {
		if item.IsExploit && item.TargetsSystem == systemID {
			exploitBonus += item.AugmentBonus // re-using AugmentBonus as exploit potency
		}
	}

	phases, bounty, err := hacking.HackMulti(db, room, systemID, hackSkill, exploitBonus)
	if err != nil {
		return Result{Output: fmt.Sprintf("hack failed: %v", err)}
	}

	var out strings.Builder
	var lastEv *Event

	for _, pr := range phases {
		out.WriteString(fmt.Sprintf("[%s] %s\n", pr.Phase, pr.Message))

		topic := ""
		switch pr.Phase {
		case hacking.PhaseBreach:
			topic = "mud.hack.breach"
		case hacking.PhaseExploit:
			topic = "mud.hack.exploit"
		case hacking.PhaseExfil:
			topic = "mud.hack.exfil"
		}
		if topic != "" {
			lastEv = &Event{
				Topic: topic,
				Payload: map[string]any{
					"system_id": systemID,
					"success":   pr.Success,
					"room_id":   s.RoomID,
				},
			}
		}
	}

	// Determine overall success (all phases passed).
	allSuccess := len(phases) == 3 && phases[2].Success
	if allSuccess {
		// Award XP
		awardRes, err := skills.Award(db, "hacking", 20)
		if err == nil && awardRes.LeveledUp {
			out.WriteString("\n" + awardRes.LevelUpMsg)
		}
		lastEv = &Event{
			Topic: "mud.hack.success",
			Payload: map[string]any{
				"system_id": systemID,
				"room_id":   s.RoomID,
			},
		}
		// Quest hack check
		readyQuests, _ := quests.CheckHack(db, systemID)
		for _, q := range readyQuests {
			out.WriteString(fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID))
		}
	}

	if bounty {
		out.WriteString("bounty posted — a hunter has your signature.\n")
		lastEv = &Event{
			Topic: "mud.hack.bounty",
			Payload: map[string]any{
				"system_id": systemID,
				"room_id":   s.RoomID,
			},
		}
	}

	return Result{Output: strings.TrimRight(out.String(), "\n"), Event: lastEv}
}

// Pick attempts to pick a lock in the current room.
func Pick(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "pick what? (pick <lock-id>)"}
	}
	lockID := args[0]
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nothing to pick here."}
	}

	// Find the lock in any exit
	var foundLock *world.Lock
	for i := range room.Locks {
		if room.Locks[i].ID == lockID {
			foundLock = &room.Locks[i]
			break
		}
	}
	if foundLock == nil {
		return Result{Output: fmt.Sprintf("no lock %q here.", lockID)}
	}

	if !locking.IsLocked(db, lockID) {
		return Result{Output: fmt.Sprintf("lock %q is already open.", lockID)}
	}

	pickSkill := skills.Level(db, "lockpicking")
	res := locking.Pick(db, lockID, foundLock.Difficulty, pickSkill)

	var ev *Event
	var out strings.Builder
	out.WriteString(res.Message)

	if res.Success {
		awardRes, err := skills.Award(db, "lockpicking", 15)
		if err == nil && awardRes.LeveledUp {
			out.WriteString("\n" + awardRes.LevelUpMsg)
		}
		ev = &Event{
			Topic: "mud.lock.picked",
			Payload: map[string]any{
				"lock_id":    lockID,
				"skill_used": pickSkill,
				"room_id":    s.RoomID,
			},
		}
	}

	return Result{Output: out.String(), Event: ev}
}

// Unlock uses a key item to unlock a lock without a skill roll.
func Unlock(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "unlock what? (unlock <lock-id>)"}
	}
	lockID := args[0]
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nothing to unlock here."}
	}

	var foundLock *world.Lock
	for i := range room.Locks {
		if room.Locks[i].ID == lockID {
			foundLock = &room.Locks[i]
			break
		}
	}
	if foundLock == nil {
		return Result{Output: fmt.Sprintf("no lock %q here.", lockID)}
	}

	if !locking.IsLocked(db, lockID) {
		return Result{Output: fmt.Sprintf("lock %q is already open.", lockID)}
	}

	invIDs := inventoryIDs(db)
	ok, msg := locking.UnlockWithKey(db, foundLock, invIDs)
	if !ok {
		if msg != "" {
			return Result{Output: msg}
		}
		return Result{Output: fmt.Sprintf("you don't have the right key for %q.", lockID)}
	}
	return Result{Output: msg}
}

// Offers lists what an NPC in the current room will trade.
func Offers(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "offers <npc-id> — list what an NPC will trade"}
	}
	npcID := args[0]
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nobody here."}
	}

	npc := trading.FindNPCInRoom(room, npcID)
	if npc == nil {
		return Result{Output: fmt.Sprintf("no NPC %q here.", npcID)}
	}
	if !player.NPCAlive(db, s.RoomID, npcID) {
		return Result{Output: fmt.Sprintf("%s is dead.", npc.Name)}
	}

	offers := trading.ListOffers(w, npc, db)
	if len(offers) == 0 {
		return Result{Output: fmt.Sprintf("%s has nothing to offer.", npc.Name)}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s's trades:\n", npc.Name))
	for _, offer := range offers {
		var wants, gives []string
		for _, w := range offer.Wants {
			wants = append(wants, fmt.Sprintf("%s x%d", w.ID, w.Count))
		}
		for _, o := range offer.Offers {
			name := o.Name
			if name == "" {
				name = o.ID
			}
			gives = append(gives, fmt.Sprintf("%s x%d", name, o.Count))
		}
		b.WriteString(fmt.Sprintf("  [%s] give: %s → receive: %s\n",
			offer.ID,
			strings.Join(wants, ", "),
			strings.Join(gives, ", "),
		))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Trade executes a specific trade with an NPC in the current room.
func Trade(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "trade <trade-id> — execute a trade with an NPC in the room"}
	}
	tradeID := args[0]
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nobody here to trade with."}
	}

	// Find NPC that has this trade
	var npc *world.NPC
	for i := range room.NPCs {
		for _, t := range room.NPCs[i].Trades {
			if t.ID == tradeID {
				npc = &room.NPCs[i]
				break
			}
		}
		if npc != nil {
			break
		}
	}
	if npc == nil {
		return Result{Output: fmt.Sprintf("no trade %q available in this room.", tradeID)}
	}
	if !player.NPCAlive(db, s.RoomID, npc.ID) {
		return Result{Output: fmt.Sprintf("%s is dead.", npc.Name)}
	}

	invIDs := inventoryIDs(db)
	res := trading.Execute(db, npc, tradeID, invIDs)

	var ev *Event
	if res.OK {
		var receiveItems []string
		for _, item := range res.GotItems {
			name := item.Name
			if name == "" {
				name = item.ID
			}
			receiveItems = append(receiveItems, name)
		}
		offerItem := ""
		if len(res.GaveItems) > 0 {
			offerItem = res.GaveItems[0]
		}
		receiveItem := ""
		if len(receiveItems) > 0 {
			receiveItem = receiveItems[0]
		}
		ev = &Event{
			Topic: "mud.trade.completed",
			Payload: map[string]any{
				"npc_id":       npc.ID,
				"npc_name":     npc.Name,
				"trade_id":     tradeID,
				"offer_item":   offerItem,
				"receive_item": receiveItem,
				"gave":         res.GaveItems,
				"received":     receiveItems,
				"room_id":      s.RoomID,
			},
		}
	}

	return Result{Output: res.Message, Event: ev}
}

// Craft attempts to craft an item using a recipe.
func Craft(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "craft <recipe-id> — craft an item from ingredients"}
	}
	recipeID := args[0]
	hackSkill := skills.Level(db, "hacking")
	invIDs := inventoryIDs(db)
	room := w.Room(s.RoomID)

	res := crafting.Craft(db, w, room, recipeID, invIDs, hackSkill)

	var ev *Event
	if res.OK {
		ev = &Event{
			Topic: "mud.craft.completed",
			Payload: map[string]any{
				"recipe_id":   recipeID,
				"output_item": res.OutputItem.ID,
			},
		}
	} else if len(res.MissingItems) > 0 {
		ev = &Event{
			Topic: "mud.craft.failed",
			Payload: map[string]any{
				"recipe_id": recipeID,
				"missing":   res.MissingItems,
			},
		}
	}

	return Result{Output: res.Message, Event: ev}
}

// Hide attempts to increase player stealth.
func Hide(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	st, aboveThreshold := espionage.Hide(db)
	msg := fmt.Sprintf("you melt into the shadows. stealth: %d/100.", st.Level)
	if aboveThreshold {
		msg += " you are practically invisible."
	}
	return Result{Output: msg}
}

// Disguise equips a disguise item from the player's inventory.
func Disguise(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "disguise <item-id> — wear a disguise item"}
	}
	itemID := args[0]
	invIDs := inventoryIDs(db)
	_, ok, msg := espionage.Disguise(db, w, itemID, invIDs)
	_ = ok
	return Result{Output: msg}
}

// Talk initiates dialogue with an NPC.
func Talk(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "talk <npc-id>"}
	}
	npcID := args[0]
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nobody here."}
	}

	var npc *world.NPC
	for i := range room.NPCs {
		if room.NPCs[i].ID == npcID {
			npc = &room.NPCs[i]
			break
		}
	}
	if npc == nil {
		// Try partial name match
		for i := range room.NPCs {
			if strings.Contains(strings.ToLower(room.NPCs[i].Name), npcID) {
				npc = &room.NPCs[i]
				break
			}
		}
	}
	if npc == nil {
		return Result{Output: fmt.Sprintf("no NPC %q here.", npcID)}
	}
	if !player.NPCAlive(db, s.RoomID, npc.ID) {
		return Result{Output: fmt.Sprintf("%s is dead. talking to corpses is inefficient.", npc.Name)}
	}

	if len(npc.Dialogue) == 0 {
		return Result{Output: fmt.Sprintf("%s doesn't seem interested in talking.", npc.Name)}
	}

	// Build player context
	invIDs := inventoryIDs(db)
	st := espionage.LoadStealth(db)
	rep := buildReputationMap(db)
	sk := buildSkillMap(db)

	var shardCount, totalShards int
	db.QueryRow(`SELECT COUNT(*) FROM crystal_shards WHERE collected=1`).Scan(&shardCount)   //nolint:errcheck
	db.QueryRow(`SELECT COUNT(*) FROM crystal_shards`).Scan(&totalShards)                    //nolint:errcheck

	ctx := espionage.PlayerContext{
		InventoryIDs:       invIDs,
		Reputation:         rep,
		Skills:             sk,
		Disguise:           st.Disguise,
		AllShardsCollected: totalShards >= 5 && shardCount >= 5,
	}

	text := espionage.EvalDialogue(npc.Dialogue, ctx)
	espionage.RecordMemory(db, npc.ID, "talked") //nolint:errcheck

	// Find matched line for event and quest auto-accept
	matchedTrigger := "none"
	matchedQuestID := ""
	for _, line := range npc.Dialogue {
		if line.Text == text {
			matchedTrigger = line.Trigger
			matchedQuestID = line.QuestID
			break
		}
	}

	output := fmt.Sprintf("%s: \"%s\"", npc.Name, text)

	// Auto-accept quest if the dialogue line has a quest_id
	if matchedQuestID != "" {
		wq := w.FindQuest(matchedQuestID)
		if wq != nil {
			q := quests.Quest{
				ID:             wq.ID,
				Title:          wq.Title,
				Description:    wq.Description,
				ObjType:        wq.ObjType,
				ObjTarget:      wq.ObjTarget,
				ObjRoom:        wq.ObjRoom,
				ObjCount:       wq.ObjCount,
				RewardCredits:  wq.RewardCredits,
				RewardXPSkill:  wq.RewardXPSkill,
				RewardXPAmount: wq.RewardXPAmount,
				RewardItemID:   wq.RewardItemID,
				RewardItemName: wq.RewardItemName,
				RewardItemDesc: wq.RewardItemDesc,
				GiverNPCID:     npc.ID,
			}
			if err := quests.Accept(db, q); err == nil {
				output += fmt.Sprintf("\n[QUEST ACCEPTED] %s", wq.Title)
				if wq.Description != "" {
					output += "\n" + wq.Description
				}
			}
		}
	}

	stealthState := espionage.LoadStealth(db)

	return Result{
		Output: output,
		Event: &Event{
			Topic: "mud.espionage.talked",
			Payload: map[string]any{
				"npc_id":        npc.ID,
				"npc_name":      npc.Name,
				"trigger":       matchedTrigger,
				"text":          text,
				"stealth_level": stealthState.Level,
			},
		},
	}
}

// Explore explores an unmapped direction, optionally generating a new room via Ollama.
func Explore(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "explore <direction>"}
	}
	dir := strings.ToLower(args[0])
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "you are in the void."}
	}

	// If exit exists, just go there
	if _, ok := room.Exits[dir]; ok {
		return Go(db, s, w, []string{dir})
	}

	// Attempt generation
	gen := generation.New(db)
	res := gen.Generate(context.Background(), w, room, dir)

	if res.Error != nil {
		return Result{Output: res.ErrMsg}
	}
	if res.Room == nil {
		return Result{Output: "static beyond the edge — nothing there."}
	}

	// Move player into generated room
	s.RoomID = res.Room.ID
	player.Save(db, s) //nolint:errcheck

	fromCache := ""
	if res.FromCache {
		fromCache = " [from memory]"
	}

	var ev *Event
	if !res.FromCache {
		ev = &Event{
			Topic: "mud.world.generated",
			Payload: map[string]any{
				"room_id":   res.Room.ID,
				"direction": dir,
				"model":     res.Model,
			},
		}
	}

	return Result{
		Output: res.Room.Render(false) + fromCache,
		Event:  ev,
	}
}

// Search searches a container item in the current room.
func Search(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "You are nowhere."}
	}
	if len(args) == 0 {
		return Result{Output: "search what?"}
	}
	target := strings.Join(args, " ")
	for _, item := range room.Items {
		if strings.EqualFold(item.Name, target) || item.ID == target {
			if item.IsContainer {
				if item.Capacity > 0 {
					return Result{Output: fmt.Sprintf("You search the %s. It could hold up to %d items — currently empty.", item.Name, item.Capacity)}
				}
				return Result{Output: fmt.Sprintf("You search the %s. Nothing inside.", item.Name)}
			}
			return Result{Output: fmt.Sprintf("The %s doesn't have anything hidden in it.", item.Name)}
		}
	}
	return Result{Output: "You don't see that here."}
}

// Install installs a neural augment from the current room.
func Install(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "You are nowhere."}
	}
	if len(args) == 0 {
		return Result{Output: "install what?"}
	}
	target := strings.Join(args, " ")
	for i, item := range room.Items {
		if strings.EqualFold(item.Name, target) || item.ID == target {
			if !item.IsAugment {
				return Result{Output: "That's not an installable augment."}
			}
			if err := augments.Install(db, item.AugmentSkill, item.AugmentBonus); err != nil {
				if err.Error() == "max augments reached" {
					return Result{Output: "Neural capacity full. You cannot install more augments."}
				}
				return Result{Output: fmt.Sprintf("Installation failed: %v", err)}
			}
			room.Items = append(room.Items[:i], room.Items[i+1:]...)
			return Result{Output: fmt.Sprintf("Neural interface accepted. %s +%d.", item.AugmentSkill, item.AugmentBonus)}
		}
	}
	return Result{Output: "You don't see that here."}
}

// Mod applies a mod item to a target item in the current room.
func Mod(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "You are nowhere."}
	}
	combined := strings.Join(args, " ")
	parts := strings.SplitN(combined, " with ", 2)
	if len(parts) != 2 {
		return Result{Output: "Usage: mod <item> with <mod-item>"}
	}
	targetName := strings.TrimSpace(parts[0])
	modName := strings.TrimSpace(parts[1])

	var targetItem *world.Item
	var modItem *world.Item
	var modIdx int
	for i := range room.Items {
		if strings.EqualFold(room.Items[i].Name, targetName) || room.Items[i].ID == targetName {
			targetItem = &room.Items[i]
		}
		if strings.EqualFold(room.Items[i].Name, modName) || room.Items[i].ID == modName {
			modItem = &room.Items[i]
			modIdx = i
		}
	}
	if targetItem == nil {
		return Result{Output: fmt.Sprintf("You don't see %q here.", targetName)}
	}
	if modItem == nil {
		return Result{Output: fmt.Sprintf("You don't see %q here.", modName)}
	}
	if targetItem.ModSlots <= 0 {
		return Result{Output: fmt.Sprintf("The %s has no mod slots.", targetItem.Name)}
	}
	if !modItem.IsMod {
		return Result{Output: fmt.Sprintf("The %s is not a mod.", modItem.Name)}
	}
	if err := mods.Apply(db, targetItem.ID, modItem.ID); err != nil {
		return Result{Output: fmt.Sprintf("Mod failed: %v", err)}
	}
	room.Items = append(room.Items[:modIdx], room.Items[modIdx+1:]...)
	return Result{Output: fmt.Sprintf("Mod installed on %s. Slot used.", targetItem.Name)}
}

// Blueprint decodes a blueprint item to unlock a crafting recipe.
func Blueprint(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "You are nowhere."}
	}
	if len(args) == 0 {
		return Result{Output: "blueprint what?"}
	}
	target := strings.Join(args, " ")
	for i, item := range room.Items {
		if strings.EqualFold(item.Name, target) || item.ID == target {
			if !item.IsBlueprint || item.UnlocksRecipe == "" {
				return Result{Output: "That's not a readable blueprint."}
			}
			if err := crafting.UnlockRecipe(db, item.UnlocksRecipe); err != nil {
				return Result{Output: fmt.Sprintf("Blueprint decode failed: %v", err)}
			}
			room.Items = append(room.Items[:i], room.Items[i+1:]...)
			return Result{Output: fmt.Sprintf("Blueprint decoded. Recipe unlocked: %s.", item.UnlocksRecipe)}
		}
	}
	return Result{Output: "You don't see that here."}
}

// buildReputationMap returns a map of all faction reputations.
func buildReputationMap(db *sql.DB) map[string]int {
	rows, err := db.Query(`SELECT faction, value FROM player_reputation`)
	if err != nil {
		return map[string]int{}
	}
	defer rows.Close()
	rep := make(map[string]int)
	for rows.Next() {
		var faction string
		var value int
		if rows.Scan(&faction, &value) == nil {
			rep[faction] = value
		}
	}
	return rep
}

// buildSkillMap returns a map of all skill levels.
func buildSkillMap(db *sql.DB) map[string]int {
	all, err := skills.All(db)
	if err != nil {
		return map[string]int{}
	}
	m := make(map[string]int, len(all))
	for skill, lv := range all {
		m[skill] = lv[0]
	}
	return m
}

// actionCount returns the player's current action count.
func actionCount(db *sql.DB) int {
	var c int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&c) //nolint:errcheck
	return c
}

// Credits shows the player's current credit balance.
func Credits(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	bal := credits.Get(db)
	return Result{Output: fmt.Sprintf("credits: %d ¢", bal)}
}

// Quests lists active quests or completes one.
func Quests(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) >= 2 && args[0] == "complete" {
		return questComplete(db, args[1])
	}

	active, err := quests.Active(db)
	if err != nil {
		return Result{Output: "error loading quests."}
	}
	if len(active) == 0 {
		return Result{Output: "no active quests. talk to NPCs to find work."}
	}

	var b strings.Builder
	b.WriteString("active quests:\n")
	for _, q := range active {
		bar := fmt.Sprintf("[%d/%d]", q.ObjProgress, q.ObjCount)
		room := ""
		if q.ObjRoom != "" {
			room = fmt.Sprintf(" in %s", q.ObjRoom)
		}
		b.WriteString(fmt.Sprintf("  (%s) %s %s %s%s\n", q.ID, bar, q.ObjType, q.ObjTarget, room))
		b.WriteString(fmt.Sprintf("      %s\n", q.Title))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

func questComplete(db *sql.DB, id string) Result {
	q, err := quests.Get(db, id)
	if err != nil {
		return Result{Output: fmt.Sprintf("no quest %q found.", id)}
	}
	if q.Status != "active" {
		return Result{Output: fmt.Sprintf("quest %q is already %s.", id, q.Status)}
	}
	if q.ObjProgress < q.ObjCount {
		return Result{Output: fmt.Sprintf(
			"quest not ready: need %d/%d %s %s.",
			q.ObjProgress, q.ObjCount, q.ObjType, q.ObjTarget,
		)}
	}

	if err := quests.Complete(db, id); err != nil {
		return Result{Output: "error completing quest."}
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("quest complete: %s\n", q.Title))
	out.WriteString("rewards:\n")

	if q.RewardCredits > 0 {
		credits.Add(db, q.RewardCredits) //nolint:errcheck
		out.WriteString(fmt.Sprintf("  + %d credits\n", q.RewardCredits))
	}

	if q.RewardXPSkill != "" && q.RewardXPAmount > 0 {
		awardRes, err := skills.Award(db, q.RewardXPSkill, q.RewardXPAmount)
		if err == nil {
			out.WriteString(fmt.Sprintf("  + %d %s XP\n", q.RewardXPAmount, q.RewardXPSkill))
			if awardRes.LeveledUp {
				out.WriteString(awardRes.LevelUpMsg + "\n")
			}
		}
	}

	if q.RewardItemID != "" {
		name := q.RewardItemName
		if name == "" {
			name = q.RewardItemID
		}
		desc := q.RewardItemDesc
		if desc == "" {
			desc = "quest reward"
		}
		if err := player.AddItem(db, q.RewardItemID, name, desc); err == nil {
			out.WriteString(fmt.Sprintf("  + item: %s\n", name))
		}
		// Mark crystal shard collected if this quest rewards one.
		shardIDs := map[string]bool{
			"meadow-shard": true, "forest-shard": true, "desert-shard": true,
			"mountain-shard": true, "cave-shard": true,
		}
		if shardIDs[q.RewardItemID] {
			player.MarkShardCollected(db, q.RewardItemID) //nolint:errcheck
		}
	}

	return Result{Output: strings.TrimRight(out.String(), "\n")}
}

// Events lists active world events or joins one.
func Events(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) >= 2 && args[0] == "join" {
		return eventJoin(db, args[1])
	}
	if len(args) >= 2 && args[0] == "complete" {
		return eventComplete(db, args[1])
	}

	active, err := events.Active(db)
	if err != nil {
		return Result{Output: "error loading events."}
	}

	// Seed events if none active
	if len(active) == 0 {
		e1, _ := events.SeedRandom(db, w)
		if e1 != nil {
			active = append(active, *e1)
		}
		// Seed a second event
		e2, _ := events.SeedRandom(db, w)
		if e2 != nil {
			active = append(active, *e2)
		}
	}

	if len(active) == 0 {
		return Result{Output: "no active world events."}
	}

	var b strings.Builder
	b.WriteString("active world events:\n")
	for _, ev := range active {
		b.WriteString(fmt.Sprintf("  [%s] %s\n", ev.ID, ev.Title))
		b.WriteString(fmt.Sprintf("      type: %s | room: %s | faction: %s\n", ev.Type, ev.TargetRoom, ev.Faction))
		b.WriteString(fmt.Sprintf("      payout: %d credits + %s\n", ev.PayoutCredits, ev.PayoutItemName))
		b.WriteString(fmt.Sprintf("      expires in ~%d actions\n", ev.ExpiresActions))
		b.WriteString(fmt.Sprintf("      %s\n", ev.Description))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

func eventJoin(db *sql.DB, id string) Result {
	ev, err := events.Get(db, id)
	if err != nil {
		return Result{Output: fmt.Sprintf("no event %q found.", id)}
	}
	if ev.Status != "active" {
		return Result{Output: fmt.Sprintf("event %q is %s.", id, ev.Status)}
	}
	return Result{Output: fmt.Sprintf(
		"[EVENT BRIEFING] %s\n%s\ntarget room: %s\npayout: %d credits + %s\ngo to the target room and complete the objective.",
		ev.Title, ev.Description, ev.TargetRoom, ev.PayoutCredits, ev.PayoutItemName,
	)}
}

func eventComplete(db *sql.DB, id string) Result {
	ev, err := events.Get(db, id)
	if err != nil {
		return Result{Output: fmt.Sprintf("no event %q found.", id)}
	}
	if ev.Status != "active" {
		return Result{Output: fmt.Sprintf("event %q is already %s.", id, ev.Status)}
	}
	if err := events.Complete(db, id); err != nil {
		return Result{Output: "error completing event."}
	}
	var out strings.Builder
	out.WriteString(fmt.Sprintf("[EVENT COMPLETE] %s\n", ev.Title))
	if ev.PayoutCredits > 0 {
		credits.Add(db, ev.PayoutCredits) //nolint:errcheck
		out.WriteString(fmt.Sprintf("  + %d credits\n", ev.PayoutCredits))
	}
	if ev.PayoutItemID != "" {
		name := ev.PayoutItemName
		if name == "" {
			name = ev.PayoutItemID
		}
		player.AddItem(db, ev.PayoutItemID, name, ev.PayoutItemDesc) //nolint:errcheck
		out.WriteString(fmt.Sprintf("  + item: %s\n", name))
	}
	return Result{Output: strings.TrimRight(out.String(), "\n")}
}

// Faction shows faction status or creates a faction.
func Faction(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) >= 1 && args[0] == "create" {
		name := ""
		agenda := ""
		if len(args) >= 2 {
			name = args[1]
		}
		if len(args) >= 3 {
			agenda = strings.Join(args[2:], " ")
		}
		if name == "" {
			return Result{Output: "usage: faction create <name> [agenda]"}
		}
		return factionCreate(db, s, w, name, agenda)
	}

	exists, err := factions.Exists(db)
	if err != nil || !exists {
		return Result{Output: "you have no faction. use 'faction create <name> [agenda]' to start one."}
	}

	f, err := factions.Get(db)
	if err != nil {
		return Result{Output: "error loading faction."}
	}
	count, _ := factions.MemberCount(db)
	bal := credits.Get(db)
	hideoutInfo := f.HideoutRoomID
	if hideoutInfo == "" {
		hideoutInfo = "(none set)"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("faction: %s\n", f.FactionName))
	if f.Agenda != "" {
		b.WriteString(fmt.Sprintf("agenda: %s\n", f.Agenda))
	}
	b.WriteString(fmt.Sprintf("members: %d\n", count))
	b.WriteString(fmt.Sprintf("hideout: %s\n", hideoutInfo))
	b.WriteString(fmt.Sprintf("credits: %d ¢\n", bal))
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

func factionCreate(db *sql.DB, s *player.State, w *world.World, name, agenda string) Result {
	exists, _ := factions.Exists(db)
	if exists {
		return Result{Output: "you already have a faction."}
	}
	f, err := factions.Create(db, name, agenda)
	if err != nil {
		return Result{Output: fmt.Sprintf("failed to create faction: %v", err)}
	}

	// Generate and add hideout room
	room := hideout.GenerateHideout(name)
	w.AddRoom(&room)
	if err := factions.SetHideout(db, room.ID); err != nil {
		return Result{Output: fmt.Sprintf("faction created but hideout failed: %v", err)}
	}

	return Result{
		Output: fmt.Sprintf(
			"faction created: %s (id: %s)\nhideout: %s (%s)\nuse 'hideout' to go there.",
			f.FactionName, f.FactionID, room.Name, room.ID,
		),
		Event: &Event{
			Topic: "mud.faction.created",
			Payload: map[string]any{
				"faction_id":   f.FactionID,
				"faction_name": f.FactionName,
				"hideout_room": room.ID,
			},
		},
	}
}

// Recruit recruits an NPC in the current room into the player's faction.
func Recruit(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "recruit <npc-id> — recruit an NPC into your faction"}
	}
	npcID := args[0]

	exists, _ := factions.Exists(db)
	if !exists {
		return Result{Output: "you need a faction first. use 'faction create <name>'."}
	}

	f, err := factions.Get(db)
	if err != nil {
		return Result{Output: "error loading faction."}
	}

	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "you are nowhere."}
	}

	var npc *world.NPC
	for i := range room.NPCs {
		if room.NPCs[i].ID == npcID || strings.Contains(strings.ToLower(room.NPCs[i].Name), npcID) {
			npc = &room.NPCs[i]
			break
		}
	}
	if npc == nil {
		return Result{Output: fmt.Sprintf("no NPC %q here.", npcID)}
	}
	if !player.NPCAlive(db, s.RoomID, npc.ID) {
		return Result{Output: fmt.Sprintf("%s is dead. you can't recruit a corpse.", npc.Name)}
	}

	// Check reputation with any faction (any rep >= 3)
	rows, err := db.Query(`SELECT faction, value FROM player_reputation WHERE value >= 3`)
	if err != nil {
		return Result{Output: "error checking reputation."}
	}
	defer rows.Close()
	hasRep := rows.Next()
	rows.Close()
	if !hasRep {
		return Result{Output: "you lack the standing to recruit anyone. build reputation with a faction first (rep >= 3)."}
	}

	// Cost 200 credits
	if _, err := credits.Deduct(db, 200); err != nil {
		return Result{Output: "you need 200 credits to recruit."}
	}

	if err := factions.Recruit(db, npc.ID, npc.Name, npc.Desc, "associate"); err != nil {
		// Refund on error
		credits.Add(db, 200) //nolint:errcheck
		return Result{Output: fmt.Sprintf("recruit failed: %v", err)}
	}

	// Station them at hideout if set
	if f.HideoutRoomID != "" {
		db.Exec(`UPDATE faction_members SET stationed_room=? WHERE npc_id=?`, f.HideoutRoomID, npc.ID) //nolint:errcheck
	}

	return Result{
		Output: fmt.Sprintf(
			"%s sizes you up, then nods. \"Alright. I'm in.\"\n%s is now part of %s. (-200 credits)",
			npc.Name, npc.Name, f.FactionName,
		),
		Event: &Event{
			Topic: "mud.faction.recruited",
			Payload: map[string]any{
				"npc_id":      npc.ID,
				"npc_name":    npc.Name,
				"faction_id":  f.FactionID,
				"room_id":     s.RoomID,
			},
		},
	}
}

// Hideout teleports the player to their faction hideout.
func Hideout(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	exists, _ := factions.Exists(db)
	if !exists {
		return Result{Output: "you have no faction hideout. create a faction first."}
	}
	f, err := factions.Get(db)
	if err != nil {
		return Result{Output: "error loading faction."}
	}
	if f.HideoutRoomID == "" {
		return Result{Output: "no hideout set for your faction."}
	}

	room := w.Room(f.HideoutRoomID)
	if room == nil {
		return Result{Output: fmt.Sprintf("hideout room %q not found in world.", f.HideoutRoomID)}
	}

	s.RoomID = f.HideoutRoomID
	player.Save(db, s) //nolint:errcheck

	var out strings.Builder
	out.WriteString(room.Render(true))

	// Apply hideout upgrade bonuses
	if has, _ := hideout.HasUpgrade(db, "med-bay"); has {
		s.HP = s.MaxHP
		player.Save(db, s) //nolint:errcheck
		out.WriteString("\n[med-bay] wounds patched. HP restored to full.")
	}
	if has, _ := hideout.HasUpgrade(db, "signal-jammer"); has {
		st := espionage.LoadStealth(db)
		if st.Level < 80 {
			st.Level = 80
			espionage.SaveStealth(db, st) //nolint:errcheck
		}
		out.WriteString("\n[signal-jammer] stealth reset to 80.")
	}
	if has, _ := hideout.HasUpgrade(db, "training-deck"); has {
		awardRes, err := skills.Award(db, "hacking", 50)
		if err == nil {
			out.WriteString(fmt.Sprintf("\n[training-deck] +50 hacking XP."))
			if awardRes.LeveledUp {
				out.WriteString("\n" + awardRes.LevelUpMsg)
			}
		}
	}

	// Show installed upgrades
	installed, _ := hideout.Installed(db)
	if len(installed) > 0 {
		out.WriteString("\ninstalled upgrades: " + strings.Join(installed, ", "))
	}

	return Result{Output: out.String()}
}

// Upgrade lists or purchases hideout upgrades.
func Upgrade(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "upgrade list — show upgrades\nupgrade buy <id> — purchase an upgrade"}
	}
	switch args[0] {
	case "list":
		return upgradeList(db)
	case "buy":
		if len(args) < 2 {
			return Result{Output: "upgrade buy <id>"}
		}
		return upgradeBuy(db, args[1])
	default:
		return Result{Output: "upgrade list | upgrade buy <id>"}
	}
}

func upgradeList(db *sql.DB) Result {
	installed, _ := hideout.Installed(db)
	installedSet := make(map[string]bool, len(installed))
	for _, id := range installed {
		installedSet[id] = true
	}
	bal := credits.Get(db)
	hackLevel := skills.Level(db, "hacking")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("hideout upgrades (balance: %d ¢, hacking level: %d):\n", bal, hackLevel))
	for _, u := range hideout.Catalog {
		status := "available"
		if installedSet[u.ID] {
			status = "INSTALLED"
		} else if hackLevel < u.SkillReq {
			status = fmt.Sprintf("locked (hacking level %d required)", u.SkillReq)
		}
		b.WriteString(fmt.Sprintf("  [%s] %s — %d ¢  [%s]\n", u.ID, u.Name, u.Cost, status))
		b.WriteString(fmt.Sprintf("      %s\n", u.Desc))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

func upgradeBuy(db *sql.DB, id string) Result {
	u, err := hideout.FindUpgrade(id)
	if err != nil {
		return Result{Output: fmt.Sprintf("unknown upgrade %q. use 'upgrade list' to see options.", id)}
	}

	has, _ := hideout.HasUpgrade(db, id)
	if has {
		return Result{Output: fmt.Sprintf("%s is already installed.", u.Name)}
	}

	hackLevel := skills.Level(db, "hacking")
	if hackLevel < u.SkillReq {
		return Result{Output: fmt.Sprintf(
			"%s requires hacking level %d (you have %d).",
			u.Name, u.SkillReq, hackLevel,
		)}
	}

	if _, err := credits.Deduct(db, u.Cost); err != nil {
		bal := credits.Get(db)
		return Result{Output: fmt.Sprintf(
			"not enough credits. %s costs %d ¢, you have %d ¢.",
			u.Name, u.Cost, bal,
		)}
	}

	if err := hideout.Install(db, id); err != nil {
		credits.Add(db, u.Cost) //nolint:errcheck
		return Result{Output: fmt.Sprintf("install failed: %v", err)}
	}

	flavorMap := map[string]string{
		"workbench":      "Guts spill across the table surface — circuits, tools, salvage. The workbench is live.",
		"armory":         "The rack slots into the wall. Weapons hang silent and ready.",
		"med-bay":        "The med-bay hums to life, diagnostic lights cycling green. You can rest easy here.",
		"signal-jammer":  "A low-frequency buzz fills the room briefly, then dies. Nothing gets in or out.",
		"training-deck":  "The deck boots up, running ghost scenarios. Time to get sharper.",
		"intel-hub":      "Feeds scroll across the display. The whole grid is visible from here.",
		"vault":          "The vault door locks with a satisfying thunk. Your stash is secure.",
	}
	flavor := flavorMap[id]
	if flavor == "" {
		flavor = fmt.Sprintf("%s installed.", u.Name)
	}

	return Result{Output: fmt.Sprintf("[UPGRADE INSTALLED] %s (--%d ¢)\n%s", u.Name, u.Cost, flavor)}
}
