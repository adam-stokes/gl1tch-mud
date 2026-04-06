// Package quests manages player quests backed by the database.
package quests

import (
	"context"
	"fmt"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

// Quest mirrors the quests table.
type Quest struct {
	ID             string
	Title          string
	Description    string
	Status         string
	ObjType        string
	ObjTarget      string
	ObjRoom        string
	ObjCount       int
	ObjProgress    int
	RewardCredits  int
	RewardXPSkill  string
	RewardXPAmount int
	RewardItemID   string
	RewardItemName string
	RewardItemDesc string
	GiverNPCID     string
	AcceptedAt     int64
	NextQuestID    string
}

// fromRecord converts a gamedb.QuestRecord to our Quest type.
func fromRecord(r gamedb.QuestRecord) Quest {
	return Quest{
		ID:             r.ID,
		Title:          r.Title,
		Description:    r.Description,
		Status:         r.Status,
		ObjType:        r.ObjType,
		ObjTarget:      r.ObjTarget,
		ObjRoom:        r.ObjRoom,
		ObjCount:       r.ObjCount,
		ObjProgress:    r.ObjProgress,
		RewardCredits:  r.RewardCredits,
		RewardXPSkill:  r.RewardXPSkill,
		RewardXPAmount: r.RewardXPAmount,
		RewardItemID:   r.RewardItemID,
		RewardItemName: r.RewardItemName,
		RewardItemDesc: r.RewardItemDesc,
		GiverNPCID:     r.GiverNPCID,
		AcceptedAt:     r.AcceptedAt,
		NextQuestID:    r.NextQuestID,
	}
}

// toRecord converts a Quest to a gamedb.QuestRecord.
func toRecord(q Quest) gamedb.QuestRecord {
	return gamedb.QuestRecord{
		ID:             q.ID,
		Title:          q.Title,
		Description:    q.Description,
		Status:         q.Status,
		ObjType:        q.ObjType,
		ObjTarget:      q.ObjTarget,
		ObjRoom:        q.ObjRoom,
		ObjCount:       q.ObjCount,
		ObjProgress:    q.ObjProgress,
		RewardCredits:  q.RewardCredits,
		RewardXPSkill:  q.RewardXPSkill,
		RewardXPAmount: q.RewardXPAmount,
		RewardItemID:   q.RewardItemID,
		RewardItemName: q.RewardItemName,
		RewardItemDesc: q.RewardItemDesc,
		GiverNPCID:     q.GiverNPCID,
		AcceptedAt:     q.AcceptedAt,
		NextQuestID:    q.NextQuestID,
	}
}

// Accept inserts a new quest into the database.
func Accept(gdb *gamedb.GameDB, q Quest) error {
	if q.AcceptedAt == 0 {
		q.AcceptedAt = time.Now().Unix()
	}
	return gdb.AcceptQuest(context.Background(), toRecord(q))
}

// Active returns all quests with status='active'.
func Active(gdb *gamedb.GameDB) ([]Quest, error) {
	records, err := gdb.ListActiveQuests(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]Quest, len(records))
	for i, r := range records {
		out[i] = fromRecord(r)
	}
	return out, nil
}

// Progress increments obj_progress by n for the given quest.
func Progress(gdb *gamedb.GameDB, id string, n int) error {
	return gdb.ProgressQuest(context.Background(), id, n)
}

// Complete sets quest status to 'completed'.
func Complete(gdb *gamedb.GameDB, id string) error {
	return gdb.CompleteQuest(context.Background(), id)
}

// Fail sets quest status to 'failed'.
func Fail(gdb *gamedb.GameDB, id string) error {
	return gdb.FailQuest(context.Background(), id)
}

// Get fetches a single quest by ID.
func Get(gdb *gamedb.GameDB, id string) (*Quest, error) {
	rec, err := gdb.GetQuest(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf("quest %q not found", id)
	}
	q := fromRecord(*rec)
	return &q, nil
}

// CheckKill finds active kill quests matching npcID, increments progress,
// and returns quests that just reached obj_count.
func CheckKill(gdb *gamedb.GameDB, npcID string) ([]Quest, error) {
	return checkProgress(gdb, "kill", npcID)
}

// CheckHack finds active hack quests matching systemID, increments progress,
// and returns quests that just reached obj_count.
func CheckHack(gdb *gamedb.GameDB, systemID string) ([]Quest, error) {
	return checkProgress(gdb, "hack", systemID)
}

// CheckRetrieve finds active retrieve quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckRetrieve(gdb *gamedb.GameDB, itemID string) ([]Quest, error) {
	return checkProgress(gdb, "retrieve", itemID)
}

// CheckCraft finds active craft quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckCraft(gdb *gamedb.GameDB, itemID string) ([]Quest, error) {
	return checkProgress(gdb, "craft", itemID)
}

// CheckGather finds active gather quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckGather(gdb *gamedb.GameDB, itemID string) ([]Quest, error) {
	return checkProgress(gdb, "gather", itemID)
}

// CheckSmelt finds active smelt quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckSmelt(gdb *gamedb.GameDB, itemID string) ([]Quest, error) {
	return checkProgress(gdb, "smelt", itemID)
}

// CheckAssemble finds active assemble quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckAssemble(gdb *gamedb.GameDB, itemID string) ([]Quest, error) {
	return checkProgress(gdb, "assemble", itemID)
}

// CheckMine finds active mine quests matching resourceID, increments progress,
// and returns quests that just reached obj_count.
func CheckMine(gdb *gamedb.GameDB, resourceID string) ([]Quest, error) {
	return checkProgress(gdb, "mine", resourceID)
}

// ActiveIDs returns a set of all active quest IDs.
func ActiveIDs(gdb *gamedb.GameDB) (map[string]bool, error) {
	return gdb.ListActiveQuestIDs(context.Background())
}

// checkProgress is the shared implementation for Check* functions.
func checkProgress(gdb *gamedb.GameDB, objType, target string) ([]Quest, error) {
	records, err := gdb.ListActiveQuestsByTypeTarget(context.Background(), objType, target)
	if err != nil {
		return nil, err
	}

	var matching []Quest
	for _, r := range records {
		matching = append(matching, fromRecord(r))
	}

	var ready []Quest
	for _, q := range matching {
		if err := Progress(gdb, q.ID, 1); err != nil {
			continue
		}
		q.ObjProgress++
		if q.ObjProgress >= q.ObjCount {
			ready = append(ready, q)
		}
	}
	return ready, nil
}
