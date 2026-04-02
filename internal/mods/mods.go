package mods

import (
	"database/sql"
	"time"
)

func Apply(db *sql.DB, itemInstance, modID string) error {
	_, err := db.Exec(
		`INSERT OR REPLACE INTO item_mods (item_instance, mod_id, applied_at) VALUES (?, ?, ?)`,
		itemInstance, modID, time.Now().Unix())
	return err
}

func GetMod(db *sql.DB, itemInstance string) (string, bool) {
	var modID string
	err := db.QueryRow(`SELECT mod_id FROM item_mods WHERE item_instance = ?`, itemInstance).Scan(&modID)
	if err != nil {
		return "", false
	}
	return modID, true
}
