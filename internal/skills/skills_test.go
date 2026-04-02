package skills

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS player_skills (
		skill TEXT PRIMARY KEY,
		level INTEGER NOT NULL DEFAULT 0,
		xp    INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestLevelForXP(t *testing.T) {
	cases := []struct{ xp, want int }{
		{0, 0},
		{49, 0},
		{50, 1},
		{149, 1},
		{150, 2},
		{300, 3},
		{600, 4},
		{1000, 5},
		{9999, 5},
	}
	for _, c := range cases {
		got := LevelForXP(c.xp)
		if got != c.want {
			t.Errorf("LevelForXP(%d) = %d, want %d", c.xp, got, c.want)
		}
	}
}

func TestLoadDefault(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	level, xp, err := Load(db, "hacking")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if level != 0 || xp != 0 {
		t.Errorf("expected 0,0 got %d,%d", level, xp)
	}
}

func TestAwardXP(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	res, err := Award(db, "hacking", 20)
	if err != nil {
		t.Fatalf("Award: %v", err)
	}
	if res.XP != 20 || res.LeveledUp {
		t.Errorf("unexpected result: %+v", res)
	}

	// Award enough to level up
	res2, err := Award(db, "hacking", 35) // total = 55 → level 1
	if err != nil {
		t.Fatalf("Award (level-up): %v", err)
	}
	if !res2.LeveledUp || res2.NewLevel != 1 {
		t.Errorf("expected level-up to 1, got: %+v", res2)
	}
}

func TestSkillPersists(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	Award(db, "lockpicking", 100) //nolint:errcheck

	level, xp, err := Load(db, "lockpicking")
	if err != nil {
		t.Fatalf("Load after award: %v", err)
	}
	// 100 XP → level 1 (threshold for level 2 is 150)
	if level != 1 || xp != 100 {
		t.Errorf("persist: level=%d xp=%d, want level=1 xp=100", level, xp)
	}
}

func TestAll(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	Award(db, "hacking", 10)     //nolint:errcheck
	Award(db, "lockpicking", 20) //nolint:errcheck

	m, err := All(db)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(m) != 2 {
		t.Errorf("All: got %d entries, want 2", len(m))
	}
}
