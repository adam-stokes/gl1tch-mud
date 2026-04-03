package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)


// SessionRegistry tracks active WebSocket sessions by playerID.
type SessionRegistry struct {
	mu      sync.RWMutex
	sessions map[string]*ClientSession
}

func newSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: make(map[string]*ClientSession)}
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
	world      *world.World
	worldMu    sync.RWMutex
	registry   *SessionRegistry
	passphrase string
	httpServer *http.Server
	lanURL     string
}

// New creates a GameServer sharing the given world instance.
func New(w *world.World) *GameServer {
	return &GameServer{
		world:    w,
		registry: newSessionRegistry(),
	}
}

// Start launches the HTTP listener in a background goroutine.
// Returns the LAN URL players should visit, or an error.
func (gs *GameServer) Start(port int, passphrase string) (string, error) {
	if gs.httpServer != nil {
		return gs.lanURL, nil // already running
	}
	gs.passphrase = passphrase

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gs.handleWS)
	mux.Handle("/", FileHandler())

	gs.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	ip := lanIP()
	gs.lanURL = fmt.Sprintf("http://%s:%d", ip, port)

	errCh := make(chan error, 1)
	go func() {
		if err := gs.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Give the listener a moment to fail fast (port in use, etc.)
	select {
	case err := <-errCh:
		gs.httpServer = nil
		return "", err
	case <-time.After(50 * time.Millisecond):
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

// handleWS upgrades an HTTP connection to WebSocket, performs the auth
// handshake, then hands off to the session handler.
func (gs *GameServer) handleWS(w http.ResponseWriter, r *http.Request) {
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
		world:        gs.world,
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
	defer session.Close()

	_ = writeMsg(ctx, conn, ServerMsg{
		Type:    "auth.ok",
		Payload: AuthOKPayload{PlayerID: auth.PlayerID, Level: 1, Title: "Script Kiddie", XP: 0},
	})

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
