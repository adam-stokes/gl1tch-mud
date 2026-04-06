package augments

import (
	"fmt"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

const maxAugments = 3

func Install(gdb *gamedb.GameDB, skill string, bonus int) error {
	db := gdb.SQLiteDB()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM player_augments`).Scan(&count); err != nil {
		return err
	}
	if count >= maxAugments {
		return fmt.Errorf("max augments reached")
	}
	_, err := db.Exec(`INSERT INTO player_augments (skill, bonus, installed_at) VALUES (?, ?, ?)`,
		skill, bonus, time.Now().Unix())
	return err
}

func TotalBonus(gdb *gamedb.GameDB, skill string) int {
	db := gdb.SQLiteDB()
	var total int
	_ = db.QueryRow(`SELECT COALESCE(SUM(bonus), 0) FROM player_augments WHERE skill = ?`, skill).Scan(&total)
	return total
}

func List(gdb *gamedb.GameDB) ([]struct {
	Skill string
	Bonus int
}, error) {
	db := gdb.SQLiteDB()
	rows, err := db.Query(`SELECT skill, bonus FROM player_augments ORDER BY installed_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		Skill string
		Bonus int
	}
	for rows.Next() {
		var a struct {
			Skill string
			Bonus int
		}
		if err := rows.Scan(&a.Skill, &a.Bonus); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
