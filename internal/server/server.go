package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adam-stokes/gl1tch-mud/internal/achievements"
	"github.com/adam-stokes/gl1tch-mud/internal/auth"
	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/db/pgq"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// SessionRegistry tracks active WebSocket sessions by accountID.
type SessionRegistry struct {
	mu               sync.RWMutex
	sessions         map[string]*ClientSession // keyed by accountID
	busPub           func(topic string, payload any)
	onPendingRequest func(requestID, playerID string)
}

func newSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: make(map[string]*ClientSession)}
}

// PublishEvent sends an event to the bus, if connected.
func (r *SessionRegistry) PublishEvent(topic string, payload any) {
	if r.busPub != nil {
		r.busPub(topic, payload)
	}
}

// Register adds a session. Returns an error if that playerID is already connected.
func (r *SessionRegistry) Register(playerID string, s *ClientSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sessions[playerID]; exists {
		return fmt.Errorf("player already connected")
	}
	r.sessions[playerID] = s
	return nil
}

// Unregister removes a session by playerID.
func (r *SessionRegistry) Unregister(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, playerID)
}

// List returns the display names of all connected players.
func (r *SessionRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.sessions))
	for _, s := range r.sessions {
		names = append(names, s.username)
	}
	return names
}

// Broadcast sends msg to every connected session.
func (r *SessionRegistry) Broadcast(msg ServerMsg) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ctx := context.Background()
	for _, s := range r.sessions {
		_ = writeMsg(ctx, s.conn, msg)
	}
}

// GetRoomID returns the current room ID for playerID, or "" if not found.
func (r *SessionRegistry) GetRoomID(playerID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.sessions[playerID]; ok && s.state != nil {
		return s.state.RoomID
	}
	return ""
}

// GetWorldName returns the world name for playerID, or "" if not found.
func (r *SessionRegistry) GetWorldName(playerID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.sessions[playerID]; ok {
		return s.worldName
	}
	return ""
}

// PlayersInWorld returns a PlayerInfo slice for sessions in the given world.
func (r *SessionRegistry) PlayersInWorld(worldName string, w *world.World) []PlayerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]PlayerInfo, 0)
	for _, s := range r.sessions {
		if s.worldName != worldName {
			continue
		}
		roomName := ""
		if s.state != nil && w != nil {
			if room := w.Room(s.state.RoomID); room != nil {
				roomName = room.Name
			}
		}
		result = append(result, PlayerInfo{Name: s.username, RoomName: roomName})
	}
	return result
}

// OnlinePlayersInWorld returns OnlinePlayerInfo for all sessions in worldName
// except excludeID (accountID). Sessions without a known room are omitted.
func (r *SessionRegistry) OnlinePlayersInWorld(worldName string, excludeID string) []OnlinePlayerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]OnlinePlayerInfo, 0)
	for id, s := range r.sessions {
		if id == excludeID || s.worldName != worldName {
			continue
		}
		if s.state == nil || s.state.RoomID == "" {
			continue
		}
		result = append(result, OnlinePlayerInfo{Name: s.username, RoomID: s.state.RoomID})
	}
	return result
}

// OnlineCountByWorld returns the number of connected sessions per world name.
func (r *SessionRegistry) OnlineCountByWorld() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[string]int)
	for _, s := range r.sessions {
		if s.worldName != "" {
			counts[s.worldName]++
		}
	}
	return counts
}

// BroadcastToRoomInWorld sends msg to every session in the given world and room.
func (r *SessionRegistry) BroadcastToRoomInWorld(worldName, roomID string, msg ServerMsg) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ctx := context.Background()
	for _, s := range r.sessions {
		if s.worldName == worldName && s.state != nil && s.state.RoomID == roomID {
			_ = writeMsg(ctx, s.conn, msg)
		}
	}
}

// SendToPlayerByName sends msg to the session matching username. Returns true if found.
func (r *SessionRegistry) SendToPlayerByName(username string, msg ServerMsg) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.sessions {
		if s.username == username {
			_ = writeMsg(context.Background(), s.conn, msg)
			return true
		}
	}
	return false
}

