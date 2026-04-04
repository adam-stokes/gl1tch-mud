# Gamification System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `glitch-gamification` as a standalone plugin daemon and wire gl1tch-mud to publish actions and display achievements/leaderboard in chat.

**Architecture:** Two parallel tracks. Track A creates a new repo (`~/Projects/gl1tch-gamification`) with its own SQLite DB, in-memory achievement catalog, and BUSD daemon loop. Track B modifies gl1tch-mud to translate existing mud events into `game.action` events, register its achievement catalog, and render replies as glitch chat messages. The two tracks share only BUSD topic names — no compile-time coupling.

**Tech Stack:** Go 1.25, `modernc.org/sqlite` (pure Go, no cgo), `github.com/spf13/cobra`, BUSD Unix socket, `gopkg.in/yaml.v3`

---

## Track A — glitch-gamification (new repo at ~/Projects/gl1tch-gamification)

---

### Task A1: Scaffold the repo

**Files:**
- Create: `~/Projects/gl1tch-gamification/go.mod`
- Create: `~/Projects/gl1tch-gamification/main.go`
- Create: `~/Projects/gl1tch-gamification/Makefile`
- Create: `~/Projects/gl1tch-gamification/glitch-plugin.yaml`
- Create: `~/Projects/gl1tch-gamification/.gitignore`

- [ ] **Step 1: Create repo directory and go.mod**

```bash
mkdir -p ~/Projects/gl1tch-gamification
cd ~/Projects/gl1tch-gamification
git init
```

`go.mod`:
```
module github.com/adam-stokes/gl1tch-gamification

go 1.25.0

require (
	github.com/spf13/cobra v1.10.2
	modernc.org/sqlite v1.29.9
	gopkg.in/yaml.v3 v3.0.1
)
```

Run `go mod tidy` to resolve the sum file.

- [ ] **Step 2: Write main.go**

`main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/adam-stokes/gl1tch-gamification/cmd"
)

func main() {
	if err := cmd.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Write cmd/root.go**

Create `cmd/root.go`:
```go
package cmd

import "github.com/spf13/cobra"

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "glitch-gamification",
		Short: "gl1tch gamification daemon",
	}
	root.AddCommand(daemonCmd())
	return root
}
```

- [ ] **Step 4: Write Makefile**

`Makefile`:
```makefile
BINARY      := glitch-gamification
INSTALL_BIN := $(or $(shell test -w /usr/local/bin && echo /usr/local/bin),$(HOME)/.local/bin)

.PHONY: build install test clean

build:
	go build -o $(BINARY) .

install: build
	install -m 0755 $(BINARY) $(INSTALL_BIN)/$(BINARY)
	@echo "Installed $(BINARY) → $(INSTALL_BIN)/$(BINARY)"

test:
	go test ./...

clean:
	rm -f $(BINARY)
```

- [ ] **Step 5: Write glitch-plugin.yaml**

`glitch-plugin.yaml`:
```yaml
name: gamification
description: "Game-agnostic achievements, leaderboard, and NPC agent stats"
binary: glitch-gamification
version: main
install:
  go:
    module: github.com/adam-stokes/gl1tch-gamification
```

- [ ] **Step 6: Write .gitignore**

`.gitignore`:
```
glitch-gamification
*.db
```

- [ ] **Step 7: Initial commit**

```bash
git add .
git commit -m "chore: scaffold gl1tch-gamification plugin"
```

---

### Task A2: Store package

**Files:**
- Create: `~/Projects/gl1tch-gamification/internal/store/store.go`
- Create: `~/Projects/gl1tch-gamification/internal/store/store_test.go`

- [ ] **Step 1: Write failing tests**

`internal/store/store_test.go`:
```go
package store_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-gamification/internal/store"
)

func TestRecordAction(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.RecordAction("stokes", "robots", false, "combat.won", 1); err != nil {
		t.Fatalf("RecordAction: %v", err)
	}

	p, err := st.GetPlayer("stokes")
	if err != nil {
		t.Fatalf("GetPlayer: %v", err)
	}
	if p.Score != 1 {
		t.Errorf("score = %d, want 1", p.Score)
	}
	if p.Faction != "robots" {
		t.Errorf("faction = %q, want %q", p.Faction, "robots")
	}

	count, err := st.ActionCount("stokes", "combat.won")
	if err != nil {
		t.Fatalf("ActionCount: %v", err)
	}
	if count != 1 {
		t.Errorf("action count = %d, want 1", count)
	}
}

func TestRecordUnlocked(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.RecordUnlocked("stokes", "first_blood", "gl1tch-mud"); err != nil {
		t.Fatalf("first RecordUnlocked: %v", err)
	}
	// idempotent
	if err := st.RecordUnlocked("stokes", "first_blood", "gl1tch-mud"); err != nil {
		t.Fatalf("second RecordUnlocked: %v", err)
	}

	ok, err := st.IsUnlocked("stokes", "first_blood")
	if err != nil {
		t.Fatalf("IsUnlocked: %v", err)
	}
	if !ok {
		t.Error("expected first_blood to be unlocked")
	}
}

func TestTopFactions(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	st.RecordAction("stokes", "robots", false, "combat.won", 3)   //nolint:errcheck
	st.RecordAction("glitch", "robots", true, "chat.reply", 1)    //nolint:errcheck
	st.RecordAction("stokes", "observability", false, "hack.success", 2) //nolint:errcheck

	factions, err := st.TopFactions(10)
	if err != nil {
		t.Fatalf("TopFactions: %v", err)
	}
	if len(factions) == 0 {
		t.Fatal("expected factions")
	}
	if factions[0].ID != "robots" {
		t.Errorf("top faction = %q, want %q", factions[0].ID, "robots")
	}
}

