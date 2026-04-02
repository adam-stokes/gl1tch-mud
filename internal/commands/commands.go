// Package commands implements the MUD verb dispatch table.
package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
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

// Look describes the current room.
func Look(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "you are nowhere. that's unsettling."}
	}
	visited := player.HasVisited(db, s.RoomID)
	player.MarkVisited(db, s.RoomID)
	return Result{
		Output: room.Render(visited),
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
	s.RoomID = dest
	if err := player.Save(db, s); err != nil {
		return Result{Output: "system error: failed to save position."}
	}
	return Look(db, s, w, nil)
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
			ev = &Event{
				Topic: "mud.combat.ended",
				Payload: map[string]any{
					"npc_id":   npc.ID,
					"npc_name": npc.Name,
					"outcome":  "victory",
					"room_id":  s.RoomID,
				},
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
  help / ?          — this list
  quit              — disconnect`}
}
