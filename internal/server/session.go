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

	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/crafting"
	"github.com/adam-stokes/gl1tch-mud/internal/credits"
	"github.com/adam-stokes/gl1tch-mud/internal/db"
	"github.com/adam-stokes/gl1tch-mud/internal/factions"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/quests"
	"github.com/adam-stokes/gl1tch-mud/internal/skills"
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
	s.database, err = db.OpenForPlayer(s.playerID, s.worldName)
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
	s.state.PlayerID = s.playerID
	player.LoadDefense(s.database, s.state)

	// If the player's saved world differs or their room no longer exists in this
	// world, reset them to the start room.
	if s.state.World != s.worldName || s.world.Room(s.state.RoomID) == nil {
		s.state.RoomID = s.world.StartRoom
		s.state.World = s.worldName
		_ = player.Save(s.database, s.state)
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
			// If the player addressed glitch, publish a mention event so the
			// companion pipeline can generate a reply via mud.chat.reply.
			lower := strings.ToLower(text)
			if strings.Contains(lower, "glitch") {
				s.registry.PublishEvent("mud.chat.mention", map[string]any{
					"from":  s.playerID,
					"text":  text,
					"world": s.worldName,
				})
			}

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
			if strings.Contains(strings.ToLower(text), "glitch") {
				s.registry.PublishEvent("mud.chat.mention", map[string]any{
					"from":  s.playerID,
					"text":  text,
					"world": s.worldName,
				})
			}
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

	if result.Event != nil {
		s.registry.PublishEvent(result.Event.Topic, result.Event.Payload)
		// Also forward to gamification if this event maps to a game action.
		if action, ok := busd.MapMudEvent(result.Event.Topic, result.Event.Payload); ok {
			faction := "unaffiliated"
			if pf, err := factions.Get(s.database); err == nil && pf != nil {
				faction = pf.FactionID
			}
			s.registry.PublishEvent("game.action", map[string]any{
				"source":  "gl1tch-mud",
				"player":  s.playerID,
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

	if result.PendingRequestID != "" {
		s.registry.RegisterPendingRequest(result.PendingRequestID, result.PendingPlayer)
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

	s.sendStateUpdate(ctx)
}

// switchWorld performs a live world switch for the session: saves current state,
// reopens the database for the new world, updates session fields, and notifies
// the client with a world_meta message so the UI title updates immediately.
func (s *ClientSession) switchWorld(ctx context.Context, targetName string) error {
	if s.state != nil && s.database != nil {
		_ = player.Save(s.database, s.state)
		s.database.Close()
	}

	newDB, err := db.OpenForPlayer(s.playerID, targetName)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	newWorld, err := world.Load(targetName)
	if err != nil {
		newDB.Close()
		return fmt.Errorf("load world: %w", err)
	}

	newState, err := player.LoadForWorld(newDB, targetName, newWorld.StartRoom)
	if err != nil {
		newDB.Close()
		return fmt.Errorf("load player: %w", err)
	}
	if newState.World != targetName || newWorld.Room(newState.RoomID) == nil {
		newState.RoomID = newWorld.StartRoom
		newState.World = targetName
		_ = player.Save(newDB, newState)
		world.SeedCrystalShards(newDB, targetName)  //nolint:errcheck
		world.SeedStartingItems(newDB, targetName)  //nolint:errcheck
	}

	s.database = newDB
	s.world = newWorld
	s.worldName = targetName
	s.state = newState
	s.state.PlayerID = s.playerID
	player.LoadDefense(newDB, newState)

	// Tell the client to update title, theme, UI profile, and room grid.
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type: "world_meta",
		Payload: WorldMetaPayload{
			Name:      newWorld.Name,
			Tagline:   newWorld.UI.Tagline,
			Theme:     newWorld.UI.Theme,
			UIProfile: newWorld.UI.Profile,
			MapRooms:  buildMapRooms(newWorld),
		},
	})

	// Send the first look in the new world.
	res := commands.Look(newDB, newState, newWorld, nil)
	_ = writeMsg(ctx, s.conn, ServerMsg{
		Type:    "output.token",
		Payload: OutputTokenPayload{Token: res.Output + "\r\n"},
	})
	_ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
	s.sendStateUpdate(ctx)
	return nil
}

// sendStateUpdate builds and sends a state.update message with current player state.
func (s *ClientSession) sendStateUpdate(ctx context.Context) {
	if s.state == nil || s.database == nil {
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
		for _, item := range room.Items {
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
	activeQuests, _ := quests.Active(s.database)
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
	allSkills, _ := skills.All(s.database)
	skillInfos := make([]SkillInfo, 0, len(allSkills))
	for name, lv := range allSkills {
		skillInfos = append(skillInfos, SkillInfo{Name: name, Level: lv[0], XP: lv[1]})
	}
	sort.Slice(skillInfos, func(i, j int) bool { return skillInfos[i].Name < skillInfos[j].Name })

	// Inventory with signal tier.
	invItems, _ := player.Inventory(s.database)
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
	gunUnlocked := crafting.IsPlayerFlagSet(s.database, "gun_recipes_unlocked")
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

	payload := StateUpdatePayload{
		HP:            s.state.HP,
		MaxHP:         s.state.MaxHP,
		RoomID:        s.state.RoomID,
		RoomName:      roomName,
		Exits:         exits,
		Inventory:     hudInv,
		Credits:       credits.Get(s.database),
		Recipes:       recipes,
		RoomNPCs:      roomNPCs,
		RoomItems:     roomItems,
		RoomResources: roomResources,
		Quests:        questInfos,
		Skills:        skillInfos,
		OnlinePlayers: s.registry.OnlinePlayersInWorld(s.worldName, s.playerID),
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
