package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nhooyr.io/websocket"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func makeTestWorlds() map[string]*world.World {
	return map[string]*world.World{
		"alpha": {
			Name: "alpha",
			UI: world.WorldUI{
				Tagline: "alpha tagline",
				Theme:   world.WorldTheme{Accent: "#ff0000"},
			},
		},
		"beta": {
			Name: "beta",
			UI: world.WorldUI{
				Tagline: "beta tagline",
				Theme:   world.WorldTheme{Accent: "#0000ff"},
			},
		},
	}
}

func TestAPIWorldsReturnsAllWorlds(t *testing.T) {
	gs := &GameServer{worlds: makeTestWorlds(), registry: newSessionRegistry()}
	req := httptest.NewRequest(http.MethodGet, "/api/worlds", nil)
	rr := httptest.NewRecorder()
	gs.handleWorlds(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}

	var metas []world.WorldMeta
	if err := json.NewDecoder(rr.Body).Decode(&metas); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("metas count: got %d want 2", len(metas))
	}
	names := map[string]bool{}
	for _, m := range metas {
		names[m.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("missing world in response: %v", metas)
	}
}

func TestWorldForRequestMultiWorld(t *testing.T) {
	gs := &GameServer{worlds: makeTestWorlds(), registry: newSessionRegistry()}
	w, err := gs.worldForRequest("alpha")
	if err != nil {
		t.Fatalf("known world: %v", err)
	}
	if w.Name != "alpha" {
		t.Errorf("world name: got %q want alpha", w.Name)
	}
}

func TestWorldForRequestUnknownReturnsError(t *testing.T) {
	gs := &GameServer{worlds: makeTestWorlds(), registry: newSessionRegistry()}
	_, err := gs.worldForRequest("nonexistent")
	if err == nil {
		t.Error("expected error for unknown world, got nil")
	}
}

func TestWorldForRequestEmptyParamMultiWorldReturnsError(t *testing.T) {
	gs := &GameServer{worlds: makeTestWorlds(), registry: newSessionRegistry()}
	_, err := gs.worldForRequest("")
	if err == nil {
		t.Error("expected error for empty world param on multi-world server")
	}
}

func TestWorldForRequestLockedWorldIgnoresParam(t *testing.T) {
	gs := &GameServer{
		worlds:      makeTestWorlds(),
		lockedWorld: "alpha",
		registry:    newSessionRegistry(),
	}
	w, err := gs.worldForRequest("beta") // param says beta, but locked to alpha
	if err != nil {
		t.Fatalf("locked world: %v", err)
	}
	if w.Name != "alpha" {
		t.Errorf("locked world: got %q want alpha", w.Name)
	}
}

func TestWorldForRequestLockedWorldEmptyParam(t *testing.T) {
	gs := &GameServer{
		worlds:      makeTestWorlds(),
		lockedWorld: "alpha",
		registry:    newSessionRegistry(),
	}
	w, err := gs.worldForRequest("")
	if err != nil {
		t.Fatalf("locked world empty param: %v", err)
	}
	if w.Name != "alpha" {
		t.Errorf("got %q want alpha", w.Name)
	}
}

func TestWSHandlerUnknownWorldReturns400(t *testing.T) {
	gs := &GameServer{worlds: makeTestWorlds(), registry: newSessionRegistry()}
	req := httptest.NewRequest(http.MethodGet, "/ws?world=unknown", nil)
	rr := httptest.NewRecorder()
	gs.handleWS(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want 400", rr.Code)
	}
}

func TestRegistryPlayersInWorld(t *testing.T) {
	r := newSessionRegistry()
	r.sessions["player1"] = &ClientSession{worldName: "alpha"}
	r.sessions["player2"] = &ClientSession{worldName: "beta"}

	alphaWorld := &world.World{Name: "alpha"}
	players := r.PlayersInWorld("alpha", alphaWorld)

	if len(players) != 1 {
		t.Fatalf("players in alpha: got %d want 1", len(players))
	}
	if players[0].Name != "player1" {
		t.Errorf("player name: got %q want player1", players[0].Name)
	}
}

func TestNewLockedWorldMode(t *testing.T) {
	worlds := makeTestWorlds()
	gs := New(worlds, "alpha")
	if gs.lockedWorld != "alpha" {
		t.Errorf("lockedWorld: got %q want alpha", gs.lockedWorld)
	}
	// With lockedWorld set, worldForRequest ignores any param.
	w, err := gs.worldForRequest("beta")
	if err != nil {
		t.Fatalf("locked mode should not error: %v", err)
	}
	if w.Name != "alpha" {
		t.Errorf("locked mode returned %q, want alpha", w.Name)
	}
}

func TestWSConnectsToKnownWorldReturns101(t *testing.T) {
	gs := &GameServer{worlds: makeTestWorlds(), registry: newSessionRegistry()}

	srv := httptest.NewServer(http.HandlerFunc(gs.handleWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?world=alpha"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("upgrade status: got %d want 101", resp.StatusCode)
	}
}

func TestWSConnectsAndReceivesWorldMeta(t *testing.T) {
	gs := &GameServer{worlds: makeTestWorlds(), registry: newSessionRegistry()}

	srv := httptest.NewServer(http.HandlerFunc(gs.handleWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?world=alpha"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send auth
	authMsg, _ := json.Marshal(ClientMsg{
		Type:    "auth",
		Payload: json.RawMessage(`{"playerID":"testplayer","passphrase":""}`),
	})
	if err := conn.Write(ctx, websocket.MessageText, authMsg); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	// Read messages until we find world_meta (skip auth.ok and players.update)
	gotWorldMeta := false
	for i := 0; i < 5; i++ {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}
		var msg ServerMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == "world_meta" {
			gotWorldMeta = true
			// Re-marshal and unmarshal payload into WorldMetaPayload
			var payload WorldMetaPayload
			payloadBytes, _ := json.Marshal(msg.Payload)
			if err := json.Unmarshal(payloadBytes, &payload); err == nil {
				if payload.Name != "alpha" {
					t.Errorf("world_meta name: got %q want alpha", payload.Name)
				}
				if payload.Tagline != "alpha tagline" {
					t.Errorf("world_meta tagline: got %q want 'alpha tagline'", payload.Tagline)
				}
			}
			break
		}
	}
	if !gotWorldMeta {
		t.Error("expected world_meta message on connect, didn't receive one")
	}
}

