package commands

import (
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["kick"] = Kick
	Registry["ban"] = Ban
	Registry["unban"] = Unban
	Registry["announce"] = Announce
	Registry["who"] = Who
	Registry["teleport"] = Teleport
	Registry["give"] = Give
}

func adminGate(s *player.State) *Result {
	if s.Role != "admin" {
		return &Result{Output: "unknown command."}
	}
	return nil
}

// Kick disconnects a target player.
func Kick(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if r := adminGate(s); r != nil {
		return *r
	}
	if len(args) == 0 {
		return Result{Output: "kick <player>"}
	}
	return Result{
		Output:      fmt.Sprintf("kicking %s...", args[0]),
		AdminAction: &AdminAction{Type: "kick", Target: args[0]},
	}
}

// Ban bans and disconnects a target player.
func Ban(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if r := adminGate(s); r != nil {
		return *r
	}
	if len(args) == 0 {
		return Result{Output: "ban <player>"}
	}
	return Result{
		Output:      fmt.Sprintf("banning %s...", args[0]),
		AdminAction: &AdminAction{Type: "ban", Target: args[0]},
	}
}

// Unban lifts a ban on a target player.
func Unban(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if r := adminGate(s); r != nil {
		return *r
	}
	if len(args) == 0 {
		return Result{Output: "unban <player>"}
	}
	return Result{
		Output:      fmt.Sprintf("unbanning %s...", args[0]),
		AdminAction: &AdminAction{Type: "unban", Target: args[0]},
	}
}

// Announce sends a server-wide announcement.
func Announce(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if r := adminGate(s); r != nil {
		return *r
	}
	if len(args) == 0 {
		return Result{Output: "announce <message>"}
	}
	msg := strings.Join(args, " ")
	return Result{
		Output:      "announcement sent.",
		AdminAction: &AdminAction{Type: "announce", Data: msg},
	}
}

// Who lists all connected players. Available to all players.
func Who(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	// The actual player list is populated by the session handler using registry data.
	// This handler returns a placeholder; session overrides the output.
	return Result{Output: "checking who's online..."}
}

// Teleport moves a target player to a specified room.
func Teleport(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if r := adminGate(s); r != nil {
		return *r
	}
	if len(args) < 2 {
		return Result{Output: "teleport <player> <room-id>"}
	}
	return Result{
		Output:      fmt.Sprintf("teleporting %s to %s...", args[0], args[1]),
		AdminAction: &AdminAction{Type: "teleport", Target: args[0], Data: args[1]},
	}
}

// Give adds an item to a target player's inventory.
func Give(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if r := adminGate(s); r != nil {
		return *r
	}
	if len(args) < 2 {
		return Result{Output: "give <player> <item-id>"}
	}
	return Result{
		Output:      fmt.Sprintf("giving %s to %s...", args[1], args[0]),
		AdminAction: &AdminAction{Type: "give", Target: args[0], Data: args[1]},
	}
}
