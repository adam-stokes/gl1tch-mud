package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestSchemaMigration opens an in-memory DB, applies the schema, and verifies
// all expected tables exist.
func TestSchemaMigration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	tables := []string{
		"player",
		"inventory",
		"npc_state",
		"visited",
		"player_skills",
		"player_reputation",
		"system_state",
		"lock_state",
		"npc_memory",
		"player_stealth",
		"generated_content",
	}

	for _, table := range tables {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}
}

// TestSchemaIdempotent ensures applying schema twice does not fail.
func TestSchemaIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer db.Close()

	for i := 0; i < 2; i++ {
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("apply schema (pass %d): %v", i+1, err)
		}
	}
}
