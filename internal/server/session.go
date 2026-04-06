package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"nhooyr.io/websocket"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adam-stokes/gl1tch-mud/internal/analytics"
	"github.com/adam-stokes/gl1tch-mud/internal/base"
	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/chat"
	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/crafting"
	"github.com/adam-stokes/gl1tch-mud/internal/credits"
	"github.com/adam-stokes/gl1tch-mud/internal/db"
	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/db/pgq"
	"github.com/adam-stokes/gl1tch-mud/internal/factions"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/quests"
	"github.com/adam-stokes/gl1tch-mud/internal/skills"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// ClientSession represents one connected browser player.
type ClientSession struct {
	accountID    string // UUID from Postgres auth, or playerID for legacy
	username     string // display name (username for Postgres, playerID for legacy)
	role         string // "admin" or "player"
	conn         *websocket.Conn
	database     *sql.DB
	gdb          *gamedb.GameDB
	state        *player.State
	world        *world.World
	worldName    string
	pgPool       *pgxpool.Pool // non-nil for shared worlds
	cancel       context.CancelFunc
	lastActivity time.Time
	registry     *SessionRegistry

	// Analytics tracking
	loginAt           time.Time
	roomEnteredAt     time.Time
	lastRoomID        string
}

// Handle is the main read loop for a connected, authenticated session.
// It opens the player's DB, loads state, and dispatches incoming messages.
func (s *ClientSession) Handle(ctx context.Context) {
	var err error
	if s.world.IsShared() && s.pgPool != nil {
		s.gdb = gamedb.NewPostgres(s.pgPool, s.accountID, s.worldName)
	} else {
		s.database, err = db.OpenForPlayer(s.accountID, s.worldName)
		if err != nil {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "error",
				Payload: ErrorPayload{Message: fmt.Sprintf("failed to open player db: %v", err)},
			})
			return
		}
		defer s.database.Close()
		s.gdb = gamedb.NewSQLite(s.database)
	}
	s.loginAt = time.Now()
	analytics.Login(s.accountID, s.username, s.worldName)
	defer func() {
		if s.state != nil {
			_ = player.Save(s.gdb, s.state)
		}
		// Final room exit timing for the session.
		if s.lastRoomID != "" {
			analytics.RoomExit(s.accountID, s.username, s.worldName, s.lastRoomID,
				time.Since(s.roomEnteredAt).Milliseconds())
		}
		analytics.Logout(s.accountID, s.username, s.worldName, int(time.Since(s.loginAt).Seconds()))
	}()

	s.state, err = player.Load(s.gdb)
	if err != nil {
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "error",
			Payload: ErrorPayload{Message: fmt.Sprintf("failed to load player: %v", err)},
		})
		return
	}
	s.state.PlayerID = s.username
	s.state.Role = s.role
	player.LoadDefense(s.gdb, s.state)
	if s.worldName == "mudout" {
		if report := base.ResolvePendingRaids(s.gdb, s.world); report != "" {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: report + "\r\n\r\n"},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		}
	}

	// If the player's saved world differs or their room no longer exists in this
	// world, reset them to the start room.
	if s.state.World != s.worldName || s.world.Room(s.state.RoomID) == nil {
		s.state.RoomID = s.world.StartRoom
		s.state.World = s.worldName
		_ = player.Save(s.gdb, s.state)
		world.SeedCrystalShards(s.gdb, s.worldName)  //nolint:errcheck
		world.SeedStartingItems(s.gdb, s.worldName)  //nolint:errcheck
	}

	// Send welcome look.
	res := commands.Look(s.gdb, s.state, s.world, nil)
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type:    "output.token",
		Payload: OutputTokenPayload{Token: res.Output + "\r\n"},
	})
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
	s.sendStateUpdate(ctx)

	// Seed initial room tracking for analytics.
	s.lastRoomID = s.state.RoomID
	s.roomEnteredAt = time.Now()
	analytics.RoomEnter(s.accountID, s.username, s.worldName, s.state.RoomID)

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
				Payload: ChatMessagePayload{From: s.username, Text: text},
			})
			// If the player addressed glitch, publish a mention event so the
			// companion pipeline can generate a reply via mud.chat.reply.
			lower := strings.ToLower(text)
			if strings.Contains(lower, "glitch") {
				s.registry.PublishEvent("mud.chat.mention", map[string]any{
					"from":  s.username,
					"text":  text,
					"world": s.worldName,
				})
			}

		case "analytics":
			// Frontend "no result" / button-click telemetry. Server enriches
			// the payload with auth + world context before logging.
			var p map[string]any
			if err := json.Unmarshal(msg.Payload, &p); err != nil || p == nil {
				continue
			}
			p["account"] = s.accountID
			p["user"] = s.username
			p["world"] = s.worldName
			analytics.Event("client_event", p)

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

	// Analytics: log every command attempt. We log before dispatch with
	// success=true since most commands succeed; the registry path below
	// records output length once the handler returns.
	analytics.Command(s.accountID, s.username, s.worldName, s.state.RoomID, verb, args, "", true)

	// ── say: broadcast to room ───────────────────────────────────────────────
	if verb == "say" {
		result := chat.Say(s.username, args)
		if result.Output != "" {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: result.Output + "\r\n"},
			})
		}
		s.routeChatMessages(ctx, result)
		_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		return
	}

	// ── shout: broadcast to world ────────────────────────────────────────────
	if verb == "shout" {
		result := chat.Shout(s.username, args)
		if result.Output != "" {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: result.Output + "\r\n"},
			})
		}
		s.routeChatMessages(ctx, result)
		_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		return
	}

	// ── whisper/tell: private message to a player ────────────────────────────
	if verb == "whisper" || verb == "tell" {
		result := chat.Whisper(s.username, args)
		if result.Output != "" {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: result.Output + "\r\n"},
			})
		}
		s.routeChatMessages(ctx, result)
		_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		return
	}

	// ── who: list connected players (uses registry, handled in session) ─────
	if verb == "who" {
		players := s.registry.OnlinePlayersInWorld(s.worldName, "")
		var b strings.Builder
		b.WriteString(fmt.Sprintf("online in %s (%d):\r\n", s.worldName, len(players)))
		for _, p := range players {
			b.WriteString(fmt.Sprintf("  %s — %s\r\n", p.Name, p.RoomID))
		}
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "output.token",
			Payload: OutputTokenPayload{Token: b.String()},
		})
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
		if targetID == s.accountID {
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
		s.trackRoomChange()
		res := commands.Look(s.gdb, s.state, s.world, nil)
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

	result := handler(s.gdb, s.state, s.world, args)
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type:    "output.token",
		Payload: OutputTokenPayload{Token: result.Output + "\r\n"},
	})
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})

	if result.Event != nil {
		s.registry.PublishEvent(result.Event.Topic, result.Event.Payload)
		// Also forward to gamification if this event maps to a game action.
		if action, ok := busd.MapMudEvent(result.Event.Topic, result.Event.Payload); ok {
			faction := "unaffiliated"
			if pf, err := factions.Get(s.gdb); err == nil && pf != nil {
				faction = pf.FactionID
			}
			s.registry.PublishEvent("game.action", map[string]any{
				"source":  "gl1tch-mud",
				"player":  s.username,
				"faction": faction,
				"agent":   false,
				"action":  action,
				"value":   1,
				"meta": map[string]any{
					"world": s.worldName,
				},
			})
		}
	}

	s.routeChatMessages(ctx, result)

	if result.AdminAction != nil {
		s.handleAdminAction(ctx, result.AdminAction)
	}

	if result.PendingRequestID != "" {
		s.registry.RegisterPendingRequest(result.PendingRequestID, result.PendingPlayer)
	}

	if result.MoveRoom != "" {
		s.state.RoomID = result.MoveRoom
		_ = player.Save(s.gdb, s.state)
	}

	if result.SwitchWorld != "" {
		if err := s.switchWorld(ctx, result.SwitchWorld); err != nil {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: fmt.Sprintf("world switch failed: %v\r\n", err)},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		}
		return
	}

	if s.worldName == "mudout" {
		base.MaybeSpawnRaid(s.gdb)
	}
	s.trackRoomChange()
	s.sendStateUpdate(ctx)
}

