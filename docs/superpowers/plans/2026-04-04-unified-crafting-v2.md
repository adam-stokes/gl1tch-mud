# Unified Crafting v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a generic slot-assembly recipe type to the crafting engine and implement gun/armor/item crafting content for Blockhaven with a kids assembly modal UI.

**Architecture:** The existing `Craft()` function gains a `slots map[string]string` parameter and dispatches to `craftIngredient` (existing logic) or `craftAssemble` (new) based on `recipe.Type`. The frontend routes to either the existing paint grid modal or a new assembly panel based on `recipe.type`. All content is defined in world YAML — no game-specific logic in engine code.

**Tech Stack:** Go (backend), TypeScript + Astro (frontend), YAML (content), SQLite (persistence), modernc.org/sqlite (test driver), Playwright (E2E tests), Bun (test runner)

---

## File Map

**Modified:**
- `internal/world/world.go` — add `CraftingRecipeType`, `CraftingSlot`, update `CraftingRecipe`, `Item`, `Room`
- `internal/crafting/crafting.go` — add `slots` param, split into `craftIngredient`/`craftAssemble`, add errors
- `internal/crafting/crafting_test.go` — add assembly test cases
- `internal/db/schema.go` — add `player_flags` table
- `internal/commands/commands.go` — parse slots from craft args, set player flags after crafting
- `web/src/lib/mud.ts` — add `type`/`slots` to `Recipe` interface, route assembly recipes to new modal, add assembly panel JS
- `web/src/pages/game.astro` — add assembly modal HTML
- `internal/world/defaults/blockhaven/world.yaml` — add ruins-workshop rooms, component items, all recipes

**Created:**
- `web/e2e/kids-assembly-modal.spec.ts`
- `web/e2e/kids-assembly-workbench.spec.ts`

---

## Task 1: Extend world data model

**Files:**
- Modify: `internal/world/world.go`

- [ ] **Step 1: Add `CraftingRecipeType` constants and `CraftingSlot` struct**

Insert after line 57 (after the `CraftingIngredient` struct closing brace), before `CraftingRecipe`:

```go
// CraftingRecipeType controls which crafting path is used.
type CraftingRecipeType string

const (
	RecipeTypeIngredient CraftingRecipeType = "ingredient"
	RecipeTypeAssembly   CraftingRecipeType = "assembly"
)

// CraftingSlot is a named slot in an assembly recipe.
type CraftingSlot struct {
	ID         string         `yaml:"id"`
	Name       string         `yaml:"name"`
	Required   bool           `yaml:"required"`
	AcceptsTag string         `yaml:"accepts_tag"`
	StatMods   map[string]int `yaml:"stat_mods,omitempty"`
}
```

- [ ] **Step 2: Add `Type` and `Slots` to `CraftingRecipe`**

In the `CraftingRecipe` struct, add after `TierNames`:

```go
Type  CraftingRecipeType `yaml:"type,omitempty"`
Slots []CraftingSlot     `yaml:"slots,omitempty"`
```

- [ ] **Step 3: Add `Tags`, `Stats`, `StatMods`, `Quality`, `UnlocksFlag` to `Item`**

In the `Item` struct, add after `IsMod`:

```go
Tags        []string       `yaml:"tags,omitempty"`
Stats       map[string]int `yaml:"stats,omitempty"`
StatMods    map[string]int `yaml:"stat_mods,omitempty"`
Quality     string         `yaml:"quality,omitempty"`
UnlocksFlag string         `yaml:"unlocks_flag,omitempty"`
```

- [ ] **Step 4: Add `WorkbenchTypes` to `Room`**

In the `Room` struct, add after `Resources`:

```go
WorkbenchTypes []string `yaml:"workbench_types,omitempty"`
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/world/world.go
git commit -m "feat(world): add assembly recipe type, slots, item tags/stats, room workbench_types"
```

---

## Task 2: Add player_flags DB table

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add `player_flags` table to schema**

In `internal/db/schema.go`, find the `schema` SQL constant and add this table before the closing backtick:

```sql
CREATE TABLE IF NOT EXISTS player_flags (
    flag TEXT PRIMARY KEY
);
```

- [ ] **Step 2: Run schema tests to verify idempotency**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/db/... -v
```

Expected: all tests pass including `TestSchemaMigration` and `TestSchemaIdempotent`.

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add player_flags table"
```

---

## Task 3: Refactor crafting engine with assembly support

**Files:**
- Modify: `internal/crafting/crafting.go`
- Modify: `internal/crafting/crafting_test.go`
- Modify: `internal/world/world.go`

- [ ] **Step 1: Write failing assembly tests first**

Add these test functions to `internal/crafting/crafting_test.go`:

```go
func makeAssemblyWorld() *world.World {
	return &world.World{
		CraftingRecipes: []world.CraftingRecipe{
			{
				ID:   "pipe-pistol",
				Name: "Pipe Pistol",
				Type: world.RecipeTypeAssembly,
				Output: world.Item{ID: "pipe-pistol", Name: "Pipe Pistol", Desc: "Scavenged firearm."},
				Slots: []world.CraftingSlot{
					{ID: "frame", Name: "Frame", Required: true, AcceptsTag: "gun-frame"},
					{ID: "barrel", Name: "Barrel", Required: true, AcceptsTag: "gun-barrel", StatMods: map[string]int{"damage": 2}},
					{ID: "grip", Name: "Grip", Required: false, AcceptsTag: "gun-stock", StatMods: map[string]int{"range": 1}},
				},
			},
		},
		Rooms: []world.Room{{
			ID: "start",
			Items: []world.Item{
				{ID: "pipe-frame-crude", Name: "Pipe Frame", Tags: []string{"gun-frame", "component"}},
				{ID: "copper-tube-crude", Name: "Copper Tube", Tags: []string{"gun-barrel", "component"}, StatMods: map[string]int{"damage": 2}},
				{ID: "wrapped-grip-crude", Name: "Wrapped Grip", Tags: []string{"gun-stock", "component"}, StatMods: map[string]int{"range": 1}},
				{ID: "wrong-item", Name: "Wrong Item", Tags: []string{"potion"}},
			},
		}},
	}
}

func seedAssemblyInventory(t *testing.T, db *sql.DB) {
	t.Helper()
	items := []struct{ id, name string }{
		{"pipe-frame-crude", "Pipe Frame"},
		{"copper-tube-crude", "Copper Tube"},
		{"wrapped-grip-crude", "Wrapped Grip"},
		{"wrong-item", "Wrong Item"},
	}
	for _, it := range items {
		db.Exec(`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,'')`, it.id, it.name) //nolint:errcheck
	}
}

func TestAssembleMissingRequiredSlot(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	// Provide frame but no barrel (required)
	slots := map[string]string{"frame": "pipe-frame-crude"}
	res := Craft(db, w, nil, "pipe-pistol", []string{"pipe-frame-crude"}, 0, slots)
	if res.OK {
		t.Error("expected failure: required slot 'barrel' not filled")
	}
}

func TestAssembleWrongComponent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	// wrong-item has tag "potion", not "gun-frame"
	slots := map[string]string{"frame": "wrong-item", "barrel": "copper-tube-crude"}
	res := Craft(db, w, nil, "pipe-pistol", []string{"wrong-item", "copper-tube-crude"}, 0, slots)
	if res.OK {
		t.Error("expected failure: wrong component tag for frame slot")
	}
}

func TestAssembleSuccess(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	slots := map[string]string{"frame": "pipe-frame-crude", "barrel": "copper-tube-crude"}
	res := Craft(db, w, nil, "pipe-pistol", []string{"pipe-frame-crude", "copper-tube-crude"}, 0, slots)
	if !res.OK {
		t.Errorf("expected success, got: %s", res.Message)
	}
	if res.OutputItem.ID != "pipe-pistol" {
		t.Errorf("expected pipe-pistol output, got %q", res.OutputItem.ID)
	}
	// damage stat should be 2 from barrel slot's item StatMods
	if res.OutputItem.Stats["damage"] != 2 {
		t.Errorf("expected damage=2, got %d", res.OutputItem.Stats["damage"])
	}
}

func TestAssembleStatAccumulation(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	seedAssemblyInventory(t, db)

	w := makeAssemblyWorld()
	slots := map[string]string{
		"frame":  "pipe-frame-crude",
		"barrel": "copper-tube-crude",
		"grip":   "wrapped-grip-crude",
	}
	res := Craft(db, w, nil, "pipe-pistol", []string{"pipe-frame-crude", "copper-tube-crude", "wrapped-grip-crude"}, 0, slots)
	if !res.OK {
		t.Errorf("expected success, got: %s", res.Message)
	}
	if res.OutputItem.Stats["damage"] != 2 {
		t.Errorf("expected damage=2, got %d", res.OutputItem.Stats["damage"])
	}
	if res.OutputItem.Stats["range"] != 1 {
		t.Errorf("expected range=1, got %d", res.OutputItem.Stats["range"])
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail (compile error expected)**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/crafting/... -v 2>&1 | head -30
```

