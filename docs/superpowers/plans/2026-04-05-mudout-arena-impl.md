# Mudout Arena Mini-Games — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add TDM (5-raider deathmatch at barrens-3) and Tower Defense (3-wave survival at ruins-3) as single-player-vs-AI arena modes, with the player's dusthaven-4 defense score used as turret auto-damage in tower defense.

**Architecture:** New `internal/arena` package owns all match logic (start, attack, quit). `arena_sessions` DB table persists one active match per player. The `Attack` command intercepts to arena combat when a match is active; a new `Arena` command starts/shows/quits matches. `Result.MoveRoom` (new field) triggers in-room teleport handled by session.go.

**Tech Stack:** Go 1.21, SQLite (modernc.org/sqlite), encoding/json

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Modify | `internal/db/schema.go` | Add `arena_sessions` table |
| Modify | `internal/db/schema_test.go` | Add `arena_sessions` to table list |
| Create | `internal/arena/arena.go` | All match logic: types, start, attack, quit |
| Create | `internal/arena/arena_test.go` | Full arena test suite |
| Modify | `internal/commands/commands.go` | Add `MoveRoom` to Result; add `Arena` command; add arena intercept at top of `Attack`; wire registry |
| Create | `internal/commands/arena_test.go` | Arena command + Attack intercept tests |
| Modify | `internal/server/session.go` | Handle `result.MoveRoom` after command dispatch |
| Modify | `internal/world/defaults/mudout/world.yaml` | Update barrens-3 and ruins-3 desc |

---

## Task 1: DB schema — arena_sessions table

**Files:**
- Modify: `internal/db/schema.go`
- Modify: `internal/db/schema_test.go`

- [ ] **Step 1: Add arena_sessions to schema.go**

In `internal/db/schema.go`, find the `equipped_armor` table block at the end. Add immediately after it (before the closing backtick):

```sql

CREATE TABLE IF NOT EXISTS arena_sessions (
    id               TEXT    PRIMARY KEY,
    game_type        TEXT    NOT NULL,
    phase            TEXT    NOT NULL DEFAULT 'fight',
    wave             INTEGER NOT NULL DEFAULT 0,
    enemies_json     TEXT    NOT NULL DEFAULT '[]',
    reward_credits   INTEGER NOT NULL DEFAULT 0,
    reward_item_id   TEXT    NOT NULL DEFAULT '',
    reward_item_name TEXT    NOT NULL DEFAULT '',
    reward_item_desc TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'active',
    started_at       INTEGER NOT NULL DEFAULT 0
);
```

- [ ] **Step 2: Add arena_sessions to schema_test.go**

In `internal/db/schema_test.go`, find the `tables := []string{` slice. Add `"arena_sessions"` after `"equipped_armor"`:

```go
tables := []string{
    "player",
    "inventory",
    "npc_state",
    "visited",
    "player_skills",
    "player_reputation",
    "system_state",
    "lock_state",
    "npc_memory",
    "player_stealth",
    "generated_content",
    "equipped_armor",
    "arena_sessions",
}
```

- [ ] **Step 3: Run schema tests**

```
go test ./internal/db/... -v 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/db/schema.go internal/db/schema_test.go
git commit -m "feat(arena): add arena_sessions table to schema"
```

---

## Task 2: `internal/arena` package

**Files:**
- Create: `internal/arena/arena.go`
- Create: `internal/arena/arena_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/arena/arena_test.go`:

