// Package skills manages player skill levels and XP progression.
package skills

import (
	"context"
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
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
func Load(gdb *gamedb.GameDB, skill string) (level, xp int, err error) {
	return gdb.LoadSkill(context.Background(), skill)
}

// Level returns the current skill level. Returns 0 if not found or on error.
func Level(gdb *gamedb.GameDB, skill string) int {
	level, _, _ := Load(gdb, skill)
	return level
}

// XP returns the current XP for the skill. Returns 0 if not found or on error.
func XP(gdb *gamedb.GameDB, skill string) int {
	_, xp, _ := Load(gdb, skill)
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
func Award(gdb *gamedb.GameDB, skill string, xpAmount int) (*AwardResult, error) {
	oldLevel, oldXP, err := Load(gdb, skill)
	if err != nil {
		return nil, fmt.Errorf("skills: load %s: %w", skill, err)
	}

	newXP := oldXP + xpAmount
	newLevel := LevelForXP(newXP)

	if err := gdb.UpsertSkill(context.Background(), skill, newLevel, newXP); err != nil {
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
func All(gdb *gamedb.GameDB) (map[string][2]int, error) {
	return gdb.ListAllSkills(context.Background())
}
