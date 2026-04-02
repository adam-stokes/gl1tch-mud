package augments

import (
	"database/sql"
	"fmt"
	"time"
)

const maxAugments = 3

func Install(db *sql.DB, skill string, bonus int) error {
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

func TotalBonus(db *sql.DB, skill string) int {
	var total int
	_ = db.QueryRow(`SELECT COALESCE(SUM(bonus), 0) FROM player_augments WHERE skill = ?`, skill).Scan(&total)
	return total
}

func List(db *sql.DB) ([]struct {
	Skill string
	Bonus int
}, error) {
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