```go
package arena_test

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/arena"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openArenaDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE arena_sessions (
			id               TEXT    PRIMARY KEY,
			game_type        TEXT    NOT NULL,
			phase            TEXT    NOT NULL DEFAULT 'fight',
			wave             INTEGER NOT NULL DEFAULT 0,
			enemies_json     TEXT    NOT NULL DEFAULT '[]',
			reward_credits   INTEGER NOT NULL DEFAULT 0,
			reward_item_id   TEXT    NOT NULL DEFAULT '',
			reward_item_name TEXT    NOT NULL DEFAULT '',
			reward_item_desc TEXT    NOT NULL DEFAULT '',
			status           TEXT    NOT NULL DEFAULT 'active',
			started_at       INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE player_credits (
			id      INTEGER PRIMARY KEY CHECK(id=1),
			credits INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO player_credits (id, credits) VALUES (1, 0);
		CREATE TABLE inventory (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id   TEXT NOT NULL UNIQUE,
			item_name TEXT NOT NULL,
			item_desc TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE builds (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id   TEXT    NOT NULL,
			build_id  TEXT    NOT NULL,
			name      TEXT    NOT NULL,
			desc      TEXT    NOT NULL DEFAULT '',
			placed_at INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE player_actions (id INT PRIMARY KEY CHECK(id=1), count INT DEFAULT 0);
		INSERT INTO player_actions (id, count) VALUES (1, 0);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func emptyWorld() *world.World { return &world.World{} }

func freshState() *player.State {
	return &player.State{HP: 100, MaxHP: 100, Defense: 0}
}

func TestStartTDM(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()

	if err := arena.StartTDM(db); err != nil {
		t.Fatalf("StartTDM: %v", err)
	}

	m := arena.GetActive(db)
	if m == nil {
		t.Fatal("expected active match after StartTDM")
	}
	if m.GameType != "tdm" {
		t.Errorf("GameType: got %q want tdm", m.GameType)
	}
	if len(m.Enemies) != 5 {
		t.Errorf("enemy count: got %d want 5", len(m.Enemies))
	}
	for _, e := range m.Enemies {
		if !e.Alive {
			t.Errorf("enemy %s should be alive at start", e.ID)
		}
		if e.HP != 30 {
			t.Errorf("enemy HP: got %d want 30", e.HP)
		}
	}
	if m.RewardCredits != 200 {
		t.Errorf("reward: got %d want 200", m.RewardCredits)
	}
}

func TestStartTowerDefense(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()
	w := emptyWorld() // defense=0, no turret damage

	if err := arena.StartTowerDefense(db, w); err != nil {
		t.Fatalf("StartTowerDefense: %v", err)
	}

	m := arena.GetActive(db)
	if m == nil {
		t.Fatal("expected active match")
	}
	if m.GameType != "tower-defense" {
		t.Errorf("GameType: got %q want tower-defense", m.GameType)
	}
	if len(m.Enemies) != 3 {
		t.Errorf("enemy count: got %d want 3", len(m.Enemies))
	}
	if m.Wave != 0 {
		t.Errorf("wave: got %d want 0", m.Wave)
	}
	if m.RewardItemID != "pre-war-circuitry" {
		t.Errorf("reward item: got %q want pre-war-circuitry", m.RewardItemID)
	}
}

func TestGetActive_none(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()

	if m := arena.GetActive(db); m != nil {
		t.Errorf("expected nil, got %+v", m)
	}
}

func TestProcessAttack_TDM_damagesEnemy(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()
	w := emptyWorld()
	s := freshState()

	arena.StartTDM(db) //nolint:errcheck
	res := arena.ProcessAttack(db, w, s)

	if res.Lost || res.Won {
		t.Fatalf("unexpected match end after one attack: won=%v lost=%v", res.Won, res.Lost)
	}
	m := arena.GetActive(db)
	if m == nil {
		t.Fatal("match should still be active")
	}
	// First enemy should have 15 HP left (30 - 15)
	if m.Enemies[0].HP != 15 {
		t.Errorf("first enemy HP: got %d want 15", m.Enemies[0].HP)
	}
}

func TestProcessAttack_TDM_enemiesCounterattack(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()
	w := emptyWorld()
	s := freshState() // Defense=0

	arena.StartTDM(db) //nolint:errcheck
	arena.ProcessAttack(db, w, s) // one attack

	// 5 enemies attack for max(1, 8-0)=8 each → player HP = 100 - (5*8) = 60
	if s.HP != 60 {
		t.Errorf("player HP after counterattack: got %d want 60", s.HP)
	}
}

func TestProcessAttack_TDM_win(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 1000, MaxHP: 1000, Defense: 0}

	arena.StartTDM(db) //nolint:errcheck
	// Need 2 attacks per enemy (HP=30, playerDmg=15): 10 attacks total
	var res arena.AttackResult
	for i := 0; i < 10; i++ {
		res = arena.ProcessAttack(db, w, s)
	}

	if !res.Won {
		t.Errorf("expected Won after killing all 5 enemies (10 attacks), got: won=%v output=%q", res.Won, res.Output)
	}

	// Credits should be deposited
	var credits int
	db.QueryRow(`SELECT credits FROM player_credits WHERE id=1`).Scan(&credits) //nolint:errcheck
	if credits != 200 {
		t.Errorf("credits after TDM win: got %d want 200", credits)
	}
}

func TestProcessAttack_TDM_loss(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 1, MaxHP: 100, Defense: 0} // 1 HP — any counterattack kills

	arena.StartTDM(db) //nolint:errcheck
	res := arena.ProcessAttack(db, w, s)

	if !res.Lost {
		t.Errorf("expected Lost with 1 HP, got: lost=%v", res.Lost)
	}
	m := arena.GetActive(db)
	if m != nil {
		t.Error("match should not be active after loss")
	}
}

func TestStartTowerDefense_turretsDamageOnStart(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()

	// Build a structure with defense=3 in dusthaven-4
	w := &world.World{}
	w.CraftingRecipes = []world.CraftingRecipe{
		{ID: "base-walls", Name: "Walls", Output: world.Item{ID: "base-walls", Stats: map[string]int{"defense": 3}}},
	}
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Walls',1)`) //nolint:errcheck

	arena.StartTowerDefense(db, w) //nolint:errcheck

	m := arena.GetActive(db)
	if m == nil {
		t.Fatal("expected active match")
	}
	// defScore=3, 3 enemies: each takes 1 damage (3/3=1 each). HP = 25-1 = 24
	for i, e := range m.Enemies {
		if e.HP != 24 {
			t.Errorf("enemy[%d] HP: got %d want 24 (25 - 1 turret)", i, e.HP)
		}
	}
}

