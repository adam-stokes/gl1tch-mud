# Blockhaven World Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add multi-world support and a Minecraft-style Blockhaven world with mining, harvesting, building, enchanting, weather, and Minecraft-style death/respawn.

**Architecture:** New world YAML fields (biome, resources, weather_table) and 9 new DB tables extend the existing schema additively. New mechanics live in focused packages (`internal/weather`, `internal/enchanting`) and new command files (`mining.go`, `building.go`). World switching is handled via a new `SwitchWorld` field on `Result` that `main.go` acts on after each command.

**Tech Stack:** Go 1.25, modernc.org/sqlite, gopkg.in/yaml.v3. Module: `github.com/adam-stokes/gl1tch-mud`. All tests use `sql.Open("sqlite", ":memory:")`.

---

## File Map

| Action | File | Purpose |
|---|---|---|
| Modify | `internal/db/schema.go` | Add 9 new tables |
| Modify | `internal/db/db.go` | Add `OpenForWorld(worldName)` |
| Modify | `internal/world/world.go` | Add `Resource`, `WeatherEntry` types; `Biome`+`Resources` on `Room`; `WeatherTable` on `World`; `Available()` func |
| Modify | `internal/player/player.go` | Add `RemoveItem`, `DumpToDeathPile`, `GetDeathPile`, `ClaimDeathPile`, `AddEnchantingXP`, `EnchantingLevel` |
| Modify | `internal/commands/commands.go` | Add `SwitchWorld string` to `Result`; extend death in `Attack()`; inject death pile in `Look()` |
| Modify | `internal/espionage/espionage.go` | Add `AllShardsCollected bool` to `PlayerContext`; add `has_all_shards` case in `matchTrigger` |
| Modify | `main.go` | Handle `result.SwitchWorld`; use `db.OpenForWorld` on startup |
| Create | `internal/weather/weather.go` | Weather tick, condition query, yield bonus |
| Create | `internal/weather/weather_test.go` | Tests for weather logic |
| Create | `internal/enchanting/enchanting.go` | Enchant catalog, apply, list for item |
| Create | `internal/enchanting/enchanting_test.go` | Tests for enchant logic |
| Create | `internal/commands/world.go` | `world list`, `world switch` commands |
| Create | `internal/commands/mining.go` | `mine`, `harvest`, `gather`, `smelt`, `plant` commands |
| Create | `internal/commands/building.go` | `build`, `stash`, `unstash` commands |
| Create | `internal/commands/enchant_cmd.go` | `enchant` command |
| Create | `internal/commands/weather_cmd.go` | `weather` command |
| Create | `internal/commands/deathpile_cmd.go` | `deathpile` command |
| Create | `worlds/blockhaven/world.yaml` | Full Blockhaven world |
| Create | `worlds/blockhaven/story-bible.md` | Canonical narrative reference |
| Create | `worlds/blockhaven/world-state.yaml` | Pipeline idempotency tracker |

---

## Task 1: DB Schema — Add 9 New Tables

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Append tables to schema const**

Open `internal/db/schema.go`. The `schema` const ends with the `hideout_upgrades` table and closing backtick. Add these 9 tables immediately before the closing backtick:

```sql
CREATE TABLE IF NOT EXISTS room_resources (
    room_id             TEXT    NOT NULL,
    resource_id         TEXT    NOT NULL,
    depleted            INTEGER NOT NULL DEFAULT 0,
    depleted_at_action  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (room_id, resource_id)
);

CREATE TABLE IF NOT EXISTS weather_state (
    biome       TEXT    PRIMARY KEY,
    condition   TEXT    NOT NULL DEFAULT 'clear',
    expires_action INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS enchants (
    item_id     TEXT    NOT NULL,
    enchant_id  TEXT    NOT NULL,
    level       INTEGER NOT NULL DEFAULT 1,
    applied_at  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, enchant_id)
);

CREATE TABLE IF NOT EXISTS builds (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id     TEXT    NOT NULL,
    build_id    TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    desc        TEXT    NOT NULL DEFAULT '',
    placed_at   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS crystal_shards (
    shard_id        TEXT    PRIMARY KEY,
    biome           TEXT    NOT NULL,
    collected       INTEGER NOT NULL DEFAULT 0,
    collected_at    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS death_pile (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id     TEXT    NOT NULL,
    item_id     TEXT    NOT NULL,
    item_name   TEXT    NOT NULL,
    item_desc   TEXT    NOT NULL DEFAULT '',
    died_at     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS enchanting_xp (
    id      INTEGER PRIMARY KEY CHECK (id = 1),
    xp      INTEGER NOT NULL DEFAULT 0,
    level   INTEGER NOT NULL DEFAULT 1
);
INSERT OR IGNORE INTO enchanting_xp (id, xp, level) VALUES (1, 0, 1);

CREATE TABLE IF NOT EXISTS crops (
    room_id         TEXT    NOT NULL,
    slot            INTEGER NOT NULL,
    seed_id         TEXT    NOT NULL,
    planted_at_action   INTEGER NOT NULL,
    ready_at_action     INTEGER NOT NULL,
    harvested       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (room_id, slot)
);

CREATE TABLE IF NOT EXISTS chests (
    room_id     TEXT    NOT NULL,
    item_id     TEXT    NOT NULL,
    item_name   TEXT    NOT NULL,
    item_desc   TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (room_id, item_id)
);
```

- [ ] **Step 2: Verify schema compiles**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./internal/db/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add 9 new tables for blockhaven mechanics"
```

---

## Task 2: World YAML — New Struct Fields

**Files:**
- Modify: `internal/world/world.go`
- Create: `internal/world/world_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/world/world_test.go`:

```go
package world_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
	"gopkg.in/yaml.v3"
)

func TestRoomBiomeAndResources(t *testing.T) {
	raw := `
name: test
start_room: r1
narrator_model: test
rooms:
  - id: r1
    name: Test Room
    desc: A test room.
    exits: {}
    biome: forest
    resources:
      - id: oak-tree
        type: harvest
        yields:
          - item_id: wood-log
            probability: 1.0
            count_min: 1
            count_max: 3
        tool_required: ""
        respawn_actions: 10
        grow_actions: 0
weather_table:
  - biome: forest
    possible: [clear, rainy]
`
	var w world.World
	if err := yaml.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	r := w.Rooms[0]
	if r.Biome != "forest" {
		t.Errorf("biome: want forest, got %q", r.Biome)
	}
	if len(r.Resources) != 1 || r.Resources[0].ID != "oak-tree" {
		t.Errorf("resources: got %+v", r.Resources)
	}
	if len(w.WeatherTable) != 1 || w.WeatherTable[0].Biome != "forest" {
		t.Errorf("weather_table: got %+v", w.WeatherTable)
	}
}

func TestAvailable(t *testing.T) {
	names := world.Available()
	found := false
	for _, n := range names {
		if n == "cyberspace" {
			found = true
		}
	}
	if !found {
		t.Error("Available() should always include cyberspace")
	}
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -run TestRoomBiomeAndResources -v
```

Expected: compile error about unknown fields `Biome`, `Resources`, `WeatherTable`.

- [ ] **Step 3: Add new types and fields to world.go**

In `internal/world/world.go`, add these types after the `LootEntry` struct (around line 83):

```go
// Resource is a mineable or harvestable node inside a room.
type Resource struct {
	ID             string      `yaml:"id"`
	Type           string      `yaml:"type"` // "mine" | "harvest" | "plant"
	Yields         []LootEntry `yaml:"yields"`
	ToolRequired   string      `yaml:"tool_required,omitempty"`
	RespawnActions int         `yaml:"respawn_actions,omitempty"`
	GrowActions    int         `yaml:"grow_actions,omitempty"` // for plant seeds
}

// WeatherEntry lists possible weather conditions for one biome.
type WeatherEntry struct {
	Biome    string   `yaml:"biome"`
	Possible []string `yaml:"possible"`
}
```

Add `Biome` and `Resources` fields to the `Room` struct (after `Locks []Lock`):

```go
Biome     string     `yaml:"biome,omitempty"`
Resources []Resource `yaml:"resources,omitempty"`
```

Add `WeatherTable` to the `World` struct (after `Quests []WorldQuest`):

```go
WeatherTable []WeatherEntry `yaml:"weather_table,omitempty"`
```

Add `Available()` after the `Load` function:

```go
// Available returns the names of all installed worlds plus the embedded default.
// Always includes "cyberspace".
func Available() []string {
	names := []string{"cyberspace"}
	home, err := os.UserHomeDir()
	if err != nil {
		return names
	}
	dir := filepath.Join(home, ".local", "share", "gl1tch-mud", "worlds")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if n == "cyberspace" {
			continue // already included
		}
		// Only include if world.yaml exists
		p := filepath.Join(dir, n, "world.yaml")
		if _, err := os.Stat(p); err == nil {
			names = append(names, n)
		}
	}
	return names
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -v
```

Expected: PASS for both tests.

- [ ] **Step 5: Commit**

```bash
git add internal/world/world.go internal/world/world_test.go
git commit -m "feat(world): add Resource, WeatherEntry types; Biome/Resources on Room; Available()"
```

---

## Task 3: DB — Add OpenForWorld

**Files:**
- Modify: `internal/db/db.go`

- [ ] **Step 1: Add OpenForWorld function**

In `internal/db/db.go`, add after `OpenForPlayer`:

```go
// OpenForWorld opens (or creates) a per-world player database at
// ~/.local/share/gl1tch-mud/worlds/<worldName>/player.db.
// Used for single-player world switching — each world has its own save.
func OpenForWorld(worldName string) (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".local", "share", "gl1tch-mud", "worlds", worldName, "player.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db: mkdir: %w", err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if _, err := database.Exec(schema); err != nil {
		return nil, fmt.Errorf("db: schema: %w", err)
	}
	return database, nil
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./internal/db/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/db/db.go
git commit -m "feat(db): add OpenForWorld for per-world single-player saves"
```

---

## Task 4: Result.SwitchWorld + main.go World-Switch Handling

**Files:**
- Modify: `internal/commands/commands.go`
- Modify: `main.go`

- [ ] **Step 1: Add SwitchWorld to Result**

In `internal/commands/commands.go`, change the `Result` struct (lines 36–39):

```go
// Result is returned by every command handler.
type Result struct {
	Output      string
	Event       *Event
	SwitchWorld string // non-empty triggers a world switch in main.go
}
```

- [ ] **Step 2: Add world-switch handling in main.go**

In `main.go`, the game loop currently ends with:

```go
result := handler(database, s, w, args)
fmt.Println(result.Output)
if result.Event != nil {
    bus.Publish(result.Event.Topic, result.Event.Payload)
}
```

Replace that block with:

```go
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
            lanSrv = server.New(w)
            commands.SetLANServer(lanSrv)
            newState, _ := player.Load(database)
            *s = *newState
            lookResult := commands.Look(database, s, w, nil)
            fmt.Println(lookResult.Output)
        }
    }
}
```

Also change `main.go` to use `db.OpenForWorld` on startup instead of `db.Open()`. Replace:

```go
database, err := db.Open()
```

With:

```go
database, err := db.OpenForWorld("cyberspace")
```

And remove the now-unused `defer database.Close()` at the top (it's still valid since `database` is reassignable — keep it but note that `database.Close()` in the loop closes the old connection; the defer closes whatever `database` points to at exit).

- [ ] **Step 3: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/commands/commands.go main.go
git commit -m "feat: add Result.SwitchWorld; main.go handles world hot-swap"
```

---

## Task 5: World List + World Switch Commands

**Files:**
- Create: `internal/commands/world.go`

- [ ] **Step 1: Create the file**

```go
package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["world"] = World
}

// World handles "world list" and "world switch <name>".
func World(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "usage: world list | world switch <name>"}
	}

	switch strings.ToLower(args[0]) {
	case "list":
		names := world.Available()
		var b strings.Builder
		b.WriteString("available worlds:\n")
		for _, n := range names {
			marker := "  "
			if n == s.World {
				marker = "* "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", marker, n))
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}

	case "switch":
		if len(args) < 2 {
			return Result{Output: "usage: world switch <name>"}
		}
		target := strings.ToLower(args[1])
		if target == s.World {
			return Result{Output: fmt.Sprintf("you are already in %s.", target)}
		}
		// Validate world exists before returning SwitchWorld.
		_, err := world.Load(target)
		if err != nil {
			return Result{Output: fmt.Sprintf("world %q not found.", target)}
		}
		return Result{
			Output:      fmt.Sprintf("leaving %s... entering %s.", s.World, target),
			SwitchWorld: target,
		}

	default:
		return Result{Output: "usage: world list | world switch <name>"}
	}
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 3: Smoke test**

```bash
cd /Users/stokes/Projects/gl1tch-mud && echo "world list" | go run .
```

Expected: output includes `* cyberspace`.

- [ ] **Step 4: Commit**

```bash
git add internal/commands/world.go
git commit -m "feat(commands): add world list/switch"
```

---

## Task 6: Weather Package

**Files:**
- Create: `internal/weather/weather.go`
- Create: `internal/weather/weather_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/weather/weather_test.go`:

```go
package weather_test

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/weather"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE weather_state (
			biome          TEXT PRIMARY KEY,
			condition      TEXT NOT NULL DEFAULT 'clear',
			expires_action INTEGER NOT NULL DEFAULT 0
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestCurrentDefault(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	cond, err := weather.Current(db, "meadow")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if cond != "clear" {
		t.Errorf("expected default clear, got %q", cond)
	}
}

func TestTickChangesWeather(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	possible := []string{"clear", "rainy", "stormy"}
	// Set expires_action to 0, current action to 100 — should roll new weather.
	cond, err := weather.Tick(db, "meadow", 100, possible)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	found := false
	for _, p := range possible {
		if cond == p {
			found = true
		}
	}
	if !found {
		t.Errorf("condition %q not in possible set %v", cond, possible)
	}
}

