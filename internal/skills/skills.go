// Package skills manages player skill levels and XP progression.
package skills

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
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
	q := sqliteq.New(db)
	row, err := q.LoadSkill(context.Background(), skill)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return int(row.Level), int(row.Xp), nil
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

	q := sqliteq.New(db)
	err = q.UpsertSkill(context.Background(), sqliteq.UpsertSkillParams{
		Skill: skill,
		Level: int64(newLevel),
		Xp:    int64(newXP),
	})
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
	q := sqliteq.New(db)
	rows, err := q.ListAllSkills(context.Background())
	if err != nil {
		return nil, err
	}

	result := make(map[string][2]int)
	for _, row := range rows {
		result[row.Skill] = [2]int{int(row.Level), int(row.Xp)}
	}
	return result, nil
}
