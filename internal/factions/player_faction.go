// Package factions manages the player's personal faction.
package factions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
)

// PlayerFaction mirrors the player_faction table.
type PlayerFaction struct {
	FactionID     string
	FactionName   string
	Agenda        string
	HideoutRoomID string
	Credits       int
	CreatedAt     int64
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
func Exists(gdb *gamedb.GameDB) (bool, error) {
	q := sqliteq.New(gdb.SQLiteDB())
	_, err := q.FactionExists(context.Background())
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Create creates the player faction with the given name and agenda.
func Create(gdb *gamedb.GameDB, name, agenda string) (*PlayerFaction, error) {
	factionID := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	now := time.Now().Unix()
	q := sqliteq.New(gdb.SQLiteDB())
	err := q.CreateFaction(context.Background(), sqliteq.CreateFactionParams{
		FactionID:   factionID,
		FactionName: name,
		Agenda:      sql.NullString{String: agenda, Valid: agenda != ""},
		CreatedAt:   now,
	})
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
func Get(gdb *gamedb.GameDB) (*PlayerFaction, error) {
	q := sqliteq.New(gdb.SQLiteDB())
	row, err := q.GetFaction(context.Background())
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no faction exists")
	}
	if err != nil {
		return nil, fmt.Errorf("factions: get: %w", err)
	}
	return &PlayerFaction{
		FactionID:     row.FactionID,
		FactionName:   row.FactionName,
		Agenda:        row.Agenda.String,
		HideoutRoomID: row.HideoutRoomID.String,
		Credits:       int(row.Credits),
		CreatedAt:     row.CreatedAt,
	}, nil
}

// SetHideout updates the hideout room for the player's faction.
func SetHideout(gdb *gamedb.GameDB, roomID string) error {
	q := sqliteq.New(gdb.SQLiteDB())
	return q.SetFactionHideout(context.Background(), sql.NullString{String: roomID, Valid: roomID != ""})
}

// Members returns all faction members.
func Members(gdb *gamedb.GameDB) ([]FactionMember, error) {
	q := sqliteq.New(gdb.SQLiteDB())
	rows, err := q.ListFactionMembers(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]FactionMember, 0, len(rows))
	for _, r := range rows {
		out = append(out, FactionMember{
			NPCID:         r.NpcID,
			NPCName:       r.NpcName,
			NPCDesc:       r.NpcDesc.String,
			Role:          r.Role,
			StationedRoom: r.StationedRoom.String,
			Loyalty:       int(r.Loyalty),
			RecruitedAt:   r.RecruitedAt,
		})
	}
	return out, nil
}

// Recruit adds an NPC to the player's faction. Returns an error if already recruited.
func Recruit(gdb *gamedb.GameDB, npcID, npcName, npcDesc, role string) error {
	already, err := IsRecruited(gdb, npcID)
	if err != nil {
		return err
	}
	if already {
		return fmt.Errorf("%s is already part of your crew", npcName)
	}
	q := sqliteq.New(gdb.SQLiteDB())
	return q.InsertFactionMember(context.Background(), sqliteq.InsertFactionMemberParams{
		NpcID:       npcID,
		NpcName:     npcName,
		NpcDesc:     sql.NullString{String: npcDesc, Valid: npcDesc != ""},
		Role:        role,
		RecruitedAt: time.Now().Unix(),
	})
}

// IsRecruited reports whether an NPC is already in the faction.
func IsRecruited(gdb *gamedb.GameDB, npcID string) (bool, error) {
	q := sqliteq.New(gdb.SQLiteDB())
	_, err := q.GetFactionMember(context.Background(), npcID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// MemberCount returns the number of faction members.
func MemberCount(gdb *gamedb.GameDB) (int, error) {
	q := sqliteq.New(gdb.SQLiteDB())
	n, err := q.CountFactionMembers(context.Background())
	return int(n), err
}