// BroadcastToWorld sends msg to every session in the given world.
func (r *SessionRegistry) BroadcastToWorld(worldName string, msg ServerMsg) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ctx := context.Background()
	for _, s := range r.sessions {
		if s.worldName == worldName {
			_ = writeMsg(ctx, s.conn, msg)
		}
	}
}

// SendToPlayer sends msg to the session for playerID. No-op if not connected.
func (r *SessionRegistry) SendToPlayer(playerID string, msg ServerMsg) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.sessions[playerID]; ok {
		ctx := context.Background()
		_ = writeMsg(ctx, s.conn, msg)
	}
}

// RegisterPendingRequest stores a request_id → playerID mapping via the callback.
func (r *SessionRegistry) RegisterPendingRequest(requestID, playerID string) {
	if r.onPendingRequest != nil {
		r.onPendingRequest(requestID, playerID)
	}
}

// KickPlayer closes the session for the player with the given username.
func (r *SessionRegistry) KickPlayer(username string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		if s.username == username {
			_ = writeMsg(context.Background(), s.conn, ServerMsg{
				Type:    "error",
				Payload: ErrorPayload{Message: "you have been kicked by an admin"},
			})
			s.conn.Close(websocket.StatusPolicyViolation, "kicked")
			delete(r.sessions, id)
			return
		}
	}
}

// TeleportPlayer moves the target player to the given room ID.
func (r *SessionRegistry) TeleportPlayer(username, roomID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.sessions {
		if s.username == username && s.state != nil {
			s.state.RoomID = roomID
			return
		}
	}
}

// GiveItem adds an item to the target player's inventory by item ID.
// It looks up the item in the player's world definition and adds it.
func (r *SessionRegistry) GiveItem(username, itemID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.sessions {
		if s.username == username && s.gdb != nil && s.world != nil {
			wi := s.world.FindItem(itemID)
			name := itemID
			desc := ""
			if wi != nil {
				name = wi.Name
				desc = wi.Desc
			}
			_ = s.gdb.AddItem(context.Background(), itemID, name, desc)
			return
		}
	}
}

// closeAll sends a shutdown message to every session and removes them.
func (r *SessionRegistry) closeAll(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		_ = writeMsg(ctx, s.conn, ServerMsg{
			Type:    "error",
			Payload: ErrorPayload{Message: "server shutting down"},
		})
		s.conn.Close(websocket.StatusGoingAway, "server shutting down")
		delete(r.sessions, id)
	}
}

// GameServer is the embedded multiplayer HTTP/WebSocket server.
type GameServer struct {
	worlds          map[string]*world.World
	lockedWorld     string
	registry        *SessionRegistry
	pgPool          *pgxpool.Pool // nil if no Postgres configured (solo mode)
	passphrase      string        // keep for backward compat when no Postgres
	httpServer      *http.Server
	lanURL          string
	busClient       *busd.Client
	pendingMu       sync.Mutex
	pendingRequests map[string]string // requestID → playerID
}

// New creates a GameServer supporting multiple worlds.
// If lockedWorld is non-empty, all connections are routed to that world regardless of query param.
// pgPool may be nil; when nil the server uses legacy playerID+passphrase auth.
func New(worlds map[string]*world.World, lockedWorld string, pgPool *pgxpool.Pool) *GameServer {
	gs := &GameServer{
		worlds:          worlds,
		lockedWorld:     lockedWorld,
		registry:        newSessionRegistry(),
		pgPool:          pgPool,
		pendingRequests: make(map[string]string),
	}
	gs.registry.onPendingRequest = func(rid, pid string) {
		gs.pendingMu.Lock()
		gs.pendingRequests[rid] = pid
		gs.pendingMu.Unlock()
	}
	return gs
}

