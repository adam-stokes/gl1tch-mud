// Package factions manages the player's personal faction.
package factions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
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
	return gdb.FactionExists(context.Background())
}

// Create creates the player faction with the given name and agenda.
func Create(gdb *gamedb.GameDB, name, agenda string) (*PlayerFaction, error) {
	factionID := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	now := time.Now().Unix()
	err := gdb.CreateFaction(context.Background(), factionID, name, agenda, now)
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
	rec, err := gdb.GetFaction(context.Background())
	if err != nil {
		return nil, fmt.Errorf("factions: get: %w", err)
	}
	return &PlayerFaction{
		FactionID:     rec.FactionID,
		FactionName:   rec.FactionName,
		Agenda:        rec.Agenda,
		HideoutRoomID: rec.HideoutRoomID,
		Credits:       rec.Credits,
		CreatedAt:     rec.CreatedAt,
	}, nil
}

// SetHideout updates the hideout room for the player's faction.
func SetHideout(gdb *gamedb.GameDB, roomID string) error {
	return gdb.SetFactionHideout(context.Background(), roomID)
}

// Members returns all faction members.
func Members(gdb *gamedb.GameDB) ([]FactionMember, error) {
	records, err := gdb.ListFactionMembers(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]FactionMember, len(records))
	for i, r := range records {
		out[i] = FactionMember{
			NPCID:         r.NPCID,
			NPCName:       r.NPCName,
			NPCDesc:       r.NPCDesc,
			Role:          r.Role,
			StationedRoom: r.StationedRoom,
			Loyalty:       r.Loyalty,
			RecruitedAt:   r.RecruitedAt,
		}
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
	return gdb.InsertFactionMember(context.Background(), npcID, npcName, npcDesc, role, time.Now().Unix())
}

// IsRecruited reports whether an NPC is already in the faction.
func IsRecruited(gdb *gamedb.GameDB, npcID string) (bool, error) {
	return gdb.IsFactionMember(context.Background(), npcID)
}

// MemberCount returns the number of faction members.
func MemberCount(gdb *gamedb.GameDB) (int, error) {
	return gdb.CountFactionMembers(context.Background())
}
