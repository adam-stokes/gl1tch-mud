package commands_test

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openEquipTestDB(t *testing.T) *sql.DB {
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

func makeEquipWorld() *world.World {
	w := &world.World{}
	w.CraftingRecipes = []world.CraftingRecipe{
		{
			ID:   "leather-armor",
			Name: "Leather Armor",
			Output: world.Item{
				ID:    "leather-armor",
				Name:  "Leather Armor",
				Desc:  "Light armor.",
				Tags:  []string{"armor", "wearable"},
				Stats: map[string]int{"damage_resist": 2},
			},
		},
	}
	return w
}

func TestWearCommand(t *testing.T) {
	db := openEquipTestDB(t)
	defer db.Close()

	s := &player.State{RoomID: "dusthaven-0"}
	w := makeEquipWorld()
	player.AddItem(db, "leather-armor", "Leather Armor", "Light armor.") //nolint:errcheck

	res := commands.Wear(db, s, w, []string{"leather-armor"})
	if res.Output == "" {
		t.Error("Wear: expected non-empty output")
	}
	if s.Defense != 2 {
		t.Errorf("Wear: state.Defense: got %d want 2", s.Defense)
	}

	rec, err := player.GetEquippedArmor(db)
	if err != nil {
		t.Fatalf("GetEquippedArmor: %v", err)
	}
	if rec == nil || rec.ItemID != "leather-armor" {
		t.Errorf("GetEquippedArmor: expected leather-armor, got %+v", rec)
	}
}

func TestWearRequiresArmorTag(t *testing.T) {
	db := openEquipTestDB(t)
	defer db.Close()

	s := &player.State{}
	w := &world.World{}
	w.CraftingRecipes = []world.CraftingRecipe{
		{
			ID:   "bread",
			Name: "Bread",
			Output: world.Item{
				ID:   "bread",
				Name: "Bread",
				Tags: []string{"food"},
			},
		},
	}
	player.AddItem(db, "bread", "Bread", "Food.") //nolint:errcheck

	res := commands.Wear(db, s, w, []string{"bread"})
	if s.Defense != 0 {
		t.Errorf("Wear non-armor: state.Defense should be 0, got %d", s.Defense)
	}
	if res.Output == "" {
		t.Error("Wear non-armor: expected error output")
	}
}

func TestWearNotInInventory(t *testing.T) {
	db := openEquipTestDB(t)
	defer db.Close()

	s := &player.State{}
	w := makeEquipWorld()

	res := commands.Wear(db, s, w, []string{"leather-armor"})
	if s.Defense != 0 {
		t.Errorf("Wear missing item: Defense should be 0, got %d", s.Defense)
	}
	if res.Output == "" {
		t.Error("Wear missing: expected error output")
	}
}

func TestUnwearCommand(t *testing.T) {
	db := openEquipTestDB(t)
	defer db.Close()

	s := &player.State{Defense: 2}
	w := makeEquipWorld()
	player.AddItem(db, "leather-armor", "Leather Armor", "Light armor.") //nolint:errcheck
	player.EquipArmor(db, "leather-armor", "Leather Armor", 2)           //nolint:errcheck

	res := commands.Unwear(db, s, w, nil)
	if res.Output == "" {
		t.Error("Unwear: expected non-empty output")
	}
	if s.Defense != 0 {
		t.Errorf("Unwear: state.Defense: got %d want 0", s.Defense)
	}

	rec, _ := player.GetEquippedArmor(db)
	if rec != nil {
		t.Errorf("GetEquippedArmor after unwear: expected nil, got %+v", rec)
	}

	// Item should be back in inventory
	items, _ := player.Inventory(db)
	found := false
	for _, it := range items {
		if it.ID == "leather-armor" {
			found = true
		}
	}
	if !found {
		t.Error("Unwear: leather-armor should be back in inventory")
	}
}

func TestEquipmentCommand(t *testing.T) {
	db := openEquipTestDB(t)
	defer db.Close()

	s := &player.State{}
	w := makeEquipWorld()

	// Nothing equipped
	res := commands.Equipment(db, s, w, nil)
	if res.Output == "" {
		t.Error("Equipment (empty): expected output")
	}

	// Equip and check
	player.EquipArmor(db, "leather-armor", "Leather Armor", 3) //nolint:errcheck
	res = commands.Equipment(db, s, w, nil)
	if res.Output == "" {
		t.Error("Equipment (with armor): expected output")
	}
}

func TestDefenseFormula(t *testing.T) {
	cases := []struct {
		attack  int
		defense int
		want    int
	}{
		{10, 3, 7},
		{10, 0, 10},
		{10, 100, 1}, // min 1
		{5, 5, 1},    // min 1
		{5, 4, 1},    // min 1
	}
	for _, tc := range cases {
		dmg := tc.attack - tc.defense
		if dmg < 1 {
			dmg = 1
		}
		if dmg != tc.want {
			t.Errorf("attack=%d defense=%d: got dmg=%d want %d", tc.attack, tc.defense, dmg, tc.want)
		}
	}
}
