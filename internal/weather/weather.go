// Package weather manages per-biome weather state.
package weather

import (
	"database/sql"
	"fmt"
	"math/rand"
)

// TickInterval is the number of player actions between possible weather changes.
const TickInterval = 50

// Current returns the current weather condition for biome.
// Returns "clear" if no record exists.
func Current(db *sql.DB, biome string) (string, error) {
	var cond string
	err := db.QueryRow(`SELECT condition FROM weather_state WHERE biome=?`, biome).Scan(&cond)
	if err == sql.ErrNoRows {
		return "clear", nil
	}
	if err != nil {
		return "clear", err
	}
	return cond, nil
}

// Tick checks whether weather should change for biome (if currentAction >= expires_action)
// and if so rolls a new condition from possible. Returns the current (possibly new) condition.
// When err is non-nil the returned condition string was not persisted and should be discarded.
func Tick(db *sql.DB, biome string, currentAction int, possible []string) (string, error) {
	var expires int
	var cond string
	err := db.QueryRow(`SELECT condition, expires_action FROM weather_state WHERE biome=?`, biome).
		Scan(&cond, &expires)
	if err != nil && err != sql.ErrNoRows {
		return "clear", err
	}
	if err == sql.ErrNoRows || currentAction >= expires {
		if len(possible) == 0 {
			possible = []string{"clear"}
		}
		cond = possible[rand.Intn(len(possible))]
		newExpires := currentAction + TickInterval
		_, err = db.Exec(
			`INSERT INTO weather_state (biome, condition, expires_action) VALUES (?,?,?)
			 ON CONFLICT(biome) DO UPDATE SET condition=excluded.condition, expires_action=excluded.expires_action`,
			biome, cond, newExpires,
		)
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