func TestUnlockedForPlayer(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	st.RecordUnlocked("stokes", "first_blood", "gl1tch-mud") //nolint:errcheck
	st.RecordUnlocked("stokes", "ghost", "gl1tch-mud")       //nolint:errcheck

	rows, err := st.UnlockedForPlayer("stokes", "gl1tch-mud")
	if err != nil {
		t.Fatalf("UnlockedForPlayer: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("got %d unlocked, want 2", len(rows))
	}
}

func TestActionCountsForPlayer(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	st.RecordAction("stokes", "robots", false, "combat.won", 1) //nolint:errcheck
	st.RecordAction("stokes", "robots", false, "combat.won", 1) //nolint:errcheck
	st.RecordAction("stokes", "robots", false, "hack.success", 1) //nolint:errcheck

	counts, err := st.ActionCountsForPlayer("stokes")
	if err != nil {
		t.Fatalf("ActionCountsForPlayer: %v", err)
	}
	if counts["combat.won"] != 2 {
		t.Errorf("combat.won count = %d, want 2", counts["combat.won"])
	}
	if counts["hack.success"] != 1 {
		t.Errorf("hack.success count = %d, want 1", counts["hack.success"])
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd ~/Projects/gl1tch-gamification
go test ./internal/store/...
```

Expected: compilation error (package does not exist yet).

- [ ] **Step 3: Implement store.go**

`internal/store/store.go`:
```go
package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the gamification SQLite database.
type Store struct {
	db *sql.DB
}

// Player is a leaderboard entry for a single player or agent.
type Player struct {
	ID          string
	Faction     string
	Score       int
	ActionCount int
	IsAgent     bool
	LastSeen    time.Time
}

// Faction is an aggregated leaderboard entry for a group.
type Faction struct {
	ID          string
	Score       int
	MemberCount int
	LastActive  time.Time
	Members     []Player
}

// UnlockedRow is a single achievement unlock record.
type UnlockedRow struct {
	AchievementID string
	Source        string
	UnlockedAt    time.Time
}

// Open opens (or creates) the gamification database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			id           TEXT PRIMARY KEY,
			faction      TEXT NOT NULL DEFAULT '',
			score        INTEGER NOT NULL DEFAULT 0,
			action_count INTEGER NOT NULL DEFAULT 0,
			is_agent     INTEGER NOT NULL DEFAULT 0,
			last_seen    INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS factions (
			id           TEXT PRIMARY KEY,
			score        INTEGER NOT NULL DEFAULT 0,
			member_count INTEGER NOT NULL DEFAULT 0,
			last_active  INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS unlocked (
			player         TEXT NOT NULL,
			achievement_id TEXT NOT NULL,
			source         TEXT NOT NULL DEFAULT '',
			unlocked_at    INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (player, achievement_id)
		);
		CREATE TABLE IF NOT EXISTS action_counts (
			player TEXT NOT NULL,
			action TEXT NOT NULL,
			count  INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (player, action)
		);
	`)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error { return s.db.Close() }

// RecordAction upserts player, faction, and action_count rows for one game action.
func (s *Store) RecordAction(player, faction string, agent bool, action string, value int) error {
	now := time.Now().Unix()
	isAgent := 0
	if agent {
		isAgent = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO players (id, faction, score, action_count, is_agent, last_seen)
		VALUES (?, ?, ?, 1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			faction      = excluded.faction,
			score        = score + excluded.score,
			action_count = action_count + 1,
			is_agent     = excluded.is_agent,
			last_seen    = excluded.last_seen
	`, player, faction, value, isAgent, now)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO factions (id, score, member_count, last_active)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(id) DO UPDATE SET
			score        = score + excluded.score,
			last_active  = excluded.last_active
	`, faction, value, now)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO action_counts (player, action, count) VALUES (?, ?, 1)
		ON CONFLICT(player, action) DO UPDATE SET count = count + 1
	`, player, action)
	return err
}

// ActionCount returns the number of times player has performed action.
func (s *Store) ActionCount(player, action string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT count FROM action_counts WHERE player = ? AND action = ?`,
		player, action,
	).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// ActionCountsForPlayer returns all action counts for player as a map.
func (s *Store) ActionCountsForPlayer(player string) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT action, count FROM action_counts WHERE player = ?`, player)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int)
	for rows.Next() {
		var action string
		var count int
		if err := rows.Scan(&action, &count); err != nil {
			return nil, err
		}
		counts[action] = count
	}
	return counts, rows.Err()
}

// GetPlayer returns the player record for id.
func (s *Store) GetPlayer(id string) (Player, error) {
	var p Player
	var isAgent int
	var lastSeen int64
	err := s.db.QueryRow(
		`SELECT id, faction, score, action_count, is_agent, last_seen FROM players WHERE id = ?`, id,
	).Scan(&p.ID, &p.Faction, &p.Score, &p.ActionCount, &isAgent, &lastSeen)
	if err != nil {
		return p, err
	}
	p.IsAgent = isAgent == 1
	p.LastSeen = time.Unix(lastSeen, 0)
	return p, nil
}

// RecordUnlocked records an achievement unlock. Idempotent.
func (s *Store) RecordUnlocked(player, achievementID, source string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO unlocked (player, achievement_id, source, unlocked_at)
		VALUES (?, ?, ?, ?)
	`, player, achievementID, source, time.Now().Unix())
	return err
}

// IsUnlocked returns true if player has already unlocked achievementID.
func (s *Store) IsUnlocked(player, achievementID string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM unlocked WHERE player = ? AND achievement_id = ?`,
		player, achievementID,
	).Scan(&n)
	return n > 0, err
}

