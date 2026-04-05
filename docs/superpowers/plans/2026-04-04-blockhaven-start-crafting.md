# Blockhaven Start Crafting Enhancement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 4-step tutorial quest chain `q-first-forge` through Apprentice Brix that teaches gather → craft → smelt → assembly, plus new early-game recipes and resources.

**Architecture:** All changes flow through three layers: (1) Go engine additions — new quest obj_types, dialogue triggers, quest chaining; (2) world YAML — resources, recipes, Brix dialogue, 4 new quests; (3) both world YAML files kept in sync (defaults + worlds/blockhaven). No new packages needed.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), world YAML. Test: in-memory SQLite (`sql.Open("sqlite", ":memory:")`).

---

## File Map

| File | What changes |
|---|---|
| `internal/db/schema.go` | Add `next_quest_id` column to `quests` table |
| `internal/world/world.go` | Add `NextQuestID` to `WorldQuest` struct |
| `internal/quests/quests.go` | Add `NextQuestID` to `Quest`; update Accept/scans; add `CheckCraft`, `CheckGather`, `CheckSmelt`, `CheckAssemble`, `ActiveIDs` |
| `internal/quests/quests_test.go` | New: tests for new Check functions and quest chaining |
| `internal/espionage/espionage.go` | Add `ActiveQuestIDs map[string]bool` to `PlayerContext`; add `quest_active:X` trigger |
| `internal/commands/commands.go` | Load active quest IDs into dialogue ctx; call `CheckCraft`/`CheckAssemble` after craft; auto-chain in `questComplete` |
| `internal/commands/mining.go` | Add quests import; call `CheckMine`, `CheckGather`, `CheckSmelt` |
| `internal/world/defaults/blockhaven/world.yaml` | Resources, recipes, Brix dialogue, 4 quests |
| `worlds/blockhaven/world.yaml` | Mirror all YAML changes from defaults file |

---

## Task 1: Schema — add next_quest_id

**Files:**
- Modify: `internal/db/schema.go:90-108`

- [ ] **Step 1: Add next_quest_id column**

Open `internal/db/schema.go`. In the `CREATE TABLE IF NOT EXISTS quests` block, add one line before the closing `)`:

```sql
CREATE TABLE IF NOT EXISTS quests (
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
);
```

- [ ] **Step 2: Verify build passes**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(schema): add next_quest_id column to quests table"
```

---

## Task 2: WorldQuest struct — add NextQuestID

**Files:**
- Modify: `internal/world/world.go:236-251`

- [ ] **Step 1: Add field to WorldQuest**

In `internal/world/world.go`, update `WorldQuest`:

```go
type WorldQuest struct {
	ID             string `yaml:"id"`
	Title          string `yaml:"title"`
	Description    string `yaml:"description"`
	GiverNPCID     string `yaml:"giver_npc_id"`
	ObjType        string `yaml:"obj_type"`
	ObjTarget      string `yaml:"obj_target"`
	ObjRoom        string `yaml:"obj_room,omitempty"`
	ObjCount       int    `yaml:"obj_count"`
	RewardCredits  int    `yaml:"reward_credits"`
	RewardXPSkill  string `yaml:"reward_xp_skill,omitempty"`
	RewardXPAmount int    `yaml:"reward_xp_amount,omitempty"`
	RewardItemID   string `yaml:"reward_item_id,omitempty"`
	RewardItemName string `yaml:"reward_item_name,omitempty"`
	RewardItemDesc string `yaml:"reward_item_desc,omitempty"`
	NextQuestID    string `yaml:"next_quest_id,omitempty"`
}
```

- [ ] **Step 2: Verify build passes**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/world/world.go
git commit -m "feat(world): add NextQuestID field to WorldQuest"
```

---

## Task 3: Quest package — NextQuestID + new Check functions

**Files:**
- Modify: `internal/quests/quests.go`
- Create: `internal/quests/quests_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/quests/quests_test.go`:

```go
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

	// 4 gathers should not complete
	for i := 0; i < 4; i++ {
		ready, err := CheckGather(db, "stick")
		if err != nil {
			t.Fatalf("CheckGather: %v", err)
		}
		if len(ready) != 0 {
			t.Errorf("gather %d: expected not ready, got %v", i+1, ready)
		}
	}

	// 5th gather should complete
	ready, err := CheckGather(db, "stick")
	if err != nil {
		t.Fatalf("CheckGather: %v", err)
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/quests/... -v 2>&1 | head -30
```
Expected: compilation errors about missing `CheckCraft`, `CheckGather`, `CheckSmelt`, `CheckAssemble`, `ActiveIDs`, `NextQuestID`.