Expected: compile error — `Craft` called with wrong number of arguments.

- [ ] **Step 3: Update existing test calls to pass nil slots**

In `crafting_test.go`, update every existing `Craft(...)` call to add `nil` as the last argument:

```go
// TestUnknownRecipe
res := Craft(db, w, nil, "bogus", []string{}, 0, nil)

// TestSkillGate
res := Craft(db, w, nil, "sniffer", []string{"silicon", "silicon", "wire"}, 0, nil)

// TestMissingIngredients
res := Craft(db, w, nil, "sniffer", []string{}, 3, nil)

// TestSuccessfulCraft
res := Craft(db, w, nil, "ice-pick", []string{"carbon-blade"}, 0, nil)

// TestNoRecipesKnown
res := Craft(db, w, nil, "anything", []string{}, 0, nil)
```

- [ ] **Step 4: Rewrite `internal/crafting/crafting.go`**

Replace the entire file with:

```go
// Package crafting implements the craft command and recipe processing.
package crafting

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Sentinel errors for assembly crafting.
var (
	ErrMissingSlot    = errors.New("required slot not filled")
	ErrWrongComponent = errors.New("item does not fit this slot")
)

// Result is the outcome of a crafting attempt.
type Result struct {
	OK           bool
	OutputItem   world.Item
	MissingItems []string
	Message      string
	UnlocksFlag  string // non-empty if OutputItem.UnlocksFlag is set
}

// Craft attempts to craft the recipe with the given ID.
// hackingSkill is the player's current hacking skill level.
// inventoryIDs is a list of item IDs the player currently carries.
// room is the player's current room (used for workbench check).
// slots maps slotID → itemID for assembly recipes; nil for ingredient recipes.
func Craft(db *sql.DB, w *world.World, room *world.Room, recipeID string, inventoryIDs []string, hackingSkill int, slots map[string]string) Result {
	recipe := w.FindRecipe(recipeID)
	if recipe == nil {
		var names []string
		for _, r := range w.CraftingRecipes {
			names = append(names, r.ID)
		}
		if len(names) == 0 {
			return Result{Message: "no recipes known."}
		}
		return Result{
			Message: fmt.Sprintf("unknown recipe %q. known: %s", recipeID, strings.Join(names, ", ")),
		}
	}

	switch recipe.Type {
	case world.RecipeTypeAssembly:
		return craftAssemble(db, w, room, recipe, inventoryIDs, hackingSkill, slots)
	default:
		return craftIngredient(db, w, room, recipe, inventoryIDs, hackingSkill)
	}
}

// craftIngredient is the existing ingredient-list crafting path, unchanged in behaviour.
func craftIngredient(db *sql.DB, w *world.World, room *world.Room, recipe *world.CraftingRecipe, inventoryIDs []string, hackingSkill int) Result {
	// Blueprint/unlock check
	if len(recipe.TierThresholds) > 0 {
		var count int
		_ = db.QueryRow(`SELECT COUNT(*) FROM unlocked_recipes WHERE recipe_id = ?`, recipe.ID).Scan(&count)
		if count == 0 {
			return Result{Message: "You need a blueprint to craft this."}
		}
	}

	// Skill gate
	if recipe.SkillReq > 0 && hackingSkill < recipe.SkillReq {
		return Result{
			Message: fmt.Sprintf(
				"skill too low: %s requires hacking level %d (you have %d).",
				recipe.Name, recipe.SkillReq, hackingSkill,
			),
		}
	}

	// Workbench check
	if recipe.Workbench != "" && !roomHasWorkbench(room, recipe.Workbench) {
		return Result{Message: fmt.Sprintf("This recipe requires a %s.", recipe.Workbench)}
	}

	// Build inventory count map
	invCount := make(map[string]int)
	for _, id := range inventoryIDs {
		invCount[id]++
	}

	// Check ingredients
	var missing []string
	for _, ing := range recipe.Ingredients {
		if invCount[ing.ID] < ing.Count {
			missing = append(missing, fmt.Sprintf("%s x%d", ing.ID, ing.Count))
		}
	}
	if len(missing) > 0 {
		return Result{
			MissingItems: missing,
			Message:      fmt.Sprintf("missing ingredients: %s", strings.Join(missing, ", ")),
		}
	}

	// Consume ingredients
	for _, ing := range recipe.Ingredients {
		for i := 0; i < ing.Count; i++ {
			db.Exec(`DELETE FROM inventory WHERE item_id=? LIMIT 1`, ing.ID) //nolint:errcheck
		}
	}

	// Apply tier
	out := recipe.Output
	tier := ""
	if len(recipe.TierThresholds) > 0 && len(recipe.TierNames) == len(recipe.TierThresholds) {
		for i := len(recipe.TierThresholds) - 1; i >= 0; i-- {
			if hackingSkill >= recipe.TierThresholds[i] {
				tier = recipe.TierNames[i]
				break
			}
		}
		if tier != "" {
			out.Name = tier + " " + out.Name
			out.ID = out.ID + "_" + strings.ToLower(tier)
		}
	}

	db.Exec( //nolint:errcheck
		`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`,
		out.ID, out.Name, out.Desc,
	)

	return Result{
		OK:          true,
		OutputItem:  out,
		UnlocksFlag: out.UnlocksFlag,
		Message:     fmt.Sprintf("you craft %s.", out.Name),
	}
}

// craftAssemble is the slot-based assembly path.
func craftAssemble(db *sql.DB, w *world.World, room *world.Room, recipe *world.CraftingRecipe, inventoryIDs []string, hackingSkill int, slots map[string]string) Result {
	// Skill gate
	if recipe.SkillReq > 0 && hackingSkill < recipe.SkillReq {
		return Result{
			Message: fmt.Sprintf(
				"skill too low: %s requires hacking level %d (you have %d).",
				recipe.Name, recipe.SkillReq, hackingSkill,
			),
		}
	}

	// Workbench check
	if recipe.Workbench != "" && !roomHasWorkbench(room, recipe.Workbench) {
		return Result{Message: fmt.Sprintf("This recipe requires a %s.", recipe.Workbench)}
	}

	// Build inventory set for fast lookup
	invSet := make(map[string]bool)
	for _, id := range inventoryIDs {
		invSet[id] = true
	}

	// Validate all required slots are filled
	for _, slot := range recipe.Slots {
		if slot.Required {
			if _, ok := slots[slot.ID]; !ok {
				return Result{Message: fmt.Sprintf("%s: required slot '%s' not filled.", ErrMissingSlot, slot.Name)}
			}
		}
	}

	// Validate each filled slot — item must be in inventory and have the right tag
	for _, slot := range recipe.Slots {
		itemID, filled := slots[slot.ID]
		if !filled {
			continue
		}
		if !invSet[itemID] {
			return Result{Message: fmt.Sprintf("you don't have %s in your inventory.", itemID)}
		}
		item := w.FindItem(itemID)
		if item == nil {
			return Result{Message: fmt.Sprintf("unknown item: %s.", itemID)}
		}
		if !hasTag(item.Tags, slot.AcceptsTag) {
			return Result{Message: fmt.Sprintf("%s: %s doesn't fit the %s slot.", ErrWrongComponent, item.Name, slot.Name)}
		}
	}

	// Consume all slot items from inventory
	for _, itemID := range slots {
		db.Exec(`DELETE FROM inventory WHERE item_id=? LIMIT 1`, itemID) //nolint:errcheck
	}

	// Build output: start from base output, accumulate stats from slot item StatMods
	out := recipe.Output
	if out.Stats == nil {
		out.Stats = make(map[string]int)
	}
	for _, slot := range recipe.Slots {
		itemID, filled := slots[slot.ID]
		if !filled {
			continue
		}
		item := w.FindItem(itemID)
		if item == nil {
			continue
		}
		for stat, val := range item.StatMods {
			out.Stats[stat] += val
		}
	}

	db.Exec( //nolint:errcheck
		`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`,
		out.ID, out.Name, out.Desc,
	)

	return Result{
		OK:          true,
		OutputItem:  out,
		UnlocksFlag: out.UnlocksFlag,
		Message:     fmt.Sprintf("you forge %s.", out.Name),
	}
}

// roomHasWorkbench returns true if the room has the given workbench type in its WorkbenchTypes list.
func roomHasWorkbench(room *world.Room, workbench string) bool {
	if room == nil {
		return false
	}
	for _, wt := range room.WorkbenchTypes {
		if wt == workbench {
			return true
		}
	}
	return false
}

// hasTag returns true if the tag slice contains the target tag.
func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

// UnlockRecipe records that the given recipe has been unlocked via a blueprint.
func UnlockRecipe(db *sql.DB, recipeID string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO unlocked_recipes (recipe_id, unlocked_at) VALUES (?, ?)`,
		recipeID, time.Now().Unix())
	return err
}