func TestProcessAttack_TD_waveClear(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 1000, MaxHP: 1000, Defense: 0}

	arena.StartTowerDefense(db, w) //nolint:errcheck
	// Kill 3 enemies: each needs 2 attacks (HP=25, dmg=15 → 10 → dead)
	for i := 0; i < 6; i++ {
		arena.ProcessAttack(db, w, s) //nolint:errcheck
	}
	// Now all 3 dead — next attack should advance to wave 1
	res := arena.ProcessAttack(db, w, s)

	m := arena.GetActive(db)
	if m == nil {
		t.Fatal("match should still be active after wave 0")
	}
	if m.Wave != 1 {
		t.Errorf("wave: got %d want 1", m.Wave)
	}
	if len(m.Enemies) != 3 {
		t.Errorf("new wave enemy count: got %d want 3", len(m.Enemies))
	}
	_ = res
}

func TestProcessAttack_TD_win(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 10000, MaxHP: 10000, Defense: 0}

	arena.StartTowerDefense(db, w) //nolint:errcheck

	// Win by attacking enough times:
	// Wave 0: 3 enemies × 2 attacks = 6 attacks, then 1 more to advance wave
	// Wave 1: same, Wave 2: same → win on the wave-advance after wave 2
	var res arena.AttackResult
	for i := 0; i < 100 && !res.Won; i++ {
		res = arena.ProcessAttack(db, w, s)
	}

	if !res.Won {
		t.Error("expected Won after completing all 3 waves")
	}

	var credits int
	db.QueryRow(`SELECT credits FROM player_credits WHERE id=1`).Scan(&credits) //nolint:errcheck
	if credits != 300 {
		t.Errorf("credits after TD win: got %d want 300", credits)
	}

	var invCount int
	db.QueryRow(`SELECT COUNT(*) FROM inventory WHERE item_id='pre-war-circuitry'`).Scan(&invCount) //nolint:errcheck
	if invCount != 1 {
		t.Errorf("pre-war-circuitry in inventory: got %d want 1", invCount)
	}
}

