package hacking

import (
	"database/sql"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS system_state (
		room_id   TEXT NOT NULL,
		system_id TEXT NOT NULL,
		intrusion REAL NOT NULL DEFAULT 0,
		alert     INTEGER NOT NULL DEFAULT 0,
		hacked    INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (room_id, system_id)
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func testRoom() *world.Room {
	return &world.Room{
		ID: "r0",
		Systems: []world.System{
			{
				ID:            "easy-sys",
				SecurityLevel: 1,
				RewardItem:    "data-chip",
				RewardText:    "you cracked the firewall.",
			},
			{
				ID:            "hard-sys",
				SecurityLevel: 9,
			},
		},
	}
}

func TestNoSystemInRoom(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	emptyRoom := &world.Room{ID: "r-empty"}
	res := Hack(db, emptyRoom, "anything", 5)
	if !res.NoSystem {
		t.Errorf("expected NoSystem=true for room with no systems")
	}
}

func TestHackSuccess(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	room := testRoom()
	// With high skill and low security, should always succeed.
	// Force success: security_level=1 → penalty=10; skill=50 → roll = rand(1,100)+50-10
	// Minimum roll = 1+50-10 = 41 → still < 50 sometimes. Use skill=100 to guarantee.
	found := false
	for i := 0; i < 100; i++ {
		res := Hack(db, room, "easy-sys", 100)
		if res.Success {
			found = true
			if res.RewardItem != "data-chip" {
				t.Errorf("reward_item: got %q want %q", res.RewardItem, "data-chip")
			}
			break
		}
	}
	if !found {
		t.Error("expected at least one success with skill=100 and security_level=1")
	}
}

func TestAlreadyHacked(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Manually mark as hacked
	db.Exec(`INSERT INTO system_state (room_id, system_id, hacked) VALUES ('r0','easy-sys',1)`) //nolint:errcheck

	room := testRoom()
	res := Hack(db, room, "easy-sys", 100)
	if !res.AlreadyHacked {
		t.Errorf("expected AlreadyHacked=true")
	}
}

func TestAlertEscalation(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	room := testRoom()
	// hard-sys has security_level=9 → penalty=90; even with skill=0, rand(1,100)-90 → max 10
	// Always fails.
	alerts := 0
	for i := 0; i < 3; i++ {
		res := Hack(db, room, "hard-sys", 0)
		if !res.Success {
			alerts = res.AlertLevel
		}
	}
	if alerts < 1 {
		t.Errorf("expected alert to escalate, got %d", alerts)
	}
}

func TestAlertReachesThreshold(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Pre-set alert to 2
	db.Exec(`INSERT INTO system_state (room_id, system_id, alert, hacked) VALUES ('r0','hard-sys',2,0)`) //nolint:errcheck

	room := testRoom()
	res := Hack(db, room, "hard-sys", 0)
	if res.Success {
		return // rare success, skip
	}
	if res.AlertLevel < 3 {
		t.Errorf("expected alert >= 3, got %d", res.AlertLevel)
	}
	// Message should mention alarm triggered
	if res.AlertLevel >= 3 && res.Message == "" {
		t.Error("expected non-empty message at alert=3")
	}
}
