// Package weather manages per-biome weather state.
package weather

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

// TickInterval is the number of player actions between possible weather changes.
const TickInterval = 50

// Current returns the current weather condition for biome.
// Returns "clear" if no record exists.
func Current(gdb *gamedb.GameDB, biome string) (string, error) {
	cond, err := gdb.GetWeatherCondition(context.Background(), biome)
	if err == sql.ErrNoRows {
		return "clear", nil
	}
	return cond, err
}

// Tick checks whether weather should change for biome (if currentAction >= expires_action)
// and if so rolls a new condition from possible. Returns the current (possibly new) condition.
// When err is non-nil the returned condition string was not persisted and should be discarded.
func Tick(gdb *gamedb.GameDB, biome string, currentAction int, possible []string) (string, error) {
	ctx := context.Background()

	ws, err := gdb.GetWeatherState(ctx, biome)
	if err != nil && err != sql.ErrNoRows {
		return "clear", err
	}

	cond := ws.Condition
	expires := ws.ExpiresAction

	if err == sql.ErrNoRows || currentAction >= expires {
		if len(possible) == 0 {
			possible = []string{"clear"}
		}
		cond = possible[rand.Intn(len(possible))]
		newExpires := currentAction + TickInterval
		err = gdb.UpsertWeatherState(ctx, biome, cond, newExpires)
		if err != nil {
			return cond, err
		}
	}
	return cond, nil
}

// YieldBonus returns the resource yield multiplier for condition.
func YieldBonus(condition string) float64 {
	if condition == "clear" {
		return 1.1
	}
	return 1.0
}

// Description returns a player-facing description of the weather condition.
func Description(condition string) string {
	switch condition {
	case "clear":
		return "The sky is clear. Conditions are ideal."
	case "rainy":
		return "Rain patters down. Soil is rich — gathering may yield extra seeds."
	case "windy":
		return "A strong wind blows through. Nothing unusual."
	case "stormy":
		return "A fierce storm crackles overhead. Lightning might reveal buried loot."
	case "foggy":
		return "A thick fog hangs in the air. Visibility is low."
	case "sandstorm":
		return "Sand whips through the air. Ancient ruins entrances may be uncovered."
	case "scorching":
		return "The heat is brutal. Stay hydrated."
	case "light-snow":
		return "Light snowflakes drift down peacefully."
	case "blizzard":
		return "A blizzard rages. Mining may yield extra gems in these conditions."
	case "damp":
		return "The cave air is cold and damp."
	case "tremor":
		return "The cave walls shudder. A tremor has shifted the rock — new veins may be exposed."
	default:
		return fmt.Sprintf("The weather is %s.", condition)
	}
}
