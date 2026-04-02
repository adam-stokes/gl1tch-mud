// Package quests manages player quests backed by SQLite.
package quests

import (
	"database/sql"
	"fmt"
	"time"
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
}

// Accept inserts a new quest into the database.
func Accept(db *sql.DB, q Quest) error {
	if q.AcceptedAt == 0 {
		q.AcceptedAt = time.Now().Unix()
	}
	_, err := db.Exec(
		`INSERT OR IGNORE INTO quests
		 (id, title, description, status, obj_type, obj_target, obj_room,
		  obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		  reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		q.ID, q.Title, q.Description, "active",
		q.ObjType, q.ObjTarget, q.ObjRoom,
		q.ObjCount, 0,
		q.RewardCredits, q.RewardXPSkill, q.RewardXPAmount,
		q.RewardItemID, q.RewardItemName, q.RewardItemDesc,
		q.GiverNPCID, q.AcceptedAt,
	)
	return err
}

// Active returns all quests with status='active'.
func Active(db *sql.DB) ([]Quest, error) {
	rows, err := db.Query(
		`SELECT id, title, description, status, obj_type, obj_target, obj_room,
		        obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		        reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at
		 FROM quests WHERE status='active'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQuests(rows)
}

// Progress increments obj_progress by n for the given quest.
func Progress(db *sql.DB, id string, n int) error {
	_, err := db.Exec(`UPDATE quests SET obj_progress=obj_progress+? WHERE id=? AND status='active'`, n, id)
	return err
}

// Complete sets quest status to 'completed'.
func Complete(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE quests SET status='completed' WHERE id=?`, id)
	return err
}

// Fail sets quest status to 'failed'.
func Fail(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE quests SET status='failed' WHERE id=?`, id)
	return err
}

// Get fetches a single quest by ID.
func Get(db *sql.DB, id string) (*Quest, error) {
	row := db.QueryRow(
		`SELECT id, title, description, status, obj_type, obj_target, obj_room,
		        obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		        reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at
		 FROM quests WHERE id=?`, id,
	)
	q, err := scanQuest(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quest %q not found", id)
	}
	return q, err
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

// checkProgress is the shared implementation for Check* functions.
func checkProgress(db *sql.DB, objType, target string) ([]Quest, error) {
	rows, err := db.Query(
		`SELECT id, title, description, status, obj_type, obj_target, obj_room,
		        obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		        reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at
		 FROM quests WHERE status='active' AND obj_type=? AND obj_target=?`,
		objType, target,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matching, err := scanQuests(rows)
	if err != nil {
		return nil, err
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

func scanQuests(rows *sql.Rows) ([]Quest, error) {
	var quests []Quest
	for rows.Next() {
		q, err := scanQuestRow(rows)
		if err != nil {
			return nil, err
		}
		quests = append(quests, *q)
	}
	return quests, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanQuest(row *sql.Row) (*Quest, error) {
	var q Quest
	err := row.Scan(
		&q.ID, &q.Title, &q.Description, &q.Status,
		&q.ObjType, &q.ObjTarget, &q.ObjRoom,
		&q.ObjCount, &q.ObjProgress,
		&q.RewardCredits, &q.RewardXPSkill, &q.RewardXPAmount,
		&q.RewardItemID, &q.RewardItemName, &q.RewardItemDesc,
		&q.GiverNPCID, &q.AcceptedAt,
	)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func scanQuestRow(row rowScanner) (*Quest, error) {
	var q Quest
	err := row.Scan(
		&q.ID, &q.Title, &q.Description, &q.Status,
		&q.ObjType, &q.ObjTarget, &q.ObjRoom,
		&q.ObjCount, &q.ObjProgress,
		&q.RewardCredits, &q.RewardXPSkill, &q.RewardXPAmount,
		&q.RewardItemID, &q.RewardItemName, &q.RewardItemDesc,
		&q.GiverNPCID, &q.AcceptedAt,
	)
	if err != nil {
		return nil, err
	}
	return &q, nil
}
