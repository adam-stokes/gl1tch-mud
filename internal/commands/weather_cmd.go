package commands

import (
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/weather"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["weather"] = Weather
}

// Weather shows current weather in the player's biome.
func Weather(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	biome := "meadow"
	if room != nil && room.Biome != "" {
		biome = room.Biome
	}

	var possible []string
	for _, wt := range w.WeatherTable {
		if wt.Biome == biome {
			possible = wt.Possible
			break
		}
	}
	if len(possible) == 0 {
		possible = []string{"clear"}
	}

	current := actionCount(gdb)
	cond, err := weather.Tick(gdb, biome, current, possible)
	if err != nil {
		return Result{Output: "unable to check weather."}
	}

	desc := weather.Description(cond)
	bonus := weather.YieldBonus(cond)
	bonusStr := ""
	if bonus > 1.0 {
		bonusStr = fmt.Sprintf(" (+%.0f%% yield)", (bonus-1.0)*100)
	}

	return Result{Output: fmt.Sprintf("[%s — %s]\n%s%s", biome, cond, desc, bonusStr)}
}
