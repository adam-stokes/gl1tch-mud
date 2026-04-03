package weather_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/adam-stokes/gl1tch-mud/internal/weather"
)

func openMem(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS weather_state (
		biome TEXT PRIMARY KEY,
		condition TEXT NOT NULL DEFAULT 'clear',
		expires_action INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCurrentDefault(t *testing.T) {
	db := openMem(t)
	defer db.Close()

	cond, err := weather.Current(db, "meadow")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if cond != "clear" {
		t.Errorf("expected 'clear', got %q", cond)
	}
}

func TestTickChangesWeather(t *testing.T) {
	db := openMem(t)
	defer db.Close()

	possible := []string{"clear", "rainy", "stormy"}
	// Set expires_action to 0, current action to 100 — should roll new weather.
	cond, err := weather.Tick(db, "meadow", 100, possible)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	found := false
	for _, p := range possible {
		if cond == p {
			found = true
		}
	}
	if !found {
		t.Errorf("condition %q not in possible set %v", cond, possible)
	}
}

func TestYieldBonus(t *testing.T) {
	if weather.YieldBonus("clear") != 1.1 {
		t.Error("clear should give 1.1 bonus")
	}
	if weather.YieldBonus("blizzard") != 1.0 {
		t.Error("blizzard should give 1.0 bonus")
	}
}