// IsUnlocked reports whether the given recipe has been unlocked.
func IsUnlocked(db *sql.DB, recipeID string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM unlocked_recipes WHERE recipe_id = ?`, recipeID).Scan(&count)
	return count > 0, err
}

// SetPlayerFlag sets a boolean flag in the player_flags table.
func SetPlayerFlag(db *sql.DB, flag string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO player_flags (flag) VALUES (?)`, flag)
	return err
}

// IsPlayerFlagSet returns true if the flag exists in player_flags.
func IsPlayerFlagSet(db *sql.DB, flag string) bool {
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM player_flags WHERE flag = ?`, flag).Scan(&count)
	return count > 0
}
```

- [ ] **Step 5: Add `FindItem` method to `world.World`**

In `internal/world/world.go`, add this method near the other `Find*` methods:

```go
// FindItem searches all room item lists across the world for the given item ID.
func (w *World) FindItem(id string) *Item {
	for i := range w.Rooms {
		for j := range w.Rooms[i].Items {
			if w.Rooms[i].Items[j].ID == id {
				return &w.Rooms[i].Items[j]
			}
		}
	}
	return nil
}
```

- [ ] **Step 6: Run all crafting tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/crafting/... -v
```

Expected: all 9 tests pass (5 existing + 4 new assembly tests).

- [ ] **Step 7: Build to check for compile errors across all packages**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```

Expected: compile error in `internal/commands/commands.go` (wrong arity on `Craft` call) — fixed in Task 4.

- [ ] **Step 8: Commit**

```bash
git add internal/crafting/crafting.go internal/crafting/crafting_test.go internal/world/world.go
git commit -m "feat(crafting): slot-assembly path, FindItem, SetPlayerFlag, workbench_types check"
```

---

## Task 4: Update the Craft command handler

**Files:**
- Modify: `internal/commands/commands.go`

- [ ] **Step 1: Replace the `Craft` function body**

Locate the `Craft` function in `commands.go` (around line 870). Replace it entirely:

```go
func Craft(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "craft <recipe-id> [slotID=itemID ...] — craft or assemble an item"}
	}
	recipeID := args[0]
	hackSkill := skills.Level(db, "hacking")
	invIDs := inventoryIDs(db)
	room := w.Room(s.RoomID)

	// Parse optional slot assignments from args[1:]: "barrel=item-uuid-abc grip=item-uuid-def"
	var slots map[string]string
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			if slots == nil {
				slots = make(map[string]string)
			}
			slots[parts[0]] = parts[1]
		}
	}

	res := crafting.Craft(db, w, room, recipeID, invIDs, hackSkill, slots)

	// Persist any player flag unlocked by the output item
	if res.UnlocksFlag != "" {
		crafting.SetPlayerFlag(db, res.UnlocksFlag) //nolint:errcheck
	}

	var ev *Event
	if res.OK {
		ev = &Event{
			Topic: "mud.craft.completed",
			Payload: map[string]any{
				"recipe_id":   recipeID,
				"output_item": res.OutputItem.ID,
			},
		}
	} else if len(res.MissingItems) > 0 {
		ev = &Event{
			Topic: "mud.craft.failed",
			Payload: map[string]any{
				"recipe_id": recipeID,
				"missing":   res.MissingItems,
			},
		}
	}

	return Result{Output: res.Message, Event: ev}
}
```

- [ ] **Step 2: Ensure `strings` is imported**

Check the import block at the top of `commands.go`. If `"strings"` is not already present, add it.

- [ ] **Step 3: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```

Expected: no errors.

- [ ] **Step 4: Run all backend tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./... 2>&1 | tail -10
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/commands/commands.go
git commit -m "feat(commands): pass slots to Craft, persist unlocks_flag from output item"
```

---

## Task 5: Filter gun recipes by player flag in state updates

**Files:**
- Modify: `internal/server/session.go`

- [ ] **Step 1: Find where recipes are built for the state update**

```bash
grep -n "CraftingRecipe\|recipes\|crafting" /Users/stokes/Projects/gl1tch-mud/internal/server/session.go | head -20
```

Locate the function that builds the state update payload sent to the client and find where `w.CraftingRecipes` is iterated.

- [ ] **Step 2: Add the gun-recipe filter**

In that function, replace direct use of `w.CraftingRecipes` with a filtered slice:

```go
gunUnlocked := crafting.IsPlayerFlagSet(db, "gun_recipes_unlocked")

var visibleRecipes []world.CraftingRecipe
for _, r := range w.CraftingRecipes {
    if !gunUnlocked {
        isGunRecipe := false
        for _, slot := range r.Slots {
            if strings.HasPrefix(slot.AcceptsTag, "gun-") {
                isGunRecipe = true
                break
            }
        }
        if isGunRecipe {
            continue
        }
    }
    visibleRecipes = append(visibleRecipes, r)
}
// use visibleRecipes when building the recipes payload
```

- [ ] **Step 3: Add imports if needed**

Add `"strings"` and `"github.com/adam-stokes/gl1tch-mud/internal/crafting"` to imports in `session.go` if not already present.

- [ ] **Step 4: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/server/session.go
git commit -m "feat(session): filter gun recipes until gun_recipes_unlocked flag is set"
```

---

## Task 6: Update frontend Recipe interface

**Files:**
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Add `CraftingSlot` interface**

In `mud.ts`, before the `Recipe` interface, add:

```typescript
interface CraftingSlot {
  id: string;
  name: string;
  required: boolean;
  accepts_tag: string;
  stat_mods?: Record<string, number>;
}
```

- [ ] **Step 2: Extend the `Recipe` interface**

In the existing `Recipe` interface, add:

```typescript
type?: 'ingredient' | 'assembly';
slots?: CraftingSlot[];
```

- [ ] **Step 3: Extend the `InvItem` interface**

Find the `InvItem` (or equivalent inventory item) interface and add:

```typescript
tags?: string[];
stat_mods?: Record<string, number>;
quality?: string;
```

- [ ] **Step 4: Build frontend to verify no TS errors**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -10
```

Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/mud.ts
git commit -m "feat(frontend): extend Recipe and InvItem interfaces for assembly"
```

---

## Task 7: Add assembly modal HTML and CSS

**Files:**
- Modify: `web/src/pages/game.astro`

- [ ] **Step 1: Add assembly modal HTML**

Find `#kids-craft-modal` in `game.astro`. Add the following block immediately after the closing `</div>` of that modal:

```html
<!-- Kids Assembly Modal -->
<div id="kids-assembly-modal" class="kids-modal-overlay" aria-modal="true" role="dialog">
  <div class="kids-modal-box kids-assembly-box">
    <button id="kids-assembly-close" class="kids-modal-close" aria-label="Close">&#x2715;</button>
    <h2 id="kids-assembly-title" class="kids-modal-title">Forge</h2>

    <div class="kids-assembly-body">
      <div class="kids-assembly-silhouette" id="kids-assembly-silhouette">
        <svg viewBox="0 0 80 80" class="kids-assembly-svg" id="kids-assembly-svg"></svg>
      </div>
      <div class="kids-assembly-slots" id="kids-assembly-slots"></div>
    </div>

    <div class="kids-assembly-footer">
      <div class="kids-assembly-stats" id="kids-assembly-stats"></div>
      <button id="kids-assembly-forge-btn" class="kids-assembly-forge-btn" disabled>
        FORGE IT
      </button>
    </div>
  </div>
</div>

<!-- Assembly inventory picker -->
<div id="kids-assembly-inv-picker" class="kids-assembly-inv-picker" hidden>
  <div class="kids-assembly-inv-header">
    <span id="kids-assembly-inv-label">Pick component</span>
    <button id="kids-assembly-inv-close" class="kids-modal-close">&#x2715;</button>
  </div>
  <div id="kids-assembly-inv-list" class="kids-assembly-inv-list"></div>
</div>
```

- [ ] **Step 2: Add CSS for assembly modal**

In the `<style>` block of `game.astro`, scoped to `[data-ui="kids"]`, add:

```css
[data-ui="kids"] #kids-assembly-modal { display: none; }
[data-ui="kids"] #kids-assembly-modal.open { display: flex; }

[data-ui="kids"] .kids-assembly-box {
  width: min(92vw, 520px);
  max-height: 90vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.2rem;
}

[data-ui="kids"] .kids-assembly-body {
  display: grid;
  grid-template-columns: 90px 1fr;
  gap: 1rem;
  align-items: start;
}

[data-ui="kids"] .kids-assembly-silhouette {
  display: flex;
  align-items: center;
  justify-content: center;
}

[data-ui="kids"] .kids-assembly-svg {
  width: 80px;
  height: 80px;
  opacity: 0.7;
  color: #c9a84c;
}

[data-ui="kids"] .kids-assembly-slots {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

[data-ui="kids"] .kids-assembly-slot-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.6rem;
  border-radius: 6px;
  border: 2px solid #444;
  background: #1a1a2e;
  cursor: pointer;
  transition: border-color 0.15s;
}

[data-ui="kids"] .kids-assembly-slot-row.required-empty {
  border-color: #e05252;
  animation: slot-pulse 1s infinite alternate;
}

[data-ui="kids"] .kids-assembly-slot-row.filled {
  border-color: #c9a84c;
}

@keyframes slot-pulse {
  from { border-color: #e05252; }
  to   { border-color: #ff8888; }
}

[data-ui="kids"] .kids-assembly-slot-label {
  font-size: 0.75rem;
  color: #888;
  min-width: 60px;
  flex-shrink: 0;
}

[data-ui="kids"] .kids-assembly-slot-value {
  flex: 1;
  font-size: 0.8rem;
  color: #ccc;
}

[data-ui="kids"] .kids-assembly-slot-chips {
  display: flex;
  gap: 0.25rem;
  flex-wrap: wrap;
}

[data-ui="kids"] .kids-stat-chip {
  font-size: 0.65rem;
  padding: 1px 5px;
  background: #2a2a4a;
  border-radius: 4px;
  color: #c9a84c;
}

[data-ui="kids"] .kids-assembly-footer {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

[data-ui="kids"] .kids-assembly-stats {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

[data-ui="kids"] .kids-stat-bar-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
}

[data-ui="kids"] .kids-stat-bar-label {
  min-width: 50px;
  color: #888;
  text-transform: capitalize;
}

[data-ui="kids"] .kids-stat-bar-track {
  flex: 1;
  height: 8px;
  background: #2a2a4a;
  border-radius: 4px;
  overflow: hidden;
}

[data-ui="kids"] .kids-stat-bar-fill {
  height: 100%;
  background: #c9a84c;
  border-radius: 4px;
  transition: width 0.2s ease;
}

[data-ui="kids"] .kids-assembly-forge-btn {
  padding: 0.7rem 1.5rem;
  background: #c9a84c;
  color: #1a1a2e;
  font-weight: bold;
  font-size: 1rem;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  letter-spacing: 0.08em;
  align-self: stretch;
  transition: opacity 0.15s;
}

[data-ui="kids"] .kids-assembly-forge-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

[data-ui="kids"] .kids-assembly-forge-btn:not(:disabled):hover {
  opacity: 0.85;
}

[data-ui="kids"] .kids-assembly-inv-picker {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  background: #13131f;
  border-top: 2px solid #333;
  padding: 0.75rem;
  z-index: 1100;
  max-height: 50vh;
  overflow-y: auto;
}

[data-ui="kids"] .kids-assembly-inv-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
  font-size: 0.85rem;
  color: #c9a84c;
}

[data-ui="kids"] .kids-assembly-inv-list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

[data-ui="kids"] .kids-assembly-inv-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.6rem;
  background: #1a1a2e;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
}

[data-ui="kids"] .kids-assembly-inv-item:hover { background: #24243e; }

[data-ui="kids"] .kids-quality-badge {
  font-size: 0.65rem;
  padding: 1px 5px;
  border-radius: 4px;
  color: #fff;
}
[data-ui="kids"] .kids-quality-badge.crude    { background: #555; }
[data-ui="kids"] .kids-quality-badge.standard { background: #3a6a3a; }
[data-ui="kids"] .kids-quality-badge.refined  { background: #6a3a6a; }
```

- [ ] **Step 3: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -5
```

Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/game.astro
git commit -m "feat(kids-ui): assembly modal HTML and CSS"
```

---

## Task 8: Add assembly modal JavaScript

**Files:**
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Add assembly state object**

Near the `_kidscraft` state object in `mud.ts`, add:

```typescript
const _kidsassembly = {
  recipe: null as Recipe | null,
  filled: {} as Record<string, { itemId: string; itemName: string; statMods: Record<string, number>; quality: string }>,
};
```

- [ ] **Step 2: Add SVG silhouette helpers**

```typescript
const ASSEMBLY_SILHOUETTES: Record<string, string> = {
  gun:     '<path d="M10 45 L30 35 L30 30 L60 30 L70 35 L70 40 L60 42 L30 42 L30 45 Z M30 42 L28 55 L35 55 L35 42" fill="currentColor"/>',
  armor:   '<path d="M25 15 Q40 8 55 15 L60 35 Q40 45 20 35 Z M20 35 L15 60 L30 60 L35 45 Q40 50 45 45 L50 60 L65 60 L60 35" fill="currentColor"/>',
  default: '<circle cx="40" cy="40" r="25" fill="none" stroke="currentColor" stroke-width="3"/>',
};

function silhouetteForRecipe(recipe: Recipe): string {
  const n = recipe.name.toLowerCase();
  if (/pistol|rifle|cannon|launcher|sniper|repeater|barrel/.test(n)) return ASSEMBLY_SILHOUETTES.gun;
  if (/vest|coat|parka|suit|leathers|shell|wrap|exosuit/.test(n))     return ASSEMBLY_SILHOUETTES.armor;
  return ASSEMBLY_SILHOUETTES.default;
}
```

- [ ] **Step 3: Add `openKidsAssemblyModal` function**

```typescript
function openKidsAssemblyModal(recipe: Recipe): void {
  _kidsassembly.recipe = recipe;
  _kidsassembly.filled = {};

  const titleEl  = document.getElementById('kids-assembly-title')!;
  const svgEl    = document.getElementById('kids-assembly-svg')!;
  const slotsEl  = document.getElementById('kids-assembly-slots')!;

  titleEl.textContent = 'Forge: ' + recipe.name;

  // Set SVG silhouette using a safe path element — no user data involved
  svgEl.replaceChildren();
  const tmp = document.createElementNS('http://www.w3.org/2000/svg', 'g');
  // parseSVGPath is a safe helper defined below
  appendSilhouette(svgEl, silhouetteForRecipe(recipe));

  slotsEl.replaceChildren();
  for (const slot of (recipe.slots ?? [])) {
    const row = document.createElement('div');
    row.className = 'kids-assembly-slot-row' + (slot.required ? ' required-empty' : '');
    row.dataset.slotId = slot.id;

    const label = document.createElement('span');
    label.className = 'kids-assembly-slot-label';
    label.textContent = slot.name + (slot.required ? ' *' : '');

    const value = document.createElement('span');
    value.className = 'kids-assembly-slot-value';
    value.textContent = 'tap to fill';

    const chips = document.createElement('span');
    chips.className = 'kids-assembly-slot-chips';

    row.append(label, value, chips);
    row.addEventListener('click', () => openAssemblyInvPicker(slot.id, slot.accepts_tag));
    slotsEl.appendChild(row);
  }

  refreshAssemblyStats();
  refreshForgeButton();
  document.getElementById('kids-assembly-modal')!.classList.add('open');
}

// appendSilhouette sets an SVG element's content from a known-safe static string.
// The silhouette strings are defined as constants above — never from user input.
function appendSilhouette(svgEl: SVGElement, pathStr: string): void {
  const parser = new DOMParser();
  const doc = parser.parseFromString(
    `<svg xmlns="http://www.w3.org/2000/svg">${pathStr}</svg>`,
    'image/svg+xml'
  );
  const node = doc.documentElement.firstChild;
  if (node) svgEl.appendChild(svgEl.ownerDocument.importNode(node, true));
}

function closeKidsAssemblyModal(): void {
  document.getElementById('kids-assembly-modal')!.classList.remove('open');
  document.getElementById('kids-assembly-inv-picker')!.hidden = true;
}
```

- [ ] **Step 4: Add inventory picker function**

```typescript
function openAssemblyInvPicker(slotId: string, acceptsTag: string): void {
  const picker  = document.getElementById('kids-assembly-inv-picker')!;
  const list    = document.getElementById('kids-assembly-inv-list')!;
  const labelEl = document.getElementById('kids-assembly-inv-label')!;

  labelEl.textContent = 'Pick: ' + acceptsTag.replace(/-/g, ' ');
  list.replaceChildren();

  const matching = (_state?.inventory ?? []).filter(item =>
    (item.tags ?? []).includes(acceptsTag)
  );

  if (matching.length === 0) {
    const empty = document.createElement('div');
    empty.textContent = 'No matching components in inventory.';
    empty.style.cssText = 'color:#666;font-size:0.8rem';
    list.appendChild(empty);
  } else {
    for (const item of matching) {
      const el = document.createElement('div');
      el.className = 'kids-assembly-inv-item';

      if (item.quality) {
        const badge = document.createElement('span');
        badge.className = 'kids-quality-badge ' + item.quality;
        badge.textContent = item.quality;
        el.appendChild(badge);
      }

      const nameSpan = document.createElement('span');
      nameSpan.style.flex = '1';
      nameSpan.textContent = item.name;
      el.appendChild(nameSpan);

      for (const [stat, val] of Object.entries(item.stat_mods ?? {})) {
        const chip = document.createElement('span');
        chip.className = 'kids-stat-chip';
        chip.textContent = '+' + val + ' ' + stat.toUpperCase();
        el.appendChild(chip);
      }

      el.addEventListener('click', () => fillAssemblySlot(slotId, item));
      list.appendChild(el);
    }
  }

  picker.hidden = false;
}

function fillAssemblySlot(slotId: string, item: InvItem): void {
  _kidsassembly.filled[slotId] = {
    itemId:   item.id,
    itemName: item.name,
    statMods: item.stat_mods ?? {},
    quality:  item.quality ?? '',
  };

  const row = document.querySelector<HTMLElement>(`.kids-assembly-slot-row[data-slot-id="${slotId}"]`);
  if (row) {
    row.classList.remove('required-empty');
    row.classList.add('filled');
    const valueEl = row.querySelector<HTMLElement>('.kids-assembly-slot-value');
    if (valueEl) valueEl.textContent = item.name;
    const chipsEl = row.querySelector<HTMLElement>('.kids-assembly-slot-chips');
    if (chipsEl) {
      chipsEl.replaceChildren();
      for (const [stat, val] of Object.entries(item.stat_mods ?? {})) {
        const chip = document.createElement('span');
        chip.className = 'kids-stat-chip';
        chip.textContent = '+' + val + ' ' + stat.toUpperCase();
        chipsEl.appendChild(chip);
      }
    }
  }

  document.getElementById('kids-assembly-inv-picker')!.hidden = true;
  refreshAssemblyStats();
  refreshForgeButton();
}
```

- [ ] **Step 5: Add stat preview and forge button refresh functions**

```typescript
function refreshAssemblyStats(): void {
  const statsEl = document.getElementById('kids-assembly-stats')!;
  statsEl.replaceChildren();

  const totals: Record<string, number> = {};
  for (const data of Object.values(_kidsassembly.filled)) {
    for (const [stat, val] of Object.entries(data.statMods)) {
      totals[stat] = (totals[stat] ?? 0) + val;
    }
  }

  if (Object.keys(totals).length === 0) {
    const hint = document.createElement('span');
    hint.textContent = 'Fill slots to see stats';
    hint.style.cssText = 'color:#555;font-size:0.75rem';
    statsEl.appendChild(hint);
    return;
  }

  const MAX_STAT = 10;
  for (const [stat, val] of Object.entries(totals)) {
    const row = document.createElement('div');
    row.className = 'kids-stat-bar-row';

    const lbl = document.createElement('span');
    lbl.className = 'kids-stat-bar-label';
    lbl.textContent = stat;

    const track = document.createElement('div');
    track.className = 'kids-stat-bar-track';

    const fill = document.createElement('div');
    fill.className = 'kids-stat-bar-fill';
    fill.style.width = Math.min(100, (val / MAX_STAT) * 100) + '%';
    track.appendChild(fill);

    const num = document.createElement('span');
    num.style.cssText = 'font-size:0.75rem;color:#c9a84c;min-width:20px';
    num.textContent = String(val);

    row.append(lbl, track, num);
    statsEl.appendChild(row);
  }
}

function refreshForgeButton(): void {
  const recipe = _kidsassembly.recipe;
  const btn = document.getElementById('kids-assembly-forge-btn') as HTMLButtonElement | null;
  if (!recipe || !btn) return;

  const allRequiredFilled = (recipe.slots ?? [])
    .filter(s => s.required)
    .every(s => Boolean(_kidsassembly.filled[s.id]));

  btn.disabled = !allRequiredFilled;
}
```

- [ ] **Step 6: Wire event listeners**

```typescript
document.getElementById('kids-assembly-forge-btn')?.addEventListener('click', () => {
  const recipe = _kidsassembly.recipe;
  if (!recipe) return;

  const slotArgs = Object.entries(_kidsassembly.filled)
    .map(([slotId, data]) => slotId + '=' + data.itemId)
    .join(' ');

  sendCommand('craft ' + recipe.id + ' ' + slotArgs);
  closeKidsAssemblyModal();
});

document.getElementById('kids-assembly-close')?.addEventListener('click', closeKidsAssemblyModal);

document.getElementById('kids-assembly-modal')?.addEventListener('click', (e) => {
  if (e.target === e.currentTarget) closeKidsAssemblyModal();
});

document.getElementById('kids-assembly-inv-close')?.addEventListener('click', () => {
  document.getElementById('kids-assembly-inv-picker')!.hidden = true;
});
```

- [ ] **Step 7: Route recipe selection to assembly modal**

Find the recipe card click handler inside the kids recipe drawer (search for the handler that reads a recipe ID from a card click and initiates crafting). Modify it to branch on recipe type:

```typescript
// Inside the recipe drawer card click handler, find where recipeId is extracted, then:
const recipe = (_state?.recipes ?? []).find(r => r.id === recipeId);
if (recipe?.type === 'assembly') {
  closeKidsCraftModal();
  openKidsAssemblyModal(recipe);
} else {
  // existing paint grid behaviour for this recipe
  showKidsPaintRecipe(recipe ?? null); // use whatever the existing function is called
}
```

*(The exact function name for the paint grid path will be visible at the call site — do not rename it.)*

- [ ] **Step 8: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -5
```

Expected: no TypeScript errors.

- [ ] **Step 9: Commit**

```bash
git add web/src/lib/mud.ts
git commit -m "feat(kids-ui): assembly modal JS — slot picker, stat bars, forge dispatch"
```

---

## Task 9: Frontend E2E tests

**Files:**
- Create: `web/e2e/kids-assembly-modal.spec.ts`
- Create: `web/e2e/kids-assembly-workbench.spec.ts`

- [ ] **Step 1: Write `kids-assembly-modal.spec.ts`**

```typescript
import { test, expect } from '@playwright/test';

// Assumes: test server with ui_profile=kids, recipe "pipe-pistol" (assembly, required: frame+barrel),
// player inventory contains pipe-frame-crude (gun-frame) and copper-tube-crude (gun-barrel).

test.describe('Kids Assembly Modal', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('[data-ui="kids"]');
  });

  test('assembly modal opens for assembly recipe', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');

    await expect(page.locator('#kids-assembly-modal')).toHaveClass(/open/);
    await expect(page.locator('#kids-craft-modal')).not.toHaveClass(/open/);
  });

  test('required slots show red outline initially', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await expect(page.locator('.kids-assembly-slot-row[data-slot-id="frame"]')).toHaveClass(/required-empty/);
    await expect(page.locator('.kids-assembly-slot-row[data-slot-id="barrel"]')).toHaveClass(/required-empty/);
  });

  test('FORGE IT button is disabled until required slots filled', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await expect(page.locator('#kids-assembly-forge-btn')).toBeDisabled();
  });

  test('filling slots enables FORGE IT and shows stat bars', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    // Fill frame
    await page.click('.kids-assembly-slot-row[data-slot-id="frame"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    // Fill barrel
    await page.click('.kids-assembly-slot-row[data-slot-id="barrel"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await expect(page.locator('#kids-assembly-forge-btn')).toBeEnabled();
    await expect(page.locator('.kids-stat-bar-row')).toHaveCount({ min: 1 });
  });

  test('FORGE IT sends craft command with slot args and closes modal', async ({ page }) => {
    const sentMessages: string[] = [];
    page.on('websocket', ws => {
      ws.on('framesent', frame => {
        if (typeof frame.payload === 'string') sentMessages.push(frame.payload);
      });
    });

    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await page.click('.kids-assembly-slot-row[data-slot-id="frame"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await page.click('.kids-assembly-slot-row[data-slot-id="barrel"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await page.click('#kids-assembly-forge-btn');

    await expect(page.locator('#kids-assembly-modal')).not.toHaveClass(/open/);

    const craftMsg = sentMessages.find(m => m.includes('craft pipe-pistol') && m.includes('frame=') && m.includes('barrel='));
    expect(craftMsg).toBeTruthy();
  });

  test('close button dismisses modal', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await page.click('#kids-assembly-close');
    await expect(page.locator('#kids-assembly-modal')).not.toHaveClass(/open/);
  });
});
```

- [ ] **Step 2: Write `kids-assembly-workbench.spec.ts`**

```typescript
import { test, expect } from '@playwright/test';

