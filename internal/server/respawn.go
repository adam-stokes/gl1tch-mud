package server

import (
	"context"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/pgq"
)

// respawnTicker periodically respawns dead NPCs and depleted resources in shared worlds.
func (gs *GameServer) respawnTicker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if !gs.IsRunning() || gs.pgPool == nil {
			return
		}
		ctx := context.Background()
		q := pgq.New(gs.pgPool)
		for name, w := range gs.worlds {
			if !w.IsShared() {
				continue
			}
			// Respawn NPCs whose respawn_at has passed
			q.RespawnExpiredNPCs(ctx, name) //nolint:errcheck
			// Respawn resources whose respawn_at has passed
			q.RespawnExpiredResources(ctx, name) //nolint:errcheck
		}
	}
}