func TestQuit(t *testing.T) {
	db := openArenaDB(t)
	defer db.Close()

	arena.StartTDM(db) //nolint:errcheck
	msg := arena.Quit(db)

	if msg == "" {
		t.Error("expected non-empty quit message")
	}
	if m := arena.GetActive(db); m != nil {
		t.Error("match should not be active after quit")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/arena/... -v 2>&1 | tail -5
```

Expected: FAIL — package not found.

- [ ] **Step 3: Create `internal/arena/arena.go`**

Create `internal/arena/arena.go`:

```go
// Package arena manages single-player arena mini-game sessions.
package arena

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/base"
	"github.com/adam-stokes/gl1tch-mud/internal/credits"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

const (
	tdmRaiderCount   = 5
	tdWaveCount      = 3
	tdRaidersPerWave = 3
	playerDamage     = 15
)

// Enemy represents one opponent in an arena match.
type Enemy struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	HP     int    `json:"hp"`
	Attack int    `json:"attack"`
	Alive  bool   `json:"alive"`
}

// Match represents a loaded arena session row.
type Match struct {
	ID              string
	GameType        string
	Phase           string
	Wave            int
	Enemies         []Enemy
	RewardCredits   int
	RewardItemID    string
	RewardItemName  string
	RewardItemDesc  string
	Status          string
	StartedAt       int64
}

// AttackResult is returned by ProcessAttack.
type AttackResult struct {
	Output string
	Won    bool
	Lost   bool
}

// StartTDM creates a new active TDM match with 5 raiders.
func StartTDM(db *sql.DB) error {
	enemies := makeTDMEnemies()
	enemyJSON, _ := json.Marshal(enemies)
	id := fmt.Sprintf("arena-%d", time.Now().UnixNano())
	_, err := db.Exec(
		`INSERT OR REPLACE INTO arena_sessions
		 (id, game_type, phase, wave, enemies_json, reward_credits,
		  reward_item_id, reward_item_name, reward_item_desc, status, started_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, "tdm", "fight", 0, string(enemyJSON), 200,
		"", "", "", "active", time.Now().Unix(),
	)
	return err
}

// StartTowerDefense creates a new active tower-defense match with wave 0.
// Applies turret auto-damage (base.DefenseScore) to the first wave's enemies immediately.
func StartTowerDefense(db *sql.DB, w *world.World) error {
	enemies := makeTDEnemies()
	defScore := base.DefenseScore(db, w)
	enemies = applyTurretDamage(enemies, defScore)
	enemyJSON, _ := json.Marshal(enemies)
	id := fmt.Sprintf("arena-%d", time.Now().UnixNano())
	_, err := db.Exec(
		`INSERT OR REPLACE INTO arena_sessions
		 (id, game_type, phase, wave, enemies_json, reward_credits,
		  reward_item_id, reward_item_name, reward_item_desc, status, started_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, "tower-defense", "wave", 0, string(enemyJSON), 300,
		"pre-war-circuitry", "Pre-War Circuitry", "High-density pre-war circuit board.",
		"active", time.Now().Unix(),
	)
	return err
}

// GetActive returns the current active match, or nil if none exists.
func GetActive(db *sql.DB) *Match {
	var m Match
	var enemyJSON string
	err := db.QueryRow(
		`SELECT id, game_type, phase, wave, enemies_json, reward_credits,
		        reward_item_id, reward_item_name, reward_item_desc, status, started_at
		 FROM arena_sessions WHERE status='active' LIMIT 1`,
	).Scan(&m.ID, &m.GameType, &m.Phase, &m.Wave, &enemyJSON,
		&m.RewardCredits, &m.RewardItemID, &m.RewardItemName, &m.RewardItemDesc,
		&m.Status, &m.StartedAt)
	if err != nil {
		return nil
	}
	json.Unmarshal([]byte(enemyJSON), &m.Enemies) //nolint:errcheck
	return &m
}

// ProcessAttack executes one combat tick in the active arena match.
// Mutates s.HP in place. Returns output and won/lost flags.
func ProcessAttack(db *sql.DB, w *world.World, s *player.State) AttackResult {
	m := GetActive(db)
	if m == nil {
		return AttackResult{Output: "no active arena match."}
	}
	var out strings.Builder
	if m.GameType == "tdm" {
		return processTDMAttack(db, m, s, &out)
	}
	return processTDAttack(db, w, m, s, &out)
}

// Quit forfeits the active match and marks it lost.
func Quit(db *sql.DB) string {
	db.Exec(`UPDATE arena_sessions SET status='lost' WHERE status='active'`) //nolint:errcheck
	return "you forfeit the match."
}

// ── internal helpers ──────────────────────────────────────────────────────────

func makeTDMEnemies() []Enemy {
	enemies := make([]Enemy, tdmRaiderCount)
	for i := range enemies {
		enemies[i] = Enemy{
			ID:     fmt.Sprintf("raider-%d", i+1),
			Name:   "Ash Raider",
			HP:     30,
			Attack: 8,
			Alive:  true,
		}
	}
	return enemies
}

func makeTDEnemies() []Enemy {
	enemies := make([]Enemy, tdRaidersPerWave)
	for i := range enemies {
		enemies[i] = Enemy{
			ID:     fmt.Sprintf("wave-raider-%d", i+1),
			Name:   "Ash Raider",
			HP:     25,
			Attack: 6,
			Alive:  true,
		}
	}
	return enemies
}

// applyTurretDamage distributes defScore damage across enemies evenly.
// Remainder is applied to enemy index 0.
func applyTurretDamage(enemies []Enemy, defScore int) []Enemy {
	if defScore <= 0 || len(enemies) == 0 {
		return enemies
	}
	perEnemy := defScore / len(enemies)
	remainder := defScore % len(enemies)
	for i := range enemies {
		dmg := perEnemy
		if i == 0 {
			dmg += remainder
		}
		enemies[i].HP -= dmg
		if enemies[i].HP <= 0 {
			enemies[i].HP = 0
			enemies[i].Alive = false
		}
	}
	return enemies
}

func aliveCount(enemies []Enemy) int {
	n := 0
	for _, e := range enemies {
		if e.Alive {
			n++
		}
	}
	return n
}

func firstAliveIdx(enemies []Enemy) int {
	for i, e := range enemies {
		if e.Alive {
			return i
		}
	}
	return -1
}

func saveMatch(db *sql.DB, m *Match) {
	enemyJSON, _ := json.Marshal(m.Enemies)
	db.Exec( //nolint:errcheck
		`UPDATE arena_sessions SET phase=?, wave=?, enemies_json=?, status=? WHERE id=?`,
		m.Phase, m.Wave, string(enemyJSON), m.Status, m.ID,
	)
}

func processTDMAttack(db *sql.DB, m *Match, s *player.State, out *strings.Builder) AttackResult {
	idx := firstAliveIdx(m.Enemies)
	if idx == -1 {
		return AttackResult{Output: "no enemies left."}
	}

	m.Enemies[idx].HP -= playerDamage
	if m.Enemies[idx].HP <= 0 {
		m.Enemies[idx].HP = 0
		m.Enemies[idx].Alive = false
		fmt.Fprintf(out, "you fire at %s. [%d dmg → dead]\n", m.Enemies[idx].Name, playerDamage)
	} else {
		fmt.Fprintf(out, "you fire at %s. [%d dmg → %d HP]\n", m.Enemies[idx].Name, playerDamage, m.Enemies[idx].HP)
	}

	alive := aliveCount(m.Enemies)
	if alive == 0 {
		m.Status = "won"
		saveMatch(db, m)
		credits.Add(db, m.RewardCredits) //nolint:errcheck
		fmt.Fprintf(out, "--- all enemies down. match won. ---\n+%d caps deposited.", m.RewardCredits)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Won: true}
	}

	for _, e := range m.Enemies {
		if !e.Alive {
			continue
		}
		dmg := e.Attack - s.Defense
		if dmg < 1 {
			dmg = 1
		}
		s.HP -= dmg
		fmt.Fprintf(out, "%s retaliates for %d. your HP: %d/%d.\n", e.Name, dmg, s.HP, s.MaxHP)
	}
	fmt.Fprintf(out, "--- %d enemies remaining ---", alive)

	if s.HP <= 0 {
		m.Status = "lost"
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Lost: true}
	}

	saveMatch(db, m)
	return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
}