// Assumes: player starts outside a ruins-workshop room (no scrap-forge present).

test.describe('Kids Assembly Workbench Gate', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('[data-ui="kids"]');
  });

  test('server rejects forge attempt when not at scrap-forge', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await page.click('.kids-assembly-slot-row[data-slot-id="frame"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await page.click('.kids-assembly-slot-row[data-slot-id="barrel"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await page.click('#kids-assembly-forge-btn');

    await expect(page.locator('#output')).toContainText('requires a scrap-forge', { timeout: 3000 });
  });
});
```

- [ ] **Step 3: Run E2E tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run test:e2e -- --grep "Kids Assembly" 2>&1 | tail -20
```

Expected: tests compile and run; UI-only tests pass against a live server; workbench test depends on server state.

- [ ] **Step 4: Commit**

```bash
git add web/e2e/kids-assembly-modal.spec.ts web/e2e/kids-assembly-workbench.spec.ts
git commit -m "test(kids-ui): E2E tests for assembly modal — slots, forge, workbench gate"
```

---

## Task 10: Blockhaven YAML — ruins-workshop rooms

**Files:**
- Modify: `internal/world/defaults/blockhaven/world.yaml`

- [ ] **Step 1: Add one ruins-workshop room per biome**

In the `rooms:` section of `world.yaml`, add these 5 rooms:

