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
	defer cmdCancel()

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

	payload := StateUpdatePayload{
		HP:        s.state.HP,
		MaxHP:     s.state.MaxHP,
		RoomName:  roomName,
		Exits:     exits,
		Inventory: hudInv,
		Credits:   credits.Get(s.database),
	}
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "state.update", Payload: payload})

	// Also send the current player roster so new joins always see a fresh list.
	names := s.registry.List()
	plist := make([]PlayerInfo, len(names))
	for i, n := range names {
		plist[i] = PlayerInfo{Name: n}
	}
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type: "players.update",
		Payload: PlayersUpdatePayload{
			HostOnline: true,
			Players:    plist,
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
