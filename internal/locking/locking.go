// Package locking implements lock checking, picking, and key-based unlocking.
package locking

import (
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// IsLocked reports whether the lock with the given ID is currently locked.
// A lock is locked by default (no DB record = locked).
func IsLocked(gdb *gamedb.GameDB, lockID string) bool {
	db := gdb.SQLiteDB()
	var unlocked int
	err := db.QueryRow(`SELECT unlocked FROM lock_state WHERE lock_id=?`, lockID).Scan(&unlocked)
	if err == sql.ErrNoRows {
		return true // default: locked
	}
	if err != nil {
		return true
	}
	return unlocked == 0
}

// Unlock sets a lock to the unlocked state unconditionally.
func Unlock(gdb *gamedb.GameDB, lockID string) error {
	db := gdb.SQLiteDB()
	_, err := db.Exec(
		`INSERT INTO lock_state (lock_id, unlocked) VALUES (?,1)
		 ON CONFLICT(lock_id) DO UPDATE SET unlocked=1`,
		lockID,
	)
	return err
}

// PickResult is the outcome of a pick attempt.
type PickResult struct {
	Success bool
	Message string
}

// Pick attempts to pick a lock using a skill roll.
// pickingSkill is the player's lockpicking skill level.
func Pick(gdb *gamedb.GameDB, lockID string, difficulty, pickingSkill int) PickResult {
	// Roll: rand(1,100) + skill - difficulty*10 >= 50
	roll := rand.Intn(100) + 1 + pickingSkill - difficulty*10
	if roll >= 50 {
		if err := Unlock(gdb, lockID); err != nil {
			return PickResult{Message: fmt.Sprintf("pick succeeded but state error: %v", err)}
		}
		return PickResult{Success: true, Message: fmt.Sprintf("you pick lock %q. it clicks open.", lockID)}
	}
	return PickResult{
		Success: false,
		Message: fmt.Sprintf("your pick slips. lock %q holds. (difficulty: %d)", lockID, difficulty),
	}
}

// UnlockWithKey checks if any key item in the player's inventory matches the lock's keys list.
// Returns true if a valid key was found and the lock was unlocked.
func UnlockWithKey(gdb *gamedb.GameDB, lock *world.Lock, inventory []string) (bool, string) {
	for _, keyID := range lock.Keys {
		for _, itemID := range inventory {
			if itemID == keyID {
				if err := Unlock(gdb, lock.ID); err != nil {
					return false, fmt.Sprintf("key found but state error: %v", err)
				}
				return true, fmt.Sprintf("you use %q to unlock the passage.", keyID)
			}
		}
	}
	return false, ""
}