```yaml
  - id: ruins-workshop-meadow
    name: Ruins Workshop
    biome: meadow
    desc: >
      Crumbling stone walls frame a surprisingly intact workshop. Ancient tools hang
      from rusted pegs. A Scrap Forge glows faintly in the corner beside a battered
      iron Anvil.
    workbench_types: [scrap-forge, anvil]
    exits:
      south: meadow-0
    npcs: []
    items: []
    resources: []

  - id: ruins-workshop-forest
    name: Ruins Workshop
    biome: forest
    desc: >
      Vines have grown through the cracked windows but left the forge and anvil
      untouched — as if the forest respects what is made here.
    workbench_types: [scrap-forge, anvil]
    exits:
      south: forest-0
    npcs: []
    items: []
    resources: []

  - id: ruins-workshop-desert
    name: Ruins Workshop
    biome: desert
    desc: >
      Sand coats every surface but the forge is still hot — heated by a buried
      crystal vein. The anvil is scarred but solid.
    workbench_types: [scrap-forge, anvil]
    exits:
      south: desert-0
    npcs: []
    items: []
    resources: []

  - id: ruins-workshop-snow
    name: Ruins Workshop
    biome: snow
    desc: >
      Frost clings to the walls but the forge melts it away in a small circle
      of warmth. The anvil rings clearly in the cold air.
    workbench_types: [scrap-forge, anvil]
    exits:
      south: snow-0
    npcs: []
    items: []
    resources: []

  - id: ruins-workshop-caves
    name: Ruins Workshop
    biome: caves
    desc: >
      Deep in the cave network, this workshop was hidden from the surface world.
      The forge burns with a blue-tinged flame. The anvil is carved from deepstone.
    workbench_types: [scrap-forge, anvil]
    exits:
      north: caves-0
    npcs: []
    items: []
    resources: []
```

