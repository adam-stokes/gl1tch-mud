# World Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a world selection lobby to the text and web interfaces, replacing hardcoded `"cyberspace"` in `main.go`, so each world declares its own UI config and players can choose a world on startup.

**Architecture:** `world.yaml` gains a `ui:` block (banner, prompt, tagline, theme colors) parsed into a new `WorldUI` struct. `server.New()` accepts `map[string]*world.World` enabling `/api/worlds` and `/ws?world=` routing. The web UI splits into a lobby (`index.astro`) and a game page (`game.astro`); `theme.ts` applies world colors as CSS vars on connect. The text interface gains `selectWorld()` and reads banner/prompt from `w.UI`.

**Tech Stack:** Go 1.25, gopkg.in/yaml.v3, nhooyr.io/websocket, Astro 6, TypeScript, Vitest + jsdom

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/world/world.go` | Modify | Add `WorldTheme`, `WorldUI`, `WorldMeta` structs; `UI WorldUI` field on `World`; `ListAvailable() []WorldMeta` |
| `internal/world/world_test.go` | Modify | Tests for WorldUI parsing, ListAvailable, prompt/banner fallbacks |
| `internal/world/defaults/cyberspace/world.yaml` | Modify | Add `ui:` block |
| `internal/world/defaults/blockhaven/world.yaml` | Modify | Add `ui:` block |
| `worlds/cyberspace/world.yaml` | Modify | Add `ui:` block (kept in sync with embedded default) |
| `worlds/blockhaven/world.yaml` | Modify | Add `ui:` block (kept in sync with embedded default) |
| `internal/server/protocol.go` | Modify | Add `WorldMetaPayload` |
| `internal/server/server.go` | Modify | `New(worlds map, lockedWorld)`, `worldForRequest`, `/api/worlds`, `/ws?world=`, world-scoped broadcast |
| `internal/server/session.go` | Modify | Add `worldName` field; use `PlayersInWorld` |
| `internal/server/server_test.go` | Create | Tests for `/api/worlds`, `worldForRequest`, `/ws?world=unknown` 400 |
| `main.go` | Modify | `--world` flag, `selectWorld()`, pass world to `runGame`/`runServe`, use `w.UI.*` |
| `web/package.json` | Modify | Add `vitest` + `jsdom` dev deps |
| `web/vitest.config.ts` | Create | Vitest config with jsdom environment |
| `web/src/lib/theme.ts` | Create | `applyTheme(meta: WorldMeta)` — sets CSS vars on document root |
| `web/src/lib/theme.test.ts` | Create | Tests for `applyTheme` |
| `web/src/lib/mud.ts` | Modify | `buildWsUrl`, `setWorld`, handle `world_meta` message |
| `web/src/lib/mud.test.ts` | Create | Tests for `buildWsUrl` |
| `web/src/pages/index.astro` | Modify | Rewrite as lobby: fetch `/api/worlds`, render world cards |
| `web/src/pages/game.astro` | Create | Game page: reads `?world=` from URL, calls `setWorld()` + `initMUD()` |

---

## Task 1: WorldUI struct + ListAvailable (TDD)

**Files:**
- Modify: `internal/world/world.go`
- Modify: `internal/world/world_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/world/world_test.go` before the existing `TestParseMinimal`:

```go
const worldWithUI = `
name: testworld
start_room: r0
narrator_model: test
ui:
  banner: "TEST BANNER"
  prompt: "#"
  tagline: "test tagline"
  theme:
    bg: "#000000"
    fg: "#ffffff"
    accent: "#ff0000"
    dim: "#333333"
    border: "#444444"
    error: "#ff5555"
    success: "#00ff00"
rooms:
  - id: r0
    name: "Start"
    desc: "Beginning."
    exits: {}
`

const worldNoUI = `
name: noui
start_room: r0
narrator_model: test
rooms:
  - id: r0
    name: "Start"
    desc: "."
    exits: {}
`

func TestWorldUIFullParse(t *testing.T) {
	var w World
	if err := yaml.Unmarshal([]byte(worldWithUI), &w); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if w.UI.Banner != "TEST BANNER" {
		t.Errorf("banner: got %q want %q", w.UI.Banner, "TEST BANNER")
	}
	if w.UI.Prompt != "#" {
		t.Errorf("prompt: got %q want %q", w.UI.Prompt, "#")
	}
	if w.UI.Tagline != "test tagline" {
		t.Errorf("tagline: got %q want %q", w.UI.Tagline, "test tagline")
	}
	if w.UI.Theme.BG != "#000000" {
		t.Errorf("theme.bg: got %q want %q", w.UI.Theme.BG, "#000000")
	}
	if w.UI.Theme.Accent != "#ff0000" {
		t.Errorf("theme.accent: got %q want %q", w.UI.Theme.Accent, "#ff0000")
	}
}

func TestWorldUIFallbacks(t *testing.T) {
	var w World
	if err := yaml.Unmarshal([]byte(worldNoUI), &w); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := w.UIPrompt(); got != ">" {
		t.Errorf("UIPrompt: got %q want %q", got, ">")
	}
	if got := w.UIBanner(); got != "" {
		t.Errorf("UIBanner: got %q want empty", got)
	}
}

