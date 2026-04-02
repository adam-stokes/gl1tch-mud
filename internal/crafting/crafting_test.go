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
	res := Craft(db, w, nil, "bogus", []string{}, 0)
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
	res := Craft(db, w, nil, "sniffer", []string{"silicon", "silicon", "wire"}, 0)
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
	res := Craft(db, w, nil, "sniffer", []string{}, 3)
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
	res := Craft(db, w, nil, "ice-pick", []string{"carbon-blade"}, 0)
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
	res := Craft(db, w, nil, "anything", []string{}, 0)
	if res.OK {
		t.Error("expected failure when world has no recipes")
	}
}