- [ ] **Step 2: Add exits from biome entrance rooms to their workshops**

Find rooms `meadow-0`, `forest-0`, `desert-0`, `snow-0`, `caves-0`. In each, add an exit pointing to the corresponding workshop:

```yaml
# In meadow-0 exits: add
north: ruins-workshop-meadow

# In forest-0 exits: add
north: ruins-workshop-forest

# In desert-0 exits: add
north: ruins-workshop-desert

# In snow-0 exits: add
north: ruins-workshop-snow

# In caves-0 exits: add
south: ruins-workshop-caves
```

- [ ] **Step 3: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./... && echo "OK"
```

- [ ] **Step 4: Commit**

```bash
git add internal/world/defaults/blockhaven/world.yaml
git commit -m "feat(blockhaven): add ruins-workshop rooms with scrap-forge and anvil workbench_types"
```

---

## Task 11: Blockhaven YAML — component items

**Files:**
- Modify: `internal/world/defaults/blockhaven/world.yaml`

- [ ] **Step 1: Add a starter cache of components to ruins-workshop-meadow**

Under `ruins-workshop-meadow`, set:

```yaml
    items:
      - id: pipe-frame-crude
        name: Pipe Frame
        desc: A rough length of iron pipe bent into a gun frame shape.
        tags: [component, gun-frame]
        quality: crude
        stat_mods: {}
      - id: copper-tube-crude
        name: Copper Tube
        desc: A salvaged copper pipe. Could serve as a gun barrel.
        tags: [component, gun-barrel]
        quality: crude
        stat_mods: {damage: 1, range: 1}
      - id: wrapped-grip-crude
        name: Wrapped Grip
        desc: Leather strips wound around a piece of wood.
        tags: [component, gun-stock]
        quality: crude
        stat_mods: {range: 1}
