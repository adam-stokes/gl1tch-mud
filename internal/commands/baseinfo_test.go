package commands_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"

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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	s := &player.State{}
	w := makeBaseInfoWorld()

	res := commands.BaseInfo(gdb, s, w, nil)
	if res.Output == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(res.Output, "No structures") {
		t.Errorf("expected 'No structures' message, got: %q", res.Output)
	}
}

func TestBaseInfoWithStructures(t *testing.T) {
	db := openBaseInfoTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Reinforced Walls',1)`) //nolint:errcheck
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Base Foundation',2)`)  //nolint:errcheck
	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','food','Canned Food')`)                     //nolint:errcheck

	s := &player.State{}
	w := makeBaseInfoWorld()

	res := commands.BaseInfo(gdb, s, w, nil)
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
