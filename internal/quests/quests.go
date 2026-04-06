// Package quests manages player quests backed by SQLite.
package quests

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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
func Accept(db *sql.DB, q Quest) error {
	if q.AcceptedAt == 0 {
		q.AcceptedAt = time.Now().Unix()
	}
	queries := sqliteq.New(db)
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
func Active(db *sql.DB) ([]Quest, error) {
	queries := sqliteq.New(db)
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
func Progress(db *sql.DB, id string, n int) error {
	queries := sqliteq.New(db)
	return queries.ProgressQuest(context.Background(), sqliteq.ProgressQuestParams{
		ObjProgress: int64(n),
		ID:          id,
	})
}

// Complete sets quest status to 'completed'.
func Complete(db *sql.DB, id string) error {
	queries := sqliteq.New(db)
	return queries.CompleteQuest(context.Background(), id)
}

// Fail sets quest status to 'failed'.
func Fail(db *sql.DB, id string) error {
	queries := sqliteq.New(db)
	return queries.FailQuest(context.Background(), id)
}

// Get fetches a single quest by ID.
func Get(db *sql.DB, id string) (*Quest, error) {
	queries := sqliteq.New(db)
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
func CheckKill(db *sql.DB, npcID string) ([]Quest, error) {
	return checkProgress(db, "kill", npcID)
}

// CheckHack finds active hack quests matching systemID, increments progress,
// and returns quests that just reached obj_count.
func CheckHack(db *sql.DB, systemID string) ([]Quest, error) {
	return checkProgress(db, "hack", systemID)
}

// CheckRetrieve finds active retrieve quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckRetrieve(db *sql.DB, itemID string) ([]Quest, error) {
	return checkProgress(db, "retrieve", itemID)
}

// CheckCraft finds active craft quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckCraft(db *sql.DB, itemID string) ([]Quest, error) {
	return checkProgress(db, "craft", itemID)
}

// CheckGather finds active gather quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckGather(db *sql.DB, itemID string) ([]Quest, error) {
	return checkProgress(db, "gather", itemID)
}

// CheckSmelt finds active smelt quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckSmelt(db *sql.DB, itemID string) ([]Quest, error) {
	return checkProgress(db, "smelt", itemID)
}

// CheckAssemble finds active assemble quests matching itemID, increments progress,
// and returns quests that just reached obj_count.
func CheckAssemble(db *sql.DB, itemID string) ([]Quest, error) {
	return checkProgress(db, "assemble", itemID)
}

// CheckMine finds active mine quests matching resourceID, increments progress,
// and returns quests that just reached obj_count.
func CheckMine(db *sql.DB, resourceID string) ([]Quest, error) {
	return checkProgress(db, "mine", resourceID)
}

// ActiveIDs returns a set of all active quest IDs.
func ActiveIDs(db *sql.DB) (map[string]bool, error) {
	queries := sqliteq.New(db)
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
func checkProgress(db *sql.DB, objType, target string) ([]Quest, error) {
	queries := sqliteq.New(db)
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
		if err := Progress(db, q.ID, 1); err != nil {
			continue
		}
		q.ObjProgress++
		if q.ObjProgress >= q.ObjCount {
			ready = append(ready, q)
		}
	}
	return ready, nil
}
