package mods

import (
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

func Apply(gdb *gamedb.GameDB, itemInstance, modID string) error {
	db := gdb.SQLiteDB()
	_, err := db.Exec(
		`INSERT OR REPLACE INTO item_mods (item_instance, mod_id, applied_at) VALUES (?, ?, ?)`,
		itemInstance, modID, time.Now().Unix())
	return err
}

func GetMod(gdb *gamedb.GameDB, itemInstance string) (string, bool) {
	db := gdb.SQLiteDB()
	var modID string
	err := db.QueryRow(`SELECT mod_id FROM item_mods WHERE item_instance = ?`, itemInstance).Scan(&modID)
	if err != nil {
		return "", false
	}
	return modID, true
}
