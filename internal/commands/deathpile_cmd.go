package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["deathpile"] = Deathpile
}

// Deathpile shows the location and contents of the player's last death pile.
func Deathpile(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	roomID, count, err := player.AnyDeathPile(db)
	if err != nil || roomID == "" || count == 0 {
		return Result{Output: "you have no death pile."}
	}

	roomName := roomID
	if r := w.Room(roomID); r != nil {
		roomName = r.Name
	}

	pile, err := player.GetDeathPile(db, roomID)
	if err != nil || len(pile) == 0 {
		return Result{Output: "you have no death pile."}
	}

	names := make([]string, len(pile))
	for i, it := range pile {
		names[i] = it.Name
	}

	return Result{Output: fmt.Sprintf(
		"your death pile is at: %s (%s)\nitems: %s\ntravel there and use 'take death-pile' to recover them.",
		roomName, roomID, strings.Join(names, ", "),
	)}
}
