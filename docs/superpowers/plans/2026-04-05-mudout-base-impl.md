# Mudout Player Base System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add six wasteland build recipes, a `baseinfo` command, offline raid events that resolve on login, and session wiring that calls raid spawn/resolve in the mudout world.

**Architecture:** New `internal/base` package owns `DefenseScore`, `MaybeSpawnRaid`, and `ResolvePendingRaids`. These three functions are pure over `*sql.DB` + `*world.World`. Session.go calls them at start (resolve) and after each command (spawn). The `BaseInfo` command lives in commands.go and imports base. All raid data lives in the existing `world_events` table.

**Tech Stack:** Go 1.21, SQLite (modernc.org/sqlite), YAML

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Modify | `internal/world/defaults/mudout/world.yaml` | 6 build recipes; update dusthaven-4 desc |
| Create | `internal/base/base.go` | `DefenseScore`, `MaybeSpawnRaid`, `ResolvePendingRaids`, `loseChestItems` |
| Create | `internal/base/base_test.go` | All base package tests |
| Modify | `internal/commands/commands.go` | Add `BaseInfo` function + registry wiring |
| Create | `internal/commands/baseinfo_test.go` | Tests for `BaseInfo` |
| Modify | `internal/server/session.go` | Call `ResolvePendingRaids` on mudout start + world switch; call `MaybeSpawnRaid` after each mudout command |

---

## Task 1: World YAML — 6 build recipes + dusthaven-4 update

**Files:**
- Modify: `internal/world/defaults/mudout/world.yaml`

- [ ] **Step 1: Add 6 build recipes to world.yaml**

In `internal/world/defaults/mudout/world.yaml`, find the end of the `crafting_recipes:` block (after the `leather-armor` recipe, just before `rooms:`). Insert these six recipes:

```yaml
  - id: foundation
    name: "Base Foundation"
    type: ingredient
    workbench: build
    skill_req: 0
    ingredients:
      - id: scrap-iron
        name: "Scrap Iron"
        count: 5
      - id: polymer-sheet
        name: "Polymer Sheet"
        count: 3
    output:
      id: foundation
      name: "Base Foundation"
      desc: "A solid concrete slab. Everything else is built on this."
      tags: [structure]
      stats: {defense: 0}

  - id: base-walls
    name: "Reinforced Walls"
    type: ingredient
    workbench: build
    skill_req: 0
    ingredients:
      - id: scrap-iron
        name: "Scrap Iron"
        count: 4
      - id: polymer-sheet
        name: "Polymer Sheet"
        count: 2
    output:
      id: base-walls
      name: "Reinforced Walls"
      desc: "Corrugated iron sheets bolted to a steel frame. Keeps raiders out."
      tags: [structure]
      stats: {defense: 3}

  - id: base-roof
    name: "Corrugated Roof"
    type: ingredient
    workbench: build
    skill_req: 0
    ingredients:
      - id: polymer-sheet
        name: "Polymer Sheet"
        count: 3
      - id: scrap-iron
        name: "Scrap Iron"
        count: 2
    output:
      id: base-roof
      name: "Corrugated Roof"
      desc: "Keeps the ashfall off your gear. Some weather protection."
      tags: [structure]
      stats: {defense: 1}

  - id: chest
    name: "Storage Locker"
    type: ingredient
    workbench: build
    skill_req: 0
    ingredients:
      - id: scrap-iron
        name: "Scrap Iron"
        count: 3
      - id: copper-wire
        name: "Copper Wire"
        count: 2
    output:
      id: chest
      name: "Storage Locker"
      desc: "A lockable metal cabinet. Use 'stash' and 'unstash' to manage contents."
      tags: [structure]
      stats: {defense: 0}

  - id: base-generator
    name: "Diesel Generator"
    type: ingredient
    workbench: build
    skill_req: 0
    ingredients:
      - id: copper-wire
        name: "Copper Wire"
        count: 4
      - id: pre-war-circuitry
        name: "Pre-War Circuitry"
        count: 2
    output:
      id: base-generator
      name: "Diesel Generator"
      desc: "Hums with salvaged fuel. Powers the turret."
      tags: [structure]
      stats: {defense: 2}

  - id: base-turret
    name: "Sentry Turret"
    type: ingredient
    workbench: build
    skill_req: 0
    ingredients:
      - id: scrap-iron
        name: "Scrap Iron"
        count: 5
      - id: copper-wire
        name: "Copper Wire"
        count: 3
      - id: pre-war-circuitry
        name: "Pre-War Circuitry"
        count: 2
    output:
      id: base-turret
      name: "Sentry Turret"
      desc: "Tracks movement and returns fire. Highest single defense bonus."
      tags: [structure]
      stats: {defense: 5}
```

