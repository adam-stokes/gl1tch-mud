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