// UnlockedForPlayer returns all unlocked achievements for player from source.
func (s *Store) UnlockedForPlayer(player, source string) ([]UnlockedRow, error) {
	rows, err := s.db.Query(
		`SELECT achievement_id, source, unlocked_at FROM unlocked WHERE player = ? AND source = ?`,
		player, source,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []UnlockedRow
	for rows.Next() {
		var r UnlockedRow
		var ts int64
		if err := rows.Scan(&r.AchievementID, &r.Source, &ts); err != nil {
			return nil, err
		}
		r.UnlockedAt = time.Unix(ts, 0)
		result = append(result, r)
	}
	return result, rows.Err()
}

// TopFactions returns the top n factions by score, each with their members sorted by score.
func (s *Store) TopFactions(n int) ([]Faction, error) {
	frows, err := s.db.Query(
		`SELECT id, score, member_count, last_active FROM factions ORDER BY score DESC LIMIT ?`, n,
	)
	if err != nil {
		return nil, err
	}
	defer frows.Close()

	var factions []Faction
	for frows.Next() {
		var f Faction
		var lastActive int64
		if err := frows.Scan(&f.ID, &f.Score, &f.MemberCount, &lastActive); err != nil {
			return nil, err
		}
		f.LastActive = time.Unix(lastActive, 0)
		factions = append(factions, f)
	}
	if err := frows.Err(); err != nil {
		return nil, err
	}

	for i, f := range factions {
		members, err := s.membersForFaction(f.ID)
		if err != nil {
			return nil, err
		}
		factions[i].Members = members
	}
	return factions, nil
}

func (s *Store) membersForFaction(factionID string) ([]Player, error) {
	rows, err := s.db.Query(
		`SELECT id, faction, score, action_count, is_agent, last_seen
		 FROM players WHERE faction = ? ORDER BY score DESC`, factionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var players []Player
	for rows.Next() {
		var p Player
		var isAgent int
		var lastSeen int64
		if err := rows.Scan(&p.ID, &p.Faction, &p.Score, &p.ActionCount, &isAgent, &lastSeen); err != nil {
			return nil, err
		}
		p.IsAgent = isAgent == 1
		p.LastSeen = time.Unix(lastSeen, 0)
		players = append(players, p)
	}
	return players, rows.Err()
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd ~/Projects/gl1tch-gamification
go test ./internal/store/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): gamification SQLite store with players, factions, unlocked, action_counts"
```

---

### Task A3: Catalog package

**Files:**
- Create: `~/Projects/gl1tch-gamification/internal/catalog/catalog.go`
- Create: `~/Projects/gl1tch-gamification/internal/catalog/catalog_test.go`

- [ ] **Step 1: Write failing tests**

`internal/catalog/catalog_test.go`:
```go
package catalog_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-gamification/internal/catalog"
)

func TestRegisterAndLookup(t *testing.T) {
	c := catalog.New()
	c.Register("gl1tch-mud", []catalog.Achievement{
		{ID: "first_blood", Name: "First Blood", Trigger: catalog.Trigger{Action: "combat.won", Count: 1}, XP: 50},
		{ID: "ghost", Name: "Ghost", Trigger: catalog.Trigger{Action: "hack.success", Count: 10}, XP: 100},
	})

	got := c.ForSource("gl1tch-mud")
	if len(got) != 2 {
		t.Fatalf("got %d achievements, want 2", len(got))
	}
}

func TestCheckEligible(t *testing.T) {
	c := catalog.New()
	c.Register("gl1tch-mud", []catalog.Achievement{
		{ID: "first_blood", Name: "First Blood", Trigger: catalog.Trigger{Action: "combat.won", Count: 1}, XP: 50},
	})

	// Not eligible — count too low
	eligible := c.CheckEligible("gl1tch-mud", map[string]int{"combat.won": 0})
	if len(eligible) != 0 {
		t.Errorf("expected no eligible achievements, got %d", len(eligible))
	}

	// Eligible — count met
	eligible = c.CheckEligible("gl1tch-mud", map[string]int{"combat.won": 1})
	if len(eligible) != 1 {
		t.Fatalf("expected 1 eligible achievement, got %d", len(eligible))
	}
	if eligible[0].ID != "first_blood" {
		t.Errorf("got %q, want %q", eligible[0].ID, "first_blood")
	}
}

func TestRegisterOverwrite(t *testing.T) {
	c := catalog.New()
	c.Register("gl1tch-mud", []catalog.Achievement{
		{ID: "first_blood", Name: "First Blood", Trigger: catalog.Trigger{Action: "combat.won", Count: 1}},
	})
	c.Register("gl1tch-mud", []catalog.Achievement{
		{ID: "ghost", Name: "Ghost", Trigger: catalog.Trigger{Action: "hack.success", Count: 10}},
	})
	// Re-registration replaces the catalog for that source
	got := c.ForSource("gl1tch-mud")
	if len(got) != 1 || got[0].ID != "ghost" {
		t.Errorf("expected catalog replaced, got %v", got)
	}
}
```

- [ ] **Step 2: Run — verify failure**

```bash
go test ./internal/catalog/...
```

Expected: compilation error.

- [ ] **Step 3: Implement catalog.go**

`internal/catalog/catalog.go`:
```go
package catalog

import "sync"

// Trigger defines when an achievement is earned.
type Trigger struct {
	Action string `yaml:"action"`
	Count  int    `yaml:"count"`
}

// Achievement is a single achievement definition.
type Achievement struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Trigger     Trigger `yaml:"trigger"`
	XP          int     `yaml:"xp"`
}

// Catalog holds in-memory achievement definitions registered by game sources.
type Catalog struct {
	mu      sync.RWMutex
	entries map[string][]Achievement // source → achievements
}

// New returns an empty Catalog.
func New() *Catalog {
	return &Catalog{entries: make(map[string][]Achievement)}
}

// Register replaces the achievement catalog for source.
func (c *Catalog) Register(source string, achievements []Achievement) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[source] = achievements
}

// ForSource returns all achievements registered for source.
func (c *Catalog) ForSource(source string) []Achievement {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entries[source]
}

// CheckEligible returns achievements from source where actionCounts meets the trigger threshold.
func (c *Catalog) CheckEligible(source string, actionCounts map[string]int) []Achievement {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var eligible []Achievement
	for _, a := range c.entries[source] {
		if actionCounts[a.Trigger.Action] >= a.Trigger.Count {
			eligible = append(eligible, a)
		}
	}
	return eligible
}
```

- [ ] **Step 4: Run tests — verify pass**

```bash
go test ./internal/catalog/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/
git commit -m "feat(catalog): in-memory achievement catalog with source registration"
```

---

### Task A4: Engine package

**Files:**
- Create: `~/Projects/gl1tch-gamification/internal/engine/engine.go`
- Create: `~/Projects/gl1tch-gamification/internal/engine/engine_test.go`

- [ ] **Step 1: Write failing tests**

`internal/engine/engine_test.go`:
```go
package engine_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-gamification/internal/catalog"
	"github.com/adam-stokes/gl1tch-gamification/internal/engine"
	"github.com/adam-stokes/gl1tch-gamification/internal/store"
)

func setup(t *testing.T) (*store.Store, *catalog.Catalog) {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cat := catalog.New()
	cat.Register("gl1tch-mud", []catalog.Achievement{
		{ID: "first_blood", Name: "First Blood", Trigger: catalog.Trigger{Action: "combat.won", Count: 1}, XP: 50},
		{ID: "veteran", Name: "Veteran", Trigger: catalog.Trigger{Action: "combat.won", Count: 5}, XP: 200},
	})
	return st, cat
}

func TestProcessAction_NoUnlock(t *testing.T) {
	st, cat := setup(t)
	e := engine.New(st, cat)

	unlocked, err := e.ProcessAction(engine.Action{
		Source:  "gl1tch-mud",
		Player:  "stokes",
		Faction: "robots",
		Agent:   false,
		Name:    "hack.success",
		Value:   1,
	})
	if err != nil {
		t.Fatalf("ProcessAction: %v", err)
	}
	if len(unlocked) != 0 {
		t.Errorf("expected no unlocks, got %v", unlocked)
	}
}

func TestProcessAction_Unlock(t *testing.T) {
	st, cat := setup(t)
	e := engine.New(st, cat)

	unlocked, err := e.ProcessAction(engine.Action{
		Source:  "gl1tch-mud",
		Player:  "stokes",
		Faction: "robots",
		Agent:   false,
		Name:    "combat.won",
		Value:   1,
	})
	if err != nil {
		t.Fatalf("ProcessAction: %v", err)
	}
	if len(unlocked) != 1 {
		t.Fatalf("expected 1 unlock, got %d", len(unlocked))
	}
	if unlocked[0].ID != "first_blood" {
		t.Errorf("got %q, want first_blood", unlocked[0].ID)
	}
}