// trackRoomChange emits room_exit/room_enter analytics events when the
// session's current room differs from the last room we logged. Safe to
// call repeatedly; it is a no-op when the room has not changed.
func (s *ClientSession) trackRoomChange() {
	if s.state == nil {
		return
	}
	if s.state.RoomID == s.lastRoomID {
		return
	}
	if s.lastRoomID != "" {
		analytics.RoomExit(s.accountID, s.username, s.worldName, s.lastRoomID,
			time.Since(s.roomEnteredAt).Milliseconds())
	}
	s.lastRoomID = s.state.RoomID
	s.roomEnteredAt = time.Now()
	analytics.RoomEnter(s.accountID, s.username, s.worldName, s.state.RoomID)
}

// routeChatMessages dispatches ChatMessages from a command result to the
// appropriate recipients (room, world, or individual player).
func (s *ClientSession) routeChatMessages(ctx context.Context, result commands.Result) {
	for _, cm := range result.ChatMessages {
		switch cm.Type {
		case "say":
			s.registry.BroadcastToRoomInWorld(s.worldName, s.state.RoomID, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: cm.Sender, Text: cm.Body},
			})
			// Trigger glitch mention if applicable.
			if strings.Contains(strings.ToLower(cm.Body), "glitch") {
				s.registry.PublishEvent("mud.chat.mention", map[string]any{
					"from":  s.username,
					"text":  cm.Body,
					"world": s.worldName,
				})
			}
		case "shout":
			s.registry.BroadcastToWorld(s.worldName, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: "[SHOUT] " + cm.Sender, Text: cm.Body},
			})
			if strings.Contains(strings.ToLower(cm.Body), "glitch") {
				s.registry.PublishEvent("mud.chat.mention", map[string]any{
					"from":  s.username,
					"text":  cm.Body,
					"world": s.worldName,
				})
			}
		case "whisper":
			if !s.registry.SendToPlayerByName(cm.Target, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: "[from " + cm.Sender + "]", Text: cm.Body},
			}) {
				_ = writeMsg(ctx, s.conn, ServerMsg{
					Type:    "output.token",
					Payload: OutputTokenPayload{Token: fmt.Sprintf("player %q is not online.\r\n", cm.Target)},
				})
			}
		}
	}
}