- [ ] **Step 2: Update dusthaven-4 room desc**

In `internal/world/defaults/mudout/world.yaml`, find the `dusthaven-4` room block. Replace its `items:` section (the plot-claim-form stub) so the room is ready for building:

Find:
```yaml
  - id: dusthaven-4
    name: "The Base Plots"
    desc: |
      A cleared zone at the north edge of the settlement. Survey stakes mark
      out plots in the cracked earth. A faded sign reads: REGISTERED SETTLERS
      MAY CLAIM ONE PLOT. The area is quiet — potential waiting to be built on.
    biome: settlement
    exits:
      south: dusthaven-2
    npcs: []
    items:
      - id: plot-claim-form
        name: "Plot Claim Form"
        desc: "A bureaucratic relic. Base building coming in a future update."
        readable: true
        content: "This plot is available for settlement. Speak to the Settlers Council to register your claim."
```

Replace with:
```yaml
  - id: dusthaven-4
    name: "The Base Plots"
    desc: |
      A cleared zone at the north edge of the settlement. Survey stakes mark
      out plots in the cracked earth. Your plot is here. Use 'build' to start
      construction, 'baseinfo' to check status, and 'stash'/'unstash' once
      you've built a Storage Locker.
    biome: settlement
    exits:
      south: dusthaven-2
    npcs: []
    items: []
```

- [ ] **Step 3: Run world tests to verify YAML loads correctly**

```
go test ./internal/world/... -v 2>&1 | tail -15
```

Expected: all PASS. The mudout world test (`TestMudoutWorldLoads`) checks `len(CraftingRecipes) >= 2` — it will still pass with 8 recipes.