func TestProcessAction_Idempotent(t *testing.T) {
	st, cat := setup(t)
	e := engine.New(st, cat)

	act := engine.Action{Source: "gl1tch-mud", Player: "stokes", Faction: "robots", Name: "combat.won", Value: 1}

	first, _ := e.ProcessAction(act)
	second, _ := e.ProcessAction(act)

	if len(first) != 1 {
		t.Fatalf("first call: expected 1 unlock, got %d", len(first))
	}
	if len(second) != 0 {
		t.Errorf("second call: expected no new unlocks (idempotent), got %d", len(second))
	}
}

func TestProcessAction_MultipleUnlocks(t *testing.T) {
	st, cat := setup(t)
	e := engine.New(st, cat)

	act := engine.Action{Source: "gl1tch-mud", Player: "stokes", Faction: "robots", Name: "combat.won", Value: 1}
	for i := 0; i < 4; i++ {
		e.ProcessAction(act) //nolint:errcheck
	}
	unlocked, err := e.ProcessAction(act) // 5th win → veteran
	if err != nil {
		t.Fatalf("ProcessAction: %v", err)
	}
	if len(unlocked) != 1 || unlocked[0].ID != "veteran" {
		t.Errorf("expected veteran unlock, got %v", unlocked)
	}
}
```

- [ ] **Step 2: Run — verify failure**

```bash
go test ./internal/engine/...
```

Expected: compilation error.

- [ ] **Step 3: Implement engine.go**

`internal/engine/engine.go`:
```go
package engine

import (
	"github.com/adam-stokes/gl1tch-gamification/internal/catalog"
	"github.com/adam-stokes/gl1tch-gamification/internal/store"
)

// Action is a normalized game event submitted for processing.
type Action struct {
	Source  string
	Player  string
	Faction string
	Agent   bool
	Name    string // e.g. "combat.won"
	Value   int
}

// Engine processes game actions and evaluates achievement triggers.
type Engine struct {
	store   *store.Store
	catalog *catalog.Catalog
}

// New returns an Engine backed by st and cat.
func New(st *store.Store, cat *catalog.Catalog) *Engine {
	return &Engine{store: st, catalog: cat}
}

// ProcessAction records the action, evaluates achievements, and returns newly unlocked ones.
func (e *Engine) ProcessAction(a Action) ([]catalog.Achievement, error) {
	if err := e.store.RecordAction(a.Player, a.Faction, a.Agent, a.Name, a.Value); err != nil {
		return nil, err
	}

	counts, err := e.store.ActionCountsForPlayer(a.Player)
	if err != nil {
		return nil, err
	}

	eligible := e.catalog.CheckEligible(a.Source, counts)

	var newUnlocks []catalog.Achievement
	for _, ach := range eligible {
		already, err := e.store.IsUnlocked(a.Player, ach.ID)
		if err != nil {
			return nil, err
		}
		if already {
			continue
		}
		if err := e.store.RecordUnlocked(a.Player, ach.ID, a.Source); err != nil {
			return nil, err
		}
		newUnlocks = append(newUnlocks, ach)
	}
	return newUnlocks, nil
}
```

- [ ] **Step 4: Run tests — verify pass**

```bash
go test ./internal/engine/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/
git commit -m "feat(engine): action processor with achievement evaluation"
```

---

### Task A5: BUSD package

**Files:**
- Create: `~/Projects/gl1tch-gamification/internal/busd/busd.go`
- Create: `~/Projects/gl1tch-gamification/internal/busd/busd_test.go`

- [ ] **Step 1: Write failing tests**

`internal/busd/busd_test.go`:
```go
package busd_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-gamification/internal/busd"
)

// Connect returns a no-op client when socket is absent.
func TestConnect_NoSocket(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // point to empty dir — no socket
	c := busd.Connect("glitch-gamification", []string{"game.action"})
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	// Publish on a no-op client must not panic
	c.Publish("game.achievement.unlocked", map[string]any{"test": true})
}
```

- [ ] **Step 2: Run — verify failure**

```bash
go test ./internal/busd/...
```

Expected: compilation error.

- [ ] **Step 3: Implement busd.go**

`internal/busd/busd.go`:
```go
// Package busd provides a minimal BUSD client for glitch-gamification.
// Mirrors the pattern in gl1tch-mud/internal/busd.
package busd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Client is a connection to the gl1tch BUSD socket.
type Client struct {
	conn net.Conn
}

// Event is an incoming frame from the daemon.
type Event struct {
	Topic   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

// Connect opens a connection and subscribes to the given topic patterns.
// Returns a no-op client if the socket is unavailable.
func Connect(name string, subscribe []string) *Client {
	path := socketPath()
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return &Client{}
	}
	reg, _ := json.Marshal(map[string]any{"name": name, "subscribe": subscribe})
	conn.Write(append(reg, '\n')) //nolint:errcheck
	return &Client{conn: conn}
}

// Publish sends an event. Silent no-op if not connected.
func (c *Client) Publish(topic string, payload any) {
	if c.conn == nil {
		return
	}
	frame, err := json.Marshal(map[string]any{
		"action":  "publish",
		"event":   topic,
		"payload": payload,
	})
	if err != nil {
		return
	}
	c.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)) //nolint:errcheck
	fmt.Fprintf(c.conn, "%s\n", frame)
}

// Listen reads incoming events and calls fn for each. Blocks until closed.
// No-op if not connected.
func (c *Client) Listen(fn func(Event)) {
	if c.conn == nil {
		return
	}
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Topic != "" {
			fn(ev)
		}
	}
}

// Close closes the connection.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func socketPath() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "glitch", "bus.sock")
	}
	cache, _ := os.UserCacheDir()
	return filepath.Join(cache, "glitch", "bus.sock")
}
```

- [ ] **Step 4: Run tests — verify pass**

```bash
go test ./internal/busd/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/busd/
git commit -m "feat(busd): BUSD client for gamification daemon"
```

---

### Task A6: Daemon command + protocol types

**Files:**
- Create: `~/Projects/gl1tch-gamification/cmd/daemon.go`
- Create: `~/Projects/gl1tch-gamification/internal/protocol/protocol.go`

- [ ] **Step 1: Write protocol types**

`internal/protocol/protocol.go`:
```go
// Package protocol defines the BUSD event payloads for the gamification system.
package protocol