func TestYieldBonus(t *testing.T) {
	if weather.YieldBonus("clear") != 1.1 {
		t.Error("clear should give 1.1 bonus")
	}
	if weather.YieldBonus("blizzard") != 1.0 {
		t.Error("blizzard should give 1.0 bonus")
	}
}
```

- [ ] **Step 2: Run — expect failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/weather/... -v
```

Expected: compile error, package does not exist.

- [ ] **Step 3: Create weather.go**

Create `internal/weather/weather.go`:

```go
// Package weather manages per-biome weather state.
package weather

import (
	"database/sql"
	"math/rand"
)

// Current returns the current weather condition for biome.
// Returns "clear" if no record exists.
func Current(db *sql.DB, biome string) (string, error) {
	var cond string
	err := db.QueryRow(`SELECT condition FROM weather_state WHERE biome=?`, biome).Scan(&cond)
	if err == sql.ErrNoRows {
		return "clear", nil
	}
	if err != nil {
		return "clear", err
	}
	return cond, nil
}

// Tick checks whether weather should change for biome (if currentAction >= expires_action)
// and if so rolls a new condition from possible. Returns the current (possibly new) condition.
func Tick(db *sql.DB, biome string, currentAction int, possible []string) (string, error) {
	var expires int
	var cond string
	err := db.QueryRow(`SELECT condition, expires_action FROM weather_state WHERE biome=?`, biome).
		Scan(&cond, &expires)
	if err != nil && err != sql.ErrNoRows {
		return "clear", err
	}
	if err == sql.ErrNoRows || currentAction >= expires {
		// Roll new condition and set expiry 50 actions from now.
		if len(possible) == 0 {
			possible = []string{"clear"}
		}
		cond = possible[rand.Intn(len(possible))]
		newExpires := currentAction + 50
		_, err = db.Exec(
			`INSERT INTO weather_state (biome, condition, expires_action) VALUES (?,?,?)
			 ON CONFLICT(biome) DO UPDATE SET condition=excluded.condition, expires_action=excluded.expires_action`,
			biome, cond, newExpires,
		)
		if err != nil {
			return cond, err
		}
	}
	return cond, nil
}

// YieldBonus returns the resource yield multiplier for condition.
func YieldBonus(condition string) float64 {
	switch condition {
	case "clear":
		return 1.1
	default:
		return 1.0
	}
}

// Description returns a player-facing description of the weather condition.
func Description(biome, condition string) string {
	switch condition {
	case "clear":
		return "The sky is clear. Conditions are ideal."
	case "rainy":
		return "Rain patters down. Soil is rich — gathering may yield extra seeds."
	case "windy":
		return "A strong wind blows through. Nothing unusual."
	case "stormy":
		return "A fierce storm crackles overhead. Lightning might reveal buried loot."
	case "foggy":
		return "A thick fog hangs in the air. Visibility is low."
	case "sandstorm":
		return "Sand whips through the air. Ancient ruins entrances may be uncovered."
	case "scorching":
		return "The heat is brutal. Stay hydrated."
	case "light-snow":
		return "Light snowflakes drift down peacefully."
	case "blizzard":
		return "A blizzard rages. Mining may yield extra gems in these conditions."
	case "damp":
		return "The cave air is cold and damp."
	case "tremor":
		return "The cave walls shudder. A tremor has shifted the rock — new veins may be exposed."
	default:
		return fmt.Sprintf("The weather is %s.", condition)
	}
}
```

Wait, I need to add the `fmt` import. Let me fix the file:

```go
// Package weather manages per-biome weather state.
package weather

import (
	"database/sql"
	"fmt"
	"math/rand"
)

// Current returns the current weather condition for biome.
// Returns "clear" if no record exists.
func Current(db *sql.DB, biome string) (string, error) {
	var cond string
	err := db.QueryRow(`SELECT condition FROM weather_state WHERE biome=?`, biome).Scan(&cond)
	if err == sql.ErrNoRows {
		return "clear", nil
	}
	if err != nil {
		return "clear", err
	}
	return cond, nil
}

// Tick checks whether weather should change for biome (if currentAction >= expires_action)
// and if so rolls a new condition from possible. Returns the current (possibly new) condition.
func Tick(db *sql.DB, biome string, currentAction int, possible []string) (string, error) {
	var expires int
	var cond string
	err := db.QueryRow(`SELECT condition, expires_action FROM weather_state WHERE biome=?`, biome).
		Scan(&cond, &expires)
	if err != nil && err != sql.ErrNoRows {
		return "clear", err
	}
	if err == sql.ErrNoRows || currentAction >= expires {
		if len(possible) == 0 {
			possible = []string{"clear"}
		}
		cond = possible[rand.Intn(len(possible))]
		newExpires := currentAction + 50
		_, err = db.Exec(
			`INSERT INTO weather_state (biome, condition, expires_action) VALUES (?,?,?)
			 ON CONFLICT(biome) DO UPDATE SET condition=excluded.condition, expires_action=excluded.expires_action`,
			biome, cond, newExpires,
		)
		if err != nil {
			return cond, err
		}
	}
	return cond, nil
}

// YieldBonus returns the resource yield multiplier for condition.
func YieldBonus(condition string) float64 {
	if condition == "clear" {
		return 1.1
	}
	return 1.0
}

// Description returns a player-facing description of the weather condition.
func Description(biome, condition string) string {
	switch condition {
	case "clear":
		return "The sky is clear. Conditions are ideal."
	case "rainy":
		return "Rain patters down. Soil is rich — gathering may yield extra seeds."
	case "windy":
		return "A strong wind blows through. Nothing unusual."
	case "stormy":
		return "A fierce storm crackles overhead. Lightning might reveal buried loot."
	case "foggy":
		return "A thick fog hangs in the air. Visibility is low."
	case "sandstorm":
		return "Sand whips through the air. Ancient ruins entrances may be uncovered."
	case "scorching":
		return "The heat is brutal. Stay hydrated."
	case "light-snow":
		return "Light snowflakes drift down peacefully."
	case "blizzard":
		return "A blizzard rages. Mining may yield extra gems in these conditions."
	case "damp":
		return "The cave air is cold and damp."
	case "tremor":
		return "The cave walls shudder. A tremor has shifted the rock — new veins may be exposed."
	default:
		return fmt.Sprintf("The weather is %s.", condition)
	}
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/weather/... -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/weather/weather.go internal/weather/weather_test.go
git commit -m "feat(weather): add weather tick, current, yield bonus, description"
```

---

## Task 7: Enchanting Package

**Files:**
- Create: `internal/enchanting/enchanting.go`
- Create: `internal/enchanting/enchanting_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/enchanting/enchanting_test.go`:

```go
package enchanting_test

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/enchanting"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE enchants (
			item_id    TEXT NOT NULL,
			enchant_id TEXT NOT NULL,
			level      INTEGER NOT NULL DEFAULT 1,
			applied_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (item_id, enchant_id)
		);
		CREATE TABLE enchanting_xp (
			id    INTEGER PRIMARY KEY CHECK (id=1),
			xp    INTEGER NOT NULL DEFAULT 0,
			level INTEGER NOT NULL DEFAULT 1
		);
		INSERT OR IGNORE INTO enchanting_xp (id,xp,level) VALUES (1,0,1);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestApplyAndList(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := enchanting.Apply(db, "iron-sword", "sharpness", 1, 0); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := enchanting.List(db, "iron-sword")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].EnchantID != "sharpness" {
		t.Errorf("List: got %+v", got)
	}
}

func TestAddXPAndLevel(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	enchanting.AddXP(db, 100) //nolint:errcheck
	xp, level, err := enchanting.XPState(db)
	if err != nil {
		t.Fatalf("XPState: %v", err)
	}
	if xp != 100 {
		t.Errorf("xp: want 100, got %d", xp)
	}
	if level != 1 {
		t.Errorf("level: want 1, got %d", level)
	}
}

func TestEnchantBonus(t *testing.T) {
	if enchanting.AttackBonus("sharpness", 1) != 5 {
		t.Error("sharpness I should give +5 attack")
	}
	if enchanting.AttackBonus("sharpness", 3) != 15 {
		t.Error("sharpness III should give +15 attack")
	}
	if enchanting.YieldBonus("fortune", 2) != 2 {
		t.Error("fortune II should give +2 yield")
	}
}
```

- [ ] **Step 2: Run — expect compile failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/enchanting/... -v
```

Expected: compile error.

- [ ] **Step 3: Create enchanting.go**

Create `internal/enchanting/enchanting.go`:

```go
// Package enchanting manages item enchantments and enchanting XP.
package enchanting

import (
	"database/sql"
)

// Enchant is a record of an enchantment applied to an item.
type Enchant struct {
	ItemID    string
	EnchantID string
	Level     int
}

// Apply adds an enchantment to an item (or upgrades level if already present).
func Apply(db *sql.DB, itemID, enchantID string, level, actionCount int) error {
	_, err := db.Exec(
		`INSERT INTO enchants (item_id, enchant_id, level, applied_at) VALUES (?,?,?,?)
		 ON CONFLICT(item_id, enchant_id) DO UPDATE SET level=excluded.level, applied_at=excluded.applied_at`,
		itemID, enchantID, level, actionCount,
	)
	return err
}

