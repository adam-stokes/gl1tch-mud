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

	if err := player.AddItem(db, "iron-sword", "Iron Sword", "A sharp blade."); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := player.AddItem(db, "bread", "Bread", "Restores HP."); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

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

	if err := player.AddItem(db, "coal", "Coal", "Fuel."); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if err := player.RemoveItem(db, "coal"); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	items, _ := player.Inventory(db)
	if len(items) != 0 {
		t.Errorf("item should be removed, got %d", len(items))
	}
}