// ActionPayload is the payload of a game.action event.
type ActionPayload struct {
	Source  string         `json:"source"`
	Player  string         `json:"player"`
	Faction string         `json:"faction"`
	Agent   bool           `json:"agent"`
	Action  string         `json:"action"`
	Value   int            `json:"value"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// CatalogRegisterPayload is the payload of a game.catalog.register event.
type CatalogRegisterPayload struct {
	Source       string        `json:"source"`
	Version      string        `json:"version"`
	Achievements []Achievement `json:"achievements"`
}

// Achievement in the registration payload.
type Achievement struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Trigger     Trigger `json:"trigger"`
	XP          int     `json:"xp"`
}

// Trigger defines when an achievement is earned.
type Trigger struct {
	Action string `json:"action"`
	Count  int    `json:"count"`
}

// TopRequestPayload is the payload of a game.top.request event.
type TopRequestPayload struct {
	RequestID string `json:"request_id"`
	Player    string `json:"player"`
}

// TopReplyPayload is the payload of a game.top.reply event.
type TopReplyPayload struct {
	RequestID string         `json:"request_id"`
	Entries   []FactionEntry `json:"entries"`
}

// FactionEntry is one ranked faction in the leaderboard.
type FactionEntry struct {
	Rank         int           `json:"rank"`
	Faction      string        `json:"faction"`
	FactionScore int           `json:"faction_score"`
	Members      []MemberEntry `json:"members"`
}

// MemberEntry is one player/agent within a faction.
type MemberEntry struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`
	IsAgent bool   `json:"agent"`
}

