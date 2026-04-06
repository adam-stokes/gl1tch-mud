package base_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"

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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := makeBaseWorld()

	score := base.DefenseScore(gdb, w)
	if score != 0 {
		t.Errorf("empty base: got %d want 0", score)
	}
}

func TestDefenseScore(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := makeBaseWorld()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Reinforced Walls',1)`) //nolint:errcheck
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-turret','Sentry Turret',2)`)   //nolint:errcheck

	score := base.DefenseScore(gdb, w)
	if score != 8 {
		t.Errorf("walls+turret: got %d want 8", score)
	}
}

func TestDefenseScoreIgnoresOtherRooms(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := makeBaseWorld()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-0','base-walls','Walls',1)`) //nolint:errcheck

	score := base.DefenseScore(gdb, w)
	if score != 0 {
		t.Errorf("other room structure: got %d want 0", score)
	}
}

func TestMaybeSpawnRaid_spawns(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Foundation',1)`) //nolint:errcheck
	setActionCount(db, 30)

	base.MaybeSpawnRaid(gdb)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid' AND target_room='dusthaven-4' AND status='active'`).Scan(&cnt) //nolint:errcheck
	if cnt != 1 {
		t.Errorf("raid not spawned: got %d active base-raid events, want 1", cnt)
	}
}

func TestMaybeSpawnRaid_noSpawnWithoutStructures(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	setActionCount(db, 30)
	base.MaybeSpawnRaid(gdb)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid'`).Scan(&cnt) //nolint:errcheck
	if cnt != 0 {
		t.Errorf("raid spawned without structures: got %d, want 0", cnt)
	}
}

func TestMaybeSpawnRaid_noSpawnIfActiveRaid(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Foundation',1)`) //nolint:errcheck
	db.Exec(`INSERT INTO world_events (id, type, title, description, target_room, faction, payout_credits, payout_item_id, payout_item_name, payout_item_desc, status, expires_actions, created_actions, created_at) VALUES ('existing-raid','base-raid','Raid','Desc','dusthaven-4','ash-raiders',0,'','','','active',30,0,0)`) //nolint:errcheck
	setActionCount(db, 30)

	base.MaybeSpawnRaid(gdb)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid' AND status='active'`).Scan(&cnt) //nolint:errcheck
	if cnt != 1 {
		t.Errorf("duplicate raid spawned: got %d active raids, want 1", cnt)
	}
}

func TestMaybeSpawnRaid_noSpawnIfNotMultipleOf30(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','foundation','Foundation',1)`) //nolint:errcheck
	setActionCount(db, 31)

	base.MaybeSpawnRaid(gdb)

	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM world_events WHERE type='base-raid'`).Scan(&cnt) //nolint:errcheck
	if cnt != 0 {
		t.Errorf("raid spawned at non-multiple of 30: got %d, want 0", cnt)
	}
}

func TestResolvePendingRaids_noPending(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := makeBaseWorld()

	report := base.ResolvePendingRaids(gdb, w)
	if report != "" {
		t.Errorf("no raids: expected empty report, got %q", report)
	}
}

func TestResolvePendingRaids_repelled(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()
	w := makeBaseWorld()

	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-walls','Walls',1)`)   //nolint:errcheck
	db.Exec(`INSERT INTO builds (room_id, build_id, name, placed_at) VALUES ('dusthaven-4','base-turret','Turret',2)`) //nolint:errcheck
	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','food-1','Canned Food')`)          //nolint:errcheck

	db.Exec(`INSERT INTO world_events (id, type, title, description, target_room, faction, payout_credits, payout_item_id, payout_item_name, payout_item_desc, status, expires_actions, created_actions, created_at) VALUES ('raid-1','base-raid','Raid','Desc','dusthaven-4','ash-raiders',0,'','','','active',30,0,0)`) //nolint:errcheck
	setActionCount(db, 60)

	report := base.ResolvePendingRaids(gdb, w)

	if report == "" {
		t.Error("expected non-empty report")
	}
	if !strings.Contains(report, "RAID REPORT") {
		t.Errorf("report missing RAID REPORT header: %q", report)
	}

	var status string
	db.QueryRow(`SELECT status FROM world_events WHERE id='raid-1'`).Scan(&status) //nolint:errcheck
	if status != "resolved" {
		t.Errorf("event status: got %q want 'resolved'", status)
	}
}

func TestResolvePendingRaids_brokenThrough(t *testing.T) {
	db := openBaseTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	w := &world.World{} // no recipes = defense 0, raid always breaks through

	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','item-1','Scrap Iron')`)  //nolint:errcheck
	db.Exec(`INSERT INTO chests (room_id, item_id, item_name) VALUES ('dusthaven-4','item-2','Canned Food')`) //nolint:errcheck

	db.Exec(`INSERT INTO world_events (id, type, title, description, target_room, faction, payout_credits, payout_item_id, payout_item_name, payout_item_desc, status, expires_actions, created_actions, created_at) VALUES ('raid-2','base-raid','Raid','Desc','dusthaven-4','ash-raiders',0,'','','','active',30,0,0)`) //nolint:errcheck
	setActionCount(db, 60)

	report := base.ResolvePendingRaids(gdb, w)

	if report == "" {
		t.Error("expected non-empty report")
	}
	if !strings.Contains(report, "RAID REPORT") {
		t.Errorf("report missing RAID REPORT: %q", report)
	}

	var status string
	db.QueryRow(`SELECT status FROM world_events WHERE id='raid-2'`).Scan(&status) //nolint:errcheck
	if status != "resolved" {
		t.Errorf("event status: got %q want 'resolved'", status)
	}

	var remaining int
	db.QueryRow(`SELECT COUNT(*) FROM chests WHERE room_id='dusthaven-4'`).Scan(&remaining) //nolint:errcheck
	if remaining == 2 {
		t.Error("raid broke through with defense=0 but no items were lost")
	}
}