// List returns all enchantments on an item.
func List(db *sql.DB, itemID string) ([]Enchant, error) {
	rows, err := db.Query(`SELECT item_id, enchant_id, level FROM enchants WHERE item_id=?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Enchant
	for rows.Next() {
		var e Enchant
		if err := rows.Scan(&e.ItemID, &e.EnchantID, &e.Level); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AddXP adds enchanting experience points and recalculates level (100 XP per level, cap 30).
func AddXP(db *sql.DB, amount int) error {
	_, err := db.Exec(`UPDATE enchanting_xp SET xp = xp + ? WHERE id = 1`, amount)
	if err != nil {
		return err
	}
	// Recalculate level: level = min(xp/100 + 1, 30)
	_, err = db.Exec(`
		UPDATE enchanting_xp
		SET level = MIN((xp / 100) + 1, 30)
		WHERE id = 1
	`)
	return err
}

// XPState returns current enchanting XP and level.
func XPState(db *sql.DB) (xp, level int, err error) {
	err = db.QueryRow(`SELECT xp, level FROM enchanting_xp WHERE id=1`).Scan(&xp, &level)
	return
}

// AttackBonus returns the attack bonus granted by an enchantment at a given level.
func AttackBonus(enchantID string, level int) int {
	switch enchantID {
	case "sharpness":
		return level * 5
	case "flame-touch":
		return 5
	case "frost-edge":
		return 8
	default:
		return 0
	}
}

// YieldBonus returns the extra yield count granted by an enchantment at a given level.
func YieldBonus(enchantID string, level int) int {
	switch enchantID {
	case "fortune":
		return level
	default:
		return 0
	}
}

// AvailableForItemType returns enchantment IDs applicable to a category.
// Categories: "sword", "pickaxe", "axe", "boots", "any"
func AvailableForItemType(category string) []string {
	switch category {
	case "sword":
		return []string{"sharpness", "flame-touch", "frost-edge", "diamond-luck"}
	case "pickaxe":
		return []string{"fortune", "silk-touch", "diamond-luck"}
	case "axe":
		return []string{"fortune", "sharpness", "diamond-luck"}
	case "boots":
		return []string{"swift-feet", "feather-fall", "diamond-luck"}
	default:
		return []string{"diamond-luck"}
	}
}

// EnchantName returns the display name for an enchantment ID and level.
func EnchantName(id string, level int) string {
	levelNames := []string{"", "I", "II", "III"}
	lv := ""
	if level >= 1 && level <= 3 {
		lv = " " + levelNames[level]
	}
	names := map[string]string{
		"sharpness":   "Sharpness",
		"fortune":     "Fortune",
		"swift-feet":  "Swift Feet",
		"flame-touch": "Flame Touch",
		"silk-touch":  "Silk Touch",
		"feather-fall": "Feather Fall",
		"frost-edge":  "Frost Edge",
		"diamond-luck": "Diamond Luck",
	}
	if n, ok := names[id]; ok {
		return n + lv
	}
	return id + lv
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/enchanting/... -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/enchanting/enchanting.go internal/enchanting/enchanting_test.go
git commit -m "feat(enchanting): add enchant apply/list, XP tracking, bonus lookups"
```

---

## Task 8: Player — Death Pile + Enchanting XP Helpers

**Files:**
- Modify: `internal/player/player.go`
- Create: `internal/player/player_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/player/player_test.go`:

```go
package player_test

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE inventory (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id   TEXT NOT NULL UNIQUE,
			item_name TEXT NOT NULL,
			item_desc TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE death_pile (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id   TEXT NOT NULL,
			item_id   TEXT NOT NULL,
			item_name TEXT NOT NULL,
			item_desc TEXT NOT NULL DEFAULT '',
			died_at   INTEGER NOT NULL DEFAULT 0
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestDumpAndClaimDeathPile(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	player.AddItem(db, "iron-sword", "Iron Sword", "A sharp blade.") //nolint:errcheck
	player.AddItem(db, "bread", "Bread", "Restores HP.")             //nolint:errcheck

	if err := player.DumpToDeathPile(db, "forest-1", 42); err != nil {
		t.Fatalf("DumpToDeathPile: %v", err)
	}

	// Inventory should be empty
	items, _ := player.Inventory(db)
	if len(items) != 0 {
		t.Errorf("inventory should be empty after dump, got %d items", len(items))
	}

	// Death pile should have 2 items
	pile, err := player.GetDeathPile(db, "forest-1")
	if err != nil {
		t.Fatalf("GetDeathPile: %v", err)
	}
	if len(pile) != 2 {
		t.Errorf("death pile: want 2 items, got %d", len(pile))
	}

	// Claim death pile
	if err := player.ClaimDeathPile(db, "forest-1"); err != nil {
		t.Fatalf("ClaimDeathPile: %v", err)
	}

	// Inventory should have 2 items again
	items, _ = player.Inventory(db)
	if len(items) != 2 {
		t.Errorf("inventory after claim: want 2, got %d", len(items))
	}

	// Death pile should be empty
	pile, _ = player.GetDeathPile(db, "forest-1")
	if len(pile) != 0 {
		t.Errorf("death pile after claim: want 0, got %d", len(pile))
	}
}

func TestRemoveItem(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	player.AddItem(db, "coal", "Coal", "Fuel.") //nolint:errcheck

	if err := player.RemoveItem(db, "coal"); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	items, _ := player.Inventory(db)
	if len(items) != 0 {
		t.Errorf("item should be removed, got %d", len(items))
	}
}
```

- [ ] **Step 2: Run — expect failure**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/player/... -v
```

Expected: compile errors for missing functions.

- [ ] **Step 3: Add new functions to player.go**

Append to `internal/player/player.go`:

```go
// RemoveItem removes an item from inventory by ID.
func RemoveItem(db *sql.DB, itemID string) error {
	_, err := db.Exec(`DELETE FROM inventory WHERE item_id=?`, itemID)
	return err
}

// DumpToDeathPile moves all inventory items to the death_pile table for roomID.
// actionCount is the current player_actions count (for expiry tracking).
func DumpToDeathPile(db *sql.DB, roomID string, actionCount int) error {
	items, err := Inventory(db)
	if err != nil {
		return err
	}
	for _, it := range items {
		if _, err := db.Exec(
			`INSERT INTO death_pile (room_id, item_id, item_name, item_desc, died_at) VALUES (?,?,?,?,?)`,
			roomID, it.ID, it.Name, it.Desc, actionCount,
		); err != nil {
			return err
		}
	}
	_, err = db.Exec(`DELETE FROM inventory`)
	return err
}

// GetDeathPile returns death pile items for a given room.
func GetDeathPile(db *sql.DB, roomID string) ([]InventoryItem, error) {
	rows, err := db.Query(`SELECT item_id, item_name, item_desc FROM death_pile WHERE room_id=?`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []InventoryItem
	for rows.Next() {
		var it InventoryItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Desc); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ClaimDeathPile moves all death pile items for roomID back to inventory and deletes them.
func ClaimDeathPile(db *sql.DB, roomID string) error {
	items, err := GetDeathPile(db, roomID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := AddItem(db, it.ID, it.Name, it.Desc); err != nil {
			return err
		}
	}
	_, err = db.Exec(`DELETE FROM death_pile WHERE room_id=?`, roomID)
	return err
}

// AnyDeathPile returns the room_id of the most recent death pile, or "" if none.
func AnyDeathPile(db *sql.DB) (roomID string, count int) {
	db.QueryRow(`SELECT room_id, COUNT(*) FROM death_pile GROUP BY room_id ORDER BY died_at DESC LIMIT 1`). //nolint:errcheck
		Scan(&roomID, &count)
	return
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/player/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/player/player.go internal/player/player_test.go
git commit -m "feat(player): add RemoveItem, death pile helpers"
```

---

## Task 9: Extend Attack() for Death Pile + Look() for Death Pile Display

**Files:**
- Modify: `internal/commands/commands.go`

- [ ] **Step 1: Extend the player-death block in Attack()**

In `internal/commands/commands.go`, find the death block at line ~406:

```go
if s.HP <= 0 {
    s.HP = s.MaxHP
    s.RoomID = w.StartRoom
    player.Save(db, s) //nolint:errcheck
    out.WriteString("\nyou died. jacking back in at the entry node.")
```

Replace it with:

```go
if s.HP <= 0 {
    // Get action count for deathpile timestamp.
    var actionCnt int
    db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&actionCnt) //nolint:errcheck

    deathRoom := s.RoomID
    player.DumpToDeathPile(db, deathRoom, actionCnt) //nolint:errcheck

    s.HP = s.MaxHP
    s.RoomID = w.StartRoom
    player.Save(db, s) //nolint:errcheck

    deathRoomName := deathRoom
    if r := w.Room(deathRoom); r != nil {
        deathRoomName = r.Name
    }
    out.WriteString(fmt.Sprintf(
        "\nyou were defeated! your items lie at %s.\nyou wake up at %s.",
        deathRoomName, w.Room(w.StartRoom).Name,
    ))
```

- [ ] **Step 2: Add death pile rendering to Look()**

In the `Look` function, after `output := room.Render(visited)`, add:

```go
// Show death pile if player died in this room.
pile, _ := player.GetDeathPile(db, s.RoomID)
if len(pile) > 0 {
    output += "\n[your death pile is here — use 'take death-pile' to recover your items]"
}
```

You also need to handle `take death-pile` in the `Take` command. In the `Take` function, add a check at the top of the item-matching loop:

Find the `Take` function and add before the room items loop:

```go
// Special: claim death pile.
if len(args) > 0 && strings.Join(args, "-") == "death-pile" {
    pile, _ := player.GetDeathPile(db, s.RoomID)
    if len(pile) == 0 {
        return Result{Output: "there is no death pile here."}
    }
    player.ClaimDeathPile(db, s.RoomID) //nolint:errcheck
    names := make([]string, len(pile))
    for i, it := range pile {
        names[i] = it.Name
    }
    return Result{Output: fmt.Sprintf("you recover your items: %s.", strings.Join(names, ", "))}
}
```

- [ ] **Step 3: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/commands/commands.go
git commit -m "feat(combat): death pile on death; look shows pile; take death-pile recovers items"
```

---

## Task 10: Mining Commands (mine, harvest, gather, smelt, plant)

**Files:**
- Create: `internal/commands/mining.go`

- [ ] **Step 1: Create the file**

Create `internal/commands/mining.go`:

```go
package commands

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/enchanting"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/weather"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["mine"]    = Mine
	Registry["harvest"] = Harvest
	Registry["gather"]  = Gather
	Registry["smelt"]   = Smelt
	Registry["plant"]   = Plant
}

// actionCount reads the current player_actions count from DB.
func actionCount(db *sql.DB) int {
	var n int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&n) //nolint:errcheck
	return n
}

// bumpActions increments the action counter.
func bumpActions(db *sql.DB) {
	db.Exec(`INSERT INTO player_actions (id,count) VALUES (1,1) ON CONFLICT(id) DO UPDATE SET count=count+1`) //nolint:errcheck
}

// isResourceDepleted checks if a resource in a room is currently depleted.
func isResourceDepleted(db *sql.DB, roomID, resourceID string, respawnActions int) bool {
	var depleted, depletedAt int
	err := db.QueryRow(
		`SELECT depleted, depleted_at_action FROM room_resources WHERE room_id=? AND resource_id=?`,
		roomID, resourceID,
	).Scan(&depleted, &depletedAt)
	if err != nil {
		return false // no record = not depleted
	}
	if depleted == 0 {
		return false
	}
	// Check if respawn window has passed.
	current := actionCount(db)
	if current >= depletedAt+respawnActions {
		// Auto-restore.
		db.Exec(`UPDATE room_resources SET depleted=0 WHERE room_id=? AND resource_id=?`, roomID, resourceID) //nolint:errcheck
		return false
	}
	return true
}

// depleteResource marks a resource as depleted.
func depleteResource(db *sql.DB, roomID, resourceID string) {
	current := actionCount(db)
	db.Exec( //nolint:errcheck
		`INSERT INTO room_resources (room_id, resource_id, depleted, depleted_at_action) VALUES (?,?,1,?)
		 ON CONFLICT(room_id, resource_id) DO UPDATE SET depleted=1, depleted_at_action=excluded.depleted_at_action`,
		roomID, resourceID, current,
	)
}

// rollYield rolls a loot yield from a resource's yields list, applying weather + fortune enchant.
func rollYield(db *sql.DB, yields []world.LootEntry, biome string) []world.LootEntry {
	var bonusCount int
	// Check fortune enchant on any pickaxe/axe in inventory.
	items, _ := player.Inventory(db)
	for _, it := range items {
		enchants, _ := enchanting.List(db, it.ID)
		for _, e := range enchants {
			if e.EnchantID == "fortune" {
				bonusCount += enchanting.YieldBonus("fortune", e.Level)
			}
		}
	}

	// Get weather bonus.
	weatherBonus := 1.0
	var possible []string
	weatherBonus = 1.0
	cond, _ := weather.Current(db, biome)
	weatherBonus = weather.YieldBonus(cond)

	_ = weatherBonus // used in probability below

	var out []world.LootEntry
	for _, entry := range yields {
		if rand.Float64() > entry.Probability*weatherBonus {
			continue
		}
		count := entry.CountMin + rand.Intn(entry.CountMax-entry.CountMin+1) + bonusCount
		out = append(out, world.LootEntry{
			ItemID:   entry.ItemID,
			Name:     entry.Name,
			Desc:     entry.Desc,
			CountMin: count,
			CountMax: count,
		})
	}
	_ = possible
	return out
}

// Mine lists or mines a resource in the current room.
func Mine(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nowhere to mine."}
	}

	// Filter mine-type resources.
	var mineResources []world.Resource
	for _, r := range room.Resources {
		if r.Type == "mine" {
			mineResources = append(mineResources, r)
		}
	}

	if len(args) == 0 {
		if len(mineResources) == 0 {
			return Result{Output: "nothing to mine here."}
		}
		var b strings.Builder
		b.WriteString("mineable resources:\n")
		for _, r := range mineResources {
			status := ""
			if isResourceDepleted(db, s.RoomID, r.ID, r.RespawnActions) {
				status = " (depleted)"
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", r.ID, status))
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	target := strings.ToLower(args[0])
	var res *world.Resource
	for i := range mineResources {
		if mineResources[i].ID == target {
			res = &mineResources[i]
			break
		}
	}
	if res == nil {
		return Result{Output: fmt.Sprintf("no mineable resource %q here.", target)}
	}
	if isResourceDepleted(db, s.RoomID, res.ID, res.RespawnActions) {
		return Result{Output: fmt.Sprintf("the %s is exhausted. come back later.", res.ID)}
	}

	// Check tool requirement.
	if res.ToolRequired != "" {
		invIDs := inventoryIDs(db)
		hasTool := false
		for _, id := range invIDs {
			if strings.Contains(id, res.ToolRequired) {
				hasTool = true
				break
			}
		}
		if !hasTool {
			return Result{Output: fmt.Sprintf("you need a %s to mine this.", res.ToolRequired)}
		}
	}

	bumpActions(db)
	depleteResource(db, s.RoomID, res.ID)

	// Award enchanting XP for mining.
	enchanting.AddXP(db, 5) //nolint:errcheck

	yields := rollYield(db, res.Yields, room.Biome)
	if len(yields) == 0 {
		return Result{Output: fmt.Sprintf("you mine the %s but find nothing useful.", res.ID)}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("you mine the %s...\n", res.ID))
	for _, y := range yields {
		player.AddItem(db, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		b.WriteString(fmt.Sprintf("  + %dx %s\n", y.CountMin, y.Name))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Harvest lists or harvests a resource in the current room.
func Harvest(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nowhere to harvest."}
	}

	var harvestResources []world.Resource
	for _, r := range room.Resources {
		if r.Type == "harvest" {
			harvestResources = append(harvestResources, r)
		}
	}

	// Also add ready crops.
	current := actionCount(db)
	rows, _ := db.Query(
		`SELECT seed_id FROM crops WHERE room_id=? AND ready_at_action<=? AND harvested=0`,
		s.RoomID, current,
	)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var seedID string
			rows.Scan(&seedID) //nolint:errcheck
			harvestResources = append(harvestResources, world.Resource{ID: seedID + "-crop", Type: "harvest"})
		}
	}

	if len(args) == 0 {
		if len(harvestResources) == 0 {
			return Result{Output: "nothing to harvest here."}
		}
		var b strings.Builder
		b.WriteString("harvestable resources:\n")
		for _, r := range harvestResources {
			status := ""
			if isResourceDepleted(db, s.RoomID, r.ID, r.RespawnActions) {
				status = " (depleted)"
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", r.ID, status))
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	target := strings.ToLower(args[0])

	// Check if it's a ready crop.
	if strings.HasSuffix(target, "-crop") {
		seedID := strings.TrimSuffix(target, "-crop")
		var cropCount int
		db.QueryRow(`SELECT COUNT(*) FROM crops WHERE room_id=? AND seed_id=? AND ready_at_action<=? AND harvested=0`,
			s.RoomID, seedID, current).Scan(&cropCount) //nolint:errcheck
		if cropCount == 0 {
			return Result{Output: "no ready crops of that type here."}
		}
		db.Exec(`UPDATE crops SET harvested=1 WHERE room_id=? AND seed_id=? AND ready_at_action<=? AND harvested=0`, //nolint:errcheck
			s.RoomID, seedID, current)
		player.AddItem(db, seedID+"-harvest", strings.Title(seedID), "A freshly harvested crop.") //nolint:errcheck
		return Result{Output: fmt.Sprintf("you harvest the %s.", seedID)}
	}

	var res *world.Resource
	for i := range harvestResources {
		if harvestResources[i].ID == target {
			res = &harvestResources[i]
			break
		}
	}
	if res == nil {
		return Result{Output: fmt.Sprintf("no harvestable resource %q here.", target)}
	}
	if isResourceDepleted(db, s.RoomID, res.ID, res.RespawnActions) {
		return Result{Output: fmt.Sprintf("the %s is exhausted. come back later.", res.ID)}
	}

	bumpActions(db)
	depleteResource(db, s.RoomID, res.ID)

	yields := rollYield(db, res.Yields, room.Biome)
	if len(yields) == 0 {
		return Result{Output: fmt.Sprintf("you harvest the %s but find nothing useful.", res.ID)}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("you harvest the %s...\n", res.ID))
	for _, y := range yields {
		player.AddItem(db, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		b.WriteString(fmt.Sprintf("  + %dx %s\n", y.CountMin, y.Name))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Gather picks up ambient resources from the environment (no tool required).
func Gather(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	biome := "meadow"
	if room != nil {
		biome = room.Biome
	}

	// 20-action gather cooldown.
	const cooldown = 20
	var lastGather int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&lastGather) //nolint:errcheck

	// Check last gather time stored in a separate simple mechanism:
	// reuse room_resources with id="gather-cooldown" and depleted_at_action.
	if isResourceDepleted(db, s.RoomID+"-gather", "gather-cooldown", cooldown) {
		return Result{Output: "you need to rest before gathering again."}
	}

	depleteResource(db, s.RoomID+"-gather", "gather-cooldown")
	bumpActions(db)

	ambient := map[string][]world.LootEntry{
		"meadow": {
			{ItemID: "flint", Name: "Flint", Desc: "A sharp piece of flint.", Probability: 0.8, CountMin: 1, CountMax: 2},
			{ItemID: "stick", Name: "Stick", Desc: "A sturdy stick.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "wildflower", Name: "Wildflower", Desc: "A cheerful wildflower.", Probability: 0.5, CountMin: 1, CountMax: 1},
		},
		"forest": {
			{ItemID: "stick", Name: "Stick", Desc: "A sturdy stick.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "berry", Name: "Berry", Desc: "Wild berries.", Probability: 0.6, CountMin: 1, CountMax: 3},
			{ItemID: "leaf", Name: "Leaf", Desc: "A large leaf.", Probability: 0.7, CountMin: 1, CountMax: 2},
		},
		"desert": {
			{ItemID: "sand", Name: "Sand", Desc: "Fine desert sand.", Probability: 0.95, CountMin: 2, CountMax: 5},
			{ItemID: "flint", Name: "Flint", Desc: "A sharp piece of flint.", Probability: 0.6, CountMin: 1, CountMax: 2},
			{ItemID: "bone", Name: "Bone", Desc: "An old bone.", Probability: 0.4, CountMin: 1, CountMax: 1},
		},
		"snow": {
			{ItemID: "ice", Name: "Ice", Desc: "A chunk of ice.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "pebble", Name: "Pebble", Desc: "A smooth pebble.", Probability: 0.7, CountMin: 1, CountMax: 2},
			{ItemID: "snowball", Name: "Snowball", Desc: "A perfectly packed snowball.", Probability: 0.8, CountMin: 1, CountMax: 2},
		},
		"caves": {
			{ItemID: "gravel", Name: "Gravel", Desc: "Loose gravel.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "coal", Name: "Coal", Desc: "A lump of coal.", Probability: 0.5, CountMin: 1, CountMax: 2},
			{ItemID: "moss", Name: "Moss", Desc: "Damp cave moss.", Probability: 0.6, CountMin: 1, CountMax: 2},
		},
	}

	pool, ok := ambient[biome]
	if !ok {
		pool = ambient["meadow"]
	}

	yields := rollYield(db, pool, biome)
	if len(yields) == 0 {
		return Result{Output: "you search the area but find nothing useful."}
	}

	var b strings.Builder
	b.WriteString("you gather from the surroundings...\n")
	for _, y := range yields {
		player.AddItem(db, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		b.WriteString(fmt.Sprintf("  + %dx %s\n", y.CountMin, y.Name))
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Smelt converts ores to ingots using a furnace.
func Smelt(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "smelt <item-id> — requires a furnace and fuel (wood or coal)"}
	}

	// Check for furnace in room (world YAML builds or player-built).
	hasFurnace := false
	if room := w.Room(s.RoomID); room != nil {
		for _, sys := range room.Systems {
			if sys.ID == "furnace" {
				hasFurnace = true
				break
			}
		}
	}
	if !hasFurnace {
		// Check builds table.
		var cnt int
		db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=? AND build_id='furnace'`, s.RoomID).Scan(&cnt) //nolint:errcheck
		hasFurnace = cnt > 0
	}
	if !hasFurnace {
		return Result{Output: "you need a furnace to smelt. build one with 'build furnace' or find one."}
	}

	// Check fuel.
	invIDs := inventoryIDs(db)
	fuel := ""
	for _, id := range invIDs {
		if id == "coal" || id == "wood-log" || id == "charcoal" {
			fuel = id
			break
		}
	}
	if fuel == "" {
		return Result{Output: "you need fuel (coal, wood-log, or charcoal) to smelt."}
	}

	itemID := strings.ToLower(args[0])
	smeltMap := map[string][2]string{
		"iron-ore":  {"iron-ingot", "Iron Ingot"},
		"gold-ore":  {"gold-ingot", "Gold Ingot"},
		"sand":      {"glass", "Glass"},
		"clay":      {"brick", "Brick"},
		"coal-ore":  {"coal", "Coal"},
		"wood-log":  {"charcoal", "Charcoal"},
	}

	result, ok := smeltMap[itemID]
	if !ok {
		return Result{Output: fmt.Sprintf("%s cannot be smelted.", itemID)}
	}

	// Check item in inventory.
	hasItem := false
	for _, id := range invIDs {
		if id == itemID {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return Result{Output: fmt.Sprintf("you don't have %s.", itemID)}
	}

	player.RemoveItem(db, itemID) //nolint:errcheck
	player.RemoveItem(db, fuel)   //nolint:errcheck
	player.AddItem(db, result[0], result[1], fmt.Sprintf("Smelted from %s.", itemID)) //nolint:errcheck
	bumpActions(db)

	return Result{Output: fmt.Sprintf(
		"you feed the furnace with %s and smelt the %s.\nyou receive: 1x %s.",
		fuel, itemID, result[1],
	)}
}

// Plant plants a seed in the current room.
func Plant(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "plant <seed-id>"}
	}

	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "you can't plant here."}
	}

	// Must be meadow biome or have garden-plot build.
	canPlant := room.Biome == "meadow"
	if !canPlant {
		var cnt int
		db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=? AND build_id='garden-plot'`, s.RoomID).Scan(&cnt) //nolint:errcheck
		canPlant = cnt > 0
	}
	if !canPlant {
		return Result{Output: "you need farmland to plant. try a meadow room or build a garden-plot."}
	}

	seedID := strings.ToLower(args[0])
	invIDs := inventoryIDs(db)
	hasSeed := false
	for _, id := range invIDs {
		if id == seedID {
			hasSeed = true
			break
		}
	}
	if !hasSeed {
		return Result{Output: fmt.Sprintf("you don't have %s.", seedID)}
	}

	// Find grow_actions from world resources.
	growActions := 15 // default
	for _, r := range room.Resources {
		if r.Type == "plant" && r.ID == seedID {
			if r.GrowActions > 0 {
				growActions = r.GrowActions
			}
			break
		}
	}

	current := actionCount(db)
	// Find next open slot (max 4 crops per room).
	var slot int
	db.QueryRow(`SELECT COALESCE(MAX(slot)+1, 0) FROM crops WHERE room_id=? AND harvested=0`, s.RoomID).Scan(&slot) //nolint:errcheck
	if slot >= 4 {
		return Result{Output: "the farmland is full. harvest some crops first."}
	}

	player.RemoveItem(db, seedID) //nolint:errcheck
	db.Exec( //nolint:errcheck
		`INSERT INTO crops (room_id, slot, seed_id, planted_at_action, ready_at_action) VALUES (?,?,?,?,?)`,
		s.RoomID, slot, seedID, current, current+growActions,
	)
	bumpActions(db)

	return Result{Output: fmt.Sprintf(
		"you plant the %s in the soil. it will be ready to harvest in about %d actions.",
		seedID, growActions,
	)}
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/commands/mining.go
git commit -m "feat(commands): add mine, harvest, gather, smelt, plant"
```

---

## Task 11: Build + Stash + Unstash Commands

**Files:**
- Create: `internal/commands/building.go`

- [ ] **Step 1: Create the file**

Create `internal/commands/building.go`:

```go
package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["build"]   = Build
	Registry["stash"]   = Stash
	Registry["unstash"] = Unstash
}

// Build constructs a structure in the current room using world crafting recipes
// tagged with workbench type "build".
func Build(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		// List available build recipes.
		invIDs := inventoryIDs(db)
		invSet := make(map[string]bool, len(invIDs))
		for _, id := range invIDs {
			invSet[id] = true
		}
		var b strings.Builder
		b.WriteString("build recipes:\n")
		found := false
		for _, r := range w.CraftingRecipes {
			if r.Workbench != "build" {
				continue
			}
			found = true
			affordable := true
			for _, ing := range r.Ingredients {
				if !invSet[ing.ID] {
					affordable = false
					break
				}
			}
			suffix := ""
			if !affordable {
				suffix = " (need more materials)"
			}
			b.WriteString(fmt.Sprintf("  %s — %s%s\n", r.ID, r.Name, suffix))
		}
		if !found {
			return Result{Output: "no build recipes available in this world."}
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	recipeID := strings.ToLower(args[0])
	var recipe *world.CraftingRecipe
	for i := range w.CraftingRecipes {
		if w.CraftingRecipes[i].ID == recipeID && w.CraftingRecipes[i].Workbench == "build" {
			recipe = &w.CraftingRecipes[i]
			break
		}
	}
	if recipe == nil {
		return Result{Output: fmt.Sprintf("no build recipe %q.", recipeID)}
	}

	// Check ingredients.
	invIDs := inventoryIDs(db)
	invSet := make(map[string]bool, len(invIDs))
	for _, id := range invIDs {
		invSet[id] = true
	}
	for _, ing := range recipe.Ingredients {
		if !invSet[ing.ID] {
			return Result{Output: fmt.Sprintf("you need %dx %s.", ing.Count, ing.ID)}
		}
	}

	// Consume ingredients.
	for _, ing := range recipe.Ingredients {
		for i := 0; i < ing.Count; i++ {
			player.RemoveItem(db, ing.ID) //nolint:errcheck
		}
	}

	// Place build in room.
	current := actionCount(db)
	db.Exec( //nolint:errcheck
		`INSERT INTO builds (room_id, build_id, name, desc, placed_at) VALUES (?,?,?,?,?)`,
		s.RoomID, recipe.ID, recipe.Name, recipe.Output.Desc, current,
	)
	bumpActions(db)

	unlocks := buildUnlockMessage(recipe.ID)
	return Result{Output: fmt.Sprintf("you build a %s.%s", recipe.Name, unlocks)}
}

func buildUnlockMessage(buildID string) string {
	switch buildID {
	case "workbench":
		return "\nthe workbench unlocks advanced crafting recipes."
	case "furnace":
		return "\nthe furnace is ready. use 'smelt <ore>' to process materials."
	case "enchanting-table":
		return "\nthe enchanting table glows softly. use 'enchant <item>' to enchant your gear."
	case "chest":
		return "\na chest sits in the corner. use 'stash <item>' to store items."
	case "garden-plot":
		return "\nfertile soil is ready. use 'plant <seed>' to grow crops."
	}
	return ""
}

// Stash puts an item from inventory into a chest in the current room.
func Stash(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "stash <item-id> — store an item in the room's chest"}
	}

	// Check for chest.
	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=? AND build_id='chest'`, s.RoomID).Scan(&cnt) //nolint:errcheck
	if cnt == 0 {
		return Result{Output: "there is no chest here. build one first."}
	}

	itemID := strings.ToLower(args[0])
	invIDs := inventoryIDs(db)
	var found player.InventoryItem
	for _, it := range func() []player.InventoryItem {
		items, _ := player.Inventory(db)
		return items
	}() {
		if it.ID == itemID {
			found = it
			break
		}
	}
	if found.ID == "" {
		_ = invIDs
		return Result{Output: fmt.Sprintf("you don't have %q.", itemID)}
	}

	player.RemoveItem(db, itemID) //nolint:errcheck
	db.Exec( //nolint:errcheck
		`INSERT OR IGNORE INTO chests (room_id, item_id, item_name, item_desc) VALUES (?,?,?,?)`,
		s.RoomID, found.ID, found.Name, found.Desc,
	)
	return Result{Output: fmt.Sprintf("you store %s in the chest.", found.Name)}
}