// AchievementUnlockedPayload is the payload of a game.achievement.unlocked event.
type AchievementUnlockedPayload struct {
	Player        string `json:"player"`
	AchievementID string `json:"achievement_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	XP            int    `json:"xp"`
	Source        string `json:"source"`
}

// AchievementsRequestPayload is the payload of a game.achievements.request event.
type AchievementsRequestPayload struct {
	RequestID string `json:"request_id"`
	Player    string `json:"player"`
	Source    string `json:"source"`
}

// AchievementsReplyPayload is the payload of a game.achievements.reply event.
type AchievementsReplyPayload struct {
	RequestID  string              `json:"request_id"`
	Player     string              `json:"player"`
	Unlocked   []UnlockedEntry     `json:"unlocked"`
	InProgress []InProgressEntry   `json:"in_progress"`
}

// UnlockedEntry is one unlocked achievement in the reply.
type UnlockedEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// InProgressEntry is one in-progress achievement with current count.
type InProgressEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Progress    int    `json:"progress"`
	Total       int    `json:"total"`
}
```

- [ ] **Step 2: Write daemon.go**

`cmd/daemon.go`:
```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adam-stokes/gl1tch-gamification/internal/busd"
	"github.com/adam-stokes/gl1tch-gamification/internal/catalog"
	"github.com/adam-stokes/gl1tch-gamification/internal/engine"
	"github.com/adam-stokes/gl1tch-gamification/internal/protocol"
	"github.com/adam-stokes/gl1tch-gamification/internal/store"
	"github.com/spf13/cobra"
)

func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the gamification daemon",
		RunE:  runDaemon,
	}
}

func runDaemon(_ *cobra.Command, _ []string) error {
	dbPath, err := dbFilePath()
	if err != nil {
		return fmt.Errorf("db path: %w", err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	cat := catalog.New()
	eng := engine.New(st, cat)

	topics := []string{
		"game.action",
		"game.catalog.register",
		"game.top.request",
		"game.achievements.request",
	}
	client := busd.Connect("glitch-gamification", topics)
	defer client.Close()

	fmt.Fprintln(os.Stderr, "glitch-gamification daemon running")

	client.Listen(func(ev busd.Event) {
		switch ev.Topic {
		case "game.catalog.register":
			handleCatalogRegister(cat, ev.Payload)
		case "game.action":
			handleAction(eng, cat, client, ev.Payload)
		case "game.top.request":
			handleTopRequest(st, client, ev.Payload)
		case "game.achievements.request":
			handleAchievementsRequest(st, cat, client, ev.Payload)
		}
	})
	return nil
}

func handleCatalogRegister(cat *catalog.Catalog, raw json.RawMessage) {
	var p protocol.CatalogRegisterPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	achievements := make([]catalog.Achievement, len(p.Achievements))
	for i, a := range p.Achievements {
		achievements[i] = catalog.Achievement{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			XP:          a.XP,
			Trigger:     catalog.Trigger{Action: a.Trigger.Action, Count: a.Trigger.Count},
		}
	}
	cat.Register(p.Source, achievements)
	fmt.Fprintf(os.Stderr, "catalog registered: source=%s count=%d\n", p.Source, len(achievements))
}

func handleAction(eng *engine.Engine, cat *catalog.Catalog, client *busd.Client, raw json.RawMessage) {
	var p protocol.ActionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	if p.Player == "" || p.Source == "" {
		return
	}
	faction := p.Faction
	if faction == "" {
		faction = "unaffiliated"
	}
	unlocked, err := eng.ProcessAction(engine.Action{
		Source:  p.Source,
		Player:  p.Player,
		Faction: faction,
		Agent:   p.Agent,
		Name:    p.Action,
		Value:   max(p.Value, 1),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "process action error: %v\n", err)
		return
	}
	for _, ach := range unlocked {
		client.Publish("game.achievement.unlocked", protocol.AchievementUnlockedPayload{
			Player:        p.Player,
			AchievementID: ach.ID,
			Name:          ach.Name,
			Description:   ach.Description,
			XP:            ach.XP,
			Source:        p.Source,
		})
	}
}

func handleTopRequest(st *store.Store, client *busd.Client, raw json.RawMessage) {
	var req protocol.TopRequestPayload
	if err := json.Unmarshal(raw, &req); err != nil {
		return
	}
	factions, err := st.TopFactions(10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "top factions error: %v\n", err)
		return
	}
	entries := make([]protocol.FactionEntry, len(factions))
	for i, f := range factions {
		members := make([]protocol.MemberEntry, len(f.Members))
		for j, m := range f.Members {
			members[j] = protocol.MemberEntry{Name: m.ID, Score: m.Score, IsAgent: m.IsAgent}
		}
		entries[i] = protocol.FactionEntry{
			Rank:         i + 1,
			Faction:      f.ID,
			FactionScore: f.Score,
			Members:      members,
		}
	}
	client.Publish("game.top.reply", protocol.TopReplyPayload{
		RequestID: req.RequestID,
		Entries:   entries,
	})
}

func handleAchievementsRequest(st *store.Store, cat *catalog.Catalog, client *busd.Client, raw json.RawMessage) {
	var req protocol.AchievementsRequestPayload
	if err := json.Unmarshal(raw, &req); err != nil {
		return
	}

	unlockedRows, err := st.UnlockedForPlayer(req.Player, req.Source)
	if err != nil {
		return
	}
	unlockedIDs := make(map[string]bool, len(unlockedRows))
	unlockedEntries := make([]protocol.UnlockedEntry, 0, len(unlockedRows))
	for _, r := range unlockedRows {
		unlockedIDs[r.AchievementID] = true
		unlockedEntries = append(unlockedEntries, protocol.UnlockedEntry{
			ID:          r.AchievementID,
			Name:        r.AchievementID, // name resolved from catalog below
			Description: "",
		})
	}

	// Resolve names from catalog
	all := cat.ForSource(req.Source)
	byID := make(map[string]catalog.Achievement, len(all))
	for _, a := range all {
		byID[a.ID] = a
	}
	for i, e := range unlockedEntries {
		if a, ok := byID[e.ID]; ok {
			unlockedEntries[i].Name = a.Name
			unlockedEntries[i].Description = a.Description
		}
	}

	// In-progress: not unlocked, but count > 0
	counts, err := st.ActionCountsForPlayer(req.Player)
	if err != nil {
		return
	}
	var inProgress []protocol.InProgressEntry
	for _, a := range all {
		if unlockedIDs[a.ID] {
			continue
		}
		current := counts[a.Trigger.Action]
		if current > 0 {
			inProgress = append(inProgress, protocol.InProgressEntry{
				ID:          a.ID,
				Name:        a.Name,
				Description: a.Description,
				Progress:    current,
				Total:       a.Trigger.Count,
			})
		}
	}

	client.Publish("game.achievements.reply", protocol.AchievementsReplyPayload{
		RequestID:  req.RequestID,
		Player:     req.Player,
		Unlocked:   unlockedEntries,
		InProgress: inProgress,
	})
}

func dbFilePath() (string, error) {
	data, err := os.UserDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(data, "glitch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "gamification.db"), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 3: Build and verify it compiles**

```bash
cd ~/Projects/gl1tch-gamification
go build ./...
```

Expected: no errors, `glitch-gamification` binary produced.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ internal/protocol/
git commit -m "feat(daemon): BUSD daemon with action handling, top, and achievements"
```

---

### Task A7: Install and smoke test

**Files:** none new

- [ ] **Step 1: Install**

```bash
cd ~/Projects/gl1tch-gamification
make install
```

Expected: `glitch-gamification` installed to `~/.local/bin/`.

- [ ] **Step 2: Verify binary runs**

```bash
glitch-gamification --help
```

Expected: usage output with `daemon` subcommand.

- [ ] **Step 3: Commit Makefile if unchanged**

```bash
git status
# Nothing to commit if build was clean
```

---

## Track B — gl1tch-mud changes

---

### Task B1: Achievement catalog YAML

**Files:**
- Create: `achievements.yaml` (repo root of gl1tch-mud)

- [ ] **Step 1: Write achievements.yaml**

`achievements.yaml`:
```yaml
source: gl1tch-mud
version: "1.0.0"
achievements:
  - id: first_blood
    name: "First Blood"
    description: "Win your first combat"
    trigger:
      action: combat.won
      count: 1
    xp: 50

  - id: soldier
    name: "Soldier"
    description: "Win 10 combats"
    trigger:
      action: combat.won
      count: 10
    xp: 150

  - id: ghost
    name: "Ghost"
    description: "Successfully hack 10 systems"
    trigger:
      action: hack.success
      count: 10
    xp: 200

  - id: merchant
    name: "Merchant"
    description: "Complete 5 trades"
    trigger:
      action: trade.completed
      count: 5
    xp: 100

  - id: maker
    name: "Maker"
    description: "Craft 5 items"
    trigger:
      action: craft.completed
      count: 5
    xp: 100

  - id: locksmith
    name: "Locksmith"
    description: "Pick 3 locks"
    trigger:
      action: lock.picked
      count: 3
    xp: 75

  - id: cartographer
    name: "Cartographer"
    description: "Discover 50 new rooms"
    trigger:
      action: room.explored
      count: 50
    xp: 300

  - id: survivor
    name: "Survivor"
    description: "Die and come back 3 times"
    trigger:
      action: player.died
      count: 3
    xp: 25
```

- [ ] **Step 2: Commit**

```bash
cd ~/Projects/gl1tch-mud
git add achievements.yaml
git commit -m "feat(achievements): gl1tch-mud achievement catalog"
```

---

### Task B2: Event adapter

**Files:**
- Create: `internal/busd/gamification.go`
- Create: `internal/busd/gamification_test.go`

- [ ] **Step 1: Write failing tests**

`internal/busd/gamification_test.go`:
```go
package busd_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/busd"
)

func TestMapMudEvent_CombatWon(t *testing.T) {
	action, ok := busd.MapMudEvent("mud.combat.ended", map[string]any{"outcome": "won"})
	if !ok {
		t.Fatal("expected ok")
	}
	if action != "combat.won" {
		t.Errorf("got %q, want %q", action, "combat.won")
	}
}

func TestMapMudEvent_CombatLost(t *testing.T) {
	action, ok := busd.MapMudEvent("mud.combat.ended", map[string]any{"outcome": "lost"})
	if !ok {
		t.Fatal("expected ok")
	}
	if action != "combat.lost" {
		t.Errorf("got %q, want %q", action, "combat.lost")
	}
}

func TestMapMudEvent_RoomEnteredFirst(t *testing.T) {
	action, ok := busd.MapMudEvent("mud.room.entered", map[string]any{"first": true})
	if !ok {
		t.Fatal("expected ok=true for first room")
	}
	if action != "room.explored" {
		t.Errorf("got %q, want %q", action, "room.explored")
	}
}

func TestMapMudEvent_RoomEnteredNotFirst(t *testing.T) {
	_, ok := busd.MapMudEvent("mud.room.entered", map[string]any{"first": false})
	if ok {
		t.Error("expected ok=false for revisit")
	}
}

func TestMapMudEvent_Unknown(t *testing.T) {
	_, ok := busd.MapMudEvent("mud.session.started", nil)
	if ok {
		t.Error("expected ok=false for unmapped topic")
	}
}

func TestMapMudEvent_SimpleTopics(t *testing.T) {
	cases := []struct {
		topic  string
		action string
	}{
		{"mud.hack.success", "hack.success"},
		{"mud.trade.completed", "trade.completed"},
		{"mud.craft.completed", "craft.completed"},
		{"mud.lock.picked", "lock.picked"},
		{"mud.player.died", "player.died"},
	}
	for _, tc := range cases {
		got, ok := busd.MapMudEvent(tc.topic, nil)
		if !ok {
			t.Errorf("topic %q: expected ok=true", tc.topic)
			continue
		}
		if got != tc.action {
			t.Errorf("topic %q: got %q, want %q", tc.topic, got, tc.action)
		}
	}
}
```

- [ ] **Step 2: Run — verify failure**

```bash
go test ./internal/busd/...
```

Expected: `MapMudEvent undefined`.

- [ ] **Step 3: Implement gamification.go**

`internal/busd/gamification.go`:
```go
package busd

// MapMudEvent maps a mud BUSD topic and its payload to a game.action name.
// Returns the action name and true if the event should be forwarded to gamification.
// Returns "", false if the event should be ignored.
func MapMudEvent(topic string, payload map[string]any) (string, bool) {
	switch topic {
	case "mud.combat.ended":
		outcome, _ := payload["outcome"].(string)
		switch outcome {
		case "won":
			return "combat.won", true
		case "lost":
			return "combat.lost", true
		}
		return "", false
	case "mud.room.entered":
		first, _ := payload["first"].(bool)
		if !first {
			return "", false
		}
		return "room.explored", true
	case "mud.hack.success":
		return "hack.success", true
	case "mud.trade.completed":
		return "trade.completed", true
	case "mud.craft.completed":
		return "craft.completed", true
	case "mud.lock.picked":
		return "lock.picked", true
	case "mud.player.died":
		return "player.died", true
	default:
		return "", false
	}
}
```

- [ ] **Step 4: Run tests — verify pass**

```bash
go test ./internal/busd/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/busd/gamification.go internal/busd/gamification_test.go
git commit -m "feat(busd): mud→gamification event adapter"
```

---

### Task B3: Publish game.action from command results

**Files:**
- Modify: `internal/server/session.go`

The session's command dispatch loop already calls `gs.busClient.Publish` for mud events. We need to also publish `game.action` when a mud event maps to one.

- [ ] **Step 1: Read the relevant section**

Read `internal/server/session.go` lines 88-260. Find the block where `ev.Topic` is published to the bus (search for `busPub`). It looks like:

```go
if cmd.Event != nil {
    s.registry.PublishEvent(cmd.Event.Topic, cmd.Event.Payload)
}
```

- [ ] **Step 2: Extend event publishing to also emit game.action**

In `internal/server/session.go`, find the `if cmd.Event != nil` block (around line 248). Extend it:

```go
if cmd.Event != nil {
    s.registry.PublishEvent(cmd.Event.Topic, cmd.Event.Payload)
    // Also forward to gamification if this event maps to a game action.
    payload, _ := cmd.Event.Payload.(map[string]any)
    if action, ok := busd.MapMudEvent(cmd.Event.Topic, payload); ok {
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
```

`factions.Get` is in `internal/factions/player_faction.go` — it queries `player_faction WHERE id=1` and returns `*PlayerFaction` with a `FactionID` field. Import `github.com/adam-stokes/gl1tch-mud/internal/factions`.

- [ ] **Step 4: Build to verify no compilation errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/server/session.go
git commit -m "feat(session): forward mud events to game.action for gamification"
```

---

### Task B4: Register achievement catalog on server start

**Files:**
- Modify: `internal/server/server.go`
- Create: `internal/achievements/achievements.go`

- [ ] **Step 1: Write achievements loader**

`internal/achievements/achievements.go`:
```go
package achievements

import (
	"os"

	"gopkg.in/yaml.v3"
)

// CatalogFile is the on-disk schema for achievements.yaml.
type CatalogFile struct {
	Source       string        `yaml:"source"`
	Version      string        `yaml:"version"`
	Achievements []Achievement `yaml:"achievements"`
}

// Achievement is one achievement definition.
type Achievement struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Trigger     Trigger `yaml:"trigger"`
	XP          int     `yaml:"xp"`
}