- [ ] **Step 3: Update Quest struct and Accept in quests.go**

Replace the `Quest` struct and `Accept` function in `internal/quests/quests.go`:

```go
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

// Accept inserts a new quest into the database.
func Accept(db *sql.DB, q Quest) error {
	if q.AcceptedAt == 0 {
		q.AcceptedAt = time.Now().Unix()
	}
	_, err := db.Exec(
		`INSERT OR IGNORE INTO quests
		 (id, title, description, status, obj_type, obj_target, obj_room,
		  obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		  reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
		  next_quest_id)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		q.ID, q.Title, q.Description, "active",
		q.ObjType, q.ObjTarget, q.ObjRoom,
		q.ObjCount, 0,
		q.RewardCredits, q.RewardXPSkill, q.RewardXPAmount,
		q.RewardItemID, q.RewardItemName, q.RewardItemDesc,
		q.GiverNPCID, q.AcceptedAt, q.NextQuestID,
	)
	return err
}
```

- [ ] **Step 4: Update all Scan calls to include NextQuestID**

The `scanQuestRow` function scans 17 columns. Add `next_quest_id` as the 18th. Update both `scanQuest` and `scanQuestRow` in `internal/quests/quests.go`:

```go
func scanQuest(row *sql.Row) (*Quest, error) {
	var q Quest
	err := row.Scan(
		&q.ID, &q.Title, &q.Description, &q.Status,
		&q.ObjType, &q.ObjTarget, &q.ObjRoom,
		&q.ObjCount, &q.ObjProgress,
		&q.RewardCredits, &q.RewardXPSkill, &q.RewardXPAmount,
		&q.RewardItemID, &q.RewardItemName, &q.RewardItemDesc,
		&q.GiverNPCID, &q.AcceptedAt, &q.NextQuestID,
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
		&q.GiverNPCID, &q.AcceptedAt, &q.NextQuestID,
	)
	if err != nil {
		return nil, err
	}
	return &q, nil
}
```

Update `Active` and `Get` queries to include `next_quest_id` in SELECT:

```go
func Active(db *sql.DB) ([]Quest, error) {
	rows, err := db.Query(
		`SELECT id, title, description, status, obj_type, obj_target, obj_room,
		        obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		        reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
		        next_quest_id
		 FROM quests WHERE status='active'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQuests(rows)
}

func Get(db *sql.DB, id string) (*Quest, error) {
	row := db.QueryRow(
		`SELECT id, title, description, status, obj_type, obj_target, obj_room,
		        obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		        reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
		        next_quest_id
		 FROM quests WHERE id=?`, id,
	)
	q, err := scanQuest(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quest %q not found", id)
	}
	return q, err
}
```

Also update `checkProgress` SELECT:

```go
func checkProgress(db *sql.DB, objType, target string) ([]Quest, error) {
	rows, err := db.Query(
		`SELECT id, title, description, status, obj_type, obj_target, obj_room,
		        obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
		        reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
		        next_quest_id
		 FROM quests WHERE status='active' AND obj_type=? AND obj_target=?`,
		objType, target,
	)
	// ... rest unchanged
```

- [ ] **Step 5: Add new Check functions and ActiveIDs**

Append to `internal/quests/quests.go` before the closing of the file:

```go
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
	rows, err := db.Query(`SELECT id FROM quests WHERE status='active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}
```

- [ ] **Step 6: Run tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/quests/... -v
```
Expected: all 6 tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/quests/quests.go internal/quests/quests_test.go
git commit -m "feat(quests): add NextQuestID, CheckCraft/Gather/Smelt/Assemble/Mine, ActiveIDs"
```

---

## Task 4: Dialogue trigger — quest_active:X

**Files:**
- Modify: `internal/espionage/espionage.go:102-166`

- [ ] **Step 1: Add ActiveQuestIDs to PlayerContext**

In `internal/espionage/espionage.go`, update `PlayerContext`:

```go
// PlayerContext holds the info needed to evaluate dialogue triggers.
type PlayerContext struct {
	InventoryIDs       []string
	Reputation         map[string]int // faction → rep value
	Skills             map[string]int // skill → level
	Disguise           string
	AllShardsCollected bool           // true when all crystal_shards rows have collected=1
	ActiveQuestIDs     map[string]bool // questID → true when status='active'
}
```

- [ ] **Step 2: Add quest_active trigger case**

In `matchTrigger`, add a new case after `has_all_shards`:

```go
	case strings.HasPrefix(trigger, "quest_active:"):
		questID := strings.TrimPrefix(trigger, "quest_active:")
		return ctx.ActiveQuestIDs[questID]
```

- [ ] **Step 3: Run existing espionage tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/espionage/... -v
```
Expected: all existing tests pass (the new field is zero-value safe).

- [ ] **Step 4: Commit**

```bash
git add internal/espionage/espionage.go
git commit -m "feat(espionage): add quest_active trigger and ActiveQuestIDs to PlayerContext"
```

---

## Task 5: Wire quest checks in commands.go

**Files:**
- Modify: `internal/commands/commands.go`

Three changes in this file: (a) load active quest IDs into dialogue context, (b) call craft/assemble quest checks, (c) auto-chain in questComplete.

- [ ] **Step 1: Load active quest IDs into dialogue context**

In `internal/commands/commands.go` around line 980-996, update the context build:

```go
	// Build player context
	invIDs := inventoryIDs(db)
	st := espionage.LoadStealth(db)
	rep := buildReputationMap(db)
	sk := buildSkillMap(db)

	var shardCount, totalShards int
	db.QueryRow(`SELECT COUNT(*) FROM crystal_shards WHERE collected=1`).Scan(&shardCount)   //nolint:errcheck
	db.QueryRow(`SELECT COUNT(*) FROM crystal_shards`).Scan(&totalShards)                    //nolint:errcheck

	activeQuestIDs, _ := quests.ActiveIDs(db)

	ctx := espionage.PlayerContext{
		InventoryIDs:       invIDs,
		Reputation:         rep,
		Skills:             sk,
		Disguise:           st.Disguise,
		AllShardsCollected: totalShards >= 5 && shardCount >= 5,
		ActiveQuestIDs:     activeQuestIDs,
	}
```

- [ ] **Step 2: Add quest checks after successful Craft**

In `internal/commands/commands.go`, find the Craft function handler. After `res := crafting.Craft(...)` and the `if res.OK {` block (around line 891-917), add quest checks before `return Result{Output: res.Message, Event: ev}`:

```go
	if res.OK {
		ev = &Event{
			Topic: "mud.craft.completed",
			Payload: map[string]any{
				"recipe_id":   recipeID,
				"output_item": res.OutputItem.ID,
			},
		}
		// Quest checks
		recipe := w.FindRecipe(recipeID)
		var readyQuests []quests.Quest
		if recipe != nil && recipe.Type == world.RecipeTypeAssembly {
			readyQuests, _ = quests.CheckAssemble(db, res.OutputItem.ID)
		} else {
			readyQuests, _ = quests.CheckCraft(db, res.OutputItem.ID)
		}
		for _, q := range readyQuests {
			res.Message += fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID)
		}
	}
```

Note: remove the old `ev = &Event{...}` block that was inside the `if res.OK` and fold it into this new block.

- [ ] **Step 3: Auto-chain in questComplete**

In `internal/commands/commands.go`, in the `questComplete` function after `quests.Complete(db, id)` succeeds and rewards are granted, add auto-chaining before the final return:

```go
	// Auto-chain: accept the next quest in the chain if set
	if q.NextQuestID != "" {
		wq := w.FindQuest(q.NextQuestID)
		if wq != nil {
			nextQ := quests.Quest{
				ID:             wq.ID,
				Title:          wq.Title,
				Description:    wq.Description,
				ObjType:        wq.ObjType,
				ObjTarget:      wq.ObjTarget,
				ObjRoom:        wq.ObjRoom,
				ObjCount:       wq.ObjCount,
				RewardCredits:  wq.RewardCredits,
				RewardXPSkill:  wq.RewardXPSkill,
				RewardXPAmount: wq.RewardXPAmount,
				RewardItemID:   wq.RewardItemID,
				RewardItemName: wq.RewardItemName,
				RewardItemDesc: wq.RewardItemDesc,
				GiverNPCID:     wq.GiverNPCID,
				NextQuestID:    wq.NextQuestID,
			}
			if err := quests.Accept(db, nextQ); err == nil {
				out.WriteString(fmt.Sprintf("\n[NEW QUEST] %s\n%s", wq.Title, wq.Description))
			}
		}
	}
```

Note: `questComplete` takes `(db *sql.DB, id string)` — it doesn't currently have access to `w *world.World`. You need to add `w` as a parameter. Find the `questComplete` call site and update:

Current call (around line 1285):
```go
return questComplete(db, args[1])
```

Change to:
```go
return questComplete(db, w, args[1])
```

Change the function signature from:
```go
func questComplete(db *sql.DB, id string) Result {
```
to:
```go
func questComplete(db *sql.DB, w *world.World, id string) Result {
```

- [ ] **Step 4: Build and verify**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/commands/commands.go
git commit -m "feat(commands): wire craft/assemble quest checks and quest chain auto-accept"
```

---

## Task 6: Wire quest checks in mining.go

**Files:**
- Modify: `internal/commands/mining.go`

- [ ] **Step 1: Add quests import**

In `internal/commands/mining.go`, add to imports:

```go
import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/enchanting"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/quests"
	"github.com/adam-stokes/gl1tch-mud/internal/weather"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)
```

- [ ] **Step 2: Call CheckMine after successful mine**

In `Mine`, after the `for _, y := range yields` loop (around line 160-166), append quest check before `return`:

```go
	out := strings.TrimRight(b.String(), "\n")
	// Quest mine check
	readyQuests, _ := quests.CheckMine(db, res.ID)
	for _, q := range readyQuests {
		out += fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID)
	}
	return Result{Output: out}