// Unstash retrieves an item from the chest in the current room.
func Unstash(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	// Check for chest.
	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=? AND build_id='chest'`, s.RoomID).Scan(&cnt) //nolint:errcheck
	if cnt == 0 {
		return Result{Output: "there is no chest here."}
	}

	if len(args) == 0 {
		// List chest contents.
		rows, err := db.Query(`SELECT item_id, item_name FROM chests WHERE room_id=?`, s.RoomID)
		if err != nil || rows == nil {
			return Result{Output: "the chest is empty."}
		}
		defer rows.Close()
		var b strings.Builder
		b.WriteString("chest contents:\n")
		found := false
		for rows.Next() {
			var id, name string
			rows.Scan(&id, &name) //nolint:errcheck
			b.WriteString(fmt.Sprintf("  %s (%s)\n", name, id))
			found = true
		}
		if !found {
			return Result{Output: "the chest is empty."}
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	itemID := strings.ToLower(args[0])
	var name, desc string
	err := db.QueryRow(`SELECT item_name, item_desc FROM chests WHERE room_id=? AND item_id=?`, s.RoomID, itemID).
		Scan(&name, &desc)
	if err != nil {
		return Result{Output: fmt.Sprintf("no %q in the chest.", itemID)}
	}

	db.Exec(`DELETE FROM chests WHERE room_id=? AND item_id=?`, s.RoomID, itemID) //nolint:errcheck
	player.AddItem(db, itemID, name, desc)                                         //nolint:errcheck
	return Result{Output: fmt.Sprintf("you take %s from the chest.", name)}
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/commands/building.go
git commit -m "feat(commands): add build, stash, unstash"
```

---

## Task 12: Enchant Command

**Files:**
- Create: `internal/commands/enchant_cmd.go`

- [ ] **Step 1: Create the file**

Create `internal/commands/enchant_cmd.go`:

```go
package commands

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/enchanting"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["enchant"] = Enchant
}

// itemCategory guesses the enchant category of an item by ID conventions.
// sword/axe/pickaxe/boots suffixes.
func itemCategory(itemID string) string {
	switch {
	case strings.Contains(itemID, "sword"):
		return "sword"
	case strings.Contains(itemID, "pickaxe"):
		return "pickaxe"
	case strings.Contains(itemID, "axe"):
		return "axe"
	case strings.Contains(itemID, "boots"):
		return "boots"
	default:
		return "any"
	}
}

// Enchant enchants an item using the enchanting table.
func Enchant(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	// Check for enchanting table in room.
	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=? AND build_id='enchanting-table'`, s.RoomID).Scan(&cnt) //nolint:errcheck
	if cnt == 0 {
		return Result{Output: "you need an enchanting table. build one with 'build enchanting-table'."}
	}

	xp, level, _ := enchanting.XPState(db)

	if len(args) == 0 {
		return Result{Output: fmt.Sprintf(
			"enchanting level: %d (XP: %d)\nusage: enchant <item-id>", level, xp,
		)}
	}

	itemID := strings.ToLower(args[0])
	invIDs := inventoryIDs(db)
	hasItem := false
	for _, id := range invIDs {
		if id == itemID {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return Result{Output: fmt.Sprintf("you don't have %q.", itemID)}
	}

	if level < 1 {
		return Result{Output: "you need at least level 1 enchanting XP. earn XP by mining, fighting, and questing."}
	}

	// Pick a random available enchant for this item type.
	category := itemCategory(itemID)
	available := enchanting.AvailableForItemType(category)
	if len(available) == 0 {
		return Result{Output: fmt.Sprintf("no enchantments available for %s.", itemID)}
	}

	// Scale enchant level by player enchanting level (1-10 → tier 1, 11-20 → tier 2, 21+ → tier 3).
	enchantLevel := 1
	if level >= 20 {
		enchantLevel = 3
	} else if level >= 10 {
		enchantLevel = 2
	}

	chosenID := available[rand.Intn(len(available))]

	// Spend XP (10 per enchant level).
	xpCost := enchantLevel * 10
	if xp < xpCost {
		return Result{Output: fmt.Sprintf(
			"not enough enchanting XP. need %d, have %d.", xpCost, xp,
		)}
	}

	current := actionCount(db)
	enchanting.Apply(db, itemID, chosenID, enchantLevel, current) //nolint:errcheck
	db.Exec(`UPDATE enchanting_xp SET xp=xp-? WHERE id=1`, xpCost) //nolint:errcheck

	name := enchanting.EnchantName(chosenID, enchantLevel)
	return Result{Output: fmt.Sprintf(
		"the enchanting table glows...\nyour %s has been enchanted with %s! (-%d XP)",
		itemID, name, xpCost,
	)}
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/commands/enchant_cmd.go
git commit -m "feat(commands): add enchant"
```

---

## Task 13: Weather + Deathpile Commands

**Files:**
- Create: `internal/commands/weather_cmd.go`
- Create: `internal/commands/deathpile_cmd.go`

- [ ] **Step 1: Create weather_cmd.go**

```go
package commands

import (
	"database/sql"
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/weather"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["weather"] = Weather
}

// Weather shows current weather in the player's biome.
func Weather(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	biome := "meadow"
	if room != nil && room.Biome != "" {
		biome = room.Biome
	}

	// Find weather table entry for this biome.
	var possible []string
	for _, wt := range w.WeatherTable {
		if wt.Biome == biome {
			possible = wt.Possible
			break
		}
	}
	if len(possible) == 0 {
		possible = []string{"clear"}
	}

	current := actionCount(db)
	cond, err := weather.Tick(db, biome, current, possible)
	if err != nil {
		return Result{Output: "unable to check weather."}
	}

	desc := weather.Description(biome, cond)
	bonus := weather.YieldBonus(cond)
	bonusStr := ""
	if bonus > 1.0 {
		bonusStr = fmt.Sprintf(" (+%.0f%% yield)", (bonus-1.0)*100)
	}

	return Result{Output: fmt.Sprintf("[%s — %s]\n%s%s", biome, cond, desc, bonusStr)}
}
```

- [ ] **Step 2: Create deathpile_cmd.go**

```go
package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["deathpile"] = Deathpile
}

