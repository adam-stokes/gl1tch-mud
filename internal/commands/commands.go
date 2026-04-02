// Package commands implements the MUD verb dispatch table.
package commands

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/crafting"
	"github.com/adam-stokes/gl1tch-mud/internal/espionage"
	"github.com/adam-stokes/gl1tch-mud/internal/generation"
	"github.com/adam-stokes/gl1tch-mud/internal/hacking"
	"github.com/adam-stokes/gl1tch-mud/internal/locking"
	"github.com/adam-stokes/gl1tch-mud/internal/looting"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
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
	Output string
	Event  *Event
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
	"explore":  Explore,
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

	// Check for low-stealth auto-detection
	detection := checkStealthDetection(db, s, room)

	output := room.Render(visited)
	if detection != "" {
		output += "\n" + detection
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
			return Result{Output: item.Desc}
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
	for _, item := range room.Items {
		if strings.Contains(strings.ToLower(item.Name), target) {
			if err := player.AddItem(db, item.ID, item.Name, item.Desc); err != nil {
				return Result{Output: fmt.Sprintf("can't take %s — already carrying it.", item.Name)}
			}
			return Result{
				Output: fmt.Sprintf("you pick up %s.", item.Name),
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
			lootItems := looting.Roll(w, npc.ID)
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
			s.HP = s.MaxHP
			s.RoomID = w.StartRoom
			player.Save(db, s) //nolint:errcheck
			out.WriteString("\nyou died. jacking back in at the entry node.")
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
		b.WriteString(fmt.Sprintf("  - %s\n", it.Name))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Help lists available commands.
func Help(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	return Result{Output: `commands:
  look / l          — describe current room
  go <dir>          — move (north/south/east/west, or just: n/s/e/w)
  examine <thing>   — examine an NPC or item
  take <item>       — pick up an item
  attack <npc>      — attack an enemy
  inventory / i     — list carried items
  skills            — show skill levels and XP
  hack <system>     — attempt to hack a system in the room
  pick <lock>       — attempt to pick a lock
  unlock <lock>     — use a key to unlock a lock
  offers <npc>      — list what an NPC will trade
  trade <trade-id>  — execute a trade with an NPC in the room
  craft <recipe>    — craft an item from ingredients
  hide              — attempt to increase stealth
  disguise <item>   — equip a disguise item
  talk <npc>        — speak to an NPC
  explore <dir>     — explore an unmapped direction (may generate new rooms)
  help / ?          — this list
  quit              — disconnect`}
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

// Hack attempts to compromise a hackable system in the current room.
func Hack(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "hack what? (hack <system-id>)"}
	}
	systemID := args[0]
	room := w.Room(s.RoomID)

	hackSkill := skills.Level(db, "hacking")
	res := hacking.Hack(db, room, systemID, hackSkill)

	var ev *Event
	var out strings.Builder
	out.WriteString(res.Message)

	if res.Success {
		// Deliver reward item if specified
		if res.RewardItem != "" {
			player.AddItem(db, res.RewardItem, res.RewardItem, "system reward") //nolint:errcheck
			out.WriteString(fmt.Sprintf("\nyou receive: %s", res.RewardItem))
		}

		// Award XP
		awardRes, err := skills.Award(db, "hacking", 20)
		if err == nil && awardRes.LeveledUp {
			out.WriteString("\n" + awardRes.LevelUpMsg)
		}

		ev = &Event{
			Topic: "mud.hack.success",
			Payload: map[string]any{
				"system_id": systemID,
				"reward":    res.RewardItem,
				"room_id":   s.RoomID,
			},
		}
	} else if !res.NoSystem && !res.AlreadyHacked {
		ev = &Event{
			Topic: "mud.hack.alert",
			Payload: map[string]any{
				"system_id":   systemID,
				"alert_level": res.AlertLevel,
				"room_id":     s.RoomID,
			},
		}
	}

	return Result{Output: out.String(), Event: ev}
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
		ev = &Event{
			Topic: "mud.trade.completed",
			Payload: map[string]any{
				"npc_id":   npc.ID,
				"gave":     res.GaveItems,
				"received": res.FactionInc,
				"room_id":  s.RoomID,
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

	res := crafting.Craft(db, w, recipeID, invIDs, hackSkill)

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

	ctx := espionage.PlayerContext{
		InventoryIDs: invIDs,
		Reputation:   rep,
		Skills:       sk,
		Disguise:     st.Disguise,
	}

	text := espionage.EvalDialogue(npc.Dialogue, ctx)
	espionage.RecordMemory(db, npc.ID, "talked") //nolint:errcheck

	// Find matched trigger for event
	matchedTrigger := "none"
	for _, line := range npc.Dialogue {
		if line.Text == text {
			matchedTrigger = line.Trigger
			break
		}
	}

	return Result{
		Output: fmt.Sprintf("%s: \"%s\"", npc.Name, text),
		Event: &Event{
			Topic: "mud.espionage.talked",
			Payload: map[string]any{
				"npc_id":  npc.ID,
				"trigger": matchedTrigger,
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