// Trigger defines when an achievement is earned.
type Trigger struct {
	Action string `yaml:"action"`
	Count  int    `yaml:"count"`
}

// Load reads and parses the catalog file at path.
func Load(path string) (*CatalogFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cf CatalogFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}
```

- [ ] **Step 2: Write test for loader**

`internal/achievements/achievements_test.go`:
```go
package achievements_test

import (
	"os"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/achievements"
)

func TestLoad(t *testing.T) {
	yaml := `
source: gl1tch-mud
version: "1.0.0"
achievements:
  - id: first_blood
    name: "First Blood"
    description: "Win your first combat"
    trigger:
      action: combat.won
      count: 1
    xp: 50
`
	f, err := os.CreateTemp(t.TempDir(), "achievements*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(yaml)
	f.Close()

	cf, err := achievements.Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cf.Source != "gl1tch-mud" {
		t.Errorf("source = %q, want gl1tch-mud", cf.Source)
	}
	if len(cf.Achievements) != 1 {
		t.Fatalf("got %d achievements, want 1", len(cf.Achievements))
	}
	if cf.Achievements[0].ID != "first_blood" {
		t.Errorf("id = %q, want first_blood", cf.Achievements[0].ID)
	}
}

func TestLoad_Missing(t *testing.T) {
	_, err := achievements.Load("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
```

- [ ] **Step 3: Run tests — verify pass**

```bash
go test ./internal/achievements/... -v
```

Expected: PASS.

- [ ] **Step 4: Add yaml dependency**

```bash
go get gopkg.in/yaml.v3
go mod tidy
```

- [ ] **Step 5: Publish catalog on server start**

In `internal/server/server.go`, find `func (gs *GameServer) Start(` and after the bus client is connected add:

```go
// Register achievement catalog with gamification daemon (best-effort).
go func() {
    cf, err := achievements.Load("achievements.yaml")
    if err != nil {
        // No catalog file — skip registration silently.
        return
    }
    type achPayload struct {
        ID          string `json:"id"`
        Name        string `json:"name"`
        Description string `json:"description"`
        Trigger     struct {
            Action string `json:"action"`
            Count  int    `json:"count"`
        } `json:"trigger"`
        XP int `json:"xp"`
    }
    achs := make([]achPayload, len(cf.Achievements))
    for i, a := range cf.Achievements {
        achs[i] = achPayload{ID: a.ID, Name: a.Name, Description: a.Description, XP: a.XP}
        achs[i].Trigger.Action = a.Trigger.Action
        achs[i].Trigger.Count = a.Trigger.Count
    }
    gs.busClient.Publish("game.catalog.register", map[string]any{
        "source":       cf.Source,
        "version":      cf.Version,
        "achievements": achs,
    })
}()
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/achievements/ internal/server/server.go go.mod go.sum
git commit -m "feat(server): register achievement catalog with gamification on startup"
```

---

### Task B5: Server subscriptions + player routing

**Files:**
- Modify: `internal/server/server.go`

The server needs to:
1. Subscribe to `game.achievement.unlocked`, `game.top.reply`, `game.achievements.reply`
2. Route top/achievements replies to the requesting player (by request_id)
3. Show achievement unlocks as glitch chat messages to the relevant player

- [ ] **Step 1: Add SendToPlayer to SessionRegistry**

In `internal/server/server.go`, add after the `BroadcastToWorld` function:

```go
// SendToPlayer sends msg to the session for playerID. No-op if not connected.
func (r *SessionRegistry) SendToPlayer(playerID string, msg ServerMsg) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.sessions[playerID]; ok {
		ctx := context.Background()
		_ = writeMsg(ctx, s.conn, msg)
	}
}
```

- [ ] **Step 2: Add pending request map to GameServer**

In `internal/server/server.go`, find the `GameServer` struct and add two fields:

```go
type GameServer struct {
    // ... existing fields ...
    pendingMu      sync.Mutex
    pendingRequests map[string]string // requestID → playerID
}
```

Initialize it in `newGameServer` (or wherever the struct is constructed):

```go
gs.pendingRequests = make(map[string]string)
```

- [ ] **Step 3: Extend bus subscriptions**

Find the `ConnectWithSubscriptions([]string{"mud.chat.reply"})` call in `server.go` and extend the topics:

```go
gs.busClient = busd.ConnectWithSubscriptions([]string{
    "mud.chat.reply",
    "game.achievement.unlocked",
    "game.top.reply",
    "game.achievements.reply",
})
```

- [ ] **Step 4: Extend the Listen handler**

Find the `go gs.busClient.Listen(func(ev busd.Event)` block and add cases for the new topics after the `mud.chat.reply` case:

```go
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
```

- [ ] **Step 5: Build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/server/server.go
git commit -m "feat(server): subscribe to gamification events and route to players"
```

---

### Task B6: top and achievements commands

**Files:**
- Create: `internal/commands/gamification.go`
- Modify: `internal/commands/commands.go`

- [ ] **Step 1: Write the commands**

`internal/commands/gamification.go`:
```go
package commands

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	// Commands registered in commands.go Registry.
}

// handleTop publishes a game.top.request and returns immediately.
// The reply arrives via the server's bus listener and is sent to the player as a chat message.
func handleTop(db *sql.DB, s *player.State, _ *world.World, _ []string) Result {
	requestID := fmt.Sprintf("top-%d", time.Now().UnixNano())
	return Result{
		Output: "fetching leaderboard…",
		Event: &Event{
			Topic: "game.top.request",
			Payload: map[string]any{
				"request_id": requestID,
				"player":     s.PlayerID,
			},
		},
		PendingRequestID: requestID,
		PendingPlayer:    s.PlayerID,
	}
}

// handleAchievements publishes a game.achievements.request.
func handleAchievements(db *sql.DB, s *player.State, _ *world.World, _ []string) Result {
	requestID := fmt.Sprintf("ach-%d", time.Now().UnixNano())
	return Result{
		Output: "fetching achievements…",
		Event: &Event{
			Topic: "game.achievements.request",
			Payload: map[string]any{
				"request_id": requestID,
				"player":     s.PlayerID,
				"source":     "gl1tch-mud",
			},
		},
		PendingRequestID: requestID,
		PendingPlayer:    s.PlayerID,
	}
}
```

- [ ] **Step 2: Extend the Result type**

In `internal/commands/commands.go`, find the `Result` struct and add two fields:

```go
type Result struct {
    Output           string
    Event            *Event
    SwitchWorld      string
    PendingRequestID string // non-empty: register this request_id → player in server
    PendingPlayer    string
}
```

- [ ] **Step 3: Register the commands**

In `internal/commands/commands.go`, find the `Registry` map and add:

```go
"top":          handleTop,
"achievements": handleAchievements,
```

- [ ] **Step 4: Handle PendingRequestID in session**

In `internal/server/session.go`, find where `cmd.Event` is checked and published. After the gamification `game.action` forwarding added in Task B3, add:

```go
if cmd.PendingRequestID != "" {
    s.registry.RegisterPendingRequest(cmd.PendingRequestID, cmd.PendingPlayer)
}
```

Add `RegisterPendingRequest` to `SessionRegistry` in `server.go`:

```go
func (r *SessionRegistry) RegisterPendingRequest(requestID, playerID string) {
    r.gs.pendingMu.Lock()
    r.gs.pendingRequests[requestID] = playerID
    r.gs.pendingMu.Unlock()
}
```

Note: `SessionRegistry` needs a reference to `GameServer` for this. Look at the existing registry structure — if it doesn't have `gs`, use a callback instead:

```go
// In SessionRegistry struct, add:
onPendingRequest func(requestID, playerID string)

// Set it in GameServer.Start():
gs.registry.onPendingRequest = func(rid, pid string) {
    gs.pendingMu.Lock()
    gs.pendingRequests[rid] = pid
    gs.pendingMu.Unlock()
}

// In RegisterPendingRequest:
func (r *SessionRegistry) RegisterPendingRequest(requestID, playerID string) {
    if r.onPendingRequest != nil {
        r.onPendingRequest(requestID, playerID)
    }
}
```

- [ ] **Step 5: Add PlayerID to player.State**

`player.State` currently has: `Name, RoomID, HP, MaxHP, World` — no `PlayerID`. Add it:

In `internal/player/player.go`, add `PlayerID string` to the `State` struct:

```go
type State struct {
    PlayerID string  // authenticated player ID (set by session, not DB)
    Name     string
    RoomID   string
    HP       int
    MaxHP    int
    World    string
}
```

In `internal/server/session.go`, find where `player.LoadForWorld` is called and set `PlayerID` immediately after:

```go
s.state, err = player.LoadForWorld(s.database, s.worldName, startRoom)
if err != nil { ... }
s.state.PlayerID = s.playerID
```

Now `handleTop` and `handleAchievements` can use `s.PlayerID` correctly.

- [ ] **Step 6: Build**

```bash
go build ./...
```

Fix any compilation errors from the struct changes.

- [ ] **Step 7: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/commands/gamification.go internal/commands/commands.go internal/server/
git commit -m "feat(commands): top and achievements commands via gamification daemon"
```

---

## Integration smoke test

After both tracks are complete:

- [ ] **Step 1: Start gamification daemon in background**

```bash
glitch-gamification daemon &
```

- [ ] **Step 2: Start gl1tch-mud server**

```bash
go run . --world neon-city  # or however the server starts
```

- [ ] **Step 3: Connect and trigger an event**

In the game client, win a combat. Verify:
- No crash in server
- `game.action` published (check daemon stderr)
- `game.achievement.unlocked` fires for `first_blood`
- Achievement appears as glitch chat message in-game

- [ ] **Step 4: Run `top` command in game**

Type `top` in the game. Verify the leaderboard appears as a glitch chat message.

- [ ] **Step 5: Run `achievements` command**

Type `achievements` in the game. Verify `First Blood` appears as unlocked.

- [ ] **Step 6: Final commit in each repo**

```bash
# gl1tch-gamification
cd ~/Projects/gl1tch-gamification
git tag v0.1.0

# gl1tch-mud
cd ~/Projects/gl1tch-mud
git add -A
git commit -m "feat(gamification): full integration with glitch-gamification daemon"
```