// Deathpile shows the location and contents of the player's last death pile.
func Deathpile(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	roomID, count := player.AnyDeathPile(db)
	if roomID == "" || count == 0 {
		return Result{Output: "you have no death pile."}
	}

	roomName := roomID
	if r := w.Room(roomID); r != nil {
		roomName = r.Name
	}

	pile, err := player.GetDeathPile(db, roomID)
	if err != nil || len(pile) == 0 {
		return Result{Output: "you have no death pile."}
	}

	names := make([]string, len(pile))
	for i, it := range pile {
		names[i] = it.Name
	}

	return Result{Output: fmt.Sprintf(
		"your death pile is at: %s (%s)\nitems: %s\ntravel there and use 'take death-pile' to recover them.",
		roomName, roomID, strings.Join(names, ", "),
	)}
}
```

- [ ] **Step 3: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/commands/weather_cmd.go internal/commands/deathpile_cmd.go
git commit -m "feat(commands): add weather, deathpile"
```

---

## Task 14: has_all_shards Dialogue Trigger

**Files:**
- Modify: `internal/espionage/espionage.go`

- [ ] **Step 1: Add AllShardsCollected to PlayerContext**

In `internal/espionage/espionage.go`, add `AllShardsCollected bool` to the `PlayerContext` struct:

```go
// PlayerContext holds the info needed to evaluate dialogue triggers.
type PlayerContext struct {
	InventoryIDs       []string
	Reputation         map[string]int // faction → rep value
	Skills             map[string]int // skill → level
	Disguise           string
	AllShardsCollected bool // true when all crystal_shards rows have collected=1
}
```

- [ ] **Step 2: Add has_all_shards case to matchTrigger**

In `matchTrigger`, add before the final `return false`:

```go
case trigger == "has_all_shards":
    return ctx.AllShardsCollected
```

- [ ] **Step 3: Populate AllShardsCollected in commands.go Talk handler**

In the `Talk` function in `internal/commands/commands.go`, the `PlayerContext` is built around line 910–916. Add after building the `ctx`:

```go
// Check if all 5 crystal shards are collected.
var shardCount, totalShards int
db.QueryRow(`SELECT COUNT(*) FROM crystal_shards WHERE collected=1`).Scan(&shardCount)   //nolint:errcheck
db.QueryRow(`SELECT COUNT(*) FROM crystal_shards`).Scan(&totalShards)                    //nolint:errcheck
ctx.AllShardsCollected = totalShards >= 5 && shardCount >= 5
```

- [ ] **Step 4: Add test for has_all_shards trigger**

In `internal/espionage/espionage_test.go`, add:

```go
func TestHasAllShardsTrigger(t *testing.T) {
	line := world.DialogueLine{Trigger: "has_all_shards", Text: "All shards collected!"}
	
	ctxNo := espionage.PlayerContext{AllShardsCollected: false}
	if espionage.EvalDialogue([]world.DialogueLine{line}, ctxNo) == "All shards collected!" {
		t.Error("should not match when AllShardsCollected=false")
	}

	ctxYes := espionage.PlayerContext{AllShardsCollected: true}
	if got := espionage.EvalDialogue([]world.DialogueLine{line}, ctxYes); got != "All shards collected!" {
		t.Errorf("should match, got %q", got)
	}
}
```

Wait — `EvalDialogue` and `PlayerContext` are in `package espionage`, so the test file uses `package espionage` (not `_test`). Change the test to use the internal package:

```go
func TestHasAllShardsTrigger(t *testing.T) {
	line := world.DialogueLine{Trigger: "has_all_shards", Text: "All shards collected!"}
	
	ctxNo := PlayerContext{AllShardsCollected: false}
	if EvalDialogue([]world.DialogueLine{line}, ctxNo) == "All shards collected!" {
		t.Error("should not match when AllShardsCollected=false")
	}

	ctxYes := PlayerContext{AllShardsCollected: true}
	if got := EvalDialogue([]world.DialogueLine{line}, ctxYes); got != "All shards collected!" {
		t.Errorf("should match, got %q", got)
	}
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/espionage/... -v
```

Expected: all tests PASS including new `TestHasAllShardsTrigger`.

- [ ] **Step 6: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/espionage/espionage.go internal/espionage/espionage_test.go internal/commands/commands.go
git commit -m "feat(espionage): add has_all_shards dialogue trigger"
```

---

## Task 15: Blockhaven world.yaml

**Files:**
- Create: `worlds/blockhaven/world.yaml`

- [ ] **Step 1: Create the worlds/blockhaven directory**

```bash
mkdir -p /Users/stokes/Projects/gl1tch-mud/worlds/blockhaven
```

- [ ] **Step 2: Create world.yaml**

Create `worlds/blockhaven/world.yaml` with the following content. This is the canonical initial world file — idempotent pipelines will expand it later.

```yaml
name: blockhaven
start_room: meadow-0
narrator_model: claude-haiku-4-5-20251001

weather_table:
  - biome: meadow
    possible: [clear, rainy, windy, stormy]
  - biome: forest
    possible: [clear, rainy, foggy]
  - biome: desert
    possible: [clear, sandstorm, scorching]
  - biome: snow
    possible: [clear, light-snow, blizzard]
  - biome: caves
    possible: [clear, damp, tremor]

factions:
  - id: stoneguard
    name: "The Stoneguard"
    desc: |
      Master builders and craftspeople who maintain Meadow Town. They remember the Crystal Core
      best — and feel its absence most keenly in the slow wilting of the meadow flowers.
    agenda: "Restore the Crystal Core and rebuild Meadow Town to its former glory."
    territory: [meadow-0, meadow-1]
    allies: [thornwalkers, dunekeepers, frostborn, deepborn]
    enemies: []

  - id: thornwalkers
    name: "The Thornwalkers"
    desc: |
      Rangers and herbalists who walk the ancient forest paths. They speak to the trees,
      and the trees have been very quiet since the Crystal Core shattered.
    agenda: "Protect the Forest and restore the ancient trees to full bloom."
    territory: [forest-0, forest-1]
    allies: [stoneguard]
    enemies: []

  - id: dunekeepers
    name: "The Dunekeepers"
    desc: |
      Archaeologists and puzzle-solvers who comb the desert ruins for lost knowledge.
      They believe the Crystal Core shards hold clues to even older mysteries.
    agenda: "Unlock the secrets buried in the desert ruins before the sand reclaims them."
    territory: [desert-0, desert-1]
    allies: [stoneguard]
    enemies: []

  - id: frostborn
    name: "The Frostborn"
    desc: |
      Hardy mountain miners and smiths who carve their homes from the Snow Peaks.
      The blizzards have worsened since Cinder fled here, and the Frostborn are not pleased.
    agenda: "Forge the tools to fix what was broken. Find the dragon. Get answers."
    territory: [snow-0, snow-1]
    allies: [stoneguard, deepborn]
    enemies: []

  - id: deepborn
    name: "The Deepborn"
    desc: |
      Ancient cave dwellers who have lived underground since before memory. They were
      the ones who first built the Crystal Core's foundation. They know how to fix it.
    agenda: "Guide the right person to the Crystal Cavern and restore what was lost."
    territory: [caves-0, caves-1]
    allies: [stoneguard, frostborn]
    enemies: []

