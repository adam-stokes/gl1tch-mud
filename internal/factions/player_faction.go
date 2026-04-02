// Package factions manages the player's personal faction.
package factions

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PlayerFaction mirrors the player_faction table.
type PlayerFaction struct {
	FactionID    string
	FactionName  string
	Agenda       string
	HideoutRoomID string
	Credits      int
	CreatedAt    int64
}

// FactionMember mirrors the faction_members table.
type FactionMember struct {
	NPCID         string
	NPCName       string
	NPCDesc       string
	Role          string
	StationedRoom string
	Loyalty       int
	RecruitedAt   int64
}

// Exists reports whether the player already has a faction.
func Exists(db *sql.DB) (bool, error) {
	var id int
	err := db.QueryRow(`SELECT id FROM player_faction WHERE id=1`).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Create creates the player faction with the given name and agenda.
func Create(db *sql.DB, name, agenda string) (*PlayerFaction, error) {
	factionID := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	now := time.Now().Unix()
	_, err := db.Exec(
		`INSERT INTO player_faction (id, faction_id, faction_name, agenda, hideout_room_id, credits, created_at)
		 VALUES (1, ?, ?, ?, '', 0, ?)`,
		factionID, name, agenda, now,
	)
	if err != nil {
		return nil, fmt.Errorf("factions: create: %w", err)
	}
	return &PlayerFaction{
		FactionID:   factionID,
		FactionName: name,
		Agenda:      agenda,
		CreatedAt:   now,
	}, nil
}

// Get returns the player's faction.
func Get(db *sql.DB) (*PlayerFaction, error) {
	var f PlayerFaction
	err := db.QueryRow(
		`SELECT faction_id, faction_name, agenda, hideout_room_id, credits, created_at
		 FROM player_faction WHERE id=1`,
	).Scan(&f.FactionID, &f.FactionName, &f.Agenda, &f.HideoutRoomID, &f.Credits, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no faction exists")
	}
	if err != nil {
		return nil, fmt.Errorf("factions: get: %w", err)
	}
	return &f, nil
}

// SetHideout updates the hideout room for the player's faction.
func SetHideout(db *sql.DB, roomID string) error {
	_, err := db.Exec(`UPDATE player_faction SET hideout_room_id=? WHERE id=1`, roomID)
	return err
}

// Members returns all faction members.
func Members(db *sql.DB) ([]FactionMember, error) {
	rows, err := db.Query(
		`SELECT npc_id, npc_name, npc_desc, role, stationed_room, loyalty, recruited_at
		 FROM faction_members`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []FactionMember
	for rows.Next() {
		var m FactionMember
		if err := rows.Scan(&m.NPCID, &m.NPCName, &m.NPCDesc, &m.Role, &m.StationedRoom, &m.Loyalty, &m.RecruitedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// Recruit adds an NPC to the player's faction. Returns an error if already recruited.
func Recruit(db *sql.DB, npcID, npcName, npcDesc, role string) error {
	already, err := IsRecruited(db, npcID)
	if err != nil {
		return err
	}
	if already {
		return fmt.Errorf("%s is already part of your crew", npcName)
	}
	_, err = db.Exec(
		`INSERT INTO faction_members (npc_id, npc_name, npc_desc, role, stationed_room, loyalty, recruited_at)
		 VALUES (?, ?, ?, ?, '', 50, ?)`,
		npcID, npcName, npcDesc, role, time.Now().Unix(),
	)
	return err
}

// IsRecruited reports whether an NPC is already in the faction.
func IsRecruited(db *sql.DB, npcID string) (bool, error) {
	var id string
	err := db.QueryRow(`SELECT npc_id FROM faction_members WHERE npc_id=?`, npcID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// MemberCount returns the number of faction members.
func MemberCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM faction_members`).Scan(&n)
	return n, err
}
