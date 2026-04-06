package enchanting_test

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"

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
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	if err := enchanting.Apply(gdb, "iron-sword", "sharpness", 1, 0); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := enchanting.List(gdb, "iron-sword")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].EnchantID != "sharpness" {
		t.Errorf("List: got %+v", got)
	}
}

func TestAddXPAndLevel(t *testing.T) {
	db := openTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	if err := enchanting.AddXP(gdb, 100); err != nil {
		t.Fatalf("AddXP: %v", err)
	}
	xp, level, err := enchanting.XPState(gdb)
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