```

- [ ] **Step 2: Add gun components to loot tables**

Find or create a surface-enemy loot table. Add gun component entries:

```yaml
  - id: surface-enemy-loot
    entries:
      - item_id: pipe-frame-crude
        item_name: Pipe Frame
        item_desc: A rough iron pipe frame.
        tags: [component, gun-frame]
        quality: crude
        stat_mods: {}
        weight: 20
      - item_id: pipe-frame-standard
        item_name: Pipe Frame
        item_desc: A well-bent iron pipe frame.
        tags: [component, gun-frame]
        quality: standard
        stat_mods: {}
        weight: 8
      - item_id: copper-tube-crude
        item_name: Copper Tube
        item_desc: A salvaged copper pipe for gun barrels.
        tags: [component, gun-barrel]
        quality: crude
        stat_mods: {damage: 1, range: 1}
        weight: 20
      - item_id: copper-tube-standard
        item_name: Copper Tube
        item_desc: A clean copper pipe, well-suited for a barrel.
        tags: [component, gun-barrel]
        quality: standard
        stat_mods: {damage: 2, range: 2}
        weight: 8
      - item_id: wrapped-grip-crude
        item_name: Wrapped Grip
        item_desc: Leather-wrapped wood grip.
        tags: [component, gun-stock]
        quality: crude
        stat_mods: {range: 1}
        weight: 25
      - item_id: carved-handle-standard
        item_name: Carved Handle
        item_desc: A smooth carved wooden handle.
        tags: [component, gun-stock]
        quality: standard
        stat_mods: {range: 2}
        weight: 10
```

Add remaining gun components (iron-sights-crude, iron-sights-standard, crystal-lens-refined, coal-igniter-standard, drum-cylinder-standard, drum-cylinder-refined, bone-stabilizer-refined, rifled-pipe-standard, reinforced-barrel-refined, iron-receiver-standard, iron-receiver-refined) to appropriate loot tables (dungeon/cave/boss) following the same pattern.

- [ ] **Step 3: Add armor components to faction loot tables**

Add to the Stoneguard faction enemy loot table:

```yaml
      - item_id: leather-plate-crude
        item_name: Leather Plate
        item_desc: A piece of cured leather for chest armor.
        tags: [component, armor-chest]
        quality: crude
        stat_mods: {defense: 1}
        weight: 30
      - item_id: iron-sheet-standard
        item_name: Iron Sheet
        item_desc: A flat iron plate, hammered smooth.
        tags: [component, armor-chest]
        quality: standard
        stat_mods: {defense: 3, weight: 1}
        weight: 15
      - item_id: chain-links-standard
        item_name: Chain Links
        item_desc: Interlocked iron rings for shoulder protection.
        tags: [component, armor-shoulder]
        quality: standard
        stat_mods: {defense: 2}
        weight: 20
      - item_id: iron-gauntlets-standard
        item_name: Iron Gauntlets
        item_desc: Solid iron hand guards.
        tags: [component, armor-gauntlets]
        quality: standard
        stat_mods: {defense: 1, strength: 1}
        weight: 15
```

Add remaining armor components (hide-pad-crude, carved-pauldron-refined, woven-moss-crude, spider-silk-standard, ember-cloth-refined, bone-helm-standard, fur-boots-crude, fur-boots-standard, desert-veil-standard, shadow-hood-refined, deepstone-slab-refined, iron-gauntlets-refined) to appropriate faction loot tables (thornwalker, dunekeeper, frostborn, deepborn).

- [ ] **Step 4: Build**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./... && echo "OK"
```

- [ ] **Step 5: Commit**

```bash
git add internal/world/defaults/blockhaven/world.yaml
git commit -m "feat(blockhaven): add gun and armor component items to rooms and loot tables"
```

---

## Task 12: Blockhaven YAML — assembly and ingredient recipes

**Files:**
- Modify: `internal/world/defaults/blockhaven/world.yaml`

- [ ] **Step 1: Add gun assembly recipes to `crafting_recipes:`**

```yaml
  - id: pipe-pistol
    name: Pipe Pistol
    type: assembly
    workbench: scrap-forge
    skill_req: 1
    output:
      id: pipe-pistol
      name: Pipe Pistol
      desc: A crude but functional scavenged pistol.
    slots:
      - id: frame
        name: Frame
        required: true
        accepts_tag: gun-frame
      - id: barrel
        name: Barrel
        required: true
        accepts_tag: gun-barrel
      - id: grip
        name: Grip
        required: false
        accepts_tag: gun-stock

  - id: bolt-rifle
    name: Bolt Rifle
    type: assembly
    workbench: scrap-forge
    skill_req: 3
    output:
      id: bolt-rifle
      name: Bolt Rifle
      desc: A mid-range rifle with a manual bolt action.
    slots:
      - id: frame
        name: Frame
        required: true
        accepts_tag: gun-frame
      - id: barrel
        name: Barrel
        required: true
        accepts_tag: gun-barrel
      - id: stock
        name: Stock
        required: true
        accepts_tag: gun-stock
      - id: sight
        name: Sight
        required: false
        accepts_tag: gun-sight

  - id: scatter-cannon
    name: Scatter Cannon
    type: assembly
    workbench: scrap-forge
    skill_req: 3
    output:
      id: scatter-cannon
      name: Scatter Cannon
      desc: Wide spread, short range. Great for crowds.
    slots:
      - id: frame
        name: Frame
        required: true
        accepts_tag: gun-frame
      - id: barrel
        name: Barrel
        required: true
        accepts_tag: gun-barrel
      - id: choke
        name: Choke
        required: false
        accepts_tag: gun-barrel
      - id: brace
        name: Brace
        required: false
        accepts_tag: gun-stock

  - id: flare-launcher
    name: Flare Launcher
    type: assembly
    workbench: scrap-forge
    skill_req: 4
    output:
      id: flare-launcher
      name: Flare Launcher
      desc: Lights up caves and sets things on fire.
    slots:
      - id: frame
        name: Frame
        required: true
        accepts_tag: gun-frame
      - id: barrel
        name: Barrel
        required: true
        accepts_tag: gun-barrel
      - id: igniter
        name: Igniter
        required: true
        accepts_tag: gun-igniter

  - id: twin-barrel
    name: Twin Barrel
    type: assembly
    workbench: scrap-forge
    skill_req: 4
    output:
      id: twin-barrel
      name: Twin Barrel
      desc: Two barrels side by side. Double the trouble.
    slots:
      - id: frame
        name: Frame
        required: true
        accepts_tag: gun-frame
      - id: barrel-left
        name: Left Barrel
        required: true
        accepts_tag: gun-barrel
      - id: barrel-right
        name: Right Barrel
        required: true
        accepts_tag: gun-barrel
      - id: grip
        name: Grip
        required: false
        accepts_tag: gun-stock
      - id: trigger
        name: Trigger
        required: false
        accepts_tag: gun-stock

  - id: bone-sniper
    name: Bone Sniper
    type: assembly
    workbench: scrap-forge
    skill_req: 5
    output:
      id: bone-sniper
      name: Bone Sniper
      desc: A long-range precision rifle carved from deepbone and salvaged steel.
    slots:
      - id: frame
        name: Frame
        required: true
        accepts_tag: gun-frame
      - id: barrel
        name: Barrel
        required: true
        accepts_tag: gun-barrel
      - id: stock
        name: Stock
        required: true
        accepts_tag: gun-stock
      - id: sight
        name: Sight
        required: true
        accepts_tag: gun-sight
      - id: stabilizer
        name: Stabilizer
        required: false
        accepts_tag: gun-stabilizer

  - id: vault-repeater
    name: Vault Repeater
    type: assembly
    workbench: scrap-forge
    skill_req: 7
    output:
      id: vault-repeater
      name: Vault Repeater
      desc: An ancient rapid-fire design. Whoever built this knew what they were doing.
    slots:
      - id: frame
        name: Frame
        required: true
        accepts_tag: gun-frame
      - id: barrel
        name: Barrel
        required: true
        accepts_tag: gun-barrel
      - id: drum
        name: Drum
        required: true
        accepts_tag: gun-drum
      - id: stock
        name: Stock
        required: true
        accepts_tag: gun-stock
      - id: sight
        name: Sight
        required: false
        accepts_tag: gun-sight
```

