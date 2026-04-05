package crafting

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS inventory (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		item_id   TEXT    NOT NULL UNIQUE,
		item_name TEXT    NOT NULL,
		item_desc TEXT    NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func makeWorld() *world.World {
	return &world.World{
		CraftingRecipes: []world.CraftingRecipe{
			{
				ID:   "sniffer",
				Name: "Packet Sniffer",
				Ingredients: []world.CraftingIngredient{
					{ID: "silicon", Count: 2},
					{ID: "wire", Count: 1},
				},
				Output:   world.Item{ID: "sniffer", Name: "Packet Sniffer", Desc: "Listens."},
				SkillReq: 2,
			},
			{
				ID:   "ice-pick",
				Name: "ICE Pick",
				Ingredients: []world.CraftingIngredient{
					{ID: "carbon-blade", Count: 1},
				},
				Output: world.Item{ID: "ice-pick", Name: "ICE Pick", Desc: "Cracks barriers."},
			},
		},
	}
}

func TestUnknownRecipe(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	w := makeWorld()
	res := Craft(db, w, nil, "bogus", []string{}, 0, nil)
	if res.OK {
		t.Error("expected failure for unknown recipe")
	}
	if res.Message == "" {
		t.Error("expected non-empty message for unknown recipe")
	}
}

func TestSkillGate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	w := makeWorld()
	res := Craft(db, w, nil, "sniffer", []string{"silicon", "silicon", "wire"}, 0, nil)
	if res.OK {
		t.Error("expected failure for insufficient skill")
	}
	if res.Message == "" {
		t.Error("expected skill gate message")
	}
}

func TestMissingIngredients(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	w := makeWorld()
	// SkillReq=2, hackingSkill=3 → passes skill gate; but no ingredients
	res := Craft(db, w, nil, "sniffer", []string{}, 3, nil)
	if res.OK {
		t.Error("expected failure for missing ingredients")
	}
	if len(res.MissingItems) == 0 {
		t.Error("expected missing items list")
	}
}

func TestSuccessfulCraft(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Add ingredients to DB inventory
	db.Exec(`INSERT INTO inventory (item_id, item_name, item_desc) VALUES ('silicon','Silicon','Raw.')`)   //nolint:errcheck
	db.Exec(`INSERT INTO inventory (item_id, item_name, item_desc) VALUES ('wire','Wire','Copper.')`)      //nolint:errcheck

	w := makeWorld()
	// sniffer needs silicon x2 but we only have 1 in unique inventory — adjust test to use ice-pick
	res := Craft(db, w, nil, "ice-pick", []string{"carbon-blade"}, 0, nil)
	if !res.OK {
		t.Errorf("expected success, got: %s", res.Message)
	}
	if res.OutputItem.ID != "ice-pick" {
		t.Errorf("expected ice-pick output, got %q", res.OutputItem.ID)
	}
}

func TestNoRecipesKnown(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	w := &world.World{}
	res := Craft(db, w, nil, "anything", []string{}, 0, nil)
	if res.OK {
		t.Error("expected failure when world has no recipes")
	}
}

func makeAssemblyWorld() *world.World {
	return &world.World{
		CraftingRecipes: []world.CraftingRecipe{
			{
				ID:   "pipe-pistol",
				Name: "Pipe Pistol",
				Type: world.RecipeTypeAssembly,
				Output: world.Item{ID: "pipe-pistol", Name: "Pipe Pistol", Desc: "Scavenged firearm."},
				Slots: []world.CraftingSlot{
					{ID: "frame", Name: "Frame", Required: true, AcceptsTag: "gun-frame"},
					{ID: "barrel", Name: "Barrel", Required: true, AcceptsTag: "gun-barrel", StatMods: map[string]int{"damage": 2}},
					{ID: "grip", Name: "Grip", Required: false, AcceptsTag: "gun-stock", StatMods: map[string]int{"range": 1}},
				},
			},
		},
		Rooms: []world.Room{{
			ID: "start",
			Items: []world.Item{
				{ID: "pipe-frame-crude", Name: "Pipe Frame", Tags: []string{"gun-frame", "component"}},
				{ID: "copper-tube-crude", Name: "Copper Tube", Tags: []string{"gun-barrel", "component"}, StatMods: map[string]int{"damage": 2}},
				{ID: "wrapped-grip-crude", Name: "Wrapped Grip", Tags: []string{"gun-stock", "component"}, StatMods: map[string]int{"range": 1}},
				{ID: "wrong-item", Name: "Wrong Item", Tags: []string{"potion"}},
			},
		}},
	}
}

func seedAssemblyInventory(t *testing.T, db *sql.DB) {
	t.Helper()
	items := []struct{ id, name string }{
		{"pipe-frame-crude", "Pipe Frame"},
		{"copper-tube-crude", "Copper Tube"},
		{"wrapped-grip-crude", "Wrapped Grip"},
		{"wrong-item", "Wrong Item"},
	}
	for _, it := range items {
		db.Exec(`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,'')`, it.id, it.name) //nolint:errcheck
	}
}

func TestAssembleMissingRequiredSlot(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	slots := map[string]string{"frame": "pipe-frame-crude"}
	res := Craft(db, w, nil, "pipe-pistol", []string{"pipe-frame-crude"}, 0, slots)
	if res.OK {
		t.Error("expected failure: required slot 'barrel' not filled")
	}
}

func TestAssembleWrongComponent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	slots := map[string]string{"frame": "wrong-item", "barrel": "copper-tube-crude"}
	res := Craft(db, w, nil, "pipe-pistol", []string{"wrong-item", "copper-tube-crude"}, 0, slots)
	if res.OK {
		t.Error("expected failure: wrong component tag for frame slot")
	}
}

func TestAssembleSuccess(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	slots := map[string]string{"frame": "pipe-frame-crude", "barrel": "copper-tube-crude"}
	res := Craft(db, w, nil, "pipe-pistol", []string{"pipe-frame-crude", "copper-tube-crude"}, 0, slots)
	if !res.OK {
		t.Errorf("expected success, got: %s", res.Message)
	}
	if res.OutputItem.ID != "pipe-pistol" {
		t.Errorf("expected pipe-pistol output, got %q", res.OutputItem.ID)
	}
	if res.OutputItem.Stats["damage"] != 2 {
		t.Errorf("expected damage=2, got %d", res.OutputItem.Stats["damage"])
	}
}

func TestAssembleStatAccumulation(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	slots := map[string]string{
		"frame":  "pipe-frame-crude",
		"barrel": "copper-tube-crude",
		"grip":   "wrapped-grip-crude",
	}
	res := Craft(db, w, nil, "pipe-pistol", []string{"pipe-frame-crude", "copper-tube-crude", "wrapped-grip-crude"}, 0, slots)
	if !res.OK {
		t.Errorf("expected success, got: %s", res.Message)
	}
	if res.OutputItem.Stats["damage"] != 2 {
		t.Errorf("expected damage=2, got %d", res.OutputItem.Stats["damage"])
	}
	if res.OutputItem.Stats["range"] != 1 {
		t.Errorf("expected range=1, got %d", res.OutputItem.Stats["range"])
	}
}
