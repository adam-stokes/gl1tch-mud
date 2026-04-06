package commands

import (
	"fmt"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// handleTop publishes a game.top.request and returns immediately.
// The reply arrives via the server's bus listener and is sent to the player as a chat message.
func handleTop(gdb *gamedb.GameDB, s *player.State, _ *world.World, _ []string) Result {
	requestID := fmt.Sprintf("top-%d", time.Now().UnixNano())
	return Result{
		Output: "fetching leaderboard…",
		Event: &Event{
			Topic: "game.top.request",
			Payload: map[string]any{
				"request_id": requestID,
				"player":     s.PlayerID,
			},
		},
		PendingRequestID: requestID,
		PendingPlayer:    s.PlayerID,
	}
}

// handleAchievements publishes a game.achievements.request.
func handleAchievements(gdb *gamedb.GameDB, s *player.State, _ *world.World, _ []string) Result {
	requestID := fmt.Sprintf("ach-%d", time.Now().UnixNano())
	return Result{
		Output: "fetching achievements…",
		Event: &Event{
			Topic: "game.achievements.request",
			Payload: map[string]any{
				"request_id": requestID,
				"player":     s.PlayerID,
				"source":     "gl1tch-mud",
			},
		},
		PendingRequestID: requestID,
		PendingPlayer:    s.PlayerID,
	}
}
