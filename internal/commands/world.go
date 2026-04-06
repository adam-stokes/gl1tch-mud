package commands

import (
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["world"] = World
}

// World handles "world list" and "world switch <name>".
func World(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "usage: world list | world switch <name>"}
	}

	switch strings.ToLower(args[0]) {
	case "list":
		names := world.Available()
		var b strings.Builder
		b.WriteString("available worlds:\n")
		for _, n := range names {
			marker := "  "
			if n == s.World {
				marker = "* "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", marker, n))
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}

	case "switch":
		if len(args) < 2 {
			return Result{Output: "usage: world switch <name>"}
		}
		target := strings.ToLower(args[1])
		if target == s.World {
			return Result{Output: fmt.Sprintf("you are already in %s.", target)}
		}
		// Validate world exists before returning SwitchWorld.
		_, err := world.Load(target)
		if err != nil {
			return Result{Output: fmt.Sprintf("world %q not found.", target)}
		}
		return Result{
			Output:      fmt.Sprintf("leaving %s... entering %s.", s.World, target),
			SwitchWorld: target,
		}

	default:
		return Result{Output: "usage: world list | world switch <name>"}
	}
}
