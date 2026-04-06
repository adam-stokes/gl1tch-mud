package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// migrate applies idempotent ALTER TABLE migrations for columns added after
// the original schema was authored. SQLite has no IF NOT EXISTS for ADD COLUMN,
// so each call probes pragma_table_info first.
func migrate(db *sql.DB) {
	addColumnIfMissing := func(table, col, ddl string) {
		var name string
		err := db.QueryRow(
			`SELECT name FROM pragma_table_info(?) WHERE name=?`, table, col,
		).Scan(&name)
		if err == sql.ErrNoRows {
			_, _ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, ddl))
		}
	}
	addColumnIfMissing("player", "class", "class TEXT NOT NULL DEFAULT ''")
	addColumnIfMissing("quests", "giver_faction", "giver_faction TEXT NOT NULL DEFAULT ''")
	addColumnIfMissing("quests", "min_rep", "min_rep INTEGER NOT NULL DEFAULT 0")
	addColumnIfMissing("quests", "reward_rep_faction", "reward_rep_faction TEXT NOT NULL DEFAULT ''")
	addColumnIfMissing("quests", "reward_rep_delta", "reward_rep_delta INTEGER NOT NULL DEFAULT 0")
}

// Open opens (or creates) the player database at ~/.local/share/gl1tch-mud/world.db.
func Open() (*sql.DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("db: schema: %w", err)
	}
	migrate(db)
	return db, nil
}

// OpenForPlayer opens (or creates) a per-player, per-world database at
// ~/.local/share/gl1tch-mud/players/<playerID>/<worldName>.db.
// Each world gets its own save file so inventory, quests, etc. don't bleed.
func OpenForPlayer(playerID, worldName string) (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".local", "share", "gl1tch-mud", "players", playerID, worldName+".db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db: mkdir: %w", err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if _, err := database.Exec(schema); err != nil {
		return nil, fmt.Errorf("db: schema: %w", err)
	}
	migrate(database)
	return database, nil
}

// OpenForWorld opens (or creates) a per-world player database at
// ~/.local/share/gl1tch-mud/worlds/<worldName>/player.db.
// Used for single-player world switching — each world has its own save.
func OpenForWorld(worldName string) (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".local", "share", "gl1tch-mud", "worlds", worldName, "player.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db: mkdir: %w", err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if _, err := database.Exec(schema); err != nil {
		return nil, fmt.Errorf("db: schema: %w", err)
	}
	migrate(database)
	return database, nil
}

func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "gl1tch-mud", "world.db"), nil
}
