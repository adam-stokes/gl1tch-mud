package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

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
	return database, nil
}

func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "gl1tch-mud", "world.db"), nil
}
