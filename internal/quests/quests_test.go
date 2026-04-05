package quests

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS quests (
		id               TEXT    PRIMARY KEY,
		title            TEXT    NOT NULL,
		description      TEXT,
		status           TEXT    NOT NULL DEFAULT 'active',
		obj_type         TEXT    NOT NULL,
		obj_target       TEXT    NOT NULL,
		obj_room         TEXT,
		obj_count        INTEGER NOT NULL DEFAULT 1,
		obj_progress     INTEGER NOT NULL DEFAULT 0,
		reward_credits   INTEGER NOT NULL DEFAULT 0,
		reward_xp_skill  TEXT,
		reward_xp_amount INTEGER NOT NULL DEFAULT 0,
		reward_item_id   TEXT,
		reward_item_name TEXT,
		reward_item_desc TEXT,
		giver_npc_id     TEXT,
		accepted_at      INTEGER NOT NULL,
		next_quest_id    TEXT    NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestCheckCraft(t *testing.T) {
	db := openTestDB(t)
	q := Quest{
		ID: "q-craft-test", Title: "Craft Test",
		ObjType: "craft", ObjTarget: "stone-sword", ObjCount: 1,
		AcceptedAt: 1,
	}
	if err := Accept(db, q); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	ready, err := CheckCraft(db, "stone-sword")
	if err != nil {
		t.Fatalf("CheckCraft: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "q-craft-test" {
		t.Errorf("expected q-craft-test ready, got %v", ready)
	}
}

func TestCheckGather(t *testing.T) {
	db := openTestDB(t)
	q := Quest{
		ID: "q-gather-test", Title: "Gather Test",
		ObjType: "gather", ObjTarget: "stick", ObjCount: 5,
		AcceptedAt: 1,
	}
	if err := Accept(db, q); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	for i := 0; i < 4; i++ {
		ready, err := CheckGather(db, "stick")
		if err != nil {
			t.Fatalf("CheckGather iter %d: %v", i, err)
		}
		if len(ready) != 0 {
			t.Errorf("gather %d: expected not ready, got %v", i+1, ready)
		}
	}
	ready, err := CheckGather(db, "stick")
	if err != nil {
		t.Fatalf("CheckGather final: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "q-gather-test" {
		t.Errorf("expected q-gather-test ready on 5th gather, got %v", ready)
	}
}

func TestCheckSmelt(t *testing.T) {
	db := openTestDB(t)
	q := Quest{
		ID: "q-smelt-test", Title: "Smelt Test",
		ObjType: "smelt", ObjTarget: "iron-ingot", ObjCount: 1,
		AcceptedAt: 1,
	}
	if err := Accept(db, q); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	ready, err := CheckSmelt(db, "iron-ingot")
	if err != nil {
		t.Fatalf("CheckSmelt: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "q-smelt-test" {
		t.Errorf("expected q-smelt-test ready, got %v", ready)
	}
}

func TestCheckAssemble(t *testing.T) {
	db := openTestDB(t)
	q := Quest{
		ID: "q-assemble-test", Title: "Assemble Test",
		ObjType: "assemble", ObjTarget: "pipe-pistol", ObjCount: 1,
		AcceptedAt: 1,
	}
	if err := Accept(db, q); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	ready, err := CheckAssemble(db, "pipe-pistol")
	if err != nil {
		t.Fatalf("CheckAssemble: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "q-assemble-test" {
		t.Errorf("expected q-assemble-test ready, got %v", ready)
	}
}

func TestNextQuestID(t *testing.T) {
	db := openTestDB(t)
	q := Quest{
		ID: "q-chain-1", Title: "Chain 1",
		ObjType: "gather", ObjTarget: "stick", ObjCount: 1,
		NextQuestID: "q-chain-2",
		AcceptedAt:  1,
	}
	if err := Accept(db, q); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	got, err := Get(db, "q-chain-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.NextQuestID != "q-chain-2" {
		t.Errorf("expected NextQuestID=q-chain-2, got %q", got.NextQuestID)
	}
}

func TestActiveIDs(t *testing.T) {
	db := openTestDB(t)
	q1 := Quest{ID: "q-a1", Title: "A1", ObjType: "gather", ObjTarget: "stick", ObjCount: 1, AcceptedAt: 1}
	q2 := Quest{ID: "q-a2", Title: "A2", ObjType: "gather", ObjTarget: "flint", ObjCount: 1, AcceptedAt: 1}
	Accept(db, q1) //nolint:errcheck
	Accept(db, q2) //nolint:errcheck
	Complete(db, "q-a2") //nolint:errcheck

	ids, err := ActiveIDs(db)
	if err != nil {
		t.Fatalf("ActiveIDs: %v", err)
	}
	if !ids["q-a1"] {
		t.Errorf("expected q-a1 to be active")
	}
	if ids["q-a2"] {
		t.Errorf("expected q-a2 not to be active (completed)")
	}
}
