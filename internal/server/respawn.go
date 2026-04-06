package server

import (
	"context"
	"log"
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
			// Respawn NPCs whose respawn_at has passed.
			if err := q.RespawnExpiredNPCs(ctx, name); err != nil {
				log.Printf("respawn: NPCs in %s: %v", name, err)
			}
			// Respawn resources whose respawn_at has passed.
			if err := q.RespawnExpiredResources(ctx, name); err != nil {
				log.Printf("respawn: resources in %s: %v", name, err)
			}
		}

		// Clean up expired auth sessions.
		if err := q.DeleteExpiredSessions(ctx); err != nil {
			log.Printf("respawn: delete expired sessions: %v", err)
		}
	}
}