loot_tables:
  - id: stoneling-loot
    entries:
      - item_id: stone
        name: "Stone"
        desc: "A rough stone block."
        probability: 0.9
        count_min: 1
        count_max: 3
      - item_id: flint
        name: "Flint"
        desc: "A sharp piece of flint."
        probability: 0.5
        count_min: 1
        count_max: 2
      - item_id: meadow-shard-fragment
        name: "Glowing Fragment"
        desc: "A warm shard that pulses with soft light. Part of something larger."
        probability: 0.05
        count_min: 1
        count_max: 1

  - id: raider-loot
    entries:
      - item_id: iron-ore
        name: "Iron Ore"
        desc: "Raw iron ore. Smelt it into an ingot."
        probability: 0.6
        count_min: 1
        count_max: 2
      - item_id: bread
        name: "Bread"
        desc: "Restores 20 HP when eaten."
        probability: 0.7
        count_min: 1
        count_max: 2

  - id: thornsprite-loot
    entries:
      - item_id: seed
        name: "Forest Seed"
        desc: "A seed from the ancient forest. Plant it in fertile soil."
        probability: 0.8
        count_min: 1
        count_max: 3
      - item_id: leaf
        name: "Leaf"
        desc: "A large forest leaf."
        probability: 0.9
        count_min: 1
        count_max: 4

  - id: vine-creeper-loot
    entries:
      - item_id: vine
        name: "Vine"
        desc: "Strong and flexible. Used in crafting."
        probability: 0.8
        count_min: 1
        count_max: 3
      - item_id: root
        name: "Root"
        desc: "A gnarled tree root."
        probability: 0.6
        count_min: 1
        count_max: 2

  - id: sand-golem-loot
    entries:
      - item_id: sand
        name: "Sand"
        desc: "Fine desert sand."
        probability: 0.9
        count_min: 2
        count_max: 5
      - item_id: sandstone
        name: "Sandstone"
        desc: "Compressed sandstone block."
        probability: 0.7
        count_min: 1
        count_max: 3
      - item_id: gold-ore
        name: "Gold Ore"
        desc: "Raw gold ore glittering in the sunlight."
        probability: 0.1
        count_min: 1
        count_max: 1

  - id: frost-wraith-loot
    entries:
      - item_id: frost-essence
        name: "Frost Essence"
        desc: "A cold, shimmering vial of frozen energy. Used in the Frozen Forge quest."
        probability: 0.6
        count_min: 1
        count_max: 1
      - item_id: ice-shard
        name: "Ice Shard"
        desc: "A sharp shard of magical ice."
        probability: 0.8
        count_min: 1
        count_max: 2

  - id: cave-lurker-loot
    entries:
      - item_id: coal
        name: "Coal"
        desc: "A lump of coal. Burns as fuel."
        probability: 0.8
        count_min: 1
        count_max: 3
      - item_id: iron-ore
        name: "Iron Ore"
        desc: "Raw iron ore."
        probability: 0.5
        count_min: 1
        count_max: 2

  - id: crystal-shade-loot
    entries:
      - item_id: glowstone
        name: "Glowstone"
        desc: "A warm, softly glowing stone."
        probability: 0.7
        count_min: 1
        count_max: 2
      - item_id: crystal-fragment
        name: "Crystal Fragment"
        desc: "A piece of ancient crystal. The Deepborn would want to see this."
        probability: 0.4
        count_min: 1
        count_max: 1

crafting_recipes:
  - id: wooden-pickaxe
    name: "Wooden Pickaxe"
    ingredients:
      - {id: wood-log, count: 3}
      - {id: stick, count: 2}
    output:
      id: wooden-pickaxe
      name: "Wooden Pickaxe"
      desc: "A basic pickaxe. Required for mining stone and ore."
    skill_req: 0

  - id: wooden-sword
    name: "Wooden Sword"
    ingredients:
      - {id: wood-log, count: 2}
      - {id: stick, count: 1}
    output:
      id: wooden-sword
      name: "Wooden Sword"
      desc: "A basic sword. 5 attack."
    skill_req: 0

  - id: stone-pickaxe
    name: "Stone Pickaxe"
    ingredients:
      - {id: stone, count: 3}
      - {id: stick, count: 2}
    output:
      id: stone-pickaxe
      name: "Stone Pickaxe"
      desc: "A stone pickaxe. Mines faster than wood."
    skill_req: 0

  - id: iron-pickaxe
    name: "Iron Pickaxe"
    ingredients:
      - {id: iron-ingot, count: 3}
      - {id: stick, count: 2}
    output:
      id: iron-pickaxe
      name: "Iron Pickaxe"
      desc: "An iron pickaxe. Can mine gold and rare ores."
    skill_req: 0
    workbench: workbench

  - id: iron-sword
    name: "Iron Sword"
    ingredients:
      - {id: iron-ingot, count: 2}
      - {id: stick, count: 1}
    output:
      id: iron-sword
      name: "Iron Sword"
      desc: "A sharp iron sword. 12 attack."
    skill_req: 0
    workbench: workbench

  - id: frostcore-ingot
    name: "Frostcore Ingot"
    ingredients:
      - {id: frost-essence, count: 1}
      - {id: iron-ingot, count: 1}
    output:
      id: frostcore-ingot
      name: "Frostcore Ingot"
      desc: "A shimmering ingot of iron and frost magic. Required for the Mountain Shard quest."
    skill_req: 0
    workbench: furnace

  # Build recipes (workbench: build)
  - id: workbench
    name: "Workbench"
    ingredients:
      - {id: wood-log, count: 4}
    output:
      id: workbench
      name: "Workbench"
      desc: "A sturdy workbench. Unlocks advanced crafting recipes."
    workbench: build

  - id: furnace
    name: "Furnace"
    ingredients:
      - {id: stone, count: 8}
    output:
      id: furnace
      name: "Furnace"
      desc: "A stone furnace. Used to smelt ores."
    workbench: build

  - id: enchanting-table
    name: "Enchanting Table"
    ingredients:
      - {id: diamond, count: 2}
      - {id: obsidian, count: 4}
      - {id: wood-log, count: 2}
    output:
      id: enchanting-table
      name: "Enchanting Table"
      desc: "A glowing table of ancient magic. Used to enchant items."
    workbench: build

  - id: chest
    name: "Chest"
    ingredients:
      - {id: wood-log, count: 8}
    output:
      id: chest
      name: "Chest"
      desc: "A wooden storage chest."
    workbench: build

  - id: garden-plot
    name: "Garden Plot"
    ingredients:
      - {id: dirt, count: 4}
      - {id: stick, count: 2}
    output:
      id: garden-plot
      name: "Garden Plot"
      desc: "A small plot of fertile soil for growing crops."
    workbench: build

quests:
  - id: q-meadow-shard
    title: "The Wilting Meadow"
    description: |
      Elder Mason says the Meadow Shard landed somewhere in the meadow fields — likely carried
      by the Stoneling Chieftain, who has been unusually aggressive since the Core shattered.
      Defeat the Stoneling Chieftain and retrieve the Meadow Shard.
    giver_npc_id: elder-mason
    obj_type: kill
    obj_target: stoneling-chieftain
    obj_count: 1
    reward_credits: 150
    reward_xp_skill: combat
    reward_xp_amount: 30
    reward_item_id: meadow-shard
    reward_item_name: "Meadow Shard"
    reward_item_desc: "A warm, flower-scented shard of the Crystal Core. One of five."

  - id: q-forest-shard
    title: "Root of the Problem"
    description: |
      Warden Sylara says the Forest Shard sank into the Ancient Root Vein beneath the oldest oak.
      Mine it out — but bring a strong pickaxe.
    giver_npc_id: warden-sylara
    obj_type: mine
    obj_target: ancient-root-vein
    obj_count: 1
    reward_credits: 120
    reward_xp_skill: mining
    reward_xp_amount: 25
    reward_item_id: forest-shard
    reward_item_name: "Forest Shard"
    reward_item_desc: "A green, leaf-scented shard of the Crystal Core. One of five."

  - id: q-desert-shard
    title: "Lost to the Sand"
    description: |
      Archivist Dunes says the Desert Shard is sealed inside the Sandstone Ruins. The ancient
      lock can only be opened by solving the ruin's security terminal.
    giver_npc_id: archivist-dunes
    obj_type: hack
    obj_target: ruin-terminal
    obj_count: 1
    reward_credits: 200
    reward_xp_skill: hacking
    reward_xp_amount: 30
    reward_item_id: desert-shard
    reward_item_name: "Desert Shard"
    reward_item_desc: "A sun-warm shard of the Crystal Core. One of five."

  - id: q-mountain-shard
    title: "The Frozen Forge"
    description: |
      Ironmaster Breck says the Mountain Shard is embedded in a block of magical ice deep in the
      Snow Peaks. You need to forge a Frostcore Ingot to unlock it. Smelt frost-essence with
      iron-ingot in a furnace to create the ingot.
    giver_npc_id: ironmaster-breck
    obj_type: retrieve
    obj_target: frostcore-ingot
    obj_count: 1
    reward_credits: 175
    reward_xp_skill: mining
    reward_xp_amount: 25
    reward_item_id: mountain-shard
    reward_item_name: "Mountain Shard"
    reward_item_desc: "A cold, frost-kissed shard of the Crystal Core. One of five."

  - id: q-cave-shard
    title: "Into the Dark"
    description: |
      Elder Voss says the Cave Shard rests in the Crystal Cavern at the deepest point of the
      caves. The Deepborn have been guarding it since the Core shattered. Re-light the Crystal
      Flame to claim the shard.
    giver_npc_id: elder-voss
    obj_type: hack
    obj_target: flame-terminal
    obj_count: 1
    reward_credits: 250
    reward_xp_skill: hacking
    reward_xp_amount: 30
    reward_item_id: cave-shard
    reward_item_name: "Cave Shard"
    reward_item_desc: "A deep-glowing shard of the Crystal Core. One of five."

  - id: q-cinder-return
    title: "Cinder's Return"
    description: |
      Elder Mason says a golden glow has appeared in the Snow Peaks — Cinder's hiding place.
      You carry all five Crystal Shards. Travel to Cinder's Hideaway and convince the dragon
      to help restore the Crystal Core.
    giver_npc_id: elder-mason
    obj_type: retrieve
    obj_target: crystal-key
    obj_count: 1
    reward_credits: 500
    reward_xp_skill: combat
    reward_xp_amount: 100
    reward_item_id: crystal-key
    reward_item_name: "Crystal Key"
    reward_item_desc: "A key formed from the warmth of five Crystal Shards. Opens the path to Cinder."

