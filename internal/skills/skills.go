// Package skills manages player skill levels and XP progression.
package skills

import (
	"database/sql"
	"fmt"
)

// xpThresholds maps level → minimum XP needed to reach that level.
// Level 0 is default. Level N requires thresholds[N-1] XP.
var xpThresholds = []int{50, 150, 300, 600, 1000}

// LevelForXP returns the skill level for a given XP total.
func LevelForXP(xp int) int {
	level := 0
	for _, threshold := range xpThresholds {
		if xp >= threshold {
			level++
		} else {
			break
		}
	}
	return level
}

// Load returns the level and XP for the given skill. Returns 0,0 if not found.
func Load(db *sql.DB, skill string) (level, xp int, err error) {
	err = db.QueryRow(`SELECT level, xp FROM player_skills WHERE skill = ?`, skill).
		Scan(&level, &xp)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return level, xp, err
}

// Level returns the current skill level. Returns 0 if not found or on error.
func Level(db *sql.DB, skill string) int {
	level, _, _ := Load(db, skill)
	return level
}

// XP returns the current XP for the skill. Returns 0 if not found or on error.
func XP(db *sql.DB, skill string) int {
	_, xp, _ := Load(db, skill)
	return xp
}

// AwardResult describes the outcome of an XP award.
type AwardResult struct {
	Skill      string
	OldLevel   int
	NewLevel   int
	XP         int
	LeveledUp  bool
	LevelUpMsg string
}

// Award adds xpAmount XP to the skill and returns the result (including any level-up).
func Award(db *sql.DB, skill string, xpAmount int) (*AwardResult, error) {
	oldLevel, oldXP, err := Load(db, skill)
	if err != nil {
		return nil, fmt.Errorf("skills: load %s: %w", skill, err)
	}

	newXP := oldXP + xpAmount
	newLevel := LevelForXP(newXP)

	_, err = db.Exec(
		`INSERT INTO player_skills (skill, level, xp) VALUES (?, ?, ?)
		 ON CONFLICT(skill) DO UPDATE SET level=excluded.level, xp=excluded.xp`,
		skill, newLevel, newXP,
	)
	if err != nil {
		return nil, fmt.Errorf("skills: upsert %s: %w", skill, err)
	}

	res := &AwardResult{
		Skill:    skill,
		OldLevel: oldLevel,
		NewLevel: newLevel,
		XP:       newXP,
	}
	if newLevel > oldLevel {
		res.LeveledUp = true
		res.LevelUpMsg = fmt.Sprintf(">>> skill up: %s is now level %d <<<", skill, newLevel)
	}
	return res, nil
}

// All returns a map of all skills with their level and XP.
func All(db *sql.DB) (map[string][2]int, error) {
	rows, err := db.Query(`SELECT skill, level, xp FROM player_skills`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][2]int)
	for rows.Next() {
		var skill string
		var level, xp int
		if err := rows.Scan(&skill, &level, &xp); err != nil {
			return nil, err
		}
		result[skill] = [2]int{level, xp}
	}
	return result, rows.Err()
}