func processTDAttack(db *sql.DB, w *world.World, m *Match, s *player.State, out *strings.Builder) AttackResult {
	// All current wave enemies dead — advance wave or win
	if aliveCount(m.Enemies) == 0 {
		m.Wave++
		if m.Wave >= tdWaveCount {
			m.Status = "won"
			saveMatch(db, m)
			credits.Add(db, m.RewardCredits) //nolint:errcheck
			if m.RewardItemID != "" {
				player.AddItem(db, m.RewardItemID, m.RewardItemName, m.RewardItemDesc) //nolint:errcheck
			}
			fmt.Fprintf(out, "--- all waves survived. match won. ---\n+%d caps deposited.", m.RewardCredits)
			if m.RewardItemID != "" {
				fmt.Fprintf(out, "\n%s added to inventory.", m.RewardItemName)
			}
			return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Won: true}
		}

		s.HP += 15
		if s.HP > s.MaxHP {
			s.HP = s.MaxHP
		}
		enemies := makeTDEnemies()
		defScore := base.DefenseScore(db, w)
		enemies = applyTurretDamage(enemies, defScore)
		m.Enemies = enemies
		fmt.Fprintf(out, "Wave %d cleared. +15 HP. [HP: %d/%d]\n--- Wave %d incoming ---\n", m.Wave, s.HP, s.MaxHP, m.Wave+1)
		if defScore > 0 {
			perEnemy := defScore / tdRaidersPerWave
			remainder := defScore % tdRaidersPerWave
			for i, e := range m.Enemies {
				applied := perEnemy
				if i == 0 {
					applied += remainder
				}
				if applied > 0 {
					fmt.Fprintf(out, "  %s takes %d turret damage. [%d HP]\n", e.Name, applied, e.HP)
				}
			}
		}
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
	}

	// Attack first alive enemy
	idx := firstAliveIdx(m.Enemies)
	m.Enemies[idx].HP -= playerDamage
	if m.Enemies[idx].HP <= 0 {
		m.Enemies[idx].HP = 0
		m.Enemies[idx].Alive = false
		fmt.Fprintf(out, "you fire at %s. [%d dmg → dead]\n", m.Enemies[idx].Name, playerDamage)
	} else {
		fmt.Fprintf(out, "you fire at %s. [%d dmg → %d HP]\n", m.Enemies[idx].Name, playerDamage, m.Enemies[idx].HP)
	}

	alive := aliveCount(m.Enemies)
	if alive == 0 {
		fmt.Fprintf(out, "--- wave cleared. type 'attack' to continue. ---")
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
	}

	for _, e := range m.Enemies {
		if !e.Alive {
			continue
		}
		dmg := e.Attack - s.Defense
		if dmg < 1 {
			dmg = 1
		}
		s.HP -= dmg
		fmt.Fprintf(out, "%s retaliates for %d. your HP: %d/%d.\n", e.Name, dmg, s.HP, s.MaxHP)
	}
	fmt.Fprintf(out, "--- %d enemies remaining ---", alive)

	if s.HP <= 0 {
		m.Status = "lost"
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Lost: true}
	}

	saveMatch(db, m)
	return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
}
```

- [ ] **Step 4: Run tests — verify they pass**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/arena/... -v 2>&1 | tail -30
```

Expected: all PASS (12 tests).

Note on `TestProcessAttack_TDM_win`: 5 enemies × 2 hits each = 10 attacks. `s.HP=1000` ensures no loss during the 10 attacks.

