// Package hacking implements the hack command and system intrusion logic.
package hacking

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

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

// HackPhase is a named stage of a multi-phase hack.
type HackPhase string

const (
	PhaseBreach  HackPhase = "breach"
	PhaseExploit HackPhase = "exploit"
	PhaseExfil   HackPhase = "exfil"
)

// PhaseResult is the outcome of one phase of a multi-phase hack.
type PhaseResult struct {
	Phase   HackPhase
	Success bool
	Message string
}

// HackMulti runs a three-phase hack: breach, exploit, exfil.
// exploitBonus is added to the exploit roll (e.g. from exploit fragment items).
// Returns per-phase results and a bounty flag (true if exfil failed after successful exploit).
func HackMulti(db *sql.DB, room *world.Room, systemID string, hackingSkill int, exploitBonus int) ([]PhaseResult, bool, error) {
	sys := room.FindSystem(systemID)
	if sys == nil {
		return nil, false, fmt.Errorf("system %q not found in room %q", systemID, room.ID)
	}

	// Load current alert level.
	var alert int
	_ = db.QueryRow(`SELECT alert FROM system_state WHERE room_id = ? AND system_id = ?`, room.ID, systemID).Scan(&alert)

	var results []PhaseResult
	bounty := false

	// Phase 1: Breach
	breachRoll := rand.Intn(100) + 1 + hackingSkill - sys.SecurityLevel*8
	breachOK := breachRoll >= 50
	br := PhaseResult{Phase: PhaseBreach, Success: breachOK}
	if breachOK {
		br.Message = fmt.Sprintf("Breach successful. Roll: %d.", breachRoll)
	} else {
		alert++
		_, _ = db.Exec(`INSERT INTO system_state (room_id, system_id, intrusion, alert) VALUES (?, ?, 0, ?)
            ON CONFLICT(room_id, system_id) DO UPDATE SET alert = ?`, room.ID, systemID, alert, alert)
		br.Message = fmt.Sprintf("Breach failed. Roll: %d. Alert level: %d.", breachRoll, alert)
	}
	results = append(results, br)
	if !breachOK {
		return results, false, nil
	}

	// Phase 2: Exploit
	exploitRoll := rand.Intn(100) + 1 + hackingSkill + exploitBonus - sys.SecurityLevel*10
	exploitOK := exploitRoll >= 50
	er := PhaseResult{Phase: PhaseExploit, Success: exploitOK}
	if exploitOK {
		er.Message = fmt.Sprintf("Exploit delivered. Roll: %d.", exploitRoll)
	} else {
		alert++
		_, _ = db.Exec(`INSERT INTO system_state (room_id, system_id, intrusion, alert) VALUES (?, ?, 0, ?)
            ON CONFLICT(room_id, system_id) DO UPDATE SET alert = ?`, room.ID, systemID, alert, alert)
		er.Message = fmt.Sprintf("Exploit failed. Roll: %d. Alert level: %d.", exploitRoll, alert)
	}
	results = append(results, er)
	if !exploitOK {
		return results, false, nil
	}

	// Phase 3: Exfil
	exfilRoll := rand.Intn(100) + 1 - alert*15
	exfilOK := exfilRoll >= 20
	xr := PhaseResult{Phase: PhaseExfil, Success: exfilOK}
	if exfilOK {
		xr.Message = fmt.Sprintf("Exfil clean. Roll: %d.", exfilRoll)
	} else {
		bounty = true
		_, _ = db.Exec(`INSERT OR REPLACE INTO bounties (room_id, npc_id, created_at) VALUES (?, ?, ?)`,
			room.ID, "bounty-hunter-"+systemID, time.Now().Unix())
		xr.Message = fmt.Sprintf("Exfil dirty — you left traces. Roll: %d. Expect company.", exfilRoll)
	}
	results = append(results, xr)
	return results, bounty, nil
}

// SetVulnWindow sets a temporary vulnerability window for a system.
// The window expires after currentAction+3 actions have elapsed.
func SetVulnWindow(db *sql.DB, systemID string, bonus int, currentAction int) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO vuln_windows (system_id, bonus, expires_action) VALUES (?, ?, ?)`,
		systemID, bonus, currentAction+3)
	return err
}

// VulnBonus returns the current vulnerability bonus for a system, or 0 if expired/absent.
func VulnBonus(db *sql.DB, systemID string, currentAction int) (int, error) {
	var bonus, expires int
	err := db.QueryRow(`SELECT bonus, expires_action FROM vuln_windows WHERE system_id = ?`, systemID).Scan(&bonus, &expires)
	if err != nil {
		return 0, nil
	}
	if currentAction > expires {
		_, _ = db.Exec(`DELETE FROM vuln_windows WHERE system_id = ?`, systemID)
		return 0, nil
	}
	return bonus, nil
}