// Start launches the HTTP listener in a background goroutine.
// Returns the LAN URL players should visit, or an error.
func (gs *GameServer) Start(port int, passphrase string) (string, error) {
	if gs.httpServer != nil {
		return gs.lanURL, nil // already running
	}
	gs.passphrase = passphrase

	// Connect to gl1tch event bus and subscribe to chat replies and gamification events.
	gs.busClient = busd.ConnectWithSubscriptions([]string{
		"mud.chat.reply",
		"game.achievement.unlocked",
		"game.top.reply",
		"game.achievements.reply",
	})
	gs.registry.busPub = gs.busClient.Publish
	go gs.busClient.Listen(func(ev busd.Event) {
		switch ev.Topic {
		case "mud.chat.reply":
			var p struct {
				Text  string `json:"text"`
				World string `json:"world"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err != nil || p.Text == "" {
				return
			}
			targetWorld := p.World
			if targetWorld == "" {
				// broadcast to all worlds if no world specified
				gs.registry.Broadcast(ServerMsg{
					Type:    "chat.message",
					Payload: ChatMessagePayload{From: "glitch", Text: p.Text},
				})
				return
			}
			gs.registry.BroadcastToWorld(targetWorld, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: "glitch", Text: p.Text},
			})

		case "game.achievement.unlocked":
			var p struct {
				Player      string `json:"player"`
				Name        string `json:"name"`
				Description string `json:"description"`
				XP          int    `json:"xp"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err != nil || p.Player == "" {
				return
			}
			text := fmt.Sprintf("achievement unlocked: %s", p.Name)
			if p.Description != "" {
				text += fmt.Sprintf("\n%s", p.Description)
			}
			if p.XP > 0 {
				text += fmt.Sprintf(" · +%dxp", p.XP)
			}
			gs.registry.SendToPlayer(p.Player, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: "glitch", Text: text},
			})

		case "game.top.reply":
			var p struct {
				RequestID string `json:"request_id"`
				Entries   []struct {
					Rank         int    `json:"rank"`
					Faction      string `json:"faction"`
					FactionScore int    `json:"faction_score"`
					Members      []struct {
						Name    string `json:"name"`
						Score   int    `json:"score"`
						IsAgent bool   `json:"agent"`
					} `json:"members"`
				} `json:"entries"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
				return
			}
			gs.pendingMu.Lock()
			playerID := gs.pendingRequests[p.RequestID]
			delete(gs.pendingRequests, p.RequestID)
			gs.pendingMu.Unlock()
			if playerID == "" {
				return
			}
			text := "── game top ──────────────────\n"
			text += fmt.Sprintf("  %-2s %-16s %6s  %s\n", "#", "FACTION", "SCORE", "MEMBERS")
			for _, e := range p.Entries {
				agents := 0
				for _, m := range e.Members {
					if m.IsAgent {
						agents++
					}
				}
				memberStr := fmt.Sprintf("%d", len(e.Members))
				if agents > 0 {
					memberStr += fmt.Sprintf(" (%d agent)", agents)
				}
				text += fmt.Sprintf("  %-2d %-16s %6d  %s\n", e.Rank, e.Faction, e.FactionScore, memberStr)
				for _, m := range e.Members {
					name := m.Name
					if m.IsAgent {
						name += " †"
					}
					text += fmt.Sprintf("    · %-16s %6d\n", name, m.Score)
				}
			}
			text += "  † = agent"
			gs.registry.SendToPlayer(playerID, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: "glitch", Text: text},
			})

		case "game.achievements.reply":
			var p struct {
				RequestID  string `json:"request_id"`
				Player     string `json:"player"`
				Unlocked   []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"unlocked"`
				InProgress []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					Progress int    `json:"progress"`
					Total    int    `json:"total"`
				} `json:"in_progress"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
				return
			}
			gs.pendingMu.Lock()
			playerID := gs.pendingRequests[p.RequestID]
			delete(gs.pendingRequests, p.RequestID)
			gs.pendingMu.Unlock()
			if playerID == "" {
				return
			}
			text := "── your achievements ─────────\n"
			for _, u := range p.Unlocked {
				text += fmt.Sprintf("  ✓ %-16s — %s\n", u.Name, u.Description)
			}
			for _, ip := range p.InProgress {
				text += fmt.Sprintf("    %-16s — (%d/%d)\n", ip.Name, ip.Progress, ip.Total)
			}
			if len(p.Unlocked) == 0 && len(p.InProgress) == 0 {
				text += "  no achievements yet"
			}
			gs.registry.SendToPlayer(playerID, ServerMsg{
				Type:    "chat.message",
				Payload: ChatMessagePayload{From: "glitch", Text: text},
			})
		}
	})

	// Register achievement catalog with gamification daemon (best-effort).
	go func() {
		cf, err := achievements.Load("achievements.yaml")
		if err != nil {
			// No catalog file — skip registration silently.
			return
		}
		type triggerPayload struct {
			Action string `json:"action"`
			Count  int    `json:"count"`
		}
		type achPayload struct {
			ID          string         `json:"id"`
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Trigger     triggerPayload `json:"trigger"`
			XP          int            `json:"xp"`
		}
		achs := make([]achPayload, len(cf.Achievements))
		for i, a := range cf.Achievements {
			achs[i] = achPayload{
				ID:          a.ID,
				Name:        a.Name,
				Description: a.Description,
				XP:          a.XP,
				Trigger:     triggerPayload{Action: a.Trigger.Action, Count: a.Trigger.Count},
			}
		}
		gs.busClient.Publish("game.catalog.register", map[string]any{
			"source":       cf.Source,
			"version":      cf.Version,
			"achievements": achs,
		})
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/worlds", gs.handleWorlds)
	mux.HandleFunc("/ws", gs.handleWS)
	mux.Handle("/", FileHandler())

	gs.httpServer = &http.Server{Handler: mux}

	addr := fmt.Sprintf(":%d", port)

	// Listen on IPv4 explicitly; require it to succeed.
	ln4, err := net.Listen("tcp4", addr)
	if err != nil {
		gs.httpServer = nil
		return "", err
	}

	// Also listen on IPv6 so that "localhost" works whether it resolves to
	// 127.0.0.1 or ::1. Failure is non-fatal (IPv6 may be disabled).
	ln6, _ := net.Listen("tcp6", addr)

	ip := lanIP()
	gs.lanURL = fmt.Sprintf("http://%s:%d", ip, port)

	go gs.httpServer.Serve(ln4) //nolint:errcheck
	if ln6 != nil {
		go gs.httpServer.Serve(ln6) //nolint:errcheck
	}

	// Start idle timeout watcher.
	go gs.idleWatcher()

	// Start NPC/resource respawn ticker for shared worlds.
	go gs.respawnTicker()

	return gs.lanURL, nil
}

// Stop gracefully shuts down the server and disconnects all sessions.
func (gs *GameServer) Stop() {
	if gs.httpServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gs.registry.closeAll(ctx)
	_ = gs.httpServer.Shutdown(ctx)
	gs.httpServer = nil
	gs.lanURL = ""
	if gs.busClient != nil {
		gs.busClient.Close()
		gs.busClient = nil
	}
}

// IsRunning reports whether the server is active.
func (gs *GameServer) IsRunning() bool {
	return gs.httpServer != nil
}

// LanURL returns the current LAN URL or empty string if not running.
func (gs *GameServer) LanURL() string {
	return gs.lanURL
}

// ConnectedPlayers returns the list of connected player names.
func (gs *GameServer) ConnectedPlayers() []string {
	return gs.registry.List()
}

// Broadcast sends msg to every connected session.
func (gs *GameServer) Broadcast(msg ServerMsg) {
	gs.registry.Broadcast(msg)
}

// broadcastPlayerListForWorld sends a players.update message to all sessions in the given world.
func (gs *GameServer) broadcastPlayerListForWorld(worldName string) {
	wld := gs.worlds[worldName]
	gs.registry.BroadcastToWorld(worldName, ServerMsg{
		Type: "players.update",
		Payload: PlayersUpdatePayload{
			HostOnline: true,
			Players:    gs.registry.PlayersInWorld(worldName, wld),
		},
	})
}

// worldForRequest resolves a world from the given name parameter.
// If the server has a lockedWorld, that world is always returned regardless of name.
// For multi-world servers, name must be non-empty and refer to a known world.
func (gs *GameServer) worldForRequest(name string) (*world.World, error) {
	if gs.lockedWorld != "" {
		w, ok := gs.worlds[gs.lockedWorld]
		if !ok {
			return nil, fmt.Errorf("locked world %q not found", gs.lockedWorld)
		}
		return w, nil
	}
	if name == "" {
		return nil, fmt.Errorf("world param required")
	}
	w, ok := gs.worlds[name]
	if !ok {
		return nil, fmt.Errorf("unknown world: %q", name)
	}
	return w, nil
}

// handleWorlds returns JSON list of WorldMeta for all available worlds.
func (gs *GameServer) handleWorlds(w http.ResponseWriter, r *http.Request) {
	onlineCounts := gs.registry.OnlineCountByWorld()
	metas := make([]world.WorldMeta, 0, len(gs.worlds))
	for _, wld := range gs.worlds {
		mode := wld.Mode
		if mode == "" {
			mode = "solo"
		}
		metas = append(metas, world.WorldMeta{
			Name:    wld.Name,
			Tagline: wld.UI.Tagline,
			Mode:    mode,
			Online:  onlineCounts[wld.Name],
			Theme:   wld.UI.Theme,
		})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metas); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// buildMapRooms converts a world's rooms into MapRoomInfo slices for the
// world_meta payload. Rooms with no BFS-assigned coordinates (both GridX and
// GridY are zero and the room is not the start room) are excluded to avoid
// sending misleading positions for rooms unreachable from the start.
func buildMapRooms(w *world.World) []MapRoomInfo {
	result := make([]MapRoomInfo, 0, len(w.Rooms))
	for _, r := range w.Rooms {
		if r.GridX == 0 && r.GridY == 0 && r.ID != w.StartRoom {
			continue
		}
		result = append(result, MapRoomInfo{
			ID:    r.ID,
			Name:  r.Name,
			Biome: r.Biome,
			X:     r.GridX,
			Y:     r.GridY,
		})
	}
	return result
}

// handleWS upgrades an HTTP connection to WebSocket, performs the auth
// handshake, then hands off to the session handler.
func (gs *GameServer) handleWS(w http.ResponseWriter, r *http.Request) {
	worldName := r.URL.Query().Get("world")
	selectedWorld, err := gs.worldForRequest(worldName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // LAN-only; no origin check needed
	})
	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Auth handshake — first message must be auth, login, or resume.
	_, data, err := conn.Read(ctx)
	if err != nil {
		return
	}

	var first ClientMsg
	if err := json.Unmarshal(data, &first); err != nil {
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: "auth required"},
		})
		conn.Close(websocket.StatusPolicyViolation, "auth required")
		return
	}

	var accountID, username, role, token string

	switch {
	// ── Postgres login ──────────────────────────────────────────────────
	case first.Type == "login" && gs.pgPool != nil:
		var lp LoginPayload
		if err := json.Unmarshal(first.Payload, &lp); err != nil || lp.Username == "" {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "invalid login payload"},
			})
			conn.Close(websocket.StatusPolicyViolation, "invalid payload")
			return
		}

		account, err := pgq.New(gs.pgPool).GetAccountByUsername(ctx, lp.Username)
		if err != nil {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "unknown username"},
			})
			conn.Close(websocket.StatusPolicyViolation, "unknown username")
			return
		}
		if account.Banned {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "account banned"},
			})
			conn.Close(websocket.StatusPolicyViolation, "banned")
			return
		}
		if !auth.CheckPassword(lp.Password, account.PasswordHash) {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "invalid password"},
			})
			conn.Close(websocket.StatusPolicyViolation, "invalid password")
			return
		}

		tok, err := auth.GenerateToken()
		if err != nil {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "internal error"},
			})
			conn.Close(websocket.StatusInternalError, "token gen failed")
			return
		}

		_, err = pgq.New(gs.pgPool).CreateSession(ctx, pgq.CreateSessionParams{
			AccountID: account.ID,
			Token:     tok,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
		})
		if err != nil {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "internal error"},
			})
			conn.Close(websocket.StatusInternalError, "session create failed")
			return
		}

		accountID = formatUUID(account.ID.Bytes)
		username = account.Username
		role = account.Role
		token = tok

	// ── Postgres resume ─────────────────────────────────────────────────
	case first.Type == "resume" && gs.pgPool != nil:
		var rp ResumePayload
		if err := json.Unmarshal(first.Payload, &rp); err != nil || rp.Token == "" {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "invalid resume payload"},
			})
			conn.Close(websocket.StatusPolicyViolation, "invalid payload")
			return
		}

		sess, err := pgq.New(gs.pgPool).GetSession(ctx, rp.Token)
		if err != nil {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "session expired or invalid"},
			})
			conn.Close(websocket.StatusPolicyViolation, "invalid session")
			return
		}
		if sess.Banned {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "account banned"},
			})
			conn.Close(websocket.StatusPolicyViolation, "banned")
			return
		}

		// Extend session expiry by 7 days.
		_ = pgq.New(gs.pgPool).TouchSession(ctx, pgq.TouchSessionParams{
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
			Token:     rp.Token,
		})

		accountID = formatUUID(sess.AccountID.Bytes)
		username = sess.Username
		role = sess.Role
		token = rp.Token

	// ── Legacy passphrase auth (no Postgres) ────────────────────────────
	case first.Type == "auth":
		var ap AuthPayload
		if err := json.Unmarshal(first.Payload, &ap); err != nil {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "invalid auth payload"},
			})
			conn.Close(websocket.StatusPolicyViolation, "invalid payload")
			return
		}

		if err := ValidatePlayerID(ap.PlayerID); err != nil {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: err.Error()},
			})
			conn.Close(websocket.StatusPolicyViolation, "invalid playerID")
			return
		}

		if !ValidatePassphrase(ap.Passphrase, gs.passphrase) {
			_ = writeMsg(ctx, conn, ServerMsg{
				Type:    "auth.fail",
				Payload: AuthFailPayload{Reason: "invalid passphrase"},
			})
			conn.Close(websocket.StatusPolicyViolation, "invalid passphrase")
			return
		}

		accountID = ap.PlayerID
		username = ap.PlayerID
		role = "player"

	default:
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: "auth required"},
		})
		conn.Close(websocket.StatusPolicyViolation, "auth required")
		return
	}

	session := &ClientSession{
		accountID:    accountID,
		username:     username,
		role:         role,
		conn:         conn,
		world:        selectedWorld,
		worldName:    selectedWorld.Name,
		pgPool:       gs.pgPool,
		cancel:       cancel,
		lastActivity: time.Now(),
		registry:     gs.registry,
	}

	if err := gs.registry.Register(accountID, session); err != nil {
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: err.Error()},
		})
		conn.Close(websocket.StatusPolicyViolation, "already connected")
		return
	}
	defer gs.broadcastPlayerListForWorld(session.worldName) // registered first → runs second (after Close removes session)
	defer session.Close()                                   // registered second → runs first

	authOK := AuthOKPayload{Level: 1, Title: "Script Kiddie", XP: 0}
	if token != "" {
		authOK.AccountID = accountID
		authOK.Username = username
		authOK.Token = token
		authOK.Role = role
	} else {
		authOK.PlayerID = username
	}
	_ = writeMsg(ctx, conn, ServerMsg{Type: "auth.ok", Payload: authOK})

	mapRooms := buildMapRooms(selectedWorld)
	worldMode := selectedWorld.Mode
	if worldMode == "" {
		worldMode = "solo"
	}
	_ = writeMsg(ctx, conn, ServerMsg{
		Type: "world_meta",
		Payload: WorldMetaPayload{
			Name:      selectedWorld.Name,
			Tagline:   selectedWorld.UI.Tagline,
			Mode:      worldMode,
			Theme:     selectedWorld.UI.Theme,
			UIProfile: selectedWorld.UI.Profile,
			MapRooms:  mapRooms,
		},
	})

	// Notify all clients (including new joiner) of updated roster.
	gs.broadcastPlayerListForWorld(session.worldName)

	session.Handle(ctx)
}

// idleWatcher closes sessions that haven't sent a message in 30 minutes.
func (gs *GameServer) idleWatcher() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if !gs.IsRunning() {
			return
		}
		deadline := time.Now().Add(-30 * time.Minute)
		gs.registry.mu.Lock()
		for id, s := range gs.registry.sessions {
			if s.lastActivity.Before(deadline) {
				_ = writeMsg(context.Background(), s.conn, ServerMsg{
					Type:    "error",
					Payload: ErrorPayload{Message: "session timeout"},
				})
				s.conn.Close(websocket.StatusGoingAway, "session timeout")
				delete(gs.registry.sessions, id)
			}
		}
		gs.registry.mu.Unlock()
	}
}

// formatUUID converts a [16]byte UUID to its string representation.
func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// lanIP returns the machine's first non-loopback IPv4 address.
func lanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			s := ip4.String()
			if !strings.HasPrefix(s, "169.254") { // skip link-local
				return s
			}
		}
	}
	return "localhost"
}
