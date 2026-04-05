package espionage

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
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS player_stealth (
			id       INTEGER PRIMARY KEY CHECK (id = 1),
			level    INTEGER NOT NULL DEFAULT 50,
			disguise TEXT    NOT NULL DEFAULT 'none'
		);
		CREATE TABLE IF NOT EXISTS npc_memory (
			npc_id TEXT NOT NULL,
			action TEXT NOT NULL,
			ts     INTEGER NOT NULL,
			PRIMARY KEY (npc_id, action)
		);
	`); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func TestLoadStealthDefault(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := LoadStealth(db)
	if s.Level != 50 || s.Disguise != "none" {
		t.Errorf("expected level=50 disguise=none, got %+v", s)
	}
}

func TestHide(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s, _ := Hide(db)
	if s.Level < 50 {
		t.Errorf("stealth should not decrease from hide: got %d", s.Level)
	}
	if s.Level > 100 {
		t.Errorf("stealth should not exceed 100: got %d", s.Level)
	}
}

func TestHideCap(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Manually set stealth to 98
	db.Exec(`INSERT INTO player_stealth (id, level, disguise) VALUES (1, 98, 'none')`) //nolint:errcheck

	s, _ := Hide(db)
	if s.Level > 100 {
		t.Errorf("stealth exceeded 100: got %d", s.Level)
	}
}

func TestEvalDialogueTriggers(t *testing.T) {
	lines := []world.DialogueLine{
		{Trigger: "has_item:root-key", Text: "Where'd you get that key?"},
		{Trigger: "rep_gte:netrunners:10", Text: "One of us."},
		{Trigger: "skill_gte:hacking:3", Text: "Impressive skills."},
		{Trigger: "disguise:hazmat-suit", Text: "Nice suit."},
		{Trigger: "always", Text: "System's hot. Watch yourself."},
	}

	// Test always matches when nothing else does
	ctx := PlayerContext{
		InventoryIDs: []string{},
		Reputation:   map[string]int{},
		Skills:       map[string]int{},
		Disguise:     "none",
	}
	got := EvalDialogue(lines, ctx)
	if got != "System's hot. Watch yourself." {
		t.Errorf("expected always trigger, got: %q", got)
	}

	// Test has_item trigger matches
	ctx.InventoryIDs = []string{"root-key"}
	got = EvalDialogue(lines, ctx)
	if got != "Where'd you get that key?" {
		t.Errorf("expected has_item trigger, got: %q", got)
	}

	// Test rep_gte trigger
	ctx.InventoryIDs = []string{}
	ctx.Reputation = map[string]int{"netrunners": 10}
	got = EvalDialogue(lines, ctx)
	if got != "One of us." {
		t.Errorf("expected rep_gte trigger, got: %q", got)
	}

	// Test skill_gte trigger
	ctx.Reputation = map[string]int{}
	ctx.Skills = map[string]int{"hacking": 3}
	got = EvalDialogue(lines, ctx)
	if got != "Impressive skills." {
		t.Errorf("expected skill_gte trigger, got: %q", got)
	}

	// Test disguise trigger
	ctx.Skills = map[string]int{}
	ctx.Disguise = "hazmat-suit"
	got = EvalDialogue(lines, ctx)
	if got != "Nice suit." {
		t.Errorf("expected disguise trigger, got: %q", got)
	}
}

func TestEvalDialogueNoMatch(t *testing.T) {
	lines := []world.DialogueLine{
		{Trigger: "has_item:special-item", Text: "Wow."},
	}
	ctx := PlayerContext{}
	got := EvalDialogue(lines, ctx)
	if got != "they don't seem interested in talking." {
		t.Errorf("expected default message, got: %q", got)
	}
}

func TestEvalDialogueEmpty(t *testing.T) {
	got := EvalDialogue(nil, PlayerContext{})
	if got != "they don't seem interested in talking." {
		t.Errorf("expected default for empty dialogue, got: %q", got)
	}
}

func TestRecordMemory(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := RecordMemory(db, "npc-0", "talked"); err != nil {
		t.Fatalf("RecordMemory: %v", err)
	}

	var action string
	db.QueryRow(`SELECT action FROM npc_memory WHERE npc_id='npc-0'`).Scan(&action) //nolint:errcheck
	if action != "talked" {
		t.Errorf("expected action=talked, got %q", action)
	}
}

func TestHasAllShardsTrigger(t *testing.T) {
	line := world.DialogueLine{Trigger: "has_all_shards", Text: "All shards collected!"}

	ctxNo := PlayerContext{AllShardsCollected: false}
	if EvalDialogue([]world.DialogueLine{line}, ctxNo) == "All shards collected!" {
		t.Error("should not match when AllShardsCollected=false")
	}

	ctxYes := PlayerContext{AllShardsCollected: true}
	if got := EvalDialogue([]world.DialogueLine{line}, ctxYes); got != "All shards collected!" {
		t.Errorf("should match when AllShardsCollected=true, got %q", got)
	}
}

func TestQuestActiveTrigger(t *testing.T) {
	line := world.DialogueLine{Trigger: "quest_active:q-foo", Text: "On it!"}
	ctxNo := PlayerContext{ActiveQuestIDs: map[string]bool{}}
	ctxYes := PlayerContext{ActiveQuestIDs: map[string]bool{"q-foo": true}}
	if EvalDialogue([]world.DialogueLine{line}, ctxNo) == "On it!" {
		t.Error("should not match when quest not active")
	}
	if got := EvalDialogue([]world.DialogueLine{line}, ctxYes); got != "On it!" {
		t.Errorf("expected 'On it!' when quest active, got %q", got)
	}
}