func TestListAvailable(t *testing.T) {
	metas := ListAvailable()
	if len(metas) == 0 {
		t.Fatal("ListAvailable returned empty slice")
	}
	found := false
	for _, m := range metas {
		if m.Name == "cyberspace" {
			found = true
		}
	}
	if !found {
		t.Error("ListAvailable should always include cyberspace")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -run 'TestWorldUI|TestListAvailable' -v 2>&1 | head -30
```

Expected: FAIL — `UIPrompt`, `UIBanner`, `ListAvailable`, `WorldUI` undefined.

- [ ] **Step 3: Add WorldTheme, WorldUI, WorldMeta, UI field, UIPrompt(), UIBanner(), ListAvailable()**

In `internal/world/world.go`, add after the `WeatherEntry` struct (before `LootTable`):

```go
// WorldTheme holds the color palette for a world's UI.
type WorldTheme struct {
	BG      string `yaml:"bg"      json:"bg,omitempty"`
	FG      string `yaml:"fg"      json:"fg,omitempty"`
	Accent  string `yaml:"accent"  json:"accent,omitempty"`
	Dim     string `yaml:"dim"     json:"dim,omitempty"`
	Border  string `yaml:"border"  json:"border,omitempty"`
	Error   string `yaml:"error"   json:"error,omitempty"`
	Success string `yaml:"success" json:"success,omitempty"`
}

// WorldUI holds the presentation config for a world, read from the ui: block.
type WorldUI struct {
	Banner  string     `yaml:"banner"`
	Prompt  string     `yaml:"prompt"`
	Tagline string     `yaml:"tagline"`
	Theme   WorldTheme `yaml:"theme"`
}

// WorldMeta is a lightweight summary of a world used in lobby listings.
type WorldMeta struct {
	Name    string     `json:"name"`
	Tagline string     `json:"tagline"`
	Theme   WorldTheme `json:"theme"`
}
```

Add `UI WorldUI` to the `World` struct after `WeatherTable`:

```go
type World struct {
	Name            string           `yaml:"name"`
	StartRoom       string           `yaml:"start_room"`
	NarratorModel   string           `yaml:"narrator_model"`
	Rooms           []Room           `yaml:"rooms"`
	CraftingRecipes []CraftingRecipe `yaml:"crafting_recipes,omitempty"`
	LootTables      []LootTable      `yaml:"loot_tables,omitempty"`
	Factions        []Faction        `yaml:"factions,omitempty"`
	Quests          []WorldQuest     `yaml:"quests,omitempty"`
	WeatherTable    []WeatherEntry   `yaml:"weather_table,omitempty"`
	UI              WorldUI          `yaml:"ui"`
	index           map[string]*Room
}
```

Add after the existing `Available()` function:

```go
// UIPrompt returns the world's prompt string, falling back to ">".
func (w *World) UIPrompt() string {
	if w.UI.Prompt != "" {
		return w.UI.Prompt
	}
	return ">"
}

// UIBanner returns the world's banner string (may be empty).
func (w *World) UIBanner() string {
	return w.UI.Banner
}

// ListAvailable returns WorldMeta for all installed worlds plus embedded defaults.
// Ordering matches Available() — embedded defaults first, then user-installed.
func ListAvailable() []WorldMeta {
	names := Available()
	metas := make([]WorldMeta, 0, len(names))
	for _, name := range names {
		w, err := Load(name)
		if err != nil {
			continue
		}
		metas = append(metas, WorldMeta{
			Name:    w.Name,
			Tagline: w.UI.Tagline,
			Theme:   w.UI.Theme,
		})
	}
	return metas
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -run 'TestWorldUI|TestListAvailable' -v
```

Expected: PASS for `TestWorldUIFullParse`, `TestWorldUIFallbacks`, `TestListAvailable`.

- [ ] **Step 5: Run full world test suite**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/world/world.go internal/world/world_test.go
git commit -m "feat(world): add WorldUI, WorldMeta, UIPrompt/UIBanner, ListAvailable"
```

---

## Task 2: Add ui: blocks to world YAMLs

**Files:**
- Modify: `internal/world/defaults/cyberspace/world.yaml`
- Modify: `internal/world/defaults/blockhaven/world.yaml`
- Modify: `worlds/cyberspace/world.yaml`
- Modify: `worlds/blockhaven/world.yaml`

- [ ] **Step 1: Add ui: block to cyberspace**

In both `internal/world/defaults/cyberspace/world.yaml` AND `worlds/cyberspace/world.yaml`, add immediately after the `name: cyberspace` line:

```yaml
ui:
  banner: |
     ██████  ██      ██ ████████  ██████ ██   ██       ███    ███ ██    ██ ██████
    ██       ██      ██    ██    ██      ██   ██       ████  ████ ██    ██ ██   ██
    ██   ███ ██      ██    ██    ██      ███████ █████ ██ ████ ██ ██    ██ ██   ██
    ██    ██ ██      ██    ██    ██      ██   ██       ██  ██  ██ ██    ██ ██   ██
     ██████  ███████ ██    ██     ██████ ██   ██       ██      ██  ██████  ██████
  prompt: ">"
  tagline: "jack in. ghost the gibson. don't get traced."
  theme:
    bg: "#282a36"
    fg: "#f8f8f2"
    accent: "#bd93f9"
    dim: "#6272a4"
    border: "#44475a"
    error: "#ff5555"
    success: "#50fa7b"
```

- [ ] **Step 2: Add ui: block to blockhaven**

In both `internal/world/defaults/blockhaven/world.yaml` AND `worlds/blockhaven/world.yaml`, add immediately after `name: blockhaven`:

```yaml
ui:
  banner: |
    ██████╗ ██╗      ██████╗  ██████╗██╗  ██╗██╗  ██╗ █████╗ ██╗   ██╗███████╗███╗   ██╗
    ██╔══██╗██║     ██╔═══██╗██╔════╝██║ ██╔╝██║  ██║██╔══██╗██║   ██║██╔════╝████╗  ██║
    ██████╔╝██║     ██║   ██║██║     █████╔╝ ███████║███████║██║   ██║█████╗  ██╔██╗ ██║
    ██╔══██╗██║     ██║   ██║██║     ██╔═██╗ ██╔══██║██╔══██║╚██╗ ██╔╝██╔══╝  ██║╚██╗██║
    ██████╔╝███████╗╚██████╔╝╚██████╗██║  ██╗██║  ██║██║  ██║ ╚████╔╝ ███████╗██║ ╚████║
  prompt: "$"
  tagline: "the ruins remember everything."
  theme:
    bg: "#1a1a2e"
    fg: "#e0d7c6"
    accent: "#c9a84c"
    dim: "#6b5d4f"
    border: "#3d3228"
    error: "#c0392b"
    success: "#27ae60"
```

- [ ] **Step 3: Update TestListAvailable to check tagline**

In `internal/world/world_test.go`, update `TestListAvailable`:

```go
func TestListAvailable(t *testing.T) {
	metas := ListAvailable()
	if len(metas) == 0 {
		t.Fatal("ListAvailable returned empty slice")
	}
	found := false
	for _, m := range metas {
		if m.Name == "cyberspace" {
			found = true
			if m.Tagline == "" {
				t.Error("cyberspace should have a non-empty tagline after adding ui: block")
			}
			if m.Theme.Accent == "" {
				t.Error("cyberspace should have a non-empty theme accent")
			}
		}
	}
	if !found {
		t.Error("ListAvailable should always include cyberspace")
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/world/defaults/cyberspace/world.yaml \
        internal/world/defaults/blockhaven/world.yaml \
        worlds/cyberspace/world.yaml \
        worlds/blockhaven/world.yaml \
        internal/world/world_test.go
git commit -m "feat(worlds): add ui: blocks to cyberspace and blockhaven world.yaml"
```

---

## Task 3: Server — multi-world routing (TDD)

**Files:**
- Modify: `internal/server/protocol.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/session.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/server/server_test.go`:

```go
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
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/server/... -v 2>&1 | head -40
```

Expected: compile errors — `handleWorlds`, `worldForRequest`, `lockedWorld`, `PlayersInWorld`, `worldName` undefined.

- [ ] **Step 3: Add WorldMetaPayload to protocol.go**

In `internal/server/protocol.go`, add `"github.com/adam-stokes/gl1tch-mud/internal/world"` to the import block and add after `InputPayload`:

```go
// WorldMetaPayload is sent to the client on WebSocket connect, before the first state.update.
type WorldMetaPayload struct {
	Name    string           `json:"name"`
	Tagline string           `json:"tagline"`
	Theme   world.WorldTheme `json:"theme"`
}
```

- [ ] **Step 4: Update server.go**

**4a.** Replace the `GameServer` struct and `New` function:

```go
// GameServer is the embedded multiplayer HTTP/WebSocket server.
type GameServer struct {
	worlds      map[string]*world.World
	lockedWorld string // non-empty when started with --world; only that world is served
	registry    *SessionRegistry
	passphrase  string
	httpServer  *http.Server
	lanURL      string
}

// New creates a GameServer for the given worlds map.
// lockedWorld is non-empty for single-world (--world flag) mode; empty for lobby mode.
func New(worlds map[string]*world.World, lockedWorld string) *GameServer {
	return &GameServer{
		worlds:      worlds,
		lockedWorld: lockedWorld,
		registry:    newSessionRegistry(),
	}
}
```

**4b.** Add `"sort"` to the import block in `server.go`.

**4c.** Add `worldForRequest` and `handleWorlds` methods before `handleWS`:

```go
// worldForRequest resolves which world a WebSocket request targets.
// In locked mode the query param is ignored and the locked world is always returned.
// In multi-world mode the "world" query param is required; unknown names return an error.
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

// handleWorlds serves GET /api/worlds — returns a JSON array of WorldMeta sorted by name.
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
```

**4d.** In `Start()`, add the `/api/worlds` route. Find:

```go
mux.HandleFunc("/ws", gs.handleWS)
mux.Handle("/", FileHandler())
```

Replace with:

```go
mux.HandleFunc("/api/worlds", gs.handleWorlds)
mux.HandleFunc("/ws", gs.handleWS)
mux.Handle("/", FileHandler())
```

**4e.** Update `handleWS` to resolve the world BEFORE upgrading the WebSocket. Replace the start of `handleWS` up to `websocket.Accept`:

```go
func (gs *GameServer) handleWS(w http.ResponseWriter, r *http.Request) {
	// Resolve world before upgrading — allows returning clean HTTP error codes.
	worldName := r.URL.Query().Get("world")
	selectedWorld, err := gs.worldForRequest(worldName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
```

**4f.** In the session construction block inside `handleWS`, replace `world: gs.world` with:

```go
session := &ClientSession{
	playerID:     auth.PlayerID,
	conn:         conn,
	world:        selectedWorld,
	worldName:    selectedWorld.Name,
	cancel:       cancel,
	lastActivity: time.Now(),
	registry:     gs.registry,
}
```

**4g.** After the `auth.ok` writeMsg, add:

```go
_ = writeMsg(ctx, conn, ServerMsg{
	Type: "world_meta",
	Payload: WorldMetaPayload{
		Name:    selectedWorld.Name,
		Tagline: selectedWorld.UI.Tagline,
		Theme:   selectedWorld.UI.Theme,
	},
})
```

**4h.** Replace both calls to `gs.broadcastPlayerList()` with `gs.broadcastPlayerListForWorld(session.worldName)`.

**4i.** Remove the old `broadcastPlayerList()` method and the old `Players(w *world.World)` method on `SessionRegistry`. Remove the `world *world.World` and `worldMu sync.RWMutex` fields from `GameServer`.

**4j.** Add the new `broadcastPlayerListForWorld`, `PlayersInWorld`, and `BroadcastToWorld` methods:

```go
// broadcastPlayerListForWorld sends a players.update only to sessions in the given world.
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

// PlayersInWorld returns a PlayerInfo slice for sessions in the named world only.
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

// BroadcastToWorld sends msg to every session in the named world.
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
```

- [ ] **Step 5: Update session.go**

Add `worldName string` to `ClientSession`:

```go
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
```

In `sendStateUpdate`, replace `s.registry.Players(s.world)` with `s.registry.PlayersInWorld(s.worldName, s.world)`.

- [ ] **Step 6: Run server tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/server/... -v
```

Expected: all PASS.

- [ ] **Step 7: Build check (expect main.go compile errors — fixed next task)**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./internal/... 2>&1
```

Expected: clean. `main.go` will fail `go build ./...` until Task 4.

- [ ] **Step 8: Commit**

```bash
git add internal/server/protocol.go internal/server/server.go \
        internal/server/session.go internal/server/server_test.go
git commit -m "feat(server): multi-world routing — New(worlds map), /api/worlds, /ws?world=, world_meta"
```

---

## Task 4: main.go — --world flag, selectWorld(), use w.UI.*

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Update main.go**

Replace the entire file with:

```go
package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/db"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/server"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

//go:embed all:web/dist
var webDist embed.FS

func main() {
	serveMode := flag.Bool("serve", false, "run LAN server only")
	servePort := flag.Int("port", 8080, "server port")
	servePass := flag.String("passphrase", "", "session passphrase")
	worldFlag := flag.String("world", "", "world to load (skips selection screen)")
	flag.Parse()

	if *serveMode {
		runServe(*servePort, *servePass, *worldFlag)
		return
	}

	worldName := *worldFlag
	if worldName == "" {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			worldName = selectWorld()
		} else {
			worldName = "cyberspace"
		}
	}

	runGame(worldName)
}

// selectWorld prints an interactive numbered world menu and returns the chosen world name.
func selectWorld() string {
	metas := world.ListAvailable()
	if len(metas) == 0 {
		return "cyberspace"
	}
	if len(metas) == 1 {
		return metas[0].Name
	}

	fmt.Println("\n  available worlds:\n")
	for i, m := range metas {
		tagline := m.Tagline
		if tagline == "" {
			tagline = "—"
		}
		fmt.Printf("  [%d] %-16s — %s\n", i+1, m.Name, tagline)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n  > ")
		if !scanner.Scan() {
			return metas[0].Name
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if n, err := strconv.Atoi(input); err == nil {
			if n >= 1 && n <= len(metas) {
				return metas[n-1].Name
			}
		}
		for _, m := range metas {
			if strings.EqualFold(input, m.Name) {
				return m.Name
			}
		}
		fmt.Printf("  invalid selection %q — enter a number (1-%d) or world name\n", input, len(metas))
	}
}

// loadAllWorlds loads every available world. When lockedWorld is non-empty, only that world is loaded.
func loadAllWorlds(lockedWorld string) (map[string]*world.World, error) {
	if lockedWorld != "" {
		w, err := world.Load(lockedWorld)
		if err != nil {
			return nil, err
		}
		return map[string]*world.World{lockedWorld: w}, nil
	}
	names := world.Available()
	worlds := make(map[string]*world.World, len(names))
	for _, name := range names {
		w, err := world.Load(name)
		if err != nil {
			return nil, fmt.Errorf("load world %q: %w", name, err)
		}
		worlds[name] = w
	}
	return worlds, nil
}

// runServe starts the HTTP/WebSocket server and blocks until SIGINT/SIGTERM.
func runServe(port int, passphrase, lockedWorld string) {
	sub, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: embed:", err)
		os.Exit(1)
	}
	server.SetFS(sub)

	worlds, err := loadAllWorlds(lockedWorld)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: world:", err)
		os.Exit(1)
	}

	srv := server.New(worlds, lockedWorld)
	if _, err := srv.Start(port, passphrase); err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: serve:", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	srv.Stop()
}

// runGame runs the interactive game loop for the named world.
func runGame(worldName string) {
	database, err := db.OpenForWorld(worldName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: db:", err)
		os.Exit(1)
	}
	defer database.Close()

	w, err := world.Load(worldName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: world:", err)
		os.Exit(1)
	}

	s, err := player.Load(database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: player:", err)
		os.Exit(1)
	}

	world.SeedCrystalShards(database, s.World)  //nolint:errcheck
	world.SeedStartingItems(database, s.World)  //nolint:errcheck

	bus := busd.Connect()
	defer bus.Close()

	bus.Publish("mud.session.started", map[string]any{
		"player":  s.Name,
		"room_id": s.RoomID,
		"world":   s.World,
	})

	commands.SetBinary(executablePath())

	lanSrv := server.New(map[string]*world.World{w.Name: w}, w.Name)
	commands.SetLANServer(lanSrv)

	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	if interactive {
		if banner := w.UIBanner(); banner != "" {
			fmt.Println(banner)
		}
		if tagline := w.UI.Tagline; tagline != "" {
			fmt.Printf("  %s\n", tagline)
		}
		fmt.Println("  type 'help' for commands. type '/lan' to start a multiplayer session.")

		res := commands.Look(database, s, w, nil)
		fmt.Println(res.Output)
		if res.Event != nil {
			bus.Publish(res.Event.Topic, res.Event.Payload)
		}
	}

	prompt := w.UIPrompt() + " "
	scanner := bufio.NewScanner(os.Stdin)
	for {
		if interactive {
			fmt.Print(prompt)
		}
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "quit" || line == "exit" || line == "q" {
			bus.Publish("mud.session.ended", map[string]any{
				"player":  s.Name,
				"room_id": s.RoomID,
			})
			if interactive {
				fmt.Println("disconnecting.")
			}
			break
		}

		line = strings.TrimPrefix(line, "/")

		verb, args := commands.Parse(line)
		handler, ok := commands.Registry[verb]
		if !ok {
			fmt.Printf("unknown command: %q — type 'help' for a list.\n", verb)
			continue
		}

		result := handler(database, s, w, args)
		fmt.Println(result.Output)
		if result.Event != nil {
			bus.Publish(result.Event.Topic, result.Event.Payload)
		}
		if result.SwitchWorld != "" {
			newDB, swErr := db.OpenForWorld(result.SwitchWorld)
			if swErr != nil {
				fmt.Fprintf(os.Stderr, "world switch: %v\n", swErr)
			} else {
				database.Close()
				database = newDB
				newWorld, swErr := world.Load(result.SwitchWorld)
				if swErr != nil {
					fmt.Fprintf(os.Stderr, "world switch: %v\n", swErr)
				} else {
					w = newWorld
					lanSrv.Stop()
					lanSrv = server.New(map[string]*world.World{w.Name: w}, w.Name)
					commands.SetLANServer(lanSrv)
					prompt = w.UIPrompt() + " "
					newState, _ := player.LoadForWorld(database, result.SwitchWorld, w.StartRoom)
					*s = *newState
					if w.Room(s.RoomID) == nil {
						s.RoomID = w.StartRoom
						s.World = result.SwitchWorld
						player.Save(database, s) //nolint:errcheck
					}
					world.SeedCrystalShards(database, s.World)  //nolint:errcheck
					world.SeedStartingItems(database, s.World)  //nolint:errcheck
					lookResult := commands.Look(database, s, w, nil)
					fmt.Println(lookResult.Output)
				}
			}
		}
	}
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "gl1tch-mud"
	}
	return exe
}
```

- [ ] **Step 2: Full build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```

Expected: clean build.

- [ ] **Step 3: Smoke test --world flag (non-interactive)**

```bash
cd /Users/stokes/Projects/gl1tch-mud && echo "quit" | ./gl1tch-mud --world blockhaven 2>&1 | head -5
```

Expected: no selection menu; blockhaven banner or room output appears; exits cleanly.

- [ ] **Step 4: Run all Go tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat(main): --world flag, selectWorld() text lobby, world UI for banner/prompt"
```

---

## Task 5: Web — Vitest setup

**Files:**
- Modify: `web/package.json`
- Create: `web/vitest.config.ts`

- [ ] **Step 1: Install vitest**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm install --save-dev vitest jsdom @vitest/coverage-v8
```

- [ ] **Step 2: Add test scripts to package.json**

In `web/package.json`, add to the `"scripts"` object:

```json
"test": "vitest run",
"test:watch": "vitest"
```

- [ ] **Step 3: Create vitest.config.ts**

```typescript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
  },
});
```

- [ ] **Step 4: Verify setup**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm test 2>&1 | tail -5
```