```

Note: the current return is `return Result{Output: strings.TrimRight(b.String(), "\n")}` — replace it with the above.

- [ ] **Step 3: Call CheckGather after successful gather**

In `Gather`, after `player.AddItem` is called for each yield (inside the loop, around line 317), append quest checks after the loop and before the return. The current code at end of Gather is:

```go
	var b strings.Builder
	b.WriteString("you gather from the surroundings...\n")
	for _, y := range yields {
		player.AddItem(db, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		fmt.Fprintf(&b, "  + %dx %s\n", y.CountMin, y.Name)
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
```

Replace with:

```go
	var b strings.Builder
	b.WriteString("you gather from the surroundings...\n")
	for _, y := range yields {
		player.AddItem(db, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		fmt.Fprintf(&b, "  + %dx %s\n", y.CountMin, y.Name)
		// Each gathered item can contribute to gather quests
		readyQuests, _ := quests.CheckGather(db, y.ItemID)
		for _, q := range readyQuests {
			fmt.Fprintf(&b, "\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID)
		}
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
```

- [ ] **Step 4: Call CheckSmelt after successful smelt**

In `Smelt`, after `player.AddItem(db, result[0], ...)` and `bumpActions(db)` (around line 393-395), the current return is:

```go
	player.AddItem(db, result[0], result[1], fmt.Sprintf("Smelted from %s.", itemID)) //nolint:errcheck
	bumpActions(db)

	return Result{Output: fmt.Sprintf(
		"you feed the furnace with %s and smelt the %s.\nyou receive: 1x %s.",
		fuel, itemID, result[1],
	)}
```

Replace with:

```go
	player.AddItem(db, result[0], result[1], fmt.Sprintf("Smelted from %s.", itemID)) //nolint:errcheck
	bumpActions(db)

	out := fmt.Sprintf("you feed the furnace with %s and smelt the %s.\nyou receive: 1x %s.", fuel, itemID, result[1])
	readyQuests, _ := quests.CheckSmelt(db, result[0])
	for _, q := range readyQuests {
		out += fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID)
	}
	return Result{Output: out}
```

- [ ] **Step 5: Build and verify**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```
Expected: no output.

- [ ] **Step 6: Run all tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1 | tail -20
```
Expected: all PASS, no FAIL.

- [ ] **Step 7: Commit**

```bash
git add internal/commands/mining.go
git commit -m "feat(mining): wire CheckMine, CheckGather, CheckSmelt quest hooks"
```

---

## Task 7: World YAML — defaults file

**Files:**
- Modify: `internal/world/defaults/blockhaven/world.yaml`

All edits in this task are YAML-only. Make them in order.

### 7a: Add tree-stump to meadow-0

- [ ] **Step 1: Add resource to meadow-0**

Find `- id: meadow-0` and its `resources: []` line. Replace `resources: []` with:

```yaml
    resources:
      - id: tree-stump
        type: harvest
        yields:
          - item_id: wood-log
            name: "Wood Log"
            desc: "A rough log pulled from an old stump."
            probability: 1.0
            count_min: 1
            count_max: 2
        tool_required: ""
        respawn_actions: 20
```

### 7b: Add iron-vein to meadow-1

- [ ] **Step 2: Add iron-vein resource to meadow-1**

Find `- id: meadow-1` and its `resources:` section. Add after the existing `stone-deposits` entry:

```yaml
      - id: iron-vein
        type: mine
        yields:
          - item_id: iron-ore
            name: "Iron Ore"
            desc: "Raw iron ore. Smelt it into an ingot."
            probability: 0.7
            count_min: 1
            count_max: 2
        tool_required: pickaxe
        respawn_actions: 30
```

### 7c: Add gun component items to meadow-1

- [ ] **Step 3: Add starter gun parts to meadow-1 items**

Find `- id: meadow-1` and its `items:` section (currently contains `coal-stack`). Add two entries after coal-stack:

```yaml
      - id: forge-frame-kit
        name: "Forge Frame Kit"
        desc: "A small crate from Brix — a pipe frame and copper tube, ready for the scrap-forge."
        signal_tier: noise
```

Note: We're using a single "kit" item here rather than two separate components, to avoid requiring meadow-1 to have both individually. The kit, when taken, seeds the two assembly components. Actually, let's use a different approach: add the two items separately. The player picks them up with `take pipe-frame-crude` and `take copper-tube-crude`.

Replace the above with:

```yaml
      - id: pipe-frame-crude
        name: "Pipe Frame"
        desc: "A rough iron pipe frame. Component for gun assembly."
        tags: [component, gun-frame]
        quality: crude
      - id: copper-tube-crude
        name: "Copper Tube"
        desc: "A salvaged copper pipe. Component for gun assembly."
        tags: [component, gun-barrel]
        quality: crude
        stat_mods:
          damage: 1
          range: 1
```

### 7d: Add new crafting recipes

- [ ] **Step 4: Add 4 new recipes**

Find `crafting_recipes:` section. After the existing `wooden-sword` recipe (around line 546-555) and before `stone-pickaxe`, insert:

```yaml
  - id: wooden-axe
    name: "Wooden Axe"
    ingredients:
      - {id: wood-log, count: 3}
      - {id: stick, count: 2}
    output:
      id: wooden-axe
      name: "Wooden Axe"
      desc: "A basic axe. Can chop down trees for wood logs."
    skill_req: 0

  - id: stone-sword
    name: "Stone Sword"
    ingredients:
      - {id: stone, count: 2}
      - {id: stick, count: 1}
    output:
      id: stone-sword
      name: "Stone Sword"
      desc: "A stone sword. 8 attack."
    skill_req: 0

  - id: stone-axe
    name: "Stone Axe"
    ingredients:
      - {id: stone, count: 3}
      - {id: stick, count: 2}
    output:
      id: stone-axe
      name: "Stone Axe"
      desc: "A stone axe. Chops wood faster than a wooden axe."
    skill_req: 0

  - id: crude-iron-chest
    name: "Crude Iron Chestplate"
    ingredients:
      - {id: iron-ingot, count: 4}
    output:
      id: crude-iron-chest
      name: "Crude Iron Chestplate"
      desc: "A rough iron chestplate. 3 defense. Better than nothing."
    skill_req: 0
    workbench: workbench
```

### 7e: Update Brix dialogue

- [ ] **Step 5: Replace Brix dialogue**

Find `- id: apprentice-brix` in the meadow-1 npcs section. Replace the entire `dialogue:` block with:

```yaml
        dialogue:
          - trigger: quest_active:q-first-forge-4
            text: "Head up to the ruins workshop — exit is 'up' from the town square. Grab those parts on the table and try the scrap-forge."
          - trigger: quest_active:q-first-forge-3
            text: "Toss that iron-ore in the furnace with some coal. Type 'smelt iron-ore'. The coal's in that pile over there."
          - trigger: quest_active:q-first-forge-2
            text: "Now craft a stone sword. You need 2 stone and 1 stick. 'craft stone-sword'. Mine the deposits here if you need more stone."
          - trigger: quest_active:q-first-forge-1
            text: "The meadow's full of sticks if you look. Type 'gather' and collect five. Oh — there's an old stump in the square too."
          - trigger: skill_gte:mining:2
            text: "Nice pickaxe work. You're getting good at this. Ask me about iron gear when you have the ingots."
          - trigger: always
            text: "The workbench is yours to use! Type 'craft' to see what you can make. I've got a few tasks for you — good way to learn the ropes."
            quest_id: q-first-forge-1
```

### 7f: Add 4 new quests

- [ ] **Step 6: Add q-first-forge quests**

Find the `quests:` section. Add 4 new entries before `q-meadow-shard`:

```yaml
  - id: q-first-forge-1
    title: "First Forge: Gather"
    description: |
      Apprentice Brix wants to see what you can do.
      Gather 5 sticks from the meadow — type 'gather'. There's also an old tree stump
      in the town square you can pull logs from.
    giver_npc_id: apprentice-brix
    obj_type: gather
    obj_target: stick
    obj_count: 5
    reward_credits: 0
    reward_xp_skill: crafting
    reward_xp_amount: 10
    reward_item_id: wood-log
    reward_item_name: "Wood Log"
    reward_item_desc: "A rough log from the meadow stump."
    next_quest_id: q-first-forge-2

  - id: q-first-forge-2
    title: "First Forge: Stone Sword"
    description: |
      Brix wants you to craft a stone sword. Mine some stone in the workshop
      with your pickaxe, then type 'craft stone-sword' (needs 2 stone + 1 stick).
    giver_npc_id: apprentice-brix
    obj_type: craft
    obj_target: stone-sword
    obj_count: 1
    reward_credits: 0
    reward_xp_skill: crafting
    reward_xp_amount: 15
    reward_item_id: iron-ore
    reward_item_name: "Iron Ore"
    reward_item_desc: "Raw iron ore. Smelt it into an ingot."
    next_quest_id: q-first-forge-3

  - id: q-first-forge-3
    title: "First Forge: Smelt"
    description: |
      Use the furnace in the Builder's Workshop to smelt 1 iron ingot.
      You need 1 iron-ore and 1 coal (or wood-log) as fuel. Type 'smelt iron-ore'.
    giver_npc_id: apprentice-brix
    obj_type: smelt
    obj_target: iron-ingot
    obj_count: 1
    reward_credits: 0
    reward_xp_skill: crafting
    reward_xp_amount: 20
    reward_item_id: crude-iron-chest
    reward_item_name: "Crude Iron Chestplate"
    reward_item_desc: "A rough iron chestplate. 3 defense."
    next_quest_id: q-first-forge-4

  - id: q-first-forge-4
    title: "First Forge: Assembly"
    description: |
      Head up to the ruins workshop above Meadow Town Square (type 'up' from the square).
      Pick up the pipe frame and copper tube on the table in the workshop, then use the
      scrap-forge to assemble a pipe-pistol: 'craft pipe-pistol frame=pipe-frame-crude barrel=copper-tube-crude'.
    giver_npc_id: apprentice-brix
    obj_type: assemble
    obj_target: pipe-pistol
    obj_count: 1
    reward_credits: 100
    reward_xp_skill: crafting
    reward_xp_amount: 30
    reward_item_id: ""
    reward_item_name: ""
    reward_item_desc: ""
    next_quest_id: ""
```

- [ ] **Step 7: Verify YAML parses**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -v 2>&1 | tail -20
```
Expected: all tests pass.

- [ ] **Step 8: Commit defaults file**

```bash
git add internal/world/defaults/blockhaven/world.yaml
git commit -m "feat(blockhaven): add start crafting resources, recipes, Brix quest chain"
```

---

## Task 8: Sync worlds/blockhaven/world.yaml

**Files:**
- Modify: `worlds/blockhaven/world.yaml`

- [ ] **Step 1: Apply all Task 7 changes to worlds/blockhaven/world.yaml**

Repeat every change from Task 7 (steps 1-6) identically in `worlds/blockhaven/world.yaml`. The two files have identical structure — apply each change in the same location.

- [ ] **Step 2: Verify both files are identical (excluding possible whitespace)**

```bash
diff internal/world/defaults/blockhaven/world.yaml worlds/blockhaven/world.yaml
```
Expected: no output (files are identical).

- [ ] **Step 3: Commit**

```bash
git add worlds/blockhaven/world.yaml
git commit -m "feat(blockhaven): sync worlds/blockhaven/world.yaml with defaults"
```

---

## Task 9: Smoke test the full journey

- [ ] **Step 1: Run all tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1 | grep -E "FAIL|ok|---"
```
Expected: all packages show `ok`, no `FAIL`.

- [ ] **Step 2: Manual smoke test (launch game, switch to blockhaven)**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go run . &
```

Then in the game:
```
world switch blockhaven
```
Expected: lands in `Meadow Town Square`. Inventory contains wooden-pickaxe, wooden-sword, bread, builders-map.

```
harvest tree-stump
```
Expected: `you harvest the tree-stump... + 1x Wood Log`

```
gather
```
Expected: yields include stick(s) and/or flint.

```
north
talk apprentice-brix
```
Expected: Brix says "The workbench is yours to use!..." and `[QUEST ACCEPTED] First Forge: Gather` appears.

```
gather
gather
gather
gather
gather
quest complete q-first-forge-1
```
Expected: each gather that yields stick(s) increments progress. After enough sticks, `quest ready: [First Forge: Gather]` appears. Completing it shows `+ item: Wood Log` and immediately accepts `[NEW QUEST] First Forge: Stone Sword`.

```
mine stone-deposits
craft stone-sword
quest complete q-first-forge-2
```
Expected: `you craft Stone Sword.` → `quest ready: [First Forge: Stone Sword]`. Completing rewards iron-ore and accepts `[NEW QUEST] First Forge: Smelt`.

```
smelt iron-ore
quest complete q-first-forge-3
```
Expected: `you receive: 1x Iron Ingot.` → `quest ready: [First Forge: Smelt]`. Completing rewards crude-iron-chest and accepts `[NEW QUEST] First Forge: Assembly`.

```
south
take pipe-frame-crude
take copper-tube-crude
up
craft pipe-pistol frame=pipe-frame-crude barrel=copper-tube-crude
quest complete q-first-forge-4
```
Expected: `you forge Pipe Pistol.` → `quest ready: [First Forge: Assembly]`. Completing rewards 100 credits.

- [ ] **Step 3: Final commit if any fixes were needed**

```bash
git add -p
git commit -m "fix(blockhaven): smoke test fixes for start crafting journey"
```

---

## Self-Review

**Spec coverage check:**
- ✅ Tree-stump resource in meadow-0 (Task 7a)
- ✅ Iron-vein in meadow-1 (Task 7b)
- ✅ wooden-axe, stone-axe, stone-sword, crude-iron-chest recipes (Task 7d)
- ✅ 4-step quest chain via q-first-forge-1 through 4 (Tasks 3, 7f)
- ✅ Quest chaining via next_quest_id (Tasks 1-3, 5)
- ✅ Brix dialogue with quest_active triggers (Tasks 4, 7e)
- ✅ Gun components in meadow-1 for assembly step (Task 7c)
- ✅ CheckCraft, CheckGather, CheckSmelt, CheckAssemble wired (Tasks 3, 5, 6)

**Type consistency:**
- `NextQuestID` is consistent across `WorldQuest` (world.go), `Quest` (quests.go), schema, and Accept INSERT
- `CheckCraft/Gather/Smelt/Assemble/Mine` all delegate to `checkProgress` — consistent with existing `CheckKill/Hack/Retrieve`
- `ActiveQuestIDs map[string]bool` is consistent between `PlayerContext` and `ActiveIDs` return type

**Ambiguity resolved:**
- Gun components (pipe-frame-crude, copper-tube-crude) are room items in meadow-1 that the player picks up with `take` before going to the ruins-workshop. Brix references them in the step-4 active dialogue.
- `questComplete` now takes `w *world.World` for the chain lookup — the call site is updated in the same task.