Note on `TestProcessAttack_TD_win`: up to 100 iterations with large HP. Each wave: 3 enemies × 2 attacks = 6 attacks to clear. Then 1 extra attack to trigger wave advance. 3 waves × 7 ≈ 21 attacks total; 100 iterations is safely over.

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/arena/arena.go internal/arena/arena_test.go
git commit -m "feat(arena): add arena package — TDM and tower defense match logic"
```

---

## Task 3: Commands — Arena, Attack intercept, MoveRoom, registry

**Files:**
- Modify: `internal/commands/commands.go`
- Create: `internal/commands/arena_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/commands/arena_test.go`:

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

func openArenaCommandDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE arena_sessions (
			id               TEXT    PRIMARY KEY,
			game_type        TEXT    NOT NULL,
			phase            TEXT    NOT NULL DEFAULT 'fight',
			wave             INTEGER NOT NULL DEFAULT 0,
			enemies_json     TEXT    NOT NULL DEFAULT '[]',
			reward_credits   INTEGER NOT NULL DEFAULT 0,
			reward_item_id   TEXT    NOT NULL DEFAULT '',
			reward_item_name TEXT    NOT NULL DEFAULT '',
			reward_item_desc TEXT    NOT NULL DEFAULT '',
			status           TEXT    NOT NULL DEFAULT 'active',
			started_at       INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE player_credits (
			id      INTEGER PRIMARY KEY CHECK(id=1),
			credits INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO player_credits (id, credits) VALUES (1, 0);
		CREATE TABLE inventory (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id   TEXT NOT NULL UNIQUE,
			item_name TEXT NOT NULL,
			item_desc TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE builds (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id   TEXT    NOT NULL,
			build_id  TEXT    NOT NULL,
			name      TEXT    NOT NULL,
			desc      TEXT    NOT NULL DEFAULT '',
			placed_at INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE npc_state (
			room_id TEXT NOT NULL, npc_id TEXT NOT NULL,
			hp INTEGER NOT NULL, alive INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (room_id, npc_id)
		);
		CREATE TABLE player_actions (id INT PRIMARY KEY CHECK(id=1), count INT DEFAULT 0);
		INSERT INTO player_actions (id, count) VALUES (1, 0);
		CREATE TABLE quests (
			id TEXT PRIMARY KEY, title TEXT NOT NULL, description TEXT,
			status TEXT NOT NULL DEFAULT 'active', obj_type TEXT NOT NULL,
			obj_target TEXT NOT NULL, obj_room TEXT, obj_count INTEGER NOT NULL DEFAULT 1,
			obj_progress INTEGER NOT NULL DEFAULT 0, reward_credits INTEGER NOT NULL DEFAULT 0,
			reward_xp_skill TEXT, reward_xp_amount INTEGER NOT NULL DEFAULT 0,
			reward_item_id TEXT, reward_item_name TEXT, reward_item_desc TEXT,
			giver_npc_id TEXT, accepted_at INTEGER NOT NULL, next_quest_id TEXT NOT NULL DEFAULT ''
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestArenaCommand_startTDM(t *testing.T) {
	db := openArenaCommandDB(t)
	defer db.Close()

	s := &player.State{RoomID: "barrens-3", HP: 100, MaxHP: 100}
	w := &world.World{}

	res := commands.Arena(db, s, w, nil)
	if res.Output == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(res.Output, "TDM") {
		t.Errorf("expected TDM in output, got: %q", res.Output)
	}
}

func TestArenaCommand_startTD(t *testing.T) {
	db := openArenaCommandDB(t)
	defer db.Close()

	s := &player.State{RoomID: "ruins-3", HP: 100, MaxHP: 100}
	w := &world.World{}

	res := commands.Arena(db, s, w, nil)
	if res.Output == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(strings.ToUpper(res.Output), "TOWER") {
		t.Errorf("expected TOWER in output, got: %q", res.Output)
	}
}

func TestArenaCommand_wrongRoom(t *testing.T) {
	db := openArenaCommandDB(t)
	defer db.Close()

	s := &player.State{RoomID: "dusthaven-0", HP: 100, MaxHP: 100}
	w := &world.World{}

	res := commands.Arena(db, s, w, nil)
	if !strings.Contains(res.Output, "arena entrance") {
		t.Errorf("expected 'arena entrance' hint, got: %q", res.Output)
	}
}

func TestArenaCommand_showStatus(t *testing.T) {
	db := openArenaCommandDB(t)
	defer db.Close()

	s := &player.State{RoomID: "barrens-3", HP: 100, MaxHP: 100}
	w := &world.World{}

	commands.Arena(db, s, w, nil) // start match
	res := commands.Arena(db, s, w, nil) // show status
	if !strings.Contains(res.Output, "ARENA") {
		t.Errorf("expected ARENA status, got: %q", res.Output)
	}
}

func TestArenaCommand_quit(t *testing.T) {
	db := openArenaCommandDB(t)
	defer db.Close()

	s := &player.State{RoomID: "barrens-3", HP: 100, MaxHP: 100}
	w := &world.World{}

	commands.Arena(db, s, w, nil) // start match
	res := commands.Arena(db, s, w, []string{"quit"})
	if res.MoveRoom != "dusthaven-0" {
		t.Errorf("MoveRoom: got %q want dusthaven-0", res.MoveRoom)
	}
}

func TestAttackIntercept_arenaActive(t *testing.T) {
	db := openArenaCommandDB(t)
	defer db.Close()

	s := &player.State{RoomID: "barrens-3", HP: 100, MaxHP: 100, Defense: 0}
	w := &world.World{}

	commands.Arena(db, s, w, nil) // start TDM match

	// Attack should route to arena, not room NPC
	res := commands.Attack(db, s, w, []string{"raider"})
	if res.Output == "" {
		t.Error("expected arena attack output")
	}
	// Should not say "nothing to attack" (which would mean it fell through to room NPC logic)
	if strings.Contains(res.Output, "nothing to attack") {
		t.Errorf("attack fell through to room NPC logic: %q", res.Output)
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/commands/... -run "TestArena|TestAttackIntercept" -v 2>&1 | tail -10
```

Expected: FAIL — "undefined: commands.Arena" and "res.MoveRoom undefined".

- [ ] **Step 3: Add MoveRoom to the Result struct**

