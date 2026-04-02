package locking

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
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS lock_state (
		lock_id  TEXT PRIMARY KEY,
		unlocked INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestIsLockedDefault(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if !IsLocked(db, "lock-1") {
		t.Error("lock should be locked by default (no record)")
	}
}

func TestUnlockFunc(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := Unlock(db, "lock-1"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if IsLocked(db, "lock-1") {
		t.Error("lock should be unlocked after Unlock()")
	}
}

func TestPickSuccess(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	found := false
	for i := 0; i < 20; i++ {
		res := Pick(db, "lock-easy", 1, 100)
		if res.Success {
			found = true
			if IsLocked(db, "lock-easy") {
				t.Error("lock should be unlocked after successful pick")
			}
			break
		}
	}
	if !found {
		t.Error("expected pick to succeed with skill=100, difficulty=1")
	}
}

func TestPickFailure(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	for i := 0; i < 20; i++ {
		res := Pick(db, "lock-hard", 10, 0)
		if res.Success {
			t.Error("pick should never succeed with skill=0 and difficulty=10")
		}
		if !IsLocked(db, "lock-hard") {
			t.Error("lock should remain locked after failed pick")
		}
	}
}

func TestUnlockWithKeyHelper(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Simulate what UnlockWithKey does without importing world
	lockKeys := []string{"root-key", "master-key"}
	playerInv := []string{"data-chip"}

	// No matching key
	matched := false
	for _, keyID := range lockKeys {
		for _, itemID := range playerInv {
			if keyID == itemID {
				matched = true
			}
		}
	}
	if matched {
		t.Error("should not match without correct key")
	}

	// With correct key
	playerInv = []string{"root-key"}
	matched = false
	for _, keyID := range lockKeys {
		for _, itemID := range playerInv {
			if keyID == itemID {
				matched = true
				Unlock(db, "vault-door") //nolint:errcheck
			}
		}
	}
	if !matched {
		t.Error("should match with root-key")
	}
	if IsLocked(db, "vault-door") {
		t.Error("vault-door should be unlocked after using key")
	}
}