- [ ] **Step 4: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/world/defaults/mudout/world.yaml
git commit -m "feat(base): add 6 mudout build recipes, update dusthaven-4 desc"
```

---

## Task 2: `internal/base` package — DefenseScore, MaybeSpawnRaid, ResolvePendingRaids

**Files:**
- Create: `internal/base/base.go`
- Create: `internal/base/base_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/base/base_test.go`:

```go
package base_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/base"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openBaseTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE builds (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id   TEXT    NOT NULL,
			build_id  TEXT    NOT NULL,
			name      TEXT    NOT NULL,
			desc      TEXT    NOT NULL DEFAULT '',
			placed_at INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE world_events (
			id               TEXT    PRIMARY KEY,
			type             TEXT    NOT NULL,
			title            TEXT    NOT NULL,
			description      TEXT,
			target_room      TEXT    NOT NULL,
			faction          TEXT,
			payout_credits   INTEGER NOT NULL DEFAULT 0,
			payout_item_id   TEXT,
			payout_item_name TEXT,
			payout_item_desc TEXT,
			status           TEXT    NOT NULL DEFAULT 'active',
			expires_actions  INTEGER NOT NULL DEFAULT 20,
			created_actions  INTEGER NOT NULL DEFAULT 0,
			created_at       INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE chests (
			room_id   TEXT NOT NULL,
			item_id   TEXT NOT NULL,
			item_name TEXT NOT NULL,
			item_desc TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (room_id, item_id)
		);
		CREATE TABLE player_actions (id INT PRIMARY KEY CHECK(id=1), count INT DEFAULT 0);
		INSERT INTO player_actions (id, count) VALUES (1, 0);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func makeBaseWorld() *world.World {
	w := &world.World{}
	w.CraftingRecipes = []world.CraftingRecipe{
		{ID: "base-walls", Name: "Reinforced Walls", Output: world.Item{ID: "base-walls", Stats: map[string]int{"defense": 3}}},
		{ID: "base-turret", Name: "Sentry Turret", Output: world.Item{ID: "base-turret", Stats: map[string]int{"defense": 5}}},
		{ID: "foundation", Name: "Base Foundation", Output: world.Item{ID: "foundation", Stats: map[string]int{"defense": 0}}},
	}
	return w
}

func setActionCount(db *sql.DB, n int) {
	db.Exec(`INSERT OR REPLACE INTO player_actions (id, count) VALUES (1, ?)`, n) //nolint:errcheck
}

func TestDefenseScoreEmpty(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()
	w := makeBaseWorld()

	score := base.DefenseScore(db, w)
	if score != 0 {
		t.Errorf("empty base: got %d want 0", score)
	}
}

func TestDefenseScore(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()
	w := makeBaseWorld()

	// Place walls (3) and turret (5) in dusthaven-4
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Reinforced Walls',1)`) //nolint:errcheck
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-turret','Sentry Turret',2)`)   //nolint:errcheck

	score := base.DefenseScore(db, w)
	if score != 8 {
		t.Errorf("walls+turret: got %d want 8", score)
	}
}

func TestDefenseScoreIgnoresOtherRooms(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()
	w := makeBaseWorld()

	// Structure in a different room should not contribute
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-0','base-walls','Walls',1)`) //nolint:errcheck

	score := base.DefenseScore(db, w)
	if score != 0 {
		t.Errorf("other room structure: got %d want 0", score)
	}
}

func TestMaybeSpawnRaid_spawns(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()

	// Add a structure and set action count to 30
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Foundation',1)`) //nolint:errcheck
	setActionCount(db, 30)

	base.MaybeSpawnRaid(db)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid' AND target_room='dusthaven-4' AND status='active'`).Scan(&cnt) //nolint:errcheck
	if cnt != 1 {
		t.Errorf("raid not spawned: got %d active base-raid events, want 1", cnt)
	}
}

func TestMaybeSpawnRaid_noSpawnWithoutStructures(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()

	setActionCount(db, 30)
	base.MaybeSpawnRaid(db)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid'`).Scan(&cnt) //nolint:errcheck
	if cnt != 0 {
		t.Errorf("raid spawned without structures: got %d, want 0", cnt)
	}
}

func TestMaybeSpawnRaid_noSpawnIfActiveRaid(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Foundation',1)`) //nolint:errcheck
	db.Exec(`INSERT INTO world_events (id, type, title, description, target_room, faction, payout_credits, payout_item_id, payout_item_name, payout_item_desc, status, expires_actions, created_actions, created_at) VALUES ('existing-raid','base-raid','Raid','Desc','dusthaven-4','ash-raiders',0,'','','','active',30,0,0)`) //nolint:errcheck
	setActionCount(db, 30)

	base.MaybeSpawnRaid(db)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid' AND status='active'`).Scan(&cnt) //nolint:errcheck
	if cnt != 1 {
		t.Errorf("duplicate raid spawned: got %d active raids, want 1", cnt)
	}
}

func TestMaybeSpawnRaid_noSpawnIfNotMultipleOf30(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Foundation',1)`) //nolint:errcheck
	setActionCount(db, 31)

	base.MaybeSpawnRaid(db)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid'`).Scan(&cnt) //nolint:errcheck
	if cnt != 0 {
		t.Errorf("raid spawned at non-multiple of 30: got %d, want 0", cnt)
	}
}

func TestResolvePendingRaids_noPending(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()
	w := makeBaseWorld()

	report := base.ResolvePendingRaids(db, w)
	if report != "" {
		t.Errorf("no raids: expected empty report, got %q", report)
	}
}

func TestResolvePendingRaids_repelled(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()
	w := makeBaseWorld()

	// Build walls(3) + turret(5) = 11 defense total (well above max raid strength 15... use defense=11)
	// Actually max raid strength is 15, so we need defense >= 15 to guarantee repel in test.
	// Instead, we seed the rand with a known strength. Since we can't control rand, test that
	// when defense is 11 the raid is sometimes repelled — but that's flaky.
	// Better: place ALL structures (max 11 defense) and insert a raid with known strength=1
	// stored in created_actions so we can verify resolve happened and event is marked resolved.
	// We test resolve behavior by checking event status and that no chest items were removed.

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Walls',1)`)   //nolint:errcheck
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-turret','Turret',2)`) //nolint:errcheck
	// defense = 8

	// Insert a chest item
	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','food-1','Canned Food')`) //nolint:errcheck

	// Insert expired raid (created at action 0, expires in 30, current = 60)
	db.Exec(`INSERT INTO world_events (id, type, title, description, target_room, faction, payout_credits, payout_item_id, payout_item_name, payout_item_desc, status, expires_actions, created_actions, created_at) VALUES ('raid-1','base-raid','Raid','Desc','dusthaven-4','ash-raiders',0,'','','','active',30,0,0)`) //nolint:errcheck
	setActionCount(db, 60)

	report := base.ResolvePendingRaids(db, w)

	if report == "" {
		t.Error("expected non-empty report")
	}

	// Event should be resolved
	var status string
	db.QueryRow(`SELECT status FROM world_events WHERE id='raid-1'`).Scan(&status) //nolint:errcheck
	if status != "resolved" {
		t.Errorf("event status: got %q want 'resolved'", status)
	}

	// Report should contain raid information
	if !strings.Contains(report, "RAID REPORT") {
		t.Errorf("report missing RAID REPORT header: %q", report)
	}
}

func TestResolvePendingRaids_brokenThrough(t *testing.T) {
	db := openBaseTestDB(t)
	defer db.Close()

	// World with no defense recipes (defense = 0)
	w := &world.World{}

	// Insert chest items
	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','item-1','Scrap Iron')`)  //nolint:errcheck
	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','item-2','Canned Food')`) //nolint:errcheck

	// Insert expired raid
	db.Exec(`INSERT INTO world_events (id, type, title, description, target_room, faction, payout_credits, payout_item_id, payout_item_name, payout_item_desc, status, expires_actions, created_actions, created_at) VALUES ('raid-2','base-raid','Raid','Desc','dusthaven-4','ash-raiders',0,'','','','active',30,0,0)`) //nolint:errcheck
	setActionCount(db, 60)

	report := base.ResolvePendingRaids(db, w)

	if report == "" {
		t.Error("expected non-empty report")
	}
	if !strings.Contains(report, "RAID REPORT") {
		t.Errorf("report missing RAID REPORT: %q", report)
	}

	// Event should be resolved
	var status string
	db.QueryRow(`SELECT status FROM world_events WHERE id='raid-2'`).Scan(&status) //nolint:errcheck
	if status != "resolved" {
		t.Errorf("event status: got %q want 'resolved'", status)
	}

	// With defense=0 and raid strength 1-15, raid always breaks through.
	// Chest should have lost some items (or be empty).
	var remaining int
	db.QueryRow(`SELECT COUNT(*) FROM chests WHERE room_id='dusthaven-4'`).Scan(&remaining) //nolint:errcheck
	// Up to 3 items lost from 2 items — chest should now have 0 or fewer (bounded by min 0)
	if remaining > 2 {
		t.Errorf("chest items: got %d remaining (should have lost some), had 2", remaining)
	}
	// Since defense=0 always < raid strength (min 1), items should be lost
	if remaining == 2 {
		t.Error("raid broke through with defense=0 but no items were lost")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/base/... -v 2>&1 | tail -5
```

Expected: FAIL — "no required module provides package github.com/adam-stokes/gl1tch-mud/internal/base"

- [ ] **Step 3: Create `internal/base/base.go`**

Create `internal/base/base.go`:

```go
// Package base manages the player's permanent base in the mudout world.
package base

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

const baseRoomID = "dusthaven-4"

// actionCount reads the player's action count from player_actions.
func actionCount(db *sql.DB) int {
	var c int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&c) //nolint:errcheck
	return c
}

// DefenseScore sums the defense stats of all structures built in dusthaven-4.
func DefenseScore(db *sql.DB, w *world.World) int {
	rows, err := db.Query(`SELECT build_id FROM builds WHERE room_id=?`, baseRoomID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	score := 0
	for rows.Next() {
		var buildID string
		rows.Scan(&buildID) //nolint:errcheck
		if r := w.FindRecipe(buildID); r != nil {
			score += r.Output.Stats["defense"]
		}
	}
	return score
}

// MaybeSpawnRaid spawns a base-raid world event if conditions are met:
// action count is a multiple of 30, at least one structure is built in
// dusthaven-4, and no active base-raid event already exists.
func MaybeSpawnRaid(db *sql.DB) {
	current := actionCount(db)
	if current == 0 || current%30 != 0 {
		return
	}

	var structCount int
	db.QueryRow(`SELECT COUNT(*) FROM builds WHERE room_id=?`, baseRoomID).Scan(&structCount) //nolint:errcheck
	if structCount == 0 {
		return
	}

	var activeRaids int
	db.QueryRow(
		`SELECT COUNT(*) FROM world_events WHERE type='base-raid' AND target_room=? AND status='active'`,
		baseRoomID,
	).Scan(&activeRaids) //nolint:errcheck
	if activeRaids > 0 {
		return
	}

	id := fmt.Sprintf("base-raid-%d", time.Now().UnixNano())
	db.Exec( //nolint:errcheck
		`INSERT INTO world_events
		 (id, type, title, description, target_room, faction,
		  payout_credits, payout_item_id, payout_item_name, payout_item_desc,
		  status, expires_actions, created_actions, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, "base-raid", "Ash Raider Attack",
		"Ash Raiders are moving on your base.",
		baseRoomID, "ash-raiders",
		0, "", "", "",
		"active", 30, current, time.Now().Unix(),
	)
}

// ResolvePendingRaids checks for expired base-raid events, resolves them,
// and returns a narrative report string. Returns empty string if no raids pending.
func ResolvePendingRaids(db *sql.DB, w *world.World) string {
	current := actionCount(db)

	rows, err := db.Query(
		`SELECT id FROM world_events
		 WHERE type='base-raid' AND target_room=? AND status='active'
		 AND (created_actions + expires_actions) <= ?`,
		baseRoomID, current,
	)
	if err != nil {
		return ""
	}

	var raidIDs []string
	for rows.Next() {
		var id string
		rows.Scan(&id) //nolint:errcheck
		raidIDs = append(raidIDs, id)
	}
	rows.Close()

	if len(raidIDs) == 0 {
		return ""
	}

	defense := DefenseScore(db, w)
	var reports []string

	for _, id := range raidIDs {
		strength := rand.Intn(15) + 1 //nolint:gosec
		var report string
		if defense >= strength {
			report = fmt.Sprintf(
				"RAID REPORT: Ash Raiders hit your base while you were gone.\nRaid strength: %d  |  Your defense: %d\nYour defenses held. Nothing was taken.",
				strength, defense,
			)
		} else {
			lost := loseChestItems(db, 3)
			if len(lost) == 0 {
				report = fmt.Sprintf(
					"RAID REPORT: Ash Raiders hit your base while you were gone.\nRaid strength: %d  |  Your defense: %d\nRaiders broke through. Your storage was empty — nothing lost.",
					strength, defense,
				)
			} else {
				report = fmt.Sprintf(
					"RAID REPORT: Ash Raiders hit your base while you were gone.\nRaid strength: %d  |  Your defense: %d\nRaiders broke through. Lost: %s.",
					strength, defense, strings.Join(lost, ", "),
				)
			}
		}
		reports = append(reports, report)
		db.Exec(`UPDATE world_events SET status='resolved' WHERE id=?`, id) //nolint:errcheck
	}

	return strings.Join(reports, "\n\n")
}

// loseChestItems deletes up to max random items from the base chest and
// returns the names of lost items.
func loseChestItems(db *sql.DB, max int) []string {
	rows, err := db.Query(
		`SELECT item_id, item_name FROM chests WHERE room_id=? ORDER BY RANDOM() LIMIT ?`,
		baseRoomID, max,
	)
	if err != nil {
		return nil
	}
	var ids, names []string
	for rows.Next() {
		var id, name string
		rows.Scan(&id, &name) //nolint:errcheck
		ids = append(ids, id)
		names = append(names, name)
	}
	rows.Close()

	for _, id := range ids {
		db.Exec(`DELETE FROM chests WHERE room_id=? AND item_id=?`, baseRoomID, id) //nolint:errcheck
	}
	return names
}
```

- [ ] **Step 4: Run tests — verify they pass**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/base/... -v 2>&1 | tail -25
```

Expected: all PASS.

Note: `TestResolvePendingRaids_repelled` tests that the event is marked resolved and a report is produced; it doesn't assert repel/break outcome because `rand.Intn(15)+1` is random. With defense=8, some runs will break through and some won't — the test only verifies the report is non-empty and the event is resolved. `TestResolvePendingRaids_brokenThrough` uses defense=0 to guarantee a break-through every time.

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/base/base.go internal/base/base_test.go
git commit -m "feat(base): add base package — DefenseScore, MaybeSpawnRaid, ResolvePendingRaids"
```

---

## Task 3: `BaseInfo` command

**Files:**
- Modify: `internal/commands/commands.go`
- Create: `internal/commands/baseinfo_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/commands/baseinfo_test.go`:

```go
package commands_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openBaseInfoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE builds (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id   TEXT    NOT NULL,
			build_id  TEXT    NOT NULL,
			name      TEXT    NOT NULL,
			desc      TEXT    NOT NULL DEFAULT '',
			placed_at INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE chests (
			room_id   TEXT NOT NULL,
			item_id   TEXT NOT NULL,
			item_name TEXT NOT NULL,
			item_desc TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (room_id, item_id)
		);
		CREATE TABLE player_actions (id INT PRIMARY KEY CHECK(id=1), count INT DEFAULT 0);
		INSERT INTO player_actions (id, count) VALUES (1, 15);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func makeBaseInfoWorld() *world.World {
	w := &world.World{}
	w.CraftingRecipes = []world.CraftingRecipe{
		{ID: "base-walls", Name: "Reinforced Walls", Output: world.Item{ID: "base-walls", Stats: map[string]int{"defense": 3}}},
		{ID: "foundation", Name: "Base Foundation", Output: world.Item{ID: "foundation", Stats: map[string]int{"defense": 0}}},
	}
	return w
}

func TestBaseInfoEmpty(t *testing.T) {
	db := openBaseInfoTestDB(t)
	defer db.Close()

	s := &player.State{}
	w := makeBaseInfoWorld()

	res := commands.BaseInfo(db, s, w, nil)
	if res.Output == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(res.Output, "No structures") {
		t.Errorf("expected 'No structures' message, got: %q", res.Output)
	}
}

func TestBaseInfoWithStructures(t *testing.T) {
	db := openBaseInfoTestDB(t)
	defer db.Close()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Reinforced Walls',1)`)   //nolint:errcheck
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Base Foundation',2)`)    //nolint:errcheck
	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','food','Canned Food')`)                       //nolint:errcheck

	s := &player.State{}
	w := makeBaseInfoWorld()

	res := commands.BaseInfo(db, s, w, nil)
	if res.Output == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(res.Output, "DEFENSE SCORE") {
		t.Errorf("output missing DEFENSE SCORE: %q", res.Output)
	}
	if !strings.Contains(res.Output, "CHEST ITEMS") {
		t.Errorf("output missing CHEST ITEMS: %q", res.Output)
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/commands/... -run "TestBaseInfo" -v 2>&1 | tail -5
```

Expected: FAIL — "undefined: commands.BaseInfo"

- [ ] **Step 3: Add BaseInfo to commands.go**

First read `internal/commands/commands.go` to find the end of the file (after the `Equipment` function added in sub-project 2). Also verify what imports are present — you'll need to add `"github.com/adam-stokes/gl1tch-mud/internal/base"` if it's not there.

Append to `internal/commands/commands.go`:

```go
// BaseInfo shows the status of the player's permanent base at dusthaven-4.
func BaseInfo(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	const baseRoom = "dusthaven-4"

	rows, err := db.Query(`SELECT build_id, name FROM builds WHERE room_id=? ORDER BY placed_at`, baseRoom)
	if err != nil {
		return Result{Output: "BASE STATUS — The Base Plots\nNo structures built. Head to dusthaven-4 and use 'build' to start."}
	}
	defer rows.Close()

	type buildRow struct{ id, name string }
	var built []buildRow
	for rows.Next() {
		var b buildRow
		rows.Scan(&b.id, &b.name) //nolint:errcheck
		built = append(built, b)
	}

	if len(built) == 0 {
		return Result{Output: "BASE STATUS — The Base Plots\nNo structures built. Head to dusthaven-4 and use 'build' to start."}
	}

	defense := base.DefenseScore(db, w)

	var sb strings.Builder
	sb.WriteString("BASE STATUS — The Base Plots\n")
	sb.WriteString(strings.Repeat("─", 35) + "\n")
	for _, b := range built {
		def := 0
		if r := w.FindRecipe(b.id); r != nil {
			def = r.Output.Stats["defense"]
		}
		fmt.Fprintf(&sb, "  %-18s %-20s [DEF %2d]\n", b.id, b.name, def)
	}
	sb.WriteString(strings.Repeat("─", 35) + "\n")
	fmt.Fprintf(&sb, "  DEFENSE SCORE: %d / 11 max\n", defense)

	var chestCount int
	db.QueryRow(`SELECT COUNT(*) FROM chests WHERE room_id=?`, baseRoom).Scan(&chestCount) //nolint:errcheck
	fmt.Fprintf(&sb, "  CHEST ITEMS: %d\n", chestCount)

	var current int
	db.QueryRow(`SELECT count FROM player_actions WHERE id=1`).Scan(&current) //nolint:errcheck
	nextRaid := 30 - (current % 30)
	if nextRaid == 30 {
		nextRaid = 0
	}
	fmt.Fprintf(&sb, "  Next raid check: ~%d actions", nextRaid)

	return Result{Output: strings.TrimRight(sb.String(), "\n")}
}
```

- [ ] **Step 4: Wire into the Registry**

In `internal/commands/commands.go`, find the `Registry` map or `init()` block where commands are registered. Add:

```go
"baseinfo": BaseInfo,
"mybase":   BaseInfo,
```

- [ ] **Step 5: Ensure `base` is imported**

At the top of `internal/commands/commands.go`, verify the import block includes:
```go
"github.com/adam-stokes/gl1tch-mud/internal/base"
```

Add it if missing.

- [ ] **Step 6: Run tests — verify they pass**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/commands/... -run "TestBaseInfo" -v 2>&1 | tail -10
```

Expected: both PASS.

- [ ] **Step 7: Run full commands suite**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/commands/... -v 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/commands/commands.go internal/commands/baseinfo_test.go
git commit -m "feat(base): add BaseInfo/mybase command"
```

---

## Task 4: Session integration

**Files:**
- Modify: `internal/server/session.go`

- [ ] **Step 1: Read session.go to find exact insertion points**

Read `internal/server/session.go`. Find:
1. The section in `Handle()` after `player.LoadDefense(s.database, s.state)` — this is where raid resolution goes on session start.
2. The end of `dispatchCommand()` just before `s.sendStateUpdate(ctx)` (line ~298) — this is where `MaybeSpawnRaid` goes.
3. The section in `switchWorld()` after `player.LoadDefense(newDB, newState)` — raid resolution when switching into mudout.

- [ ] **Step 2: Add ResolvePendingRaids call on session start in Handle()**

In the `Handle()` function, find the block:
```go
s.state.PlayerID = s.playerID
player.LoadDefense(s.database, s.state)
```

Add after it:
```go
if s.worldName == "mudout" {
    if report := base.ResolvePendingRaids(s.database, s.world); report != "" {
        _ = writeMsg(ctx, s.conn, ServerMsg{
            Type:    "output.token",
            Payload: OutputTokenPayload{Token: report + "\r\n\r\n"},
        })
        _ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
    }
}
```

- [ ] **Step 3: Add MaybeSpawnRaid call after each mudout command in dispatchCommand()**

In `dispatchCommand()`, find the line just before `s.sendStateUpdate(ctx)` at the very end of the function (after all the `result.SwitchWorld` handling):

```go
s.sendStateUpdate(ctx)
```

Add before it:
```go
if s.worldName == "mudout" {
    base.MaybeSpawnRaid(s.database)
}
```

- [ ] **Step 4: Add ResolvePendingRaids call in switchWorld()**

In `switchWorld()`, find:
```go
player.LoadDefense(newDB, newState)
```

Add after it:
```go
if targetName == "mudout" {
    if report := base.ResolvePendingRaids(s.database, s.world); report != "" {
        _ = writeMsg(ctx, s.conn, ServerMsg{
            Type:    "output.token",
            Payload: OutputTokenPayload{Token: report + "\r\n\r\n"},
        })
        _ = writeMsg(ctx, s.conn, ServerMsg{Type: "output.done"})
    }
}
```

Note: at this point in `switchWorld`, `s.database` and `s.world` have already been updated to the new world's DB and world object (they're set a few lines above at `s.database = newDB` / `s.world = newWorld`). So `s.database` and `s.world` are correct to use here.

- [ ] **Step 5: Add base import to session.go**

Ensure `internal/server/session.go` imports `"github.com/adam-stokes/gl1tch-mud/internal/base"`. Add it to the import block if not already present.

- [ ] **Step 6: Run all Go tests**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/server/session.go
git commit -m "feat(base): wire ResolvePendingRaids on session start, MaybeSpawnRaid after each mudout command"
```

---

## Task 5: Full verification

- [ ] **Step 1: Run the complete Go test suite**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1
```

Expected: all PASS, no regressions across all packages.

- [ ] **Step 2: Run the web test suite (no changes expected)**

```
cd /Users/stokes/Projects/gl1tch-mud/web && npx vitest run 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 3: Verify build recipes parse correctly**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -v -run TestMudout 2>&1
```

Expected: PASS. The `TestMudoutWorldLoads` test checks `len(CraftingRecipes) >= 2` — will pass with 8 recipes.

---

## Self-Review

**Spec coverage:**
- 6 build recipes in world.yaml → Task 1 ✓
- `stats: {defense: N}` on each recipe output → Task 1 ✓
- `dusthaven-4` desc updated (stub removed) → Task 1 ✓
- `DefenseScore` → Task 2 ✓
- `MaybeSpawnRaid` (every 30 actions, needs structure, no duplicate) → Task 2 ✓
- `ResolvePendingRaids` (resolve expired events, lose chest items on break-through) → Task 2 ✓
- `baseinfo`/`mybase` command → Task 3 ✓
- Session start calls `ResolvePendingRaids` for mudout → Task 4 ✓
- World switch calls `ResolvePendingRaids` for mudout → Task 4 ✓
- `MaybeSpawnRaid` after each mudout command → Task 4 ✓
- Raid loss bounded to 3 items → `loseChestItems(db, 3)` ✓
- No raids spawn without structures → `TestMaybeSpawnRaid_noSpawnWithoutStructures` ✓
- No duplicate active raids → `TestMaybeSpawnRaid_noSpawnIfActiveRaid` ✓

**Placeholder scan:** None found.

**Type consistency:**
- `base.DefenseScore(db *sql.DB, w *world.World) int` — used identically in Task 2, Task 3 (BaseInfo), Task 4 (session). ✓
- `base.MaybeSpawnRaid(db *sql.DB)` — called with `s.database` in Task 4. ✓
- `base.ResolvePendingRaids(db *sql.DB, w *world.World) string` — called with `s.database, s.world` in Task 4. ✓
- `world.FindRecipe(id string) *CraftingRecipe` — exists in world.go, used in base.go and commands.go. ✓