rooms:
  - id: meadow-0
    name: "Meadow Town Square"
    biome: meadow
    desc: |
      The heart of Meadow Town. Cobblestone paths wind between cheerful buildings, though
      the flower beds that once lined every wall are wilted and grey. At the center stands
      a stone pedestal — empty where the Crystal Core once floated. Elder Mason paces near it,
      his brow furrowed.
    exits:
      north: meadow-1
      east: forest-0
      south: desert-0
      west: snow-0
    npcs:
      - id: elder-mason
        name: "Elder Mason"
        hp: 0
        attack: 0
        desc: "The leader of the Stoneguard. An old dwarf with calloused hands and worried eyes."
        dialogue:
          - trigger: always
            text: |
              "Welcome to Meadow Town, traveller. The Crystal Core is gone — shattered by a young
              dragon named Cinder. Without it, our world is fading. Will you help us restore it?"
            quest_id: q-meadow-shard
          - trigger: has_all_shards
            text: |
              "You have all five shards. A golden glow appeared in the Snow Peaks just this morning.
              I believe Cinder is there. Go — and bring those shards with you."
            quest_id: q-cinder-return
      - id: stoneling-chieftain
        name: "Stoneling Chieftain"
        hp: 60
        attack: 12
        desc: "A hulking stoneling, larger than the rest. Its eyes glow faintly — like it swallowed something magical."
        loot_table_id: stoneling-loot
        dialogue: []
    items:
      - id: builders-map
        name: "Builder's Map"
        desc: "A hand-drawn map of Blockhaven with notes on all five biomes."
        readable: true
        content: |
          BLOCKHAVEN — A TRAVELLER'S GUIDE

          The Crystal Core once floated above Meadow Town Square, giving each biome its colour
          and life. A young dragon named Cinder accidentally shattered it. Five Crystal Shards
          scattered to the five biomes.

          THE FIVE BIOMES:
          — Meadow (north): Home of the Stoneguard builders. The wilting has begun.
          — Forest (east): Home of the Thornwalkers. Ancient trees are going dormant.
          — Desert (south): Home of the Dunekeepers. Ruins are sinking fast.
          — Snow Peaks (west): Home of the Frostborn. Cinder fled here.
          — Deep Caves (down from caves-0): Home of the Deepborn. The dark is spreading.

          YOUR GOAL: Collect all five Crystal Shards. Find Cinder. Restore the Core.

          NEW COMMANDS: mine, harvest, gather, smelt, plant, build, enchant, weather, deathpile
          If you die: your items drop where you fell. Use 'deathpile' to find them.
    resources: []

  - id: meadow-1
    name: "The Builder's Workshop"
    biome: meadow
    desc: |
      A wide workshop filled with the smell of sawdust and coal smoke. A stone furnace
      glows steadily in one corner, and a solid workbench dominates the center of the room.
      Apprentice Brix is sorting tool handles when you enter.
    exits:
      south: meadow-0
    npcs:
      - id: apprentice-brix
        name: "Apprentice Brix"
        hp: 0
        attack: 0
        desc: "A young Stoneguard apprentice. Enthusiastic and helpful, even on bad days."
        trades:
          - id: brix-trade-1
            wants: [{id: wood-log, count: 4}]
            offers: [{id: stick, name: "Stick", desc: "A sturdy stick.", count: 8}]
          - id: brix-trade-2
            wants: [{id: iron-ingot, count: 2}]
            offers: [{id: iron-sword, name: "Iron Sword", desc: "A sharp iron sword. 12 attack.", count: 1}]
        dialogue:
          - trigger: always
            text: "The workbench is yours to use! Type 'craft' to see what you can make. You'll need a 'build workbench' first if you're out in the field."
          - trigger: skill_gte:mining:2
            text: "Nice pickaxe work. You're getting good at this. Ask me about iron gear when you have the ingots."
    items:
      - id: coal-stack
        name: "Coal Stack"
        desc: "A small pile of coal for the furnace."
        signal_tier: noise
    resources:
      - id: stone-deposits
        type: mine
        yields:
          - item_id: stone
            name: "Stone"
            desc: "A rough stone block."
            probability: 0.9
            count_min: 2
            count_max: 4
          - item_id: flint
            name: "Flint"
            desc: "Sharp flint."
            probability: 0.4
            count_min: 1
            count_max: 2
        tool_required: pickaxe
        respawn_actions: 15
    systems:
      - id: furnace
        security_level: 0
        reward_text: "The furnace is already lit. Use 'smelt <ore>' to process materials."

  - id: forest-0
    name: "Forest Entrance"
    biome: forest
    desc: |
      The forest begins here — a wall of ancient trees whose canopy filters the light to a
      cool green. The Thornwalkers keep a small patrol post at the treeline. Warden Sylara
      stands with her arms crossed, watching the trees with concern.
    exits:
      west: meadow-0
      east: forest-1
    npcs:
      - id: warden-sylara
        name: "Warden Sylara"
        hp: 0
        attack: 0
        desc: "Tall, lean, and watchful. A Thornwalker ranger who has walked these paths for thirty years."
        dialogue:
          - trigger: always
            text: "The trees are barely breathing. The Forest Shard sank into the Ancient Root Vein in the oak grove to the east. I can't reach it — the roots are too thick. Can you mine it out?"
            quest_id: q-forest-shard
          - trigger: skill_gte:mining:1
            text: "You have a pickaxe. Good. The Ancient Root Vein is tough — a stone pickaxe or better will do it."
      - id: thornsprite
        name: "Thornsprite"
        hp: 15
        attack: 4
        desc: "A small creature made of twisted branches and thorns. Hostile but fragile."
        loot_table_id: thornsprite-loot
        dialogue: []
    items: []
    resources:
      - id: forest-herbs
        type: harvest
        yields:
          - item_id: herb
            name: "Forest Herb"
            desc: "A medicinal herb. Useful for crafting."
            probability: 0.8
            count_min: 1
            count_max: 3
        tool_required: ""
        respawn_actions: 10

  - id: forest-1
    name: "Ancient Oak Grove"
    biome: forest
    desc: |
      A cathedral of ancient oaks so tall their tops vanish into mist. Roots thick as houses
      twist across the ground. At the base of the largest oak, a faint golden glow pulses
      from deep within the roots — the Forest Shard. A Vine Creeper coils nearby.
    exits:
      west: forest-0
    npcs:
      - id: vine-creeper
        name: "Vine Creeper"
        hp: 25
        attack: 7
        desc: "A mass of animated vines with glowing green eyes. It guards the old oak."
        loot_table_id: vine-creeper-loot
        dialogue: []
    items: []
    resources:
      - id: ancient-oak
        type: harvest
        yields:
          - item_id: wood-log
            name: "Wood Log"
            desc: "A thick log from an ancient oak."
            probability: 1.0
            count_min: 2
            count_max: 4
          - item_id: rare-seed
            name: "Rare Seed"
            desc: "A seed from an ancient tree. Plant it to grow something special."
            probability: 0.2
            count_min: 1
            count_max: 1
        tool_required: axe
        respawn_actions: 30
      - id: ancient-root-vein
        type: mine
        yields:
          - item_id: forest-shard
            name: "Forest Shard"
            desc: "A green, leaf-scented shard of the Crystal Core. One of five."
            probability: 1.0
            count_min: 1
            count_max: 1
        tool_required: pickaxe
        respawn_actions: 9999

  - id: desert-0
    name: "Desert Gateway"
    biome: desert
    desc: |
      The meadow gives way to sand at a crumbling stone arch — an ancient marker that once
      read "Dunekeeper Territory." Half the inscription has been swallowed by a drifting dune.
      A Dunekeeper scout in amber robes watches the horizon with a brass spyglass.
    exits:
      north: meadow-0
      east: desert-1
    npcs:
      - id: dunekeeper-scout
        name: "Dunekeeper Scout"
        hp: 0
        attack: 0
        desc: "A lean Dunekeeper with sun-darkened skin and a sharp eye for trouble."
        dialogue:
          - trigger: always
            text: "The ruins are to the east. Archivist Dunes will want to speak with you. Watch out for Sand Golems — they've been restless since the Core shattered."
          - trigger: has_item:desert-shard
            text: "You found it! The Archivist will be overjoyed. Safe travels."
      - id: dune-stalker
        name: "Dune Stalker"
        hp: 20
        attack: 6
        desc: "A predatory desert creature that blends into the sand. Attacks without warning."
        loot_table_id: thornsprite-loot
        dialogue: []
    items: []
    resources:
      - id: desert-sand
        type: mine
        yields:
          - item_id: sand
            name: "Sand"
            desc: "Fine desert sand."
            probability: 1.0
            count_min: 3
            count_max: 6
        tool_required: ""
        respawn_actions: 5

  - id: desert-1
    name: "Sandstone Ruins"
    biome: desert
    desc: |
      Pillars of carved sandstone rise from the dunes — the remains of an ancient Dunekeeper
      archive. Most of the structure has sunk into the sand, but one sealed chamber still stands.
      Archivist Dunes crouches over a half-buried tablet. A Sand Golem stands motionless nearby,
      waiting.
    exits:
      west: desert-0
    npcs:
      - id: archivist-dunes
        name: "Archivist Dunes"
        hp: 0
        attack: 0
        desc: "An elderly Dunekeeper with ink-stained fingers and a mind full of ancient puzzles."
        dialogue:
          - trigger: always
            text: "The Desert Shard is sealed in the inner chamber. The ruin-terminal controls the lock — it was built to keep looters out. Can you solve it? The security level is only 2."
            quest_id: q-desert-shard
          - trigger: has_item:desert-shard
            text: "Incredible! You actually solved it. The shard is yours — for now. Use it to restore the Core."
      - id: sand-golem
        name: "Sand Golem"
        hp: 40
        attack: 10
        desc: "A towering construct of animated sandstone. Slow, but hits like a boulder."
        loot_table_id: sand-golem-loot
        dialogue: []
    items: []
    systems:
      - id: ruin-terminal
        security_level: 2
        reward_item: desert-shard
        reward_text: "The ancient lock clicks open. The Desert Shard tumbles out into your hands."

  - id: snow-0
    name: "Mountain Pass"
    biome: snow
    desc: |
      The path into the Snow Peaks winds upward through biting wind. Ice-covered stones
      line the pass, and a Frost Wraith circles in the grey sky above. To the west, a
      narrow trail leads deeper into the mountains. A heavy stone slab blocks a side passage
      — there's a faint golden glow underneath it.
    exits:
      east: meadow-0
      west: snow-1
    locks:
      - id: cinder-passage-lock
        exit: north
        difficulty: 0
        keys: [crystal-key]
    npcs:
      - id: frost-wraith-patrol
        name: "Frost Wraith"
        hp: 35
        attack: 12
        desc: "A spectral shape made of howling ice and wind. It attacks with frozen claws."
        loot_table_id: frost-wraith-loot
        dialogue: []
    items: []
    resources:
      - id: ice-vein
        type: mine
        yields:
          - item_id: ice
            name: "Ice"
            desc: "A clear chunk of magical ice."
            probability: 0.9
            count_min: 1
            count_max: 3
          - item_id: diamond
            name: "Diamond"
            desc: "A rare, brilliant diamond."
            probability: 0.05
            count_min: 1
            count_max: 1
        tool_required: pickaxe
        respawn_actions: 25

  - id: snow-1
    name: "Frost Village"
    biome: snow
    desc: |
      A cluster of stone-and-ice buildings carved into the mountainside. Fires burn in iron
      braziers, and the clang of hammer on anvil echoes from the forge. Ironmaster Breck stands
      at the largest anvil, a frown on his face and a Frostborn battle axe across his back.
    exits:
      east: snow-0
    npcs:
      - id: ironmaster-breck
        name: "Ironmaster Breck"
        hp: 0
        attack: 0
        desc: "A barrel-chested Frostborn smith. He trusts tools more than people, but respects both when they're good."
        trades:
          - id: breck-trade-1
            wants: [{id: iron-ingot, count: 3}]
            offers: [{id: iron-pickaxe, name: "Iron Pickaxe", desc: "An iron pickaxe. Can mine gold and rare ores.", count: 1}]
          - id: breck-trade-2
            wants: [{id: gold-ingot, count: 2}]
            offers: [{id: gold-sword, name: "Gold Sword", desc: "A gilded sword. 18 attack.", count: 1}]
        dialogue:
          - trigger: always
            text: "To forge the Frostcore Ingot you need frost-essence from a Frost Wraith, plus an iron ingot. Bring both to a furnace. That's how you claim the Mountain Shard."
            quest_id: q-mountain-shard
          - trigger: has_item:mountain-shard
            text: "So you did it. Good. The Mountain holds its shard no longer."
      - id: ice-crawler
        name: "Ice Crawler"
        hp: 20
        attack: 5
        desc: "A six-legged insect the size of a dog, armored in frost-crystal chitin."
        loot_table_id: frost-wraith-loot
        dialogue: []
    items: []
    systems:
      - id: forge-furnace
        security_level: 0
        reward_text: "The forge burns hot. Use 'smelt <ore>' to process materials."

  - id: caves-0
    name: "Cave Entrance"
    biome: caves
    desc: |
      A yawning opening in the earth, framed by hanging moss and the faint smell of minerals.
      The air gets cold fast and the light fades within a few steps. A Deepborn scout stands
      at the threshold, holding a glowstone lantern.
    exits:
      up: meadow-0
      down: caves-1
    npcs:
      - id: deepborn-scout
        name: "Deepborn Scout"
        hp: 0
        attack: 0
        desc: "A quiet, careful figure wrapped in cave-cloth. They move silently and speak rarely."
        dialogue:
          - trigger: always
            text: "The Crystal Cavern is below. Elder Voss waits for you. Bring a weapon — Cave Lurkers nest on the path."
          - trigger: has_item:cave-shard
            text: "The Crystal Flame burns again. Elder Voss will be pleased."
      - id: cave-lurker
        name: "Cave Lurker"
        hp: 30
        attack: 9
        desc: "A pale, eyeless predator that hunts by sound. It moves fast and hits hard."
        loot_table_id: cave-lurker-loot
        dialogue: []
    items: []
    resources:
      - id: coal-seam
        type: mine
        yields:
          - item_id: coal
            name: "Coal"
            desc: "A lump of coal. Burns as fuel."
            probability: 0.9
            count_min: 2
            count_max: 4
        tool_required: pickaxe
        respawn_actions: 20
      - id: iron-vein
        type: mine
        yields:
          - item_id: iron-ore
            name: "Iron Ore"
            desc: "Raw iron ore. Smelt it into an ingot."
            probability: 0.8
            count_min: 1
            count_max: 3
        tool_required: pickaxe
        respawn_actions: 20

  - id: caves-1
    name: "Crystal Cavern"
    biome: caves
    desc: |
      The deepest room in Blockhaven. Ancient crystal formations line the walls, still faintly
      glowing despite the Core's absence. At the center stands a stone altar with an extinguished
      flame — the Crystal Flame. Elder Voss of the Deepborn sits cross-legged beside it,
      waiting. A Crystal Shade circles in the dark above.
    exits:
      up: caves-0
    npcs:
      - id: elder-voss
        name: "Elder Voss"
        hp: 0
        attack: 0
        desc: "An ancient Deepborn elder. Their skin is pale as quartz, their eyes glow faintly blue."
        dialogue:
          - trigger: always
            text: "The Crystal Flame has been dark since the Core shattered. The flame-terminal controls it. Use your hacking skills — the security is modest, but the lock is old. Restore the flame and the Cave Shard is yours."
            quest_id: q-cave-shard
          - trigger: has_item:cave-shard
            text: "The Crystal Flame burns again. Now collect the remaining shards and find Cinder. The Deepborn will remember your courage."
      - id: crystal-shade
        name: "Crystal Shade"
        hp: 50
        attack: 15
        desc: "A creature of living shadow and broken crystal. The most dangerous thing in the caves."
        loot_table_id: crystal-shade-loot
        dialogue: []
    items: []
    systems:
      - id: flame-terminal
        security_level: 2
        reward_item: cave-shard
        reward_text: "The Crystal Flame ignites with a warm roar. The Cave Shard rises from the altar and hovers before you."

  - id: ember-0
    name: "Cinder's Hideaway"
    biome: snow
    desc: |
      A hidden cave behind a frozen waterfall, warm despite the blizzard outside. The walls
      are scorched in patterns that look almost like apologies. Cinder — a young dragon the
      size of a large horse, scales shimmering copper and gold — huddles near a pile of melted
      ice. She raises her head when you enter, and her eyes go wide at the shards you carry.
    exits:
      south: snow-0
    npcs:
      - id: cinder
        name: "Cinder"
        hp: 0
        attack: 0
        desc: "A young copper-and-gold dragon. She broke the Crystal Core by accident and has been hiding here ever since."
        dialogue:
          - trigger: always
            text: "Please don't be angry. I didn't mean to — I was just playing and I hit it and everything shattered and I've been so sorry ever since and I didn't know how to fix it."
          - trigger: has_all_shards
            text: "You have all five shards. I can feel them. If you give them to me, I can breathe them back together — dragon fire and crystal magic are the only thing that can restore the Core. Are you ready?"
            quest_id: q-cinder-return
    items:
      - id: dragon-scale-fragment
        name: "Dragon Scale Fragment"
        desc: "A copper-gold scale Cinder shed. Surprisingly warm to the touch."
        readable: false
```

- [ ] **Step 3: Verify world loads**

```bash
cd /Users/stokes/Projects/gl1tch-mud && echo "world switch blockhaven" | go run . 2>&1 | head -30
```

Expected: output includes "Meadow Town Square" room description.

- [ ] **Step 4: Commit**

```bash
git add worlds/blockhaven/world.yaml
git commit -m "feat(world): add blockhaven world.yaml — 11 core rooms, 5 factions, 5 quests"
```

---

## Task 16: Blockhaven Story Bible + World State

**Files:**
- Create: `worlds/blockhaven/story-bible.md`
- Create: `worlds/blockhaven/world-state.yaml`

- [ ] **Step 1: Create story-bible.md**

Create `worlds/blockhaven/story-bible.md`:

```markdown
# Blockhaven — Story Bible

## Canonical Premise