Expected: exits 0, "No test files found" or similar — no errors.

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud
git add web/package.json web/package-lock.json web/vitest.config.ts
git commit -m "test(web): add vitest + jsdom test infrastructure"
```

---

## Task 6: Web — theme.ts + tests

**Files:**
- Create: `web/src/lib/theme.ts`
- Create: `web/src/lib/theme.test.ts`

- [ ] **Step 1: Write failing tests**

Create `web/src/lib/theme.test.ts`:

```typescript
import { describe, it, expect, beforeEach } from 'vitest';
import { applyTheme } from './theme';
import type { WorldMeta } from './theme';

describe('applyTheme', () => {
  beforeEach(() => {
    document.documentElement.removeAttribute('style');
  });

  it('sets CSS custom properties from a full theme', () => {
    const meta: WorldMeta = {
      name: 'testworld',
      tagline: 'test',
      theme: {
        bg: '#000000',
        fg: '#ffffff',
        accent: '#ff0000',
        dim: '#333333',
        border: '#444444',
        error: '#ff5555',
        success: '#00ff00',
      },
    };
    applyTheme(meta);
    const s = document.documentElement.style;
    expect(s.getPropertyValue('--bg')).toBe('#000000');
    expect(s.getPropertyValue('--fg')).toBe('#ffffff');
    expect(s.getPropertyValue('--accent')).toBe('#ff0000');
    expect(s.getPropertyValue('--error')).toBe('#ff5555');
    expect(s.getPropertyValue('--success')).toBe('#00ff00');
  });

  it('skips empty theme values', () => {
    const meta: WorldMeta = {
      name: 'minimal',
      tagline: '',
      theme: { bg: '#111111', fg: '', accent: '', dim: '', border: '', error: '', success: '' },
    };
    applyTheme(meta);
    const s = document.documentElement.style;
    expect(s.getPropertyValue('--bg')).toBe('#111111');
    expect(s.getPropertyValue('--fg')).toBe('');
  });

  it('does not throw with empty theme object', () => {
    const meta: WorldMeta = {
      name: 'empty',
      tagline: '',
      theme: { bg: '', fg: '', accent: '', dim: '', border: '', error: '', success: '' },
    };
    expect(() => applyTheme(meta)).not.toThrow();
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm test 2>&1 | tail -10
```

Expected: FAIL — `./theme` not found.

- [ ] **Step 3: Create theme.ts**

Create `web/src/lib/theme.ts`:

```typescript
export interface WorldTheme {
  bg: string;
  fg: string;
  accent: string;
  dim: string;
  border: string;
  error: string;
  success: string;
}

export interface WorldMeta {
  name: string;
  tagline: string;
  theme: WorldTheme;
}

/**
 * Applies a world's theme to the document root as CSS custom properties.
 * Only non-empty values are written to avoid clobbering existing defaults.
 */
export function applyTheme(meta: WorldMeta): void {
  const root = document.documentElement;
  const t = meta.theme;
  const pairs: [string, string][] = [
    ['--bg',      t.bg],
    ['--fg',      t.fg],
    ['--accent',  t.accent],
    ['--dim',     t.dim],
    ['--border',  t.border],
    ['--error',   t.error],
    ['--success', t.success],
  ];
  for (const [prop, val] of pairs) {
    if (val) {
      root.style.setProperty(prop, val);
    }
  }
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm test 2>&1
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud
git add web/src/lib/theme.ts web/src/lib/theme.test.ts
git commit -m "feat(web): add theme.ts with applyTheme() and tests"
```

---

## Task 7: Web — update mud.ts + tests

**Files:**
- Modify: `web/src/lib/mud.ts`
- Create: `web/src/lib/mud.test.ts`

- [ ] **Step 1: Write failing test**

Create `web/src/lib/mud.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { buildWsUrl } from './mud';

describe('buildWsUrl', () => {
  it('constructs ws:// URL with world param', () => {
    const url = buildWsUrl('ws:', 'localhost:8080', 'cyberspace');
    expect(url).toBe('ws://localhost:8080/ws?world=cyberspace');
  });

  it('constructs wss:// URL for https', () => {
    const url = buildWsUrl('wss:', 'example.com', 'blockhaven');
    expect(url).toBe('wss://example.com/ws?world=blockhaven');
  });

  it('percent-encodes world names with spaces', () => {
    const url = buildWsUrl('ws:', 'localhost', 'my world');
    expect(url).toBe('ws://localhost/ws?world=my%20world');
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm test 2>&1 | tail -10
```

Expected: FAIL — `buildWsUrl` not exported from `./mud`.

- [ ] **Step 3: Update mud.ts**

**3a.** Add `_worldName` module variable and `setWorld` export. After the `let _myPlayerID = '';` line, add:

```typescript
let _worldName = 'cyberspace';

/** Set the world name before calling initMUD. Called by game.astro. */
export function setWorld(name: string): void {
  _worldName = name;
}
```

**3b.** Add `buildWsUrl` export before `initMUD`. Add before the `export function initMUD()` line:

```typescript
/**
 * Builds the WebSocket URL for the given world.
 * Exported for testing.
 */
export function buildWsUrl(protocol: string, host: string, worldName: string): string {
  return `${protocol}//${host}/ws?world=${encodeURIComponent(worldName)}`;
}
```

**3c.** Inside `initMUD`, in the `connect()` function, find:

```typescript
const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
ws = new WebSocket(`${proto}//${location.host}/ws`);
```

Replace with:

```typescript
const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
ws = new WebSocket(buildWsUrl(proto, location.host, _worldName));
```

**3d.** In `handleServerMsg` inside `initMUD`, add a `world_meta` case. Find the `switch (msg.type)` block and add before `case 'auth.ok':`:

```typescript
case 'world_meta':
  import('./theme').then(({ applyTheme }) => {
    applyTheme(msg.payload as import('./theme').WorldMeta);
  });
  break;
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm test 2>&1
```

Expected: all tests PASS (theme + mud).

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud
git add web/src/lib/mud.ts web/src/lib/mud.test.ts
git commit -m "feat(web): buildWsUrl, setWorld, world_meta handler in mud.ts"
```

---

## Task 8: Web — lobby (index.astro)

**Files:**
- Modify: `web/src/pages/index.astro`

- [ ] **Step 1: Rewrite index.astro as lobby**

Replace the entire content of `web/src/pages/index.astro`:

```astro
---
---

<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>gl1tch-mud</title>
    <style>
      *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

      :root {
        --bg:      #282a36;
        --bg-dark: #1e1f29;
        --fg:      #f8f8f2;
        --comment: #6272a4;
        --purple:  #bd93f9;
        --red:     #ff5555;
        --border:  #44475a;
      }

      html, body {
        height: 100%;
        background: var(--bg);
        color: var(--fg);
        font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
        display: flex;
        align-items: center;
        justify-content: center;
      }

      #lobby {
        width: 100%;
        max-width: 640px;
        padding: 2rem 1.5rem;
      }

      h1 {
        font-size: 1.4rem;
        color: var(--purple);
        letter-spacing: 0.12em;
        margin-bottom: 0.3rem;
      }

      .subtitle {
        font-size: 0.72rem;
        color: var(--comment);
        margin-bottom: 2rem;
        letter-spacing: 0.06em;
      }

      #world-list {
        display: flex;
        flex-direction: column;
        gap: 0.75rem;
      }

      .world-card {
        display: block;
        background: var(--bg-dark);
        border: 1px solid var(--border);
        padding: 1rem 1.25rem;
        text-decoration: none;
        color: inherit;
        cursor: pointer;
        transition: border-color 0.15s, box-shadow 0.15s;
        border-radius: 3px;
      }

      .world-card:hover {
        border-color: var(--card-accent, var(--purple));
        box-shadow: 0 0 16px rgba(0,0,0,0.4);
      }

      .world-name {
        font-size: 1rem;
        font-weight: bold;
        letter-spacing: 0.06em;
        margin-bottom: 0.25rem;
      }

      .world-tagline {
        font-size: 0.72rem;
        color: var(--comment);
      }

      #status-msg {
        font-size: 0.8rem;
        color: var(--comment);
      }

      #error-msg {
        font-size: 0.75rem;
        color: var(--red);
      }
    </style>
  </head>
  <body>
    <div id="lobby">
      <h1>gl1tch-mud</h1>
      <p class="subtitle">select a world to enter</p>
      <div id="world-list">
        <p id="status-msg">loading worlds...</p>
      </div>
      <p id="error-msg"></p>
    </div>

    <script>
      (async () => {
        const list      = document.getElementById('world-list');
        const statusMsg = document.getElementById('status-msg');
        const errorMsg  = document.getElementById('error-msg');

        let worlds: Array<{ name: string; tagline: string; theme?: { accent?: string } }> = [];

        try {
          const res = await fetch('/api/worlds');
          if (!res.ok) throw new Error(`server returned ${res.status}`);
          worlds = await res.json();
        } catch (e: any) {
          statusMsg!.remove();
          errorMsg!.textContent = `failed to load worlds: ${e.message}`;
          return;
        }

        statusMsg!.remove();

        if (worlds.length === 0) {
          const p = document.createElement('p');
          p.style.color = 'var(--comment)';
          p.textContent = 'no worlds available.';
          list!.appendChild(p);
          return;
        }

        // Single world: skip the lobby and navigate directly.
        if (worlds.length === 1) {
          window.location.replace(`/game?world=${encodeURIComponent(worlds[0].name)}`);
          return;
        }

        for (const w of worlds) {
          const card = document.createElement('a');
          card.className = 'world-card';
          card.href = `/game?world=${encodeURIComponent(w.name)}`;
          if (w.theme?.accent) {
            card.style.setProperty('--card-accent', w.theme.accent);
          }

          const nameEl = document.createElement('div');
          nameEl.className = 'world-name';
          nameEl.style.color = w.theme?.accent ?? 'var(--purple)';
          nameEl.textContent = w.name;

          const tagEl = document.createElement('div');
          tagEl.className = 'world-tagline';
          tagEl.textContent = w.tagline || '—';

          card.appendChild(nameEl);
          card.appendChild(tagEl);
          list!.appendChild(card);
        }
      })();
    </script>
  </body>
</html>
```

- [ ] **Step 2: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm run build 2>&1 | tail -5
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud
git add web/src/pages/index.astro
git commit -m "feat(web): rewrite index.astro as world selection lobby"
```

---

## Task 9: Web — game.astro (game page)

**Files:**
- Create: `web/src/pages/game.astro`

- [ ] **Step 1: Create game.astro**

The game page contains the full HUD (all CSS + HTML previously in `index.astro`) plus the new `setWorld` + `initMUD` wiring. Create `web/src/pages/game.astro` — copy all the CSS from the original `index.astro` (before Task 8 rewrote it; use `git show HEAD~1:web/src/pages/index.astro` to retrieve it) and add the updated `<script>` block below.

The `<script>` tag at the bottom of `game.astro` should be:

```astro
<script>
  import { setWorld, initMUD } from '../lib/mud.ts';

  const params = new URLSearchParams(window.location.search);
  const worldName = params.get('world');

  if (!worldName) {
    // No world param — redirect to lobby.
    window.location.replace('/');
  } else {
    const nameEl    = document.getElementById('login-world-name');
    const topbarEl  = document.getElementById('topbar-world');
    if (nameEl)   nameEl.textContent   = worldName;
    if (topbarEl) topbarEl.textContent = worldName;

    setWorld(worldName);
    initMUD();
  }
</script>
```

Also update the login card title element (was `<h1>gl1tch-mud</h1>`) to have an id:

```html
<h1 id="login-world-name">gl1tch-mud</h1>
```

And the topbar logo span (was `<span class="logo">gl1tch</span>`):

```html
<span class="logo" id="topbar-world">gl1tch</span>
```

- [ ] **Step 2: Build web**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && npm run build 2>&1 | tail -5
```

Expected: clean build. Verify output has both pages:

```bash
ls /Users/stokes/Projects/gl1tch-mud/web/dist/
ls /Users/stokes/Projects/gl1tch-mud/web/dist/game/
```

Expected: `index.html` at root, `game/index.html` exists.

- [ ] **Step 3: Full build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && make build 2>&1 | tail -5
```

Expected: clean build.

- [ ] **Step 4: Run all tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1
cd /Users/stokes/Projects/gl1tch-mud/web && npm test 2>&1
```

Expected: all PASS.

- [ ] **Step 5: Integration smoke test**

```bash
cd /Users/stokes/Projects/gl1tch-mud && ./gl1tch-mud --serve --port 9999 &
sleep 1
# Lobby returns world cards
curl -s http://localhost:9999/api/worlds | python3 -m json.tool
# Unknown world returns 400
curl -s -o /dev/null -w "%{http_code}" "http://localhost:9999/ws?world=unknown"
echo ""
kill %1 2>/dev/null; wait 2>/dev/null
```

Expected:
- `/api/worlds` → JSON array with `cyberspace` and `blockhaven`
- `/ws?world=unknown` → `400`

- [ ] **Step 6: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud
git add web/src/pages/game.astro web/dist
git commit -m "feat(web): add game.astro — world-aware HUD page reading ?world= param"
```

---

## Self-Review

**Spec coverage:**
- `world.yaml` gains `ui:` block with banner, prompt, tagline, theme — Tasks 1–2 ✅
- `WorldUI`, `WorldMeta`, `ListAvailable()` in world package — Task 1 ✅
- `UIPrompt()` fallback `">"`, `UIBanner()` fallback `""` — Task 1 ✅
- Text lobby `selectWorld()` — numbered menu, name input, re-prompt — Task 4 ✅
- `--world` flag skips selector — Task 4 ✅
- Banner/prompt/tagline from `w.UI.*` in game loop — Task 4 ✅
- `server.New(worlds map, lockedWorld)` — Task 3 ✅
- `GET /api/worlds` sorted JSON — Task 3 ✅
- `GET /ws?world=<name>` routing — Task 3 ✅
- `GET /ws?world=unknown` → HTTP 400 before WebSocket upgrade — Task 3 ✅
- `GET /ws` empty param on multi-world → 400 — Task 3 ✅
- `GET /ws` empty param on locked → uses locked world — Task 3 ✅
- `world_meta` message on connect — Task 3 ✅
- Players isolated by world (`PlayersInWorld`, `BroadcastToWorld`) — Task 3 ✅
- Lobby fetches `/api/worlds`, renders world cards with accent color — Task 8 ✅
- Single world → auto-redirect, no selection — Task 8 ✅
- `game.astro` reads `?world=`, redirects to `/` if missing — Task 9 ✅
- `theme.ts` `applyTheme()` sets CSS vars, skips empty values — Task 6 ✅
- `world_meta` triggers `applyTheme` — Task 7 ✅
- `buildWsUrl` constructs correct URL with encoded world param — Task 7 ✅
- Full Go test coverage for new functions — Tasks 1, 3 ✅
- Full TypeScript test coverage for `applyTheme`, `buildWsUrl` — Tasks 6, 7 ✅

**No placeholders.** All steps include complete code.

**Type consistency:**
- `WorldMeta{Name, Tagline, Theme}` defined in `world.go` — same fields used in `server.go` `handleWorlds` ✅
- `WorldMetaPayload{Name, Tagline, Theme world.WorldTheme}` in `protocol.go` — matches `WorldMeta` shape ✅
- `WorldTheme` yaml+json tags: `bg`, `fg`, `accent`, `dim`, `border`, `error`, `success` — match `theme.ts` `WorldTheme` interface ✅
- `buildWsUrl(protocol, host, worldName)` exported from `mud.ts` — called as `buildWsUrl(proto, location.host, _worldName)` ✅
- `setWorld(name: string)` exported from `mud.ts` — called in `game.astro` as `setWorld(worldName)` ✅
- `server.New(worlds map[string]*world.World, lockedWorld string)` — all 3 call sites in `main.go` updated ✅
- `session.worldName` set in `handleWS` as `selectedWorld.Name` — read in `PlayersInWorld` and `BroadcastToWorld` ✅