In `internal/commands/commands.go`, find the `Result` struct:

```go
type Result struct {
	Output           string
	Event            *Event
	SwitchWorld      string
	PendingRequestID string
	PendingPlayer    string
}
```

Add `MoveRoom string` after `SwitchWorld`:

```go
type Result struct {
	Output           string
	Event            *Event
	SwitchWorld      string
	MoveRoom         string // non-empty: session moves player to this room ID
	PendingRequestID string
	PendingPlayer    string
}
```

- [ ] **Step 4: Add arena intercept to the Attack command**

In `internal/commands/commands.go`, find the `Attack` function. It starts:

```go
func Attack(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "attack what?"}
	}
```

Add the arena intercept immediately after the `len(args) == 0` check:

```go
func Attack(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "attack what?"}
	}

	// Arena intercept: if an active match exists, route to arena combat.
	if match := arena.GetActive(db); match != nil {
		res := arena.ProcessAttack(db, w, s)
		if res.Lost {
			s.HP = s.MaxHP / 2
			if s.HP < 1 {
				s.HP = 1
			}
			return Result{
				Output:   res.Output + "\nyou've been knocked out. back to dusthaven.",
				MoveRoom: "dusthaven-0",
			}
		}
		return Result{Output: res.Output}
	}

	// Normal room NPC combat follows.
	target := strings.ReplaceAll(strings.Join(args, " "), "-", " ")
```

Note: the existing line `target := strings.ReplaceAll(strings.Join(args, " "), "-", " ")` immediately follows the intercept block.

- [ ] **Step 5: Add Arena command function**

Append to `internal/commands/commands.go` (after the `BaseInfo` function at the end of the file):

```go
// Arena starts, shows status, or quits an arena match.
// Use at barrens-3 for TDM, ruins-3 for Tower Defense.
func Arena(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	// arena quit
	if len(args) > 0 && args[0] == "quit" {
		if arena.GetActive(db) == nil {
			return Result{Output: "no active arena match."}
		}
		msg := arena.Quit(db)
		s.HP = s.MaxHP / 2
		if s.HP < 1 {
			s.HP = 1
		}
		return Result{Output: msg + "\nyou limp back to dusthaven.", MoveRoom: "dusthaven-0"}
	}

	// Active match: show status
	if m := arena.GetActive(db); m != nil {
		alive := 0
		for _, e := range m.Enemies {
			if e.Alive {
				alive++
			}
		}
		if m.GameType == "tdm" {
			return Result{Output: fmt.Sprintf("ARENA [TDM] — %d enemies remaining. type 'attack' to fight.", alive)}
		}
		return Result{Output: fmt.Sprintf("ARENA [TOWER DEFENSE] — wave %d/3, %d enemies remaining. type 'attack' to fight.", m.Wave+1, alive)}
	}

	// No active match: start one based on room
	switch s.RoomID {
	case "barrens-3":
		if err := arena.StartTDM(db); err != nil {
			return Result{Output: fmt.Sprintf("failed to start match: %v", err)}
		}
		return Result{Output: "COMBAT ZONE — TDM\n5 Ash Raiders. Kill them all.\nReward: 200 caps.\nType 'attack' to engage. 'arena quit' to forfeit."}
	case "ruins-3":
		if err := arena.StartTowerDefense(db, w); err != nil {
			return Result{Output: fmt.Sprintf("failed to start match: %v", err)}
		}
		return Result{Output: "TOWER DEFENSE\n3 waves incoming. Your base turrets will fire at the start of each wave.\nReward: 300 caps + pre-war-circuitry.\nType 'attack' to engage. 'arena quit' to forfeit."}
	default:
		return Result{Output: "find an arena entrance first. (Combat Zone: barrens-3, Tower Defense: ruins-3)"}
	}
}
```

- [ ] **Step 6: Add arena import to commands.go**

In the import block of `internal/commands/commands.go`, add:

```go
"github.com/adam-stokes/gl1tch-mud/internal/arena"
```

- [ ] **Step 7: Wire Arena into Registry**

In `internal/commands/commands.go`, find where commands are registered (Registry map or init blocks). Add:

```go
"arena": Arena,
```

- [ ] **Step 8: Run tests — verify they pass**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/commands/... -run "TestArena|TestAttackIntercept" -v 2>&1 | tail -15
```

Expected: all PASS.

- [ ] **Step 9: Run full commands suite**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/commands/... -v 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/commands/commands.go internal/commands/arena_test.go
git commit -m "feat(arena): add Arena command, Attack arena intercept, MoveRoom to Result"
```

---

## Task 4: Session — handle result.MoveRoom

**Files:**
- Modify: `internal/server/session.go`

- [ ] **Step 1: Read session.go to find the SwitchWorld handler**

Read `internal/server/session.go`. Find the block:

```go
if result.SwitchWorld != "" {
    if err := s.switchWorld(ctx, result.SwitchWorld); err != nil {
```

- [ ] **Step 2: Add MoveRoom handler**

After the `result.PendingRequestID` block and before (or after) the `result.SwitchWorld` block, add:

```go
if result.MoveRoom != "" {
    s.state.RoomID = result.MoveRoom
    _ = player.Save(s.database, s.state)
}
```

The exact position: add it right before the `result.SwitchWorld != ""` check.

- [ ] **Step 3: Run all Go tests**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/server/session.go
git commit -m "feat(arena): handle result.MoveRoom in session — teleport player on arena loss/quit"
```

---

## Task 5: World YAML — update arena room descs

**Files:**
- Modify: `internal/world/defaults/mudout/world.yaml`

- [ ] **Step 1: Update barrens-3 desc**

In `internal/world/defaults/mudout/world.yaml`, find the `barrens-3` room. Replace its `desc` with:

```yaml
  - id: barrens-3
    name: "Combat Zone Entrance"
    desc: |
      A weathered sign above a reinforced door reads: COMBAT ZONE — ENTER AT
      OWN RISK. The sounds of distant gunfire echo from somewhere inside.
      Spectators and gamblers cluster near the entrance, exchanging caps.
      Type 'arena' to enter a Team Deathmatch. Kill all five raiders to win.
    biome: wasteland
    exits:
      north: barrens-1
    npcs: []
    items:
      - id: combat-zone-flyer
        name: "Combat Zone Flyer"
        desc: "COMBAT ZONE: Team deathmatch. Caps paid to survivors."
        readable: true
        content: "COMBAT ZONE — TDM\nKill all five Ash Raiders before they kill you.\nReward: 200 caps. Type 'arena' to begin."
```

- [ ] **Step 2: Update ruins-3 desc**

Find the `ruins-3` room. Replace its `desc` with:

```yaml
  - id: ruins-3
    name: "Tower Defense Grid"
    desc: |
      A cleared zone in the ruins, marked with painted grid lines and numbered
      positions. Old terminal screens along the walls display wave readouts.
      Your base turrets from dusthaven will fire at the start of each wave.
      Type 'arena' to start a Tower Defense match. Survive three waves to win.
    biome: ruins
    exits:
      west: ruins-0
    npcs: []
    items:
      - id: tower-defense-manual
        name: "Tower Defense Manual"
        desc: "Place your defenses. Survive the waves. Split the loot."
        readable: true
        content: "TOWER DEFENSE\nSurvive 3 waves of Ash Raiders.\nYour dusthaven-4 base turrets auto-fire each wave.\nReward: 300 caps + Pre-War Circuitry. Type 'arena' to begin."
```

- [ ] **Step 3: Run world tests**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -v 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/world/defaults/mudout/world.yaml
git commit -m "feat(arena): update barrens-3 and ruins-3 room descs with arena instructions"
```

---

## Task 6: Full verification

- [ ] **Step 1: Run complete Go test suite**

```
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1
```

Expected: all PASS across all packages.

- [ ] **Step 2: Run web test suite**

```
cd /Users/stokes/Projects/gl1tch-mud/web && npx vitest run 2>&1 | tail -10
```

Expected: all PASS.

---

## Self-Review

**Spec coverage:**
- `arena_sessions` table → Task 1 ✓
- `Enemy`, `Match`, `AttackResult` types → Task 2 ✓
- `StartTDM` (5 raiders, HP 30, attack 8, reward 200) → Task 2 ✓
- `StartTowerDefense` (wave 0, 3 raiders, HP 25, attack 6, reward 300 caps + item) → Task 2 ✓
- `GetActive` → Task 2 ✓
- `ProcessAttack` TDM path (player 15 dmg, counterattacks, win/loss) → Task 2 ✓
- `ProcessAttack` TD path (wave advance, turret auto-damage, heal between waves, win) → Task 2 ✓
- `applyTurretDamage` uses `base.DefenseScore` → Task 2 ✓
- `Quit` → Task 2 ✓
- `MoveRoom` on Result → Task 3 ✓
- `Arena` command (start TDM/TD, show status, quit) → Task 3 ✓
- `Attack` arena intercept → Task 3 ✓
- Registry wiring `"arena"` → Task 3 ✓
- Session `result.MoveRoom` handling → Task 4 ✓
- World YAML desc updates → Task 5 ✓
- Arena-loss teleport to dusthaven-0, HP=MaxHP/2 → Task 3 (Attack) + Task 4 (session.MoveRoom) ✓

**Placeholder scan:** None.

**Type consistency:**
- `arena.GetActive(db) *Match` — used identically in Task 2, Task 3 (Arena command, Attack intercept) ✓
- `arena.ProcessAttack(db *sql.DB, w *world.World, s *player.State) AttackResult` — called with all three in Attack intercept ✓
- `arena.StartTDM(db *sql.DB) error` — no world arg needed (no turret damage for TDM) ✓
- `arena.StartTowerDefense(db *sql.DB, w *world.World) error` — world needed for DefenseScore ✓
- `Result.MoveRoom string` — set in Arena (quit), Attack (loss); handled in session.go ✓
- `m.Enemies []Enemy` — JSON round-tripped through DB; `Alive bool` field used for alive checks ✓