// handleAdminAction performs a session-level admin action (kick, ban, etc.).
func (s *ClientSession) handleAdminAction(ctx context.Context, action *commands.AdminAction) {
	switch action.Type {
	case "kick":
		s.registry.KickPlayer(action.Target)
	case "ban":
		if s.pgPool != nil {
			_ = pgq.New(s.pgPool).SetBanned(ctx, pgq.SetBannedParams{Banned: true, Username: action.Target})
		}
		s.registry.KickPlayer(action.Target)
	case "unban":
		if s.pgPool != nil {
			_ = pgq.New(s.pgPool).SetBanned(ctx, pgq.SetBannedParams{Banned: false, Username: action.Target})
		}
	case "announce":
		s.registry.Broadcast(ServerMsg{
			Type:    "chat.message",
			Payload: ChatMessagePayload{From: "[ANNOUNCE]", Text: action.Data},
		})
	case "teleport":
		s.registry.TeleportPlayer(action.Target, action.Data)
	case "give":
		s.registry.GiveItem(action.Target, action.Data)
	}
}

// switchWorld performs a live world switch for the session: saves current state,
// reopens the database for the new world, updates session fields, and notifies
// the client with a world_meta message so the UI title updates immediately.
func (s *ClientSession) switchWorld(ctx context.Context, targetName string) error {
	// Analytics: emit a final room_exit for the old world before switching.
	if s.lastRoomID != "" {
		analytics.RoomExit(s.accountID, s.username, s.worldName, s.lastRoomID,
			time.Since(s.roomEnteredAt).Milliseconds())
	}
	analytics.WorldSwitch(s.accountID, s.username, s.worldName, targetName)

	// Save current state before switching.
	if s.state != nil && s.gdb != nil {
		_ = player.Save(s.gdb, s.state)
	}
	// Close SQLite DB if we had one (solo world).
	if s.database != nil {
		s.database.Close()
		s.database = nil
	}

	newWorld, err := world.Load(targetName)
	if err != nil {
		return fmt.Errorf("load world: %w", err)
	}

	var newGDB *gamedb.GameDB
	var newDB *sql.DB
	if newWorld.IsShared() && s.pgPool != nil {
		newGDB = gamedb.NewPostgres(s.pgPool, s.accountID, targetName)
	} else {
		newDB, err = db.OpenForPlayer(s.accountID, targetName)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		newGDB = gamedb.NewSQLite(newDB)
	}

	newState, err := player.LoadForWorld(newGDB, targetName, newWorld.StartRoom)
	if err != nil {
		if newDB != nil {
			newDB.Close()
		}
		return fmt.Errorf("load player: %w", err)
	}
	if newState.World != targetName || newWorld.Room(newState.RoomID) == nil {
		newState.RoomID = newWorld.StartRoom
		newState.World = targetName
		_ = player.Save(newGDB, newState)
		world.SeedCrystalShards(newGDB, targetName)  //nolint:errcheck
		world.SeedStartingItems(newGDB, targetName)  //nolint:errcheck
	}

	s.database = newDB
	s.gdb = newGDB
	s.world = newWorld
	s.worldName = targetName
	s.state = newState
	s.state.PlayerID = s.username
	s.state.Role = s.role
	player.LoadDefense(s.gdb, newState)
	if targetName == "mudout" {
		if report := base.ResolvePendingRaids(s.gdb, s.world); report != "" {
			_ = writeMsg(ctx, s.conn, ServerMsg{
				Type:    "output.token",
				Payload: OutputTokenPayload{Token: report + "\r\n\r\n"},
			})
			_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
		}
	}

	// Tell the client to update title, theme, UI profile, and room grid.
	newMode := newWorld.Mode
	if newMode == "" {
		newMode = "solo"
	}
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type: "world_meta",
		Payload: WorldMetaPayload{
			Name:      newWorld.Name,
			Tagline:   newWorld.UI.Tagline,
			Mode:      newMode,
			Theme:     newWorld.UI.Theme,
			UIProfile: newWorld.UI.Profile,
			MapRooms:  buildMapRooms(newWorld),
		},
	})

	// Send the first look in the new world.
	res := commands.Look(s.gdb, newState, newWorld, nil)
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type:    "output.token",
		Payload: OutputTokenPayload{Token: res.Output + "\r\n"},
	})
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})

	// Reset room tracking for the new world and emit room_enter.
	s.lastRoomID = s.state.RoomID
	s.roomEnteredAt = time.Now()
	analytics.RoomEnter(s.accountID, s.username, s.worldName, s.state.RoomID)

	s.sendStateUpdate(ctx)
	return nil
}

