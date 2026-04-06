package trading

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS player_reputation (faction TEXT PRIMARY KEY, value INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE IF NOT EXISTS inventory (id INTEGER PRIMARY KEY AUTOINCREMENT, item_id TEXT NOT NULL UNIQUE, item_name TEXT NOT NULL, item_desc TEXT NOT NULL DEFAULT '');
	`); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func makeNPC() *world.NPC {
	return &world.NPC{
		ID:   "trader-0",
		Name: "The Trader",
		Trades: []world.TradeOffer{
			{
				ID:    "trade-open",
				Wants: []world.TradeIngredient{{ID: "data-chip", Count: 1}},
				Offers: []world.TradeIngredient{
					{ID: "enc-key", Name: "Encryption Key", Desc: "For secure comms.", Count: 1},
				},
			},
			{
				ID:         "trade-faction",
				FactionReq: "netrunners:5",
				Wants:      []world.TradeIngredient{{ID: "raw-data", Count: 2}},
				Offers: []world.TradeIngredient{
					{ID: "premium-key", Name: "Premium Key", Desc: "Exclusive.", Count: 1},
				},
			},
		},
	}
}

func TestListOffersNoRep(t *testing.T) {
	db := openTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	npc := makeNPC()
	offers := ListOffers(nil, npc, gdb)
	// Should only see the open trade (no faction req)
	if len(offers) != 1 {
		t.Errorf("expected 1 offer without rep, got %d", len(offers))
	}
	if offers[0].ID != "trade-open" {
		t.Errorf("expected trade-open, got %s", offers[0].ID)
	}
}

func TestListOffersWithRep(t *testing.T) {
	db := openTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	db.Exec(`INSERT INTO player_reputation (faction, value) VALUES ('netrunners', 10)`) //nolint:errcheck

	npc := makeNPC()
	offers := ListOffers(nil, npc, gdb)
	if len(offers) != 2 {
		t.Errorf("expected 2 offers with rep=10, got %d", len(offers))
	}
}

func TestExecuteMissingItems(t *testing.T) {
	db := openTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	npc := makeNPC()
	res := Execute(gdb, npc, "trade-open", []string{})
	if res.OK {
		t.Error("expected failure when missing wanted items")
	}
	if res.Message == "" {
		t.Error("expected message listing missing items")
	}
}

func TestExecuteFactionGate(t *testing.T) {
	db := openTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	npc := makeNPC()
	res := Execute(gdb, npc, "trade-faction", []string{"raw-data", "raw-data"})
	if res.OK {
		t.Error("expected failure when faction rep not met")
	}
}

func TestExecuteSuccess(t *testing.T) {
	db := openTestDB(t)
	gdb := gamedb.NewSQLite(db)
	defer db.Close()

	// Add data-chip to inventory
	db.Exec(`INSERT INTO inventory (item_id, item_name, item_desc) VALUES ('data-chip','Data Chip','A chip.')`) //nolint:errcheck

	npc := makeNPC()
	res := Execute(gdb, npc, "trade-open", []string{"data-chip"})
	if !res.OK {
		t.Errorf("expected success, got: %s", res.Message)
	}
	if len(res.GotItems) != 1 || res.GotItems[0].ID != "enc-key" {
		t.Errorf("unexpected got items: %+v", res.GotItems)
	}

	// data-chip should be gone from inventory
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM inventory WHERE item_id='data-chip'`).Scan(&count) //nolint:errcheck
	if count != 0 {
		t.Error("data-chip should have been removed from inventory after trade")
	}

	// enc-key should be in inventory
	db.QueryRow(`SELECT COUNT(*) FROM inventory WHERE item_id='enc-key'`).Scan(&count) //nolint:errcheck
	if count != 1 {
		t.Error("enc-key should be in inventory after trade")
	}
}