The Crystal Core floated above Meadow Town Square, powering the five biomes of Blockhaven
with warmth, colour, and life. A young dragon named **Cinder** was playing in the sky and
accidentally collided with the Core, shattering it into five Crystal Shards that scattered
to each biome. Cinder, ashamed and frightened, fled to a hidden cave in the Snow Peaks.

Without the Core, the biomes are slowly fading.

---

## The World

### Five Biomes

| Biome | Faction | Shard | Crisis |
|---|---|---|---|
| Meadow | The Stoneguard | Meadow Shard | Flowers wilting, pedestal empty |
| Forest | The Thornwalkers | Forest Shard | Ancient trees going dormant |
| Desert | The Dunekeepers | Desert Shard | Ruins sinking faster into sand |
| Snow Peaks | The Frostborn | Mountain Shard | Blizzards worsening, Cinder's presence |
| Deep Caves | The Deepborn | Cave Shard | Crystal Flame extinguished, deep dark spreading |

### Five Factions

All five factions want the same thing: restore the Crystal Core. They have different
personalities and skills, but NO faction is an enemy. Rivalry is friendly at worst.

**The Stoneguard** — Master builders and craftspeople. Elder Mason leads with measured wisdom.
Tone: warm, practical, quietly worried.

**The Thornwalkers** — Forest rangers and herbalists. Warden Sylara leads. They commune with
nature and are deeply distressed by the dormant trees.
Tone: calm, observant, earthy.

**The Dunekeepers** — Desert archaeologists and puzzle-lovers. Archivist Dunes leads. They
see the Core's shattering as both catastrophe and fascinating mystery.
Tone: curious, academic, dry humor.

**The Frostborn** — Mountain miners and smiths. Ironmaster Breck leads. They express care
through competence — they'll forge whatever tool you need.
Tone: blunt, direct, trustworthy.

**The Deepborn** — Ancient cave dwellers, lore keepers of the Crystal Core's original
construction. Elder Voss leads. They speak slowly and in few words.
Tone: ancient, wise, patient.

---

## Cinder

Cinder is a young dragon — copper-and-gold scales, about the size of a large horse. She is:
- Not a villain. She is a child who made a terrible mistake.
- Ashamed and frightened, not aggressive.
- Intelligent and well-meaning.
- The ONLY entity with the fire to fuse the Crystal Shards back together.

When the player brings all five shards to Cinder, she fuses them with dragon breath and
together they restore the Crystal Core. This is cooperation, not defeat.

---

## Narrative Chapters

| Chapter ID | Condition | World State |
|---|---|---|
| `shard-hunt` | Default start | Crystal Core pedestal empty, biomes fading |
| `ember-found` | All 5 shards in inventory + q-cinder-return accepted | snow-0 northern passage unlocked |
| `core-restored` | q-cinder-return completed | Core restored, biomes revive (flavor text changes) |

---

## Generation Rules for Pipelines

When generating new rooms, ALWAYS:
- Assign a biome from: meadow, forest, desert, snow, caves
- Reference the faction that controls that biome
- Include one hint connecting to the Crystal Core narrative
- Use the tone of the biome's faction (see faction tones above)
- Include at least one mineable or harvestable resource per room
- Room IDs: gen-<8 lowercase hex chars>
- Item IDs: kebab-case-descriptive-name
- No new factions. No new dragons. No new magical cores.
- Cinder is the ONLY dragon in Blockhaven.
- Combat mobs should fit the biome (see mob roster in world.yaml)

### Vocabulary by Biome

**Meadow**: cobblestone, flowers, sun, open fields, building sites, workshops, cheerful
**Forest**: ancient trees, roots, moss, filtered light, quiet, watchful, earthy
**Desert**: sand, ruins, heat, brass, spyglass, archaeology, buried, ancient
**Snow Peaks**: ice, forge, stone, cold wind, hammer, strength, endurance
**Deep Caves**: crystal, glowstone, dark, echo, ancient, quiet, deep, patient

---

## What NOT to Do

- Do not create new factions
- Do not make any faction an enemy of another
- Do not introduce a new "villain" — there is no villain in Blockhaven
- Do not add new biomes
- Do not make Cinder hostile
- Do not add gory or violent descriptions — this world is for ages 8-12
- Do not use the cyberspace world's vocabulary (no "jacking in", "ICE", "hacking" as flavor)
  Note: the hack mechanic can exist (terminals, puzzle locks) but should be described as
  "solving the ancient puzzle" not "hacking"
```

- [ ] **Step 2: Create world-state.yaml**

Create `worlds/blockhaven/world-state.yaml`:

```yaml
chapter: shard-hunt

# Tracks shards collected (updated by pipeline after quest completions)
shards_collected: 0

# Lists all room IDs currently in world.yaml — pipelines use this for idempotency
existing_room_ids:
  - meadow-0
  - meadow-1
  - forest-0
  - forest-1
  - desert-0
  - desert-1
  - snow-0
  - snow-1
  - caves-0
  - caves-1
  - ember-0

# Lists all item IDs currently in world.yaml
existing_item_ids:
  - builders-map
  - coal-stack
  - coal
  - wood-log
  - stick
  - flint
  - stone
  - iron-ore
  - gold-ore
  - iron-ingot
  - gold-ingot
  - iron-sword
  - gold-sword
  - wooden-pickaxe
  - stone-pickaxe
  - iron-pickaxe
  - wooden-sword
  - frost-essence
  - ice-shard
  - frostcore-ingot
  - diamond
  - obsidian
  - sand
  - sandstone
  - vine
  - root
  - seed
  - leaf
  - herb
  - rare-seed
  - coal-ore
  - charcoal
  - brick
  - clay
  - glass
  - glowstone
  - crystal-fragment
  - dragon-scale-fragment
  - meadow-shard
  - forest-shard
  - desert-shard
  - mountain-shard
  - cave-shard
  - crystal-key
  - meadow-shard-fragment

# Active factions — never add to this list in generation
active_factions:
  - stoneguard
  - thornwalkers
  - dunekeepers
  - frostborn
  - deepborn
```

- [ ] **Step 3: Commit**

```bash
git add worlds/blockhaven/story-bible.md worlds/blockhaven/world-state.yaml
git commit -m "docs(blockhaven): add story bible and world-state.yaml"
```

---

## Task 17: Crystal Shards — Seed DB on World Load

**Files:**
- Modify: `internal/world/world.go`
- Modify: `main.go`

The `crystal_shards` table needs to be seeded with the 5 biome rows when a player first loads the blockhaven world. This is done after `player.Load` in `main.go`, and also after world switching.

- [ ] **Step 1: Add SeedCrystalShards helper to world.go**

In `internal/world/world.go`, add:

```go
// SeedCrystalShards inserts the five Crystal Shard rows for Blockhaven if they don't exist.
// Safe to call on any world — only acts when world name is "blockhaven".
func SeedCrystalShards(db *sql.DB, worldName string) error {
	if worldName != "blockhaven" {
		return nil
	}
	shards := []struct{ id, biome string }{
		{"meadow-shard", "meadow"},
		{"forest-shard", "forest"},
		{"desert-shard", "desert"},
		{"mountain-shard", "snow"},
		{"cave-shard", "caves"},
	}
	for _, s := range shards {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO crystal_shards (shard_id, biome, collected, collected_at) VALUES (?,?,0,0)`,
			s.id, s.biome,
		); err != nil {
			return err
		}
	}
	return nil
}
```

Add `"database/sql"` to world.go's import (it doesn't currently import it — add it).

- [ ] **Step 2: Call SeedCrystalShards in main.go**

After the `player.Load` call in `main.go`, add:

```go
world.SeedCrystalShards(database, s.World) //nolint:errcheck
```

Also add it in the world-switch handler after the new `player.Load`:

```go
world.SeedCrystalShards(database, newState.World) //nolint:errcheck
```

- [ ] **Step 3: Mark shard as collected when a shard item is added to inventory**

In `internal/player/player.go`, modify `AddItem` to also mark the shard collected if the item ID matches a shard pattern:

Actually, this is cleaner done in the quest completion path. When `quests.Complete` is called with a shard quest, check if the reward item is a shard and mark it collected.

The simpler approach: add a helper `MarkShardCollected(db, shardID)` and call it from the `Quests` command handler when a shard quest is completed.

Add to `internal/player/player.go`:

```go
// MarkShardCollected marks a Crystal Shard as collected.
func MarkShardCollected(db *sql.DB, shardID string) error {
	var actionCnt int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&actionCnt) //nolint:errcheck
	_, err := db.Exec(
		`UPDATE crystal_shards SET collected=1, collected_at=? WHERE shard_id=?`,
		actionCnt, shardID,
	)
	return err
}
```

In `internal/commands/commands.go`, find the quest completion section in the `Quests` function (around line 1240). After a quest is marked complete and reward items are granted, add shard detection:

```go
// Mark crystal shard collected if this quest rewards one.
shardIDs := map[string]bool{
    "meadow-shard": true, "forest-shard": true, "desert-shard": true,
    "mountain-shard": true, "cave-shard": true,
}
if q.RewardItemID != "" && shardIDs[q.RewardItemID] {
    player.MarkShardCollected(db, q.RewardItemID) //nolint:errcheck
}
```

- [ ] **Step 4: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build .
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/world/world.go internal/player/player.go main.go internal/commands/commands.go
git commit -m "feat(blockhaven): seed crystal_shards on world load; mark collected on quest complete"
```

---

## Task 18: Integration Smoke Test

- [ ] **Step 1: Run all tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./... -v 2>&1 | tail -40
```

Expected: all tests PASS. Note any failures and fix before proceeding.

- [ ] **Step 2: Smoke test world switching**

```bash
cd /Users/stokes/Projects/gl1tch-mud && echo -e "world list\nworld switch blockhaven\nlook\nworld switch cyberspace\nlook\nquit" | go run .
```

Expected:
- `world list` shows both cyberspace and blockhaven
- `world switch blockhaven` loads blockhaven, shows Meadow Town Square
- `world switch cyberspace` loads cyberspace, shows cyberspace entry room
- No panics or errors

- [ ] **Step 3: Smoke test new commands**

```bash
cd /Users/stokes/Projects/gl1tch-mud && echo -e "world switch blockhaven\nweather\ngather\nmine\nhelp\nquit" | go run .
```

Expected:
- `weather` shows meadow weather condition
- `gather` yields 1-2 ambient items
- `mine` shows "nothing to mine here" (meadow-0 has no mine resources) or lists meadow-1 after moving
- `help` includes new commands

- [ ] **Step 4: Final build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build -o gl1tch-mud .
```

Expected: binary built successfully.

- [ ] **Step 5: Commit final build**

```bash
git add .
git commit -m "feat(blockhaven): complete world + mechanics integration"
```

---

## Self-Review Checklist

- [x] **Spec §1.1** (world list/switch commands) → Task 5
- [x] **Spec §1.2** (OpenForWorld) → Task 3
- [x] **Spec §1.3** (main.go hot-swap) → Task 4
- [x] **Spec §1.4** (world YAML new fields) → Task 2
- [x] **Spec §1.5** (9 new DB tables) → Task 1
- [x] **Spec §2.1** (story bible) → Task 16
- [x] **Spec §2.2** (biomes + factions) → world.yaml in Task 15
- [x] **Spec §2.3** (11 core rooms) → Task 15
- [x] **Spec §2.4** (starting items) → meadow-0 items + player.Load default in Task 17
- [x] **Spec §2.5** (key NPCs) → Task 15
- [x] **Spec §2.6** (mob roster) → Task 15 (loot tables + NPCs in world.yaml)
- [x] **Spec §3.1** (mine) → Task 10
- [x] **Spec §3.2** (harvest) → Task 10
- [x] **Spec §3.3** (gather) → Task 10
- [x] **Spec §3.4** (smelt) → Task 10
- [x] **Spec §3.5** (plant) → Task 10
- [x] **Spec §3.6** (build/stash/unstash) → Task 11
- [x] **Spec §3.7** (enchant) → Tasks 7 + 12
- [x] **Spec §3.8** (weather) → Tasks 6 + 13
- [x] **Spec §3.9** (death/respawn) → Tasks 8 + 9
- [x] **Spec §4.1** (5 shard quests) → world.yaml Task 15
- [x] **Spec §4.2** (Cinder's Return quest + has_all_shards) → Tasks 14 + 15 + 17
- [x] **Spec §5** (world events) → existing events engine; blockhaven-specific event templates are content for Spec 2 (pipelines)

**One gap identified:** Starting items (wooden-pickaxe, wooden-sword, 3x bread, builder's map). The player.Load function seeds defaults with `room_id: "net-0"`. For blockhaven, the start room is `meadow-0` and the player needs starting items. 

**Fix:** In Task 17, also add a `SeedStartingItems` call that adds the starting items the first time a blockhaven player is created:

Add to `internal/world/world.go`:

```go
// SeedStartingItems adds starting items for the blockhaven world if inventory is empty.
func SeedStartingItems(db *sql.DB, worldName string) error {
	if worldName != "blockhaven" {
		return nil
	}
	// Only seed if inventory is empty.
	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM inventory`).Scan(&cnt) //nolint:errcheck
	if cnt > 0 {
		return nil
	}
	items := []struct{ id, name, desc string }{
		{"wooden-pickaxe", "Wooden Pickaxe", "A basic pickaxe. Required for mining stone and ore."},
		{"wooden-sword", "Wooden Sword", "A basic sword. 5 attack."},
		{"bread", "Bread", "Restores 20 HP when eaten."},
		{"builders-map", "Builder's Map", "A hand-drawn map of Blockhaven."},
	}
	for _, it := range items {
		db.Exec(`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`, //nolint:errcheck
			it.id, it.name, it.desc)
	}
	return nil
}
```

Call this in `main.go` after `SeedCrystalShards` and also in the world-switch path.

This was not previously captured — add it to **Task 17**.
