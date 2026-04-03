package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
