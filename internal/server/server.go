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

	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// SessionRegistry tracks active WebSocket sessions by playerID.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*ClientSession
	busPub   func(topic string, payload any)
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

// List returns the names of all connected players.
func (r *SessionRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.sessions))
	for id := range r.sessions {
		names = append(names, id)
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
	for id, s := range r.sessions {
		if s.worldName != worldName {
			continue
		}
		roomName := ""
		if s.state != nil && w != nil {
			if room := w.Room(s.state.RoomID); room != nil {
				roomName = room.Name
			}
		}
		result = append(result, PlayerInfo{Name: id, RoomName: roomName})
	}
	return result
}

// OnlinePlayersInWorld returns OnlinePlayerInfo for all sessions in worldName
// except excludeID. Sessions without a known room are omitted.
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
		result = append(result, OnlinePlayerInfo{Name: id, RoomID: s.state.RoomID})
	}
	return result
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
	worlds      map[string]*world.World
	lockedWorld string
	registry    *SessionRegistry
	passphrase  string
	httpServer  *http.Server
	lanURL      string
	busClient   *busd.Client
}

// New creates a GameServer supporting multiple worlds.
// If lockedWorld is non-empty, all connections are routed to that world regardless of query param.
func New(worlds map[string]*world.World, lockedWorld string) *GameServer {
	return &GameServer{
		worlds:      worlds,
		lockedWorld: lockedWorld,
		registry:    newSessionRegistry(),
	}
}

// Start launches the HTTP listener in a background goroutine.
// Returns the LAN URL players should visit, or an error.
func (gs *GameServer) Start(port int, passphrase string) (string, error) {
	if gs.httpServer != nil {
		return gs.lanURL, nil // already running
	}
	gs.passphrase = passphrase

	// Connect to gl1tch event bus and subscribe to chat replies.
	gs.busClient = busd.ConnectWithSubscriptions([]string{"mud.chat.reply"})
	gs.registry.busPub = gs.busClient.Publish
	go gs.busClient.Listen(func(ev busd.Event) {
		if ev.Topic != "mud.chat.reply" {
			return
		}
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
	})

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
	metas := make([]world.WorldMeta, 0, len(gs.worlds))
	for _, wld := range gs.worlds {
		metas = append(metas, world.WorldMeta{
			Name:    wld.Name,
			Tagline: wld.UI.Tagline,
			Theme:   wld.UI.Theme,
		})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metas); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
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

	// Auth handshake — first message must be {"type":"auth",...}
	_, data, err := conn.Read(ctx)
	if err != nil {
		return
	}

	var first ClientMsg
	if err := json.Unmarshal(data, &first); err != nil || first.Type != "auth" {
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: "auth required"},
		})
		conn.Close(websocket.StatusPolicyViolation, "auth required")
		return
	}

	var auth AuthPayload
	if err := json.Unmarshal(first.Payload, &auth); err != nil {
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: "invalid auth payload"},
		})
		conn.Close(websocket.StatusPolicyViolation, "invalid payload")
		return
	}

	if err := ValidatePlayerID(auth.PlayerID); err != nil {
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: err.Error()},
		})
		conn.Close(websocket.StatusPolicyViolation, "invalid playerID")
		return
	}

	if !ValidatePassphrase(auth.Passphrase, gs.passphrase) {
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: "invalid passphrase"},
		})
		conn.Close(websocket.StatusPolicyViolation, "invalid passphrase")
		return
	}

	session := &ClientSession{
		playerID:     auth.PlayerID,
		conn:         conn,
		world:        selectedWorld,
		worldName:    selectedWorld.Name,
		cancel:       cancel,
		lastActivity: time.Now(),
		registry:     gs.registry,
	}

	if err := gs.registry.Register(auth.PlayerID, session); err != nil {
		_ = writeMsg(ctx, conn, ServerMsg{
			Type:    "auth.fail",
			Payload: AuthFailPayload{Reason: err.Error()},
		})
		conn.Close(websocket.StatusPolicyViolation, "already connected")
		return
	}
	defer gs.broadcastPlayerListForWorld(session.worldName) // registered first → runs second (after Close removes session)
	defer session.Close()                                   // registered second → runs first

	_ = writeMsg(ctx, conn, ServerMsg{
		Type:    "auth.ok",
		Payload: AuthOKPayload{PlayerID: auth.PlayerID, Level: 1, Title: "Script Kiddie", XP: 0},
	})

	mapRooms := make([]MapRoomInfo, 0, len(selectedWorld.Rooms))
	for _, r := range selectedWorld.Rooms {
		mapRooms = append(mapRooms, MapRoomInfo{
			ID:    r.ID,
			Name:  r.Name,
			Biome: r.Biome,
			X:     r.GridX,
			Y:     r.GridY,
		})
	}
	_ = writeMsg(ctx, conn, ServerMsg{
		Type: "world_meta",
		Payload: WorldMetaPayload{
			Name:      selectedWorld.Name,
			Tagline:   selectedWorld.UI.Tagline,
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
