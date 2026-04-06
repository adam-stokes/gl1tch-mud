package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nhooyr.io/websocket"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
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
	r.sessions["player1"] = &ClientSession{accountID: "player1", username: "player1", worldName: "alpha"}
	r.sessions["player2"] = &ClientSession{accountID: "player2", username: "player2", worldName: "beta"}

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
	gs := New(worlds, "alpha", nil)
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

func TestStateUpdatePayloadRoomFieldsJSON(t *testing.T) {
	p := StateUpdatePayload{
		HP:       10,
		MaxHP:    10,
		RoomName: "Town Square",
		Exits:    []string{"north"},
		RoomNPCs: []RoomNPCInfo{
			{ID: "elder-mason", Name: "Elder Mason", CanTalk: true, CanTrade: false, Attackable: false},
		},
		RoomResources: []RoomResourceInfo{
			{ID: "limestone-vein", Name: "limestone-vein", Action: "mine"},
		},
		Quests: []QuestInfo{
			{ID: "q1", Title: "Find the Map", ObjCount: 1, ObjProgress: 0},
		},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := out["room_npcs"]; !ok {
		t.Error("expected room_npcs key in JSON")
	}
	if _, ok := out["room_resources"]; !ok {
		t.Error("expected room_resources key in JSON")
	}
	if _, ok := out["quests"]; !ok {
		t.Error("expected quests key in JSON")
	}
}

func TestWorldMetaPayloadUIProfileJSON(t *testing.T) {
	p := WorldMetaPayload{
		Name:      "blockhaven",
		Tagline:   "the ruins remember everything.",
		UIProfile: "kids",
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["ui_profile"] != "kids" {
		t.Errorf("ui_profile: got %v want %q", out["ui_profile"], "kids")
	}
}

func TestOnlinePlayersInWorldExcludesSelf(t *testing.T) {
	reg := newSessionRegistry()
	reg.sessions["alice"] = &ClientSession{accountID: "alice", username: "alice", worldName: "bh", state: &player.State{RoomID: "room-1"}}
	reg.sessions["bob"]   = &ClientSession{accountID: "bob",   username: "bob",   worldName: "bh", state: &player.State{RoomID: "room-2"}}
	reg.sessions["carol"] = &ClientSession{accountID: "carol", username: "carol", worldName: "other", state: &player.State{RoomID: "room-x"}}

	result := reg.OnlinePlayersInWorld("bh", "alice")
	if len(result) != 1 {
		t.Fatalf("want 1 player got %d: %+v", len(result), result)
	}
	if result[0].Name != "bob" || result[0].RoomID != "room-2" {
		t.Errorf("unexpected player info: %+v", result[0])
	}
}

func TestOnlinePlayersInWorldSkipsNoRoom(t *testing.T) {
	reg := newSessionRegistry()
	// nil state — excluded
	reg.sessions["ghost"]  = &ClientSession{accountID: "ghost",  username: "ghost",  worldName: "bh", state: nil}
	// empty RoomID — excluded
	reg.sessions["newbie"] = &ClientSession{accountID: "newbie", username: "newbie", worldName: "bh", state: &player.State{RoomID: ""}}

	result := reg.OnlinePlayersInWorld("bh", "other")
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d: %+v", len(result), result)
	}
}

func TestStateUpdatePayloadEquippedArmor(t *testing.T) {
	armor := &EquippedArmorInfo{
		ItemID:   "leather-armor",
		ItemName: "Leather Armor",
		Defense:  3,
	}
	p := StateUpdatePayload{
		HP:            50,
		MaxHP:         100,
		EquippedArmor: armor,
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	armorField, ok := out["equipped_armor"]
	if !ok {
		t.Fatal("equipped_armor field missing from payload")
	}
	armorMap, ok := armorField.(map[string]any)
	if !ok {
		t.Fatal("equipped_armor is not a map")
	}
	if armorMap["item_id"] != "leather-armor" {
		t.Errorf("item_id: got %v want %q", armorMap["item_id"], "leather-armor")
	}
	if armorMap["defense"] != float64(3) {
		t.Errorf("defense: got %v want 3", armorMap["defense"])
	}
}
