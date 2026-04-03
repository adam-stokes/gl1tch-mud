package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// LANServer is an interface satisfied by *server.GameServer.
// Using an interface here breaks the import cycle between commands ↔ server.
type LANServer interface {
	Start(port int, passphrase string) (string, error)
	Stop()
	IsRunning() bool
	LanURL() string
	ConnectedPlayers() []string
}

// lanServer is the package-level server instance, set by main before the game
// loop starts. nil means the /lan command is unavailable.
var lanServer LANServer

// SetLANServer wires the embedded multiplayer server into the /lan command.
// Call this from main after constructing the GameServer.
func SetLANServer(s LANServer) {
	lanServer = s
}

const lanPort = 8080

func init() {
	Registry["lan"] = Lan
}

// Lan handles the /lan [stop|status|<passphrase>] command.
func Lan(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if lanServer == nil {
		return Result{Output: "lan: multiplayer server not available."}
	}

	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}

	switch sub {
	case "stop":
		if !lanServer.IsRunning() {
			return Result{Output: "no LAN session is active."}
		}
		lanServer.Stop()
		return Result{Output: "LAN session stopped."}

	case "status":
		if !lanServer.IsRunning() {
			return Result{Output: "no LAN session is active."}
		}
		players := lanServer.ConnectedPlayers()
		if len(players) == 0 {
			return Result{Output: fmt.Sprintf("LAN session: %s (no players connected)", lanServer.LanURL())}
		}
		return Result{Output: fmt.Sprintf("LAN session: %s\nconnected players: %s",
			lanServer.LanURL(), strings.Join(players, ", "))}

	default:
		// sub is either empty (no passphrase) or a passphrase string
		passphrase := ""
		if len(args) > 0 {
			passphrase = args[0] // preserve original case
		}

		if lanServer.IsRunning() {
			players := lanServer.ConnectedPlayers()
			return Result{Output: fmt.Sprintf("LAN session already active: %s (%d players connected)",
				lanServer.LanURL(), len(players))}
		}

		url, err := lanServer.Start(lanPort, passphrase)
		if err != nil {
			return Result{Output: fmt.Sprintf("lan: failed to start server: %v", err)}
		}

		out := fmt.Sprintf("LAN session started: %s\nShare this URL with your players.", url)
		if passphrase != "" {
			out += fmt.Sprintf("\nPassphrase: %s", passphrase)
		}
		return Result{Output: out}
	}
}
