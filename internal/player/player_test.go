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
	items, err := player.Inventory(db)
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
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
	if err = player.ClaimDeathPile(db, "forest-1"); err != nil {
		t.Fatalf("ClaimDeathPile: %v", err)
	}

	// Inventory should have 2 items again
	items, err = player.Inventory(db)
	if err != nil {
		t.Fatalf("Inventory after claim: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("inventory after claim: want 2, got %d", len(items))
	}

	// Death pile should be empty
	pile, err = player.GetDeathPile(db, "forest-1")
	if err != nil {
		t.Fatalf("GetDeathPile after claim: %v", err)
	}
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
	items, err := player.Inventory(db)
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("item should be removed, got %d", len(items))
	}
}

func openArmorTestDB(t *testing.T) *sql.DB {
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
		CREATE TABLE equipped_armor (
			id        INTEGER PRIMARY KEY CHECK(id = 1),
			item_id   TEXT    NOT NULL,
			item_name TEXT    NOT NULL,
			defense   INTEGER NOT NULL DEFAULT 0
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestEquipArmor(t *testing.T) {
	db := openArmorTestDB(t)
	defer db.Close()

	if err := player.EquipArmor(db, "leather-armor", "Leather Armor", 3); err != nil {
		t.Fatalf("EquipArmor: %v", err)
	}

	rec, err := player.GetEquippedArmor(db)
	if err != nil {
		t.Fatalf("GetEquippedArmor: %v", err)
	}
	if rec == nil {
		t.Fatal("expected equipped armor record, got nil")
	}
	if rec.ItemID != "leather-armor" {
		t.Errorf("item_id: got %q want %q", rec.ItemID, "leather-armor")
	}
	if rec.Defense != 3 {
		t.Errorf("defense: got %d want 3", rec.Defense)
	}
}

func TestEquipArmorReplaces(t *testing.T) {
	db := openArmorTestDB(t)
	defer db.Close()

	player.EquipArmor(db, "leather-armor", "Leather Armor", 2) //nolint:errcheck
	if err := player.EquipArmor(db, "iron-vest", "Iron Vest", 5); err != nil {
		t.Fatalf("second EquipArmor: %v", err)
	}

	rec, _ := player.GetEquippedArmor(db)
	if rec == nil || rec.ItemID != "iron-vest" {
		t.Errorf("expected iron-vest to replace leather-armor, got %+v", rec)
	}
}

func TestUnequipArmor(t *testing.T) {
	db := openArmorTestDB(t)
	defer db.Close()

	player.EquipArmor(db, "leather-armor", "Leather Armor", 2) //nolint:errcheck
	if err := player.UnequipArmor(db); err != nil {
		t.Fatalf("UnequipArmor: %v", err)
	}

	rec, err := player.GetEquippedArmor(db)
	if err != nil {
		t.Fatalf("GetEquippedArmor after unequip: %v", err)
	}
	if rec != nil {
		t.Errorf("expected nil after unequip, got %+v", rec)
	}
}

func TestLoadDefense(t *testing.T) {
	db := openArmorTestDB(t)
	defer db.Close()

	s := &player.State{}
	player.LoadDefense(db, s)
	if s.Defense != 0 {
		t.Errorf("defense with no armor: got %d want 0", s.Defense)
	}

	player.EquipArmor(db, "leather-armor", "Leather Armor", 4) //nolint:errcheck
	player.LoadDefense(db, s)
	if s.Defense != 4 {
		t.Errorf("defense after equip: got %d want 4", s.Defense)
	}
}
