// Package hacking implements the hack command and system intrusion logic.
package hacking

import (
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Result is the outcome of a hack attempt.
type Result struct {
	Success    bool
	AlreadyHacked bool
	NoSystem   bool
	AlertLevel int
	RewardItem string // item ID if reward delivered
	RewardText string
	Message    string
}

// isHacked reports whether a system in a room was already successfully hacked this session.
func isHacked(db *sql.DB, roomID, systemID string) bool {
	var hacked int
	err := db.QueryRow(
		`SELECT hacked FROM system_state WHERE room_id=? AND system_id=?`,
		roomID, systemID,
	).Scan(&hacked)
	if err != nil {
		return false
	}
	return hacked == 1
}

// alertLevel returns the current alert level for a system.
func alertLevel(db *sql.DB, roomID, systemID string) int {
	var alert int
	db.QueryRow( //nolint:errcheck
		`SELECT alert FROM system_state WHERE room_id=? AND system_id=?`,
		roomID, systemID,
	).Scan(&alert)
	return alert
}

// markHacked marks a system as successfully hacked.
func markHacked(db *sql.DB, roomID, systemID string) error {
	_, err := db.Exec(
		`INSERT INTO system_state (room_id, system_id, hacked, alert) VALUES (?,?,1,0)
		 ON CONFLICT(room_id, system_id) DO UPDATE SET hacked=1`,
		roomID, systemID,
	)
	return err
}

// incrementAlert increments the alert level and returns the new value.
func incrementAlert(db *sql.DB, roomID, systemID string) (int, error) {
	_, err := db.Exec(
		`INSERT INTO system_state (room_id, system_id, alert, hacked) VALUES (?,?,1,0)
		 ON CONFLICT(room_id, system_id) DO UPDATE SET alert=alert+1`,
		roomID, systemID,
	)
	if err != nil {
		return 0, err
	}
	return alertLevel(db, roomID, systemID), nil
}

// Hack attempts to compromise a system in the given room.
// hackingSkill is the player's current hacking skill level.
func Hack(db *sql.DB, room *world.Room, systemID string, hackingSkill int) Result {
	if room == nil {
		return Result{NoSystem: true, Message: "no hackable systems here."}
	}

	sys := room.FindSystem(systemID)
	if sys == nil {
		return Result{NoSystem: true, Message: fmt.Sprintf("no system %q in this room.", systemID)}
	}

	if isHacked(db, room.ID, systemID) {
		return Result{AlreadyHacked: true, Message: fmt.Sprintf("system %q is already compromised.", systemID)}
	}

	// Skill roll: rand(1,100) + skill - security_level*10 >= 50
	roll := rand.Intn(100) + 1 + hackingSkill - sys.SecurityLevel*10
	if roll >= 50 {
		markHacked(db, room.ID, systemID) //nolint:errcheck
		msg := fmt.Sprintf("access granted. you breached system %q.", systemID)
		if sys.RewardText != "" {
			msg = sys.RewardText
		}
		return Result{
			Success:    true,
			RewardItem: sys.RewardItem,
			RewardText: sys.RewardText,
			Message:    msg,
		}
	}

	// Failure — increment alert
	newAlert, err := incrementAlert(db, room.ID, systemID)
	if err != nil {
		return Result{Message: "hack failed — system error recording alert."}
	}

	msg := fmt.Sprintf("intrusion detected. alert level: %d/3.", newAlert)
	if newAlert >= 3 {
		msg += "\nalarm triggered — security programs mobilizing!"
	}
	return Result{
		Success:    false,
		AlertLevel: newAlert,
		Message:    msg,
	}
}

// AlertLevel returns the current alert level for a system (0 if no record).
func AlertLevel(db *sql.DB, roomID, systemID string) int {
	return alertLevel(db, roomID, systemID)
}