- [ ] **Step 2: Add armor assembly recipes**

```yaml
  - id: scrap-vest
    name: Scrap Vest
    type: assembly
    workbench: anvil
    skill_req: 1
    output:
      id: scrap-vest
      name: Scrap Vest
      desc: Bits of metal and leather stitched together. Better than nothing.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: shoulder
        name: Shoulder Pad
        required: false
        accepts_tag: armor-shoulder
      - id: lining
        name: Lining
        required: false
        accepts_tag: armor-lining

  - id: plate-coat
    name: Plate Coat
    type: assembly
    workbench: anvil
    skill_req: 2
    output:
      id: plate-coat
      name: Plate Coat
      desc: A layered coat of iron plates. Heavier, but reliable.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: shoulder
        name: Shoulder Pad
        required: true
        accepts_tag: armor-shoulder
      - id: gauntlets
        name: Gauntlets
        required: false
        accepts_tag: armor-gauntlets
      - id: boots
        name: Boots
        required: false
        accepts_tag: armor-boots

  - id: thornwalker-leathers
    name: Thornwalker Leathers
    type: assembly
    workbench: anvil
    skill_req: 3
    output:
      id: thornwalker-leathers
      name: Thornwalker Leathers
      desc: Supple forest leather treated with thorn-oil. Light and quiet.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: lining
        name: Lining
        required: true
        accepts_tag: armor-lining
      - id: shoulder
        name: Shoulder Pad
        required: false
        accepts_tag: armor-shoulder
      - id: hood
        name: Hood
        required: false
        accepts_tag: armor-hood

  - id: stoneguard-shell
    name: Stoneguard Shell
    type: assembly
    workbench: anvil
    skill_req: 4
    output:
      id: stoneguard-shell
      name: Stoneguard Shell
      desc: Full plate in the Stoneguard tradition. Heavy, loud, impenetrable.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: shoulder
        name: Shoulder Pad
        required: true
        accepts_tag: armor-shoulder
      - id: gauntlets
        name: Gauntlets
        required: true
        accepts_tag: armor-gauntlets
      - id: boots
        name: Boots
        required: true
        accepts_tag: armor-boots

  - id: dunekeepers-wrap
    name: Dunekeepers Wrap
    type: assembly
    workbench: anvil
    skill_req: 3
    output:
      id: dunekeepers-wrap
      name: Dunekeepers Wrap
      desc: Layered cloth and treated hide. Keeps the heat and sand out.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: lining
        name: Lining
        required: true
        accepts_tag: armor-lining
      - id: veil
        name: Veil
        required: false
        accepts_tag: armor-veil

  - id: frostborn-parka
    name: Frostborn Parka
    type: assembly
    workbench: anvil
    skill_req: 4
    output:
      id: frostborn-parka
      name: Frostborn Parka
      desc: Thick fur and layered hide, lined with ember cloth for warmth.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: shoulder
        name: Shoulder Pad
        required: true
        accepts_tag: armor-shoulder
      - id: lining
        name: Lining
        required: true
        accepts_tag: armor-lining
      - id: boots
        name: Boots
        required: false
        accepts_tag: armor-boots

  - id: deepborn-suit
    name: Deepborn Suit
    type: assembly
    workbench: anvil
    skill_req: 5
    output:
      id: deepborn-suit
      name: Deepborn Suit
      desc: Dark armour from the deep. Absorbs light and sound.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: shoulder
        name: Shoulder Pad
        required: true
        accepts_tag: armor-shoulder
      - id: lining
        name: Lining
        required: true
        accepts_tag: armor-lining
      - id: helm
        name: Helm
        required: false
        accepts_tag: armor-helm

  - id: ruin-exosuit
    name: Ruin Exosuit
    type: assembly
    workbench: anvil
    skill_req: 8
    output:
      id: ruin-exosuit
      name: Ruin Exosuit
      desc: A legendary pre-collapse powered suit. Needs the right parts to finish it.
    slots:
      - id: chest
        name: Chest Plate
        required: true
        accepts_tag: armor-chest
      - id: shoulder
        name: Shoulder Pad
        required: true
        accepts_tag: armor-shoulder
      - id: gauntlets
        name: Gauntlets
        required: true
        accepts_tag: armor-gauntlets
      - id: boots
        name: Boots
        required: true
        accepts_tag: armor-boots
      - id: helm
        name: Helm
        required: true
        accepts_tag: armor-helm
```

- [ ] **Step 3: Add new ingredient recipes**

```yaml
  - id: medkit
    name: Medkit
    type: ingredient
    output:
      id: medkit
      name: Medkit
      desc: A field kit of bandages and cave herbs. Restores health.
    ingredients:
      - id: spider-silk
        count: 1
      - id: ember-cloth
        count: 1
      - id: cave-moss
        count: 1

  - id: smoke-bomb
    name: Smoke Bomb
    type: ingredient
    output:
      id: smoke-bomb
      name: Smoke Bomb
      desc: Throw it to create a blinding cloud. Good for escaping fights.
    ingredients:
      - id: coal-dust
        count: 1
      - id: woven-moss
        count: 1
      - id: copper-tube-crude
        count: 1

  - id: lockpick-set
    name: Lockpick Set
    type: ingredient
    output:
      id: lockpick-set
      name: Lockpick Set
      desc: A set of thin metal picks. Opens most simple locks.
    ingredients:
      - id: wire
        count: 2
      - id: bone-shard
        count: 1

  - id: grapple-hook
    name: Grapple Hook
    type: ingredient
    output:
      id: grapple-hook
      name: Grapple Hook
      desc: A clawed hook on a rope. Reach high places, escape pits.
    ingredients:
      - id: iron-chain
        count: 1
      - id: carved-handle-standard
        count: 1
      - id: rope
        count: 1

  - id: faction-token
    name: Faction Token
    type: ingredient
    output:
      id: faction-token
      name: Faction Token
      desc: A crafted symbol of faction loyalty. Trade it for reputation or supplies.
    ingredients:
      - id: faction-ore
        count: 3
      - id: deepstone-shard
        count: 1

  - id: ancient-battery
    name: Ancient Battery
    type: ingredient
    output:
      id: ancient-battery
      name: Ancient Battery
      desc: >
        A reconstructed power cell from the old world. It hums with energy.
        Something clicks in your mind — you now understand how ancient guns were built.
      unlocks_flag: gun_recipes_unlocked
    ingredients:
      - id: copper
        count: 2
      - id: iron
        count: 1
      - id: crystal-lens-crude
        count: 1
```

- [ ] **Step 4: Build and run all tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./... && go test ./... 2>&1 | tail -15
```

Expected: build succeeds, all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/world/defaults/blockhaven/world.yaml
git commit -m "feat(blockhaven): add gun/armor assembly recipes and ingredient recipes"
```

---

## Self-Review Notes

- `craftAssemble` calls `w.FindItem(itemID)` — `FindItem` added to `world.World` in Task 3 Step 5
- Both `craftIngredient` and `craftAssemble` use `roomHasWorkbench` — old `room.ID` check removed from both
- `UnlocksFlag` propagated from `out.UnlocksFlag` in both paths
- `player_flags` table in schema (Task 2) exists before `SetPlayerFlag` is called (Task 3/4)
- All 5 existing crafting tests updated to pass `nil` as `slots` (Task 3 Step 3)
- Gun recipe filter in session.go handles ingredient-type recipes correctly (no slots → inner loop doesn't fire → not filtered)
- `ancient-battery` output has `unlocks_flag: gun_recipes_unlocked` — matches the exact string checked by `IsPlayerFlagSet`
- Frontend `InvItem` extended with `tags`, `stat_mods`, `quality` before assembly picker uses them (Task 6 Step 3)
- SVG silhouette is built from static constants via DOMParser — no user data flows into SVG construction
