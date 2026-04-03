package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nhooyr.io/websocket"

	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/credits"
	"github.com/adam-stokes/gl1tch-mud/internal/db"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// ClientSession represents one connected browser player.
type ClientSession struct {
	playerID     string
	conn         *websocket.Conn
	database     *sql.DB
	state        *player.State
	world        *world.World
	worldName    string
	cancel       context.CancelFunc
	lastActivity time.Time
	registry     *SessionRegistry
}

// Handle is the main read loop for a connected, authenticated session.
// It opens the player's DB, loads state, and dispatches incoming messages.
func (s *ClientSession) Handle(ctx context.Context) {
	var err error
	s.database, err = db.OpenForPlayer(s.playerID)
	if err != nil {
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "error",
			Payload: ErrorPayload{Message: fmt.Sprintf("failed to open player db: %v", err)},
		})
		return
	}
	defer s.database.Close()
	defer func() {
		if s.state != nil {
			_ = player.Save(s.database, s.state)
		}
	}()

	s.state, err = player.Load(s.database)
	if err != nil {
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "error",
			Payload: ErrorPayload{Message: fmt.Sprintf("failed to load player: %v", err)},
		})
		return
	}

	// If the player was last in a different world, reset them to this world's start room.
	if s.state.World != s.worldName {
		newState, resetErr := player.LoadForWorld(s.database, s.worldName, s.world.StartRoom)
		if resetErr == nil {
			s.state = newState
		} else {
			s.state.RoomID = s.world.StartRoom
			s.state.World = s.worldName
		}
		world.SeedCrystalShards(s.database, s.worldName)  //nolint:errcheck
		world.SeedStartingItems(s.database, s.worldName)  //nolint:errcheck
	}

	// Send welcome look.
	res := commands.Look(s.database, s.state, s.world, nil)
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type:    "output.token",
		Payload: OutputTokenPayload{Token: res.Output + "\r\n"},
	})
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
	s.sendStateUpdate(ctx)

	// Command context — replaced on each command to support interrupt.
	cmdCtx, cmdCancel := context.WithCancel(ctx)
	defer func() { cmdCancel() }()

	for {
		_, data, err := s.conn.Read(ctx)
		if err != nil {
			return
		}
		s.lastActivity = time.Now()

		var msg ClientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "error",
				Payload: ErrorPayload{Message: "invalid message"},
			})
			continue
		}

		switch msg.Type {
		case "interrupt":
			cmdCancel()
			cmdCtx, cmdCancel = context.WithCancel(ctx)
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})

		case "input":
			var p InputPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				continue
			}
			s.dispatchCommand(cmdCtx, p.Text)

		case "chat":
			var p ChatPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				continue
			}
			text := strings.TrimSpace(p.Text)
			if text == "" {
				continue
			}
			s.registry.BroadcastToWorld(s.worldName, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: s.playerID, Text: text},
			})

		default:
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "error",
				Payload: ErrorPayload{Message: fmt.Sprintf("unknown message type: %s", msg.Type)},
			})
		}
	}
}

// dispatchCommand parses and executes one game command, streaming output back.
func (s *ClientSession) dispatchCommand(ctx context.Context, input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		return
	}

	verb, args := commands.Parse(input)

	// ── say: alias for chat broadcast ────────────────────────────────────────
	if verb == "say" {
		text := strings.Join(args, " ")
		if text != "" {
			s.registry.BroadcastToWorld(s.worldName, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: s.playerID, Text: text},
			})
		}
		_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		return
	}

	// ── goto: teleport to another connected player ────────────────────────────
	if verb == "goto" {
		if len(args) == 0 {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: "goto <player> — teleport to a connected player's location\r\n"},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
			return
		}
		targetID := args[0]
		if targetID == s.playerID {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: "you can't teleport to yourself.\r\n"},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
			return
		}
		roomID := s.registry.GetRoomID(targetID)
		if roomID == "" {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: fmt.Sprintf("player %q is not connected.\r\n", targetID)},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
			return
		}
		// Block cross-world goto.
		if targetWorld := s.registry.GetWorldName(targetID); targetWorld != s.worldName {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: fmt.Sprintf("player %q is in a different world.\r\n", targetID)},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
			return
		}
		if roomID == s.state.RoomID {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: fmt.Sprintf("you are already in the same node as %s.\r\n", targetID)},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
			return
		}
		s.state.RoomID = roomID
		res := commands.Look(s.database, s.state, s.world, nil)
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "output.token",
			Payload: OutputTokenPayload{Token: fmt.Sprintf("* jacking into %s's node... *\r\n\r\n", targetID)},
		})
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "output.token",
			Payload: OutputTokenPayload{Token: res.Output + "\r\n"},
		})
		_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		s.sendStateUpdate(ctx)
		return
	}

	handler, ok := commands.Registry[verb]
	if !ok {
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "output.token",
			Payload: OutputTokenPayload{Token: fmt.Sprintf("unknown command: %q — type 'help' for a list.\r\n", verb)},
		})
		_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		return
	}

	result := handler(s.database, s.state, s.world, args)
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type:    "output.token",
		Payload: OutputTokenPayload{Token: result.Output + "\r\n"},
	})
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
	s.sendStateUpdate(ctx)
}

// sendStateUpdate builds and sends a state.update message with current player state.
func (s *ClientSession) sendStateUpdate(ctx context.Context) {
	if s.state == nil || s.database == nil {
		return
	}

	// Room name and exits.
	var roomName string
	var exits []string
	if room := s.world.Room(s.state.RoomID); room != nil {
		roomName = room.Name
		for dir := range room.Exits {
			exits = append(exits, dir)
		}
	}

	// Inventory with signal tier.
	invItems, _ := player.Inventory(s.database)
	hudInv := make([]InvItem, 0, len(invItems))
	for _, it := range invItems {
		tier := ""
		if wi := s.world.FindItem(it.ID); wi != nil {
			tier = wi.SignalTier
		}
		hudInv = append(hudInv, InvItem{
			ID:   it.ID,
			Name: it.Name,
			Desc: it.Desc,
			Tier: tier,
		})
	}

	// Crafting recipes.
	recipes := make([]Recipe, 0, len(s.world.CraftingRecipes))
	for _, r := range s.world.CraftingRecipes {
		ings := make([]RecipeIngredient, len(r.Ingredients))
		for i, ing := range r.Ingredients {
			ings[i] = RecipeIngredient{ID: ing.ID, Count: ing.Count}
		}
		recipes = append(recipes, Recipe{
			ID:          r.ID,
			Name:        r.Name,
			Ingredients: ings,
			OutputID:    r.Output.ID,
			OutputName:  r.Output.Name,
			SkillReq:    r.SkillReq,
		})
	}

	payload := StateUpdatePayload{
		HP:        s.state.HP,
		MaxHP:     s.state.MaxHP,
		RoomName:  roomName,
		Exits:     exits,
		Inventory: hudInv,
		Credits:   credits.Get(s.database),
		Recipes:   recipes,
	}
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "state.update", Payload: payload})

	// Also send the current player roster so new joins always see a fresh list.
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type: "players.update",
		Payload: PlayersUpdatePayload{
			HostOnline: true,
			Players:    s.registry.PlayersInWorld(s.worldName, s.world),
		},
	})
}

// Close cancels the session context and unregisters the player.
func (s *ClientSession) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.registry != nil {
		s.registry.Unregister(s.playerID)
	}
}