// sendStateUpdate builds and sends a state.update message with current player state.
func (s *ClientSession) sendStateUpdate(ctx context.Context) {
	if s.state == nil || s.gdb == nil {
		return
	}

	// Room name, exits, and kids-mode presence data.
	var roomName string
	var exits []string
	var roomNPCs []RoomNPCInfo
	var roomItems []RoomItemInfo
	var roomResources []RoomResourceInfo
	if room := s.world.Room(s.state.RoomID); room != nil {
		roomName = room.Name
		for dir := range room.Exits {
			exits = append(exits, dir)
		}
		sort.Strings(exits)
		for _, npc := range room.NPCs {
			roomNPCs = append(roomNPCs, RoomNPCInfo{
				ID:         npc.ID,
				Name:       npc.Name,
				CanTalk:    len(npc.Dialogue) > 0,
				CanTrade:   len(npc.Trades) > 0,
				Attackable: npc.Attack > 0,
			})
		}
		// Hide items that have been taken (shared) or already carried (solo).
		takenIDs := s.gdb.ListTakenRoomItems(ctx, s.state.RoomID)
		taken := make(map[string]bool, len(takenIDs))
		for _, id := range takenIDs {
			taken[id] = true
		}
		for _, item := range room.Items {
			if taken[item.ID] {
				continue
			}
			roomItems = append(roomItems, RoomItemInfo{
				ID:       item.ID,
				Name:     item.Name,
				Takeable: true,
			})
		}
		for _, res := range room.Resources {
			roomResources = append(roomResources, RoomResourceInfo{
				ID:     res.ID,
				Name:   res.ID,
				Action: res.Type,
			})
		}
	}

	// Active quests for kids quest tracker.
	activeQuests, _ := quests.Active(s.gdb)
	questInfos := make([]QuestInfo, 0, len(activeQuests))
	for _, q := range activeQuests {
		questInfos = append(questInfos, QuestInfo{
			ID:          q.ID,
			Title:       q.Title,
			Description: q.Description,
			ObjCount:    q.ObjCount,
			ObjProgress: q.ObjProgress,
		})
	}

	// Skills for kids skills modal.
	allSkills, _ := skills.All(s.gdb)
	skillInfos := make([]SkillInfo, 0, len(allSkills))
	for name, lv := range allSkills {
		skillInfos = append(skillInfos, SkillInfo{Name: name, Level: lv[0], XP: lv[1]})
	}
	sort.Slice(skillInfos, func(i, j int) bool { return skillInfos[i].Name < skillInfos[j].Name })

	// Inventory with signal tier.
	invItems, _ := player.Inventory(s.gdb)
	hudInv := make([]InvItem, 0, len(invItems))
	for _, it := range invItems {
		tier := ""
		var tags []string
		var statMods map[string]int
		var quality string
		if wi := s.world.FindItem(it.ID); wi != nil {
			tier = wi.SignalTier
			tags = wi.Tags
			statMods = wi.StatMods
			quality = wi.Quality
		}
		hudInv = append(hudInv, InvItem{
			ID:       it.ID,
			Name:     it.Name,
			Desc:     it.Desc,
			Tier:     tier,
			Tags:     tags,
			StatMods: statMods,
			Quality:  quality,
		})
	}

	// Crafting recipes — filter out gun recipes until player has unlocked them.
	gunUnlocked := crafting.IsPlayerFlagSet(s.gdb, "gun_recipes_unlocked")
	var visibleRecipes []world.CraftingRecipe
	for _, r := range s.world.CraftingRecipes {
		if !gunUnlocked {
			isGunRecipe := false
			for _, slot := range r.Slots {
				if strings.HasPrefix(slot.AcceptsTag, "gun-") {
					isGunRecipe = true
					break
				}
			}
			if isGunRecipe {
				continue
			}
		}
		visibleRecipes = append(visibleRecipes, r)
	}

	recipes := make([]Recipe, 0, len(visibleRecipes))
	for _, r := range visibleRecipes {
		ings := make([]RecipeIngredient, len(r.Ingredients))
		for i, ing := range r.Ingredients {
			ings[i] = RecipeIngredient{ID: ing.ID, Count: ing.Count}
		}
		wireSlots := make([]RecipeSlot, 0, len(r.Slots))
		for _, s := range r.Slots {
			wireSlots = append(wireSlots, RecipeSlot{
				ID:         s.ID,
				Name:       s.Name,
				Required:   s.Required,
				AcceptsTag: s.AcceptsTag,
				StatMods:   s.StatMods,
			})
		}
		recipes = append(recipes, Recipe{
			ID:          r.ID,
			Name:        r.Name,
			Ingredients: ings,
			OutputID:    r.Output.ID,
			OutputName:  r.Output.Name,
			SkillReq:    r.SkillReq,
			Workbench:   r.Workbench,
			Type:        string(r.Type),
			Slots:       wireSlots,
		})
	}

	// Equipped armor for wasteland HUD
	var equippedArmorInfo *EquippedArmorInfo
	if rec, err := player.GetEquippedArmor(s.gdb); err == nil && rec != nil {
		equippedArmorInfo = &EquippedArmorInfo{
			ItemID:   rec.ItemID,
			ItemName: rec.ItemName,
			Defense:  rec.Defense,
		}
	}

	payload := StateUpdatePayload{
		HP:            s.state.HP,
		MaxHP:         s.state.MaxHP,
		RoomID:        s.state.RoomID,
		RoomName:      roomName,
		Exits:         exits,
		Inventory:     hudInv,
		Credits:       credits.Get(s.gdb),
		Recipes:       recipes,
		RoomNPCs:      roomNPCs,
		RoomItems:     roomItems,
		RoomResources: roomResources,
		Quests:        questInfos,
		Skills:        skillInfos,
		OnlinePlayers: s.registry.OnlinePlayersInWorld(s.worldName, s.accountID),
		EquippedArmor: equippedArmorInfo,
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
		s.registry.Unregister(s.accountID)
	}
}
