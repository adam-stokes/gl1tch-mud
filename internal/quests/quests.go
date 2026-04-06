// Package quests manages player quests backed by SQLite.
package quests

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
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

// fromSqlcQuest converts a sqliteq.Quest model to our Quest type.
func fromSqlcQuest(q sqliteq.Quest) Quest {
	return Quest{
		ID:             q.ID,
		Title:          q.Title,
		Description:    q.Description.String,
		Status:         q.Status,
		ObjType:        q.ObjType,
		ObjTarget:      q.ObjTarget,
		ObjRoom:        q.ObjRoom.String,
		ObjCount:       int(q.ObjCount),
		ObjProgress:    int(q.ObjProgress),
		RewardCredits:  int(q.RewardCredits),
		RewardXPSkill:  q.RewardXpSkill.String,
		RewardXPAmount: int(q.RewardXpAmount),
		RewardItemID:   q.RewardItemID.String,
		RewardItemName: q.RewardItemName.String,
		RewardItemDesc: q.RewardItemDesc.String,
		GiverNPCID:     q.GiverNpcID.String,
		AcceptedAt:     q.AcceptedAt,
		NextQuestID:    q.NextQuestID,
	}
}

// Accept inserts a new quest into the database.
func Accept(gdb *gamedb.GameDB, q Quest) error {
	if q.AcceptedAt == 0 {
		q.AcceptedAt = time.Now().Unix()
	}
	queries := sqliteq.New(gdb.SQLiteDB())
	return queries.AcceptQuest(context.Background(), sqliteq.AcceptQuestParams{
		ID:             q.ID,
		Title:          q.Title,
		Description:    sql.NullString{String: q.Description, Valid: q.Description != ""},
		Status:         "active",
		ObjType:        q.ObjType,
		ObjTarget:      q.ObjTarget,
		ObjRoom:        sql.NullString{String: q.ObjRoom, Valid: q.ObjRoom != ""},
		ObjCount:       int64(q.ObjCount),
		ObjProgress:    0,
		RewardCredits:  int64(q.RewardCredits),
		RewardXpSkill:  sql.NullString{String: q.RewardXPSkill, Valid: q.RewardXPSkill != ""},
		RewardXpAmount: int64(q.RewardXPAmount),
		RewardItemID:   sql.NullString{String: q.RewardItemID, Valid: q.RewardItemID != ""},
		RewardItemName: sql.NullString{String: q.RewardItemName, Valid: q.RewardItemName != ""},
		RewardItemDesc: sql.NullString{String: q.RewardItemDesc, Valid: q.RewardItemDesc != ""},
		GiverNpcID:     sql.NullString{String: q.GiverNPCID, Valid: q.GiverNPCID != ""},
		AcceptedAt:     q.AcceptedAt,
		NextQuestID:    q.NextQuestID,
	})
}

// Active returns all quests with status='active'.
func Active(gdb *gamedb.GameDB) ([]Quest, error) {
	queries := sqliteq.New(gdb.SQLiteDB())
	rows, err := queries.ListActiveQuests(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]Quest, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqlcQuest(r))
	}
	return out, nil
}

// Progress increments obj_progress by n for the given quest.
func Progress(gdb *gamedb.GameDB, id string, n int) error {
	queries := sqliteq.New(gdb.SQLiteDB())
	return queries.ProgressQuest(context.Background(), sqliteq.ProgressQuestParams{
		ObjProgress: int64(n),
		ID:          id,
	})
}

// Complete sets quest status to 'completed'.
func Complete(gdb *gamedb.GameDB, id string) error {
	queries := sqliteq.New(gdb.SQLiteDB())
	return queries.CompleteQuest(context.Background(), id)
}

// Fail sets quest status to 'failed'.
func Fail(gdb *gamedb.GameDB, id string) error {
	queries := sqliteq.New(gdb.SQLiteDB())
	return queries.FailQuest(context.Background(), id)
}

// Get fetches a single quest by ID.
func Get(gdb *gamedb.GameDB, id string) (*Quest, error) {
	queries := sqliteq.New(gdb.SQLiteDB())
	row, err := queries.GetQuest(context.Background(), id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quest %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	q := fromSqlcQuest(row)
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
	queries := sqliteq.New(gdb.SQLiteDB())
	idList, err := queries.ListActiveQuestIDs(context.Background())
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(idList))
	for _, id := range idList {
		ids[id] = true
	}
	return ids, nil
}

// checkProgress is the shared implementation for Check* functions.
func checkProgress(gdb *gamedb.GameDB, objType, target string) ([]Quest, error) {
	queries := sqliteq.New(gdb.SQLiteDB())
	rows, err := queries.ListActiveQuestsByTypeTarget(context.Background(), sqliteq.ListActiveQuestsByTypeTargetParams{
		ObjType:   objType,
		ObjTarget: target,
	})
	if err != nil {
		return nil, err
	}

	var matching []Quest
	for _, r := range rows {
		matching = append(matching, fromSqlcQuest(r))
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
