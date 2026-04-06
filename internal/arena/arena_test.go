package arena_test

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"

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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	if err := arena.StartTDM(gdb); err != nil {
		t.Fatalf("StartTDM: %v", err)
	}

	m := arena.GetActive(gdb)
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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := emptyWorld() // defense=0, no turret damage

	if err := arena.StartTowerDefense(gdb, w); err != nil {
		t.Fatalf("StartTowerDefense: %v", err)
	}

	m := arena.GetActive(gdb)
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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	if m := arena.GetActive(gdb); m != nil {
		t.Errorf("expected nil, got %+v", m)
	}
}

func TestProcessAttack_TDM_damagesEnemy(t *testing.T) {
	db := openArenaDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := emptyWorld()
	s := freshState()

	arena.StartTDM(gdb) //nolint:errcheck
	res := arena.ProcessAttack(gdb, w, s)

	if res.Lost || res.Won {
		t.Fatalf("unexpected match end after one attack: won=%v lost=%v", res.Won, res.Lost)
	}
	m := arena.GetActive(gdb)
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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := emptyWorld()
	s := freshState() // Defense=0

	arena.StartTDM(gdb) //nolint:errcheck
	arena.ProcessAttack(gdb, w, s) // one attack

	// 5 enemies attack for max(1, 8-0)=8 each → player HP = 100 - (5*8) = 60
	if s.HP != 60 {
		t.Errorf("player HP after counterattack: got %d want 60", s.HP)
	}
}

func TestProcessAttack_TDM_win(t *testing.T) {
	db := openArenaDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 1000, MaxHP: 1000, Defense: 0}

	arena.StartTDM(gdb) //nolint:errcheck
	// Need 2 attacks per enemy (HP=30, playerDmg=15): 10 attacks total
	var res arena.AttackResult
	for i := 0; i < 10; i++ {
		res = arena.ProcessAttack(gdb, w, s)
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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 1, MaxHP: 100, Defense: 0} // 1 HP — any counterattack kills

	arena.StartTDM(gdb) //nolint:errcheck
	res := arena.ProcessAttack(gdb, w, s)

	if !res.Lost {
		t.Errorf("expected Lost with 1 HP, got: lost=%v", res.Lost)
	}
	m := arena.GetActive(gdb)
	if m != nil {
		t.Error("match should not be active after loss")
	}
}

func TestStartTowerDefense_turretsDamageOnStart(t *testing.T) {
	db := openArenaDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	// Build a structure with defense=3 in dusthaven-4
	w := &world.World{}
	w.CraftingRecipes = []world.CraftingRecipe{
		{ID: "base-walls", Name: "Walls", Output: world.Item{ID: "base-walls", Stats: map[string]int{"defense": 3}}},
	}
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Walls',1)`) //nolint:errcheck

	arena.StartTowerDefense(gdb, w) //nolint:errcheck

	m := arena.GetActive(gdb)
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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 1000, MaxHP: 1000, Defense: 0}

	arena.StartTowerDefense(gdb, w) //nolint:errcheck
	// Kill 3 enemies: each needs 2 attacks (HP=25, dmg=15 → 10 → dead)
	for i := 0; i < 6; i++ {
		arena.ProcessAttack(gdb, w, s) //nolint:errcheck
	}
	// Now all 3 dead — next attack should advance to wave 1
	res := arena.ProcessAttack(gdb, w, s)

	m := arena.GetActive(gdb)
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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := emptyWorld()
	s := &player.State{HP: 10000, MaxHP: 10000, Defense: 0}

	arena.StartTowerDefense(gdb, w) //nolint:errcheck

	// Win by attacking enough times
	var res arena.AttackResult
	for i := 0; i < 100 && !res.Won; i++ {
		res = arena.ProcessAttack(gdb, w, s)
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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	arena.StartTDM(gdb) //nolint:errcheck
	msg := arena.Quit(gdb)

	if msg == "" {
		t.Error("expected non-empty quit message")
	}
	if m := arena.GetActive(gdb); m != nil {
		t.Error("match should not be active after quit")
	}
}
