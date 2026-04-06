# Mudout Class System & Onboarding — Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a 5-class system, point-buy starting kit, mentor-led onboarding journey, and faction-rep gating to the mudout world.

**Architecture:** Three additive layers on existing infrastructure.
1. **Data layer** — extend `player` schema with `class` column, add `AdjustReputation` to gamedb, extend `WorldQuest` struct + DB schema with `giver_faction`/`min_rep`/`reward_rep_*` fields.
2. **Game logic** — new `internal/classes` package (registry, kit lists, signature verbs); new `internal/commands/character.go` (creation wizard); rep-gate check in existing dialogue-driven quest accept path.
3. **World content** — `wakeup-camp` room + 5 mentor NPCs + `ghoul-intro` quest in `internal/world/defaults/mudout/world.yaml`; tag existing Settler quests with `giver_faction: settlers, min_rep: 0`; change `start_room` to `wakeup-camp`.

**Tech Stack:** Go 1.22+, SQLite (also Postgres dual-path), YAML world definitions, embedded world FS.

**Reference spec:** `docs/superpowers/specs/2026-04-06-mudout-classes-design.md`

**Conventions for this plan:**
- The user has explicit feedback to **skip review loops during implementation**. This plan favors small, atomic commits and a build-pass gate over a write-test-first cycle. Tests are added only at boundaries where the cost of regression is high (class registry, rep math, kit point validation).
- Every task ends with `go build ./...` passing and a focused commit.
- Run from repo root: `/Users/stokes/Projects/gl1tch-mud`.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/db/schema.go` | modify | add `class` column to `player`, `giver_faction`/`min_rep`/`reward_rep_faction`/`reward_rep_delta` to `quests` |
| `internal/db/gamedb/gamedb.go` | modify | add `AdjustReputation`, extend player & quest load/save |
| `internal/db/sqliteq/general.sql.go` *(if used)* | modify only if existing player/quest funcs read those columns | match new columns |
| `internal/player/player.go` | modify | add `Class` field on `State`, load/save it |
| `internal/classes/classes.go` | create | class registry: definitions, kit lists, signature verbs, faction rep deltas |
| `internal/classes/classes_test.go` | create | sanity tests for kit budgets and verb gating |
| `internal/commands/character.go` | create | char-creation wizard (`name`, `class`, `kit`, `start`); fires when `s.Class == ""` |
| `internal/commands/signatures.go` | create | the six signature verbs (`fan`, `hotwire`, `barter`, `lift`, `stim`, `rad-feed`) |
| `internal/commands/commands.go` | modify | (1) wire char wizard intercept; (2) rep gate in talk/quest-accept; (3) reward rep on quest complete; (4) register signature verbs |
| `internal/world/world.go` | modify | add `GiverFaction`, `MinRep`, `RewardRepFaction`, `RewardRepDelta` to `WorldQuest` |
| `internal/quests/quests.go` | modify | mirror new fields on `Quest` + record conversion |
| `internal/world/defaults/mudout/world.yaml` | modify | add `wakeup-camp`, 5 mentors, `glowing-relic`, `ghoul-intro`; tag Settler quests; flip `start_room` |

---

## Task 1: Schema — add `player.class` and quest rep columns

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1:** Add `class` column to the `player` table definition. Edit lines around 4-11 in `internal/db/schema.go` so the table reads:

```sql
CREATE TABLE IF NOT EXISTS player (
    id        INTEGER PRIMARY KEY,
    name      TEXT    NOT NULL DEFAULT 'hacker',
    room_id   TEXT    NOT NULL DEFAULT 'net-0',
    hp        INTEGER NOT NULL DEFAULT 100,
    max_hp    INTEGER NOT NULL DEFAULT 100,
    world     TEXT    NOT NULL DEFAULT 'cyberspace',
    class     TEXT    NOT NULL DEFAULT ''
);
```

- [ ] **Step 2:** Add four columns to the `quests` table definition (around lines 90-109). Add after `next_quest_id`:

```sql
    giver_faction      TEXT    NOT NULL DEFAULT '',
    min_rep            INTEGER NOT NULL DEFAULT 0,
    reward_rep_faction TEXT    NOT NULL DEFAULT '',
    reward_rep_delta   INTEGER NOT NULL DEFAULT 0
```

- [ ] **Step 3:** Append a self-contained idempotent migration block at the bottom of the `schema` const so existing DBs get the new columns:

```sql
-- Idempotent column adds (SQLite has no IF NOT EXISTS for columns; wrap with PRAGMA check via Go elsewhere if needed)
```

Actually SQLite cannot do `ADD COLUMN IF NOT EXISTS`. Instead, after the schema const, locate the function in `internal/db/db.go` (or wherever `schema` is executed) and add a runtime helper that runs each `ALTER TABLE ... ADD COLUMN ...` inside a recover-from-error block:

```go
// In the same file as the schema apply call:
addColumnIfMissing := func(table, col, ddl string) {
    var name string
    row := db.QueryRow(
        `SELECT name FROM pragma_table_info(?) WHERE name=?`, table, col,
    )
    if row.Scan(&name) == sql.ErrNoRows {
        _, _ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, ddl))
    }
}
addColumnIfMissing("player", "class", "class TEXT NOT NULL DEFAULT ''")
addColumnIfMissing("quests", "giver_faction", "giver_faction TEXT NOT NULL DEFAULT ''")
addColumnIfMissing("quests", "min_rep", "min_rep INTEGER NOT NULL DEFAULT 0")
addColumnIfMissing("quests", "reward_rep_faction", "reward_rep_faction TEXT NOT NULL DEFAULT ''")
addColumnIfMissing("quests", "reward_rep_delta", "reward_rep_delta INTEGER NOT NULL DEFAULT 0")
```

(Place this in the same function that runs `db.Exec(schema)`. Search `internal/db/db.go` for where `schema` is referenced.)

- [ ] **Step 4:** Run `go build ./...`. Expected: PASS.

- [ ] **Step 5:** Commit.

```bash
git add internal/db/schema.go internal/db/db.go
git commit -m "feat(db): add player.class and quest rep-gate columns"
```

---

## Task 2: gamedb — `AdjustReputation` and player class load/save

**Files:**
- Modify: `internal/db/gamedb/gamedb.go`

- [ ] **Step 1:** Locate the existing `IncrementReputation` function (around line 2151). Add `AdjustReputation` directly below it. Both sqlite and postgres paths must be implemented:

```go
// AdjustReputation adds an arbitrary signed delta to a faction's reputation.
// Used by character creation, quest rewards, and any future reputation event.
func (g *GameDB) AdjustReputation(ctx context.Context, faction string, delta int) error {
    if g.pg != nil {
        current := g.GetReputation(ctx, faction)
        return g.pg.UpsertSharedReputation(ctx, pgq.UpsertSharedReputationParams{
            AccountID: g.pgUUID(),
            WorldID:   g.worldID,
            Faction:   faction,
            Value:     int32(current + delta),
        })
    }
    _, err := g.sqliteDB.Exec(
        `INSERT INTO player_reputation (faction, value) VALUES (?, ?)
         ON CONFLICT(faction) DO UPDATE SET value = value + excluded.value`,
        faction, delta,
    )
    return err
}
```

- [ ] **Step 2:** Find `GetPlayer` and `SeedPlayer` and `SavePlayer` (search for those names in the same file). They currently return/take `(roomID, hp, maxHP, world)`. Extend each to also return/take `class string`:

  - `GetPlayer` returns `(roomID, hp, maxHP, world, class string, err)`
  - `SeedPlayer(ctx, name, roomID, hp, maxHP, world, class string)`
  - `SavePlayer(ctx, roomID, hp, maxHP, world, class string)`

  Update the SQL to include the new column. For sqlite path, change the SELECT to `SELECT room_id, hp, max_hp, world, class FROM player WHERE id=1` and the INSERT/UPDATE to include `class`. Apply the same shape change to the postgres path (it likely uses sqlc-generated code; if there's no postgres player table support, leave a `// TODO: pg player class` comment and proceed — the project's primary path is sqlite).

- [ ] **Step 3:** Run `go build ./...`. Fix any compile errors in callers (the only callers are in `internal/player/player.go`, which Task 3 updates).

  If the build fails because `internal/player/player.go` callers don't yet pass the new arg, that's expected. Move on to Task 3 — they get fixed together.

- [ ] **Step 4:** Don't commit yet; commit at the end of Task 3 with the player.go changes so the build is green.

---

## Task 3: player.State.Class

**Files:**
- Modify: `internal/player/player.go`

- [ ] **Step 1:** Add `Class string` to the `State` struct (after `Defense int`):

```go
type State struct {
    PlayerID string
    Role     string
    Name     string
    RoomID   string
    HP       int
    MaxHP    int
    World    string
    Defense  int
    Class    string
}
```

- [ ] **Step 2:** Update `LoadForWorld` to read `class` from `gdb.GetPlayer` and seed it as empty on first run:

```go
func LoadForWorld(gdb *gamedb.GameDB, worldName, startRoom string) (*State, error) {
    ctx := context.Background()
    roomID, hp, maxHP, world, class, err := gdb.GetPlayer(ctx)
    if err == sql.ErrNoRows {
        s := &State{Name: "hacker", RoomID: startRoom, HP: 100, MaxHP: 100, World: worldName, Class: ""}
        if err := gdb.SeedPlayer(ctx, s.Name, s.RoomID, s.HP, s.MaxHP, s.World, s.Class); err != nil {
            return nil, fmt.Errorf("player: seed: %w", err)
        }
        return s, nil
    }
    if err != nil {
        return nil, fmt.Errorf("player: load: %w", err)
    }
    return &State{
        Name:   "hacker",
        RoomID: roomID,
        HP:     hp,
        MaxHP:  maxHP,
        World:  world,
        Class:  class,
    }, nil
}
```

- [ ] **Step 3:** Update `Save`:

```go
func Save(gdb *gamedb.GameDB, s *State) error {
    return gdb.SavePlayer(context.Background(), s.RoomID, s.HP, s.MaxHP, s.World, s.Class)
}
```

- [ ] **Step 4:** Run `go build ./...`. Expected: PASS.

- [ ] **Step 5:** Run `go test ./internal/player/... ./internal/db/...`. If any test fails because of the new arg, update the test fixtures to pass an empty string for `class`. Expected: PASS.

- [ ] **Step 6:** Commit.

```bash
git add internal/db/gamedb/gamedb.go internal/player/player.go internal/player/player_test.go
git commit -m "feat(player): persist class field; add gamedb.AdjustReputation"
```

---

## Task 4: Class registry package

**Files:**
- Create: `internal/classes/classes.go`
- Create: `internal/classes/classes_test.go`

- [ ] **Step 1:** Create `internal/classes/classes.go` with the full registry. This file is the single source of truth for class data — kit lists, signature verbs, skill bonuses, faction rep deltas. Use plain Go data, no DB.

```go
// Package classes defines the mudout character classes: their starting kits,
// signature verbs, skill bonuses, and faction-rep deltas applied at creation.
package classes

// KitItem is one entry in a class's starting kit list.
type KitItem struct {
    ID    string // inventory item ID
    Name  string // display name
    Desc  string // inventory description
    Cost  int    // kit-point cost (0 = free hook item)
}

// SkillBonus grants a starting skill level boost at character creation.
type SkillBonus struct {
    Skill string
    Level int
}

// RepDelta is a starting faction reputation adjustment.
type RepDelta struct {
    Faction string
    Delta   int
}

// Class is a complete class definition.
type Class struct {
    ID            string       // canonical lowercase id ("gunslinger", "ghoul", etc.)
    Name          string       // display name
    Tagline       string       // one-line flavor
    Flavor        string       // paragraph shown during creation
    MentorNPCID   string       // NPC id of the mentor in wakeup-camp
    SignatureVerb string       // command verb (e.g. "fan", "hotwire")
    GatingSkill   string       // skill that unlocks the verb for other classes at level 3
    SkillBonuses  []SkillBonus // starting skill levels
    StartingRep   []RepDelta   // starting reputation deltas
    Kit           []KitItem    // available kit items (player picks <= 10 cost)
}

// KitBudget is the total kit points each class gets to spend at creation.
const KitBudget = 10

// SignatureVerbUnlockLevel is the gating-skill level at which any class
// can use any class's signature verb.
const SignatureVerbUnlockLevel = 3

// All returns the registry of every class in load order.
func All() []Class {
    return []Class{gunslinger, mechanic, scavver, medic, ghoul}
}

// ByID returns the class with the given id, or nil if not found.
func ByID(id string) *Class {
    for i := range registryByID {
        if registryByID[i].ID == id {
            return &registryByID[i]
        }
    }
    return nil
}

var registryByID = All()

// ─── Class definitions ───────────────────────────────────────────────────

var gunslinger = Class{
    ID:            "gunslinger",
    Name:          "Gunslinger",
    Tagline:       "Old-world iron in a chrome world.",
    Flavor:        "You came up on stories of the Drift — lone shooters with nothing but a code and a six-gun. The wasteland chewed up the romance, but the iron still works.",
    MentorNPCID:   "old-cass",
    SignatureVerb: "fan",
    GatingSkill:   "combat",
    SkillBonuses:  []SkillBonus{{"combat", 1}},
    StartingRep:   nil,
    Kit: []KitItem{
        {"worn-revolver", "Worn Revolver", "Six-shot, walnut grip, fired enough to know it.", 3},
        {"lever-rifle", "Lever Rifle", "Long iron, slow but mean.", 5},
        {"speedloader-pair", "Speedloader x2", "Cuts your reload in half.", 1},
        {"patched-duster", "Patched Duster", "Heavy canvas. +2 defense.", 2},
        {"tin-star", "Tin Star Badge", "Bluffs Settler NPCs.", 1},
        {"whiskey-flask", "Whiskey Flask", "Heal 10 HP, 3 charges.", 1},
        {"bone-knife", "Bone-handle Knife", "1d4 melee.", 2},
        {"trail-rations", "Trail Rations", "Heal 5 HP, 5 charges.", 1},
        {"lucky-bullet", "Lucky Bullet", "One guaranteed crit.", 4},
        {"faded-photograph", "Faded Photograph", "A woman, a porch, a long time ago.", 0},
    },
}

var mechanic = Class{
    ID:            "mechanic",
    Name:          "Mechanic",
    Tagline:       "Keeps dead tech alive.",
    Flavor:        "Other people see junk. You see a power coupling, a working actuator, a generator that just needs a kick. The wasteland is full of dead things waiting to be coaxed back.",
    MentorNPCID:   "rust",
    SignatureVerb: "hotwire",
    GatingSkill:   "crafting",
    SkillBonuses:  []SkillBonus{{"crafting", 1}},
    Kit: []KitItem{
        {"pipe-pistol", "Pipe Pistol", "1d6, sidearm.", 3},
        {"wrench", "Wrench", "1d6 melee, doubles as crafting tool.", 2},
        {"welding-mask", "Welding Mask", "+1 defense, immune to flash.", 2},
        {"toolkit", "Toolkit", "Required for hotwiring advanced systems.", 3},
        {"scrap-metal-5", "Scrap Metal x5", "Crafting reagent.", 1},
        {"pipe-parts-3", "Pipe Parts x3", "Crafting reagent.", 1},
        {"spare-battery", "Spare Battery", "Powers hotwire once without parts.", 2},
        {"dust-goggles", "Dust Goggles", "+1 perception in dust storms.", 1},
        {"jury-rig-kit", "Jury-Rig Kit", "One field repair on a broken weapon.", 4},
        {"burnt-service-medal", "Burnt Service Medal", "Brass, soot-blackened, no name.", 0},
    },
}

var scavver = Class{
    ID:            "scavver",
    Name:          "Scavver",
    Tagline:       "Gets things. Doesn't ask how.",
    Flavor:        "Barter, lift, find. There's no lock you can't talk past, no crowd you can't disappear into, no junk pile that hasn't got something worth flipping.",
    MentorNPCID:   "halftrack",
    SignatureVerb: "barter",
    GatingSkill:   "trading",
    SkillBonuses:  []SkillBonus{{"scavenging", 1}, {"trading", 1}},
    Kit: []KitItem{
        {"sawed-off", "Sawed-Off Shotgun", "1d8 point-blank.", 4},
        {"switchblade", "Switchblade", "1d4 melee, +1 to lift.", 2},
        {"lockpicks-3", "Lockpicks x3", "For physical locks.", 1},
        {"fake-creds", "Fake Creds", "Bluffs Ironclad NPCs once.", 2},
        {"lucky-charm", "Lucky Charm", "+1 to first lift per day.", 2},
        {"big-satchel", "Big Satchel", "+5 inventory slots.", 1},
        {"rad-x-2", "Rad-X x2", "Temporary rad resistance.", 1},
        {"caps-50", "Bottle Caps x50", "Starting currency.", 2},
        {"smoke-pellet", "Smoke Pellet", "Auto-escape one combat encounter.", 3},
        {"pawned-ring", "Pawned Wedding Ring", "You meant to come back for it.", 0},
    },
}

var medic = Class{
    ID:            "medic",
    Name:          "Medic",
    Tagline:       "Chems, bandages, last-ditch miracles.",
    Flavor:        "You learned anatomy in a Vault and triage in a tent. Out here, the difference between sick and dead is whoever shows up with a stim first.",
    MentorNPCID:   "doc-vega",
    SignatureVerb: "stim",
    GatingSkill:   "survival",
    SkillBonuses:  []SkillBonus{{"survival", 1}},
    Kit: []KitItem{
        {"service-pistol", "Service Pistol", "1d6, sidearm.", 3},
        {"scalpel", "Scalpel", "1d4 melee, also crafts chems.", 1},
        {"lab-coat", "Lab Coat", "+1 defense, marks you as a medic.", 1},
        {"stimpak-3", "Stimpak x3", "Heal 25 HP each.", 2},
        {"bandages-5", "Bandages x5", "Heal 10 HP, stops bleeding.", 1},
        {"med-x", "Med-X", "+2 damage resist, 10 actions.", 2},
        {"buffout", "Buffout", "+20 max HP, 10 actions.", 2},
        {"empty-syringes-5", "Empty Syringes x5", "Crafting reagent.", 1},
        {"vault-tec-id", "Vault-Tec ID", "Bluffs Vault-related NPCs once.", 2},
        {"prewar-family-photo", "Pre-War Family Photo", "Two kids, a dog, sun.", 0},
    },
}

var ghoul = Class{
    ID:            "ghoul",
    Name:          "Ghoul",
    Tagline:       "Older than the bombs. Hungrier than them too.",
    Flavor:        "The radiation didn't kill you, it kept you. You remember the world before, mostly. The Settlers won't, and they'll spit when they see your face. The Collective will not.",
    MentorNPCID:   "mother-ash",
    SignatureVerb: "rad-feed",
    GatingSkill:   "scavenging",
    SkillBonuses:  []SkillBonus{{"scavenging", 1}},
    StartingRep: []RepDelta{
        {"ghoul-collective", 10},
        {"settlers", -10},
    },
    Kit: []KitItem{
        {"bone-club", "Bone Club", "1d6 melee.", 2},
        {"salvaged-pistol", "Salvaged Pistol", "1d6 sidearm.", 3},
        {"tattered-hood", "Tattered Hood", "+1 defense, hides ghoul features.", 1},
        {"glowing-trinket", "Glowing Trinket", "Lights dark rooms.", 2},
        {"rad-chunks-5", "Rad-Chunks x5", "rad-feed reagent: heal 15 HP each.", 1},
        {"junk-food-5", "Junk Food x5", "Heal 5 HP (irradiated).", 1},
        {"old-coin-pouch", "Old-World Coin Pouch", "30 caps.", 2},
        {"reinforced-wraps", "Reinforced Wrappings", "+1 defense, +1 fire resist.", 2},
        {"ancient-grudge", "Ancient Grudge", "+5 damage on first attack vs Ironclad.", 4},
        {"prewar-locket", "Pre-War Locket", "It still opens. The picture is gone.", 0},
    },
}
```

- [ ] **Step 2:** Create `internal/classes/classes_test.go`:

```go
package classes

import "testing"

func TestAllClassesHaveUniqueIDs(t *testing.T) {
    seen := map[string]bool{}
    for _, c := range All() {
        if seen[c.ID] {
            t.Errorf("duplicate class id: %s", c.ID)
        }
        seen[c.ID] = true
    }
    if len(All()) != 5 {
        t.Errorf("expected 5 classes, got %d", len(All()))
    }
}

func TestEveryClassHasFreeHookItem(t *testing.T) {
    for _, c := range All() {
        var has bool
        for _, k := range c.Kit {
            if k.Cost == 0 {
                has = true
                break
            }
        }
        if !has {
            t.Errorf("class %s has no free (cost=0) hook item", c.ID)
        }
    }
}

func TestKitFitsInBudget(t *testing.T) {
    // A loadout = pick the cheapest weapon + cheapest armor + free hook + as many
    // 1-pt consumables as fit. Verify a viable loadout exists at <= KitBudget for
    // each class. This guards against accidentally over-pricing the kit.
    for _, c := range All() {
        // Pick cheapest non-zero items until budget is met or items run out.
        items := append([]KitItem{}, c.Kit...)
        // simple greedy
        spent := 0
        picks := 0
        // sort by cost ascending
        for i := 0; i < len(items); i++ {
            for j := i + 1; j < len(items); j++ {
                if items[j].Cost < items[i].Cost {
                    items[i], items[j] = items[j], items[i]
                }
            }
        }
        for _, it := range items {
            if spent+it.Cost > KitBudget {
                continue
            }
            spent += it.Cost
            picks++
        }
        if picks < 3 {
            t.Errorf("class %s: only %d items fit in budget %d", c.ID, picks, KitBudget)
        }
    }
}

func TestByID(t *testing.T) {
    for _, c := range All() {
        got := ByID(c.ID)
        if got == nil || got.ID != c.ID {
            t.Errorf("ByID(%q) failed", c.ID)
        }
    }
    if ByID("nonexistent") != nil {
        t.Error("ByID for unknown id should return nil")
    }
}

func TestGhoulHasFactionRep(t *testing.T) {
    g := ByID("ghoul")
    if g == nil {
        t.Fatal("ghoul class missing")
    }
    if len(g.StartingRep) == 0 {
        t.Error("ghoul should have starting rep deltas")
    }
}
```

- [ ] **Step 3:** Run `go test ./internal/classes/...`. Expected: PASS.

- [ ] **Step 4:** Commit.

```bash
git add internal/classes/
git commit -m "feat(classes): add 5-class registry with kits and signature verbs"
```

---

## Task 5: WorldQuest + Quest rep-gating fields

**Files:**
- Modify: `internal/world/world.go`
- Modify: `internal/quests/quests.go`
- Modify: `internal/db/gamedb/gamedb.go` (the `QuestRecord` struct + AcceptQuest SQL + ListActiveQuests SQL)

- [ ] **Step 1:** In `internal/world/world.go`, add four fields to `WorldQuest` (around line 240–256):

```go
type WorldQuest struct {
    ID                string `yaml:"id"`
    Title             string `yaml:"title"`
    Description       string `yaml:"description"`
    GiverNPCID        string `yaml:"giver_npc_id"`
    GiverFaction      string `yaml:"giver_faction,omitempty"`
    MinRep            int    `yaml:"min_rep,omitempty"`
    ObjType           string `yaml:"obj_type"`
    ObjTarget         string `yaml:"obj_target"`
    ObjRoom           string `yaml:"obj_room,omitempty"`
    ObjCount          int    `yaml:"obj_count"`
    RewardCredits     int    `yaml:"reward_credits"`
    RewardXPSkill     string `yaml:"reward_xp_skill,omitempty"`
    RewardXPAmount    int    `yaml:"reward_xp_amount,omitempty"`
    RewardItemID      string `yaml:"reward_item_id,omitempty"`
    RewardItemName    string `yaml:"reward_item_name,omitempty"`
    RewardItemDesc    string `yaml:"reward_item_desc,omitempty"`
    RewardRepFaction  string `yaml:"reward_rep_faction,omitempty"`
    RewardRepDelta    int    `yaml:"reward_rep_delta,omitempty"`
    NextQuestID       string `yaml:"next_quest_id,omitempty"`
}
```

- [ ] **Step 2:** In `internal/quests/quests.go`, add the same four fields to `Quest` struct and to `fromRecord` / `toRecord`. Match exactly the names used on `WorldQuest`.

- [ ] **Step 3:** In `internal/db/gamedb/gamedb.go`, find `QuestRecord` (search for `type QuestRecord struct`). Add the four fields. Then find `AcceptQuest` and `ListActiveQuests` SQL statements (and any related Get/Update). Update INSERT column lists, value placeholders, scan calls, and SELECT column lists to include all four new columns. Apply to both sqlite and postgres paths if both exist; if postgres uses sqlc, leave a `// TODO: pg quest rep fields` comment and proceed.

- [ ] **Step 4:** Run `go build ./...`. Expected: PASS.

- [ ] **Step 5:** Run `go test ./internal/quests/... ./internal/world/...`. Expected: PASS.

- [ ] **Step 6:** Commit.

```bash
git add internal/world/world.go internal/quests/quests.go internal/db/gamedb/gamedb.go
git commit -m "feat(quests): add giver_faction, min_rep, reward_rep fields"
```

---

## Task 6: Rep gate in dialogue-driven quest accept + reward rep on complete

**Files:**
- Modify: `internal/commands/commands.go`

- [ ] **Step 1:** Locate the dialogue quest-accept block (around lines 1268–1294 in `internal/commands/commands.go`, inside the talk command, the section starting `if matchedQuestID != "" && !activeQuestIDs[matchedQuestID]`).

  Add a rep check before constructing the `quests.Quest`. Pseudocode:

```go
if matchedQuestID != "" && !activeQuestIDs[matchedQuestID] {
    wq := w.FindQuest(matchedQuestID)
    if wq != nil {
        // Faction rep gate
        if wq.GiverFaction != "" {
            currentRep := gdb.GetReputation(context.Background(), wq.GiverFaction)
            if currentRep < wq.MinRep {
                output += fmt.Sprintf(
                    "\n%s eyes you and shakes their head. \"I don't deal with your kind. Not yet.\"",
                    npc.Name,
                )
                // Skip the accept by jumping past it.
                goto skipAccept
            }
        }
        q := quests.Quest{
            ID:               wq.ID,
            Title:            wq.Title,
            Description:      wq.Description,
            ObjType:          wq.ObjType,
            ObjTarget:        wq.ObjTarget,
            ObjRoom:          wq.ObjRoom,
            ObjCount:         wq.ObjCount,
            RewardCredits:    wq.RewardCredits,
            RewardXPSkill:    wq.RewardXPSkill,
            RewardXPAmount:   wq.RewardXPAmount,
            RewardItemID:     wq.RewardItemID,
            RewardItemName:   wq.RewardItemName,
            RewardItemDesc:   wq.RewardItemDesc,
            RewardRepFaction: wq.RewardRepFaction,
            RewardRepDelta:   wq.RewardRepDelta,
            GiverNPCID:       npc.ID,
        }
        if err := quests.Accept(gdb, q); err == nil {
            output += fmt.Sprintf("\n[QUEST ACCEPTED] %s", wq.Title)
            if wq.Description != "" {
                output += "\n" + wq.Description
            }
        }
    skipAccept:
    }
}
```

  Avoid `goto` if you can refactor cleanly with an `else` branch — Go allows `goto` but a small inline closure or refactor is preferred. For example:

```go
acceptable := true
if wq.GiverFaction != "" {
    if gdb.GetReputation(context.Background(), wq.GiverFaction) < wq.MinRep {
        output += fmt.Sprintf(
            "\n%s shakes their head. \"I don't deal with your kind. Not yet.\"",
            npc.Name,
        )
        acceptable = false
    }
}
if acceptable {
    q := quests.Quest{ /* ... as before ... */ }
    if err := quests.Accept(gdb, q); err == nil {
        output += fmt.Sprintf("\n[QUEST ACCEPTED] %s", wq.Title)
        if wq.Description != "" {
            output += "\n" + wq.Description
        }
    }
}
```

- [ ] **Step 2:** Locate the quest-completion reward block (around lines 1580–1640). Find where `q.RewardCredits`, `q.RewardItemID`, etc. are applied. Add a reward-rep block:

```go
if q.RewardRepFaction != "" && q.RewardRepDelta != 0 {
    if err := gdb.AdjustReputation(context.Background(), q.RewardRepFaction, q.RewardRepDelta); err == nil {
        sign := "+"
        if q.RewardRepDelta < 0 {
            sign = ""
        }
        out.WriteString(fmt.Sprintf("  %s%d rep with %s\n", sign, q.RewardRepDelta, q.RewardRepFaction))
    }
}
```

- [ ] **Step 3:** Make sure the chained next-quest block (around line 1617) also propagates the new fields when constructing `nextQ`. Add `RewardRepFaction: wq.RewardRepFaction, RewardRepDelta: wq.RewardRepDelta` and `// rep gate not re-checked on chain — chain quests are pre-earned`.

- [ ] **Step 4:** Run `go build ./...`. Expected: PASS.

- [ ] **Step 5:** Run `go test ./...`. Fix any incidental failures. Expected: PASS.

- [ ] **Step 6:** Commit.

```bash
git add internal/commands/commands.go
git commit -m "feat(quests): rep-gate quest accept and apply rep rewards"
```

---

## Task 7: Signature verb commands

**Files:**
- Create: `internal/commands/signatures.go`
- Modify: `internal/commands/commands.go` (register the new commands in whatever dispatch map exists)

- [ ] **Step 1:** First inspect `internal/commands/commands.go` to find the command dispatch table (search for a map like `var Commands = map[string]Handler{` or similar). Note the handler signature — likely `func(db *sql.DB, s *player.State, w *world.World, args []string) Result` per the project memory note.

- [ ] **Step 2:** Create `internal/commands/signatures.go` with stub implementations that satisfy the gating rule (`s.Class == X || skills.Level(gatingSkill) >= 3`) and produce a meaningful message. Keep the actual mechanics minimal — we're shipping the *gating* and the *verb*, not full new combat math:

```go
package commands

import (
    "database/sql"
    "fmt"

    "github.com/adam-stokes/gl1tch-mud/internal/classes"
    "github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
    "github.com/adam-stokes/gl1tch-mud/internal/player"
    "github.com/adam-stokes/gl1tch-mud/internal/skills"
    "github.com/adam-stokes/gl1tch-mud/internal/world"
)

// canUseSignature returns true if the player's class owns the verb,
// or if their gating skill is at or above SignatureVerbUnlockLevel.
func canUseSignature(gdb *gamedb.GameDB, s *player.State, verb string) (bool, string) {
    for _, c := range classes.All() {
        if c.SignatureVerb != verb {
            continue
        }
        if s.Class == c.ID {
            return true, ""
        }
        if skills.Level(gdb, c.GatingSkill) >= classes.SignatureVerbUnlockLevel {
            return true, ""
        }
        return false, fmt.Sprintf(
            "you don't know how to %s yet — reach level %d in %s, or pick the %s class.",
            verb, classes.SignatureVerbUnlockLevel, c.GatingSkill, c.Name,
        )
    }
    return false, fmt.Sprintf("unknown signature: %s", verb)
}

// Fan: Gunslinger multi-shot. Hits the current target twice for normal damage.
func Fan(db *sql.DB, gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
    ok, msg := canUseSignature(gdb, s, "fan")
    if !ok {
        return Result{Output: msg}
    }
    return Result{Output: "you fan the hammer — two shots, fast as breathing. (combat hook: applies bonus damage on next attack)"}
}

// Hotwire: Mechanic system bypass. For now, flavor + a hint to use on a system in the room.
func Hotwire(db *sql.DB, gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
    ok, msg := canUseSignature(gdb, s, "hotwire")
    if !ok {
        return Result{Output: msg}
    }
    return Result{Output: "you crack the housing and start crossing wires. (system hook: bypass next lock/system in this room)"}
}

// Barter: Scavver vendor discount.
func Barter(db *sql.DB, gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
    ok, msg := canUseSignature(gdb, s, "barter")
    if !ok {
        return Result{Output: msg}
    }
    return Result{Output: "you start the dance — you'll get a better price on your next trade."}
}

// Lift: Scavver pickpocket.
func Lift(db *sql.DB, gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
    ok, msg := canUseSignature(gdb, s, "lift")
    if !ok {
        return Result{Output: msg}
    }
    return Result{Output: "you slip closer, fingers light. (target an NPC: 'lift <npc>' — pickpocket roll vs scavenging)"}
}

// Stim: Medic heal + temp damage.
func Stim(db *sql.DB, gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
    ok, msg := canUseSignature(gdb, s, "stim")
    if !ok {
        return Result{Output: msg}
    }
    heal := 25
    s.HP += heal
    if s.HP > s.MaxHP {
        s.HP = s.MaxHP
    }
    _ = player.Save(gdb, s)
    return Result{Output: fmt.Sprintf("you jam the stim home. +%d HP. you feel sharper.", heal)}
}

// RadFeed: Ghoul consume rad-junk.
func RadFeed(db *sql.DB, gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
    ok, msg := canUseSignature(gdb, s, "rad-feed")
    if !ok {
        return Result{Output: msg}
    }
    inv, _ := player.Inventory(gdb)
    for _, it := range inv {
        if it.ID == "rad-chunks-5" || it.ID == "junk-food-5" || it.ID == "rad-chunk" {
            heal := 15
            s.HP += heal
            if s.HP > s.MaxHP {
                s.HP = s.MaxHP
            }
            _ = player.Save(gdb, s)
            return Result{Output: fmt.Sprintf("you tear into the irradiated junk. +%d HP. it tastes like home.", heal)}
        }
    }
    return Result{Output: "nothing irradiated to feed on."}
}
```

- [ ] **Step 3:** Open `internal/commands/commands.go`, find the dispatch map (search for something like `map[string]` of handlers or a switch on command name), and register the six new verbs: `fan`, `hotwire`, `barter`, `lift`, `stim`, `rad-feed`.

  If the existing handler signature is `func(db *sql.DB, s *player.State, w *world.World, args []string) Result` and does not take `gdb`, you'll need to adapt — use whatever pattern other commands like `Hack` or `Examine` follow. Mirror their wiring exactly.

- [ ] **Step 4:** Run `go build ./...`. Expected: PASS.

- [ ] **Step 5:** Commit.

```bash
git add internal/commands/signatures.go internal/commands/commands.go
git commit -m "feat(commands): add 6 class signature verbs with skill-3 unlock"
```

---

## Task 8: Character creation wizard

**Files:**
- Create: `internal/commands/character.go`
- Modify: `internal/commands/commands.go` (intercept all commands when `s.Class == ""`)

- [ ] **Step 1:** Create `internal/commands/character.go`. The wizard is stateless — every call inspects `s.Class` and the inventory + a small set of `player_flags` to figure out which step the player is on. Persistent state lives entirely in the DB so the wizard survives a disconnect.

  Wizard state model:

  | Player state | Wizard step | Required command |
  |---|---|---|
  | `s.Class == "" && no flag "char_named"` | Step 1: name | `name <text>` |
  | flag `char_named` set, `s.Class == ""` | Step 2: class | `class list`, `class pick <id>` |
  | `s.Class != "" && no flag "kit_done"` | Step 3: kit | `kit list`, `kit add <item>`, `kit remove <item>`, `kit done` |
  | flag `kit_done` set, no flag `tutorial_complete` | Step 4: tutorial (mentor dialogue) | normal commands |
  | flag `tutorial_complete` | game proper | normal commands |

  Use `gdb.SetFlag(flag string)` and `gdb.HasFlag(flag string)` — check if those exist; if not, add them as thin wrappers over the existing `player_flags` table:

```go
// in gamedb.go, if not present:
func (g *GameDB) SetFlag(ctx context.Context, flag string) error {
    _, err := g.sqliteDB.Exec(`INSERT OR IGNORE INTO player_flags (flag) VALUES (?)`, flag)
    return err
}
func (g *GameDB) HasFlag(ctx context.Context, flag string) bool {
    var f string
    err := g.sqliteDB.QueryRow(`SELECT flag FROM player_flags WHERE flag=?`, flag).Scan(&f)
    return err == nil
}
```

  (Confirm by grepping `player_flags` in `gamedb.go` first; reuse if the helpers exist.)

- [ ] **Step 2:** Implement the wizard handler. Sketch:

```go
package commands

import (
    "context"
    "fmt"
    "strings"

    "github.com/adam-stokes/gl1tch-mud/internal/classes"
    "github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
    "github.com/adam-stokes/gl1tch-mud/internal/player"
    "github.com/adam-stokes/gl1tch-mud/internal/world"
)

// CharacterWizardActive returns true if the player has not finished creation.
func CharacterWizardActive(gdb *gamedb.GameDB, s *player.State) bool {
    return s.Class == "" || !gdb.HasFlag(context.Background(), "kit_done")
}

// CharacterIntercept handles ALL input while the wizard is active.
// Returns the wizard's response and a bool indicating it consumed the command.
func CharacterIntercept(gdb *gamedb.GameDB, s *player.State, w *world.World, raw string) (Result, bool) {
    ctx := context.Background()
    if !CharacterWizardActive(gdb, s) {
        return Result{}, false
    }

    fields := strings.Fields(strings.TrimSpace(raw))
    cmd := ""
    if len(fields) > 0 {
        cmd = strings.ToLower(fields[0])
    }
    args := fields[1:]

    // Step 1: name
    if !gdb.HasFlag(ctx, "char_named") {
        if cmd == "name" && len(args) >= 1 {
            name := strings.Join(args, " ")
            // Save name via existing player.Save path — extend State.Name then Save.
            s.Name = name
            _ = player.Save(gdb, s)
            _ = gdb.SetFlag(ctx, "char_named")
            return Result{Output: fmt.Sprintf("name set: %s\n\nNext: pick a class. type 'class list' to see your options.", name)}, true
        }
        return Result{Output: charPrompt("Welcome to the wasteland.\n\nFirst: what should we call you?\n\nType 'name <yourname>' to continue.")}, true
    }

    // Step 2: class
    if s.Class == "" {
        switch cmd {
        case "class":
            if len(args) == 0 || args[0] == "list" {
                return Result{Output: renderClassList()}, true
            }
            if args[0] == "pick" && len(args) >= 2 {
                c := classes.ByID(strings.ToLower(args[1]))
                if c == nil {
                    return Result{Output: "no such class. try 'class list'."}, true
                }
                s.Class = c.ID
                _ = player.Save(gdb, s)
                // Apply skill bonuses now (additive — they stay forever).
                for _, sb := range c.SkillBonuses {
                    // Award XP equal to the level-1 threshold so they actually start at level 1+
                    _, _ = skills.Award(gdb, sb.Skill, 50*sb.Level)
                }
                // Apply starting rep deltas.
                for _, rd := range c.StartingRep {
                    _ = gdb.AdjustReputation(ctx, rd.Faction, rd.Delta)
                }
                return Result{Output: fmt.Sprintf("class locked: %s.\n\nYou get %d kit points to spend.\nType 'kit list' to see what's available.", c.Name, classes.KitBudget)}, true
            }
        }
        return Result{Output: charPrompt("Pick your class.\n\nType 'class list' to see your options, then 'class pick <id>' to commit.")}, true
    }

    // Step 3: kit
    if !gdb.HasFlag(ctx, "kit_done") {
        return handleKitCommand(gdb, s, cmd, args), true
    }

    return Result{}, false
}

func charPrompt(s string) string { return s }

func renderClassList() string {
    var b strings.Builder
    b.WriteString("CLASSES:\n\n")
    for _, c := range classes.All() {
        b.WriteString(fmt.Sprintf("  %s — %s\n", c.ID, c.Tagline))
        b.WriteString(fmt.Sprintf("    %s\n", c.Flavor))
        b.WriteString(fmt.Sprintf("    signature: %s | skill bonus: ", c.SignatureVerb))
        for i, sb := range c.SkillBonuses {
            if i > 0 {
                b.WriteString(", ")
            }
            b.WriteString(fmt.Sprintf("%s +%d", sb.Skill, sb.Level))
        }
        b.WriteString("\n\n")
    }
    b.WriteString("type 'class pick <id>' to commit.\n")
    return b.String()
}

// handleKitCommand reads/writes a 'kit_picks' player_flag-style record.
// To keep things simple, we encode picks as flag rows of the form "kit:<itemid>".
func handleKitCommand(gdb *gamedb.GameDB, s *player.State, cmd string, args []string) Result {
    ctx := context.Background()
    c := classes.ByID(s.Class)
    if c == nil {
        return Result{Output: "internal error: class missing"}
    }

    isPicked := func(itemID string) bool { return gdb.HasFlag(ctx, "kit:"+itemID) }
    spent := func() int {
        total := 0
        for _, k := range c.Kit {
            if isPicked(k.ID) {
                total += k.Cost
            }
        }
        return total
    }

    switch cmd {
    case "kit":
        if len(args) == 0 || args[0] == "list" {
            var b strings.Builder
            b.WriteString(fmt.Sprintf("KIT — %d/%d points spent\n\n", spent(), classes.KitBudget))
            for _, k := range c.Kit {
                marker := "[ ]"
                if isPicked(k.ID) {
                    marker = "[x]"
                }
                b.WriteString(fmt.Sprintf("  %s %dpt  %-25s — %s\n", marker, k.Cost, k.Name, k.Desc))
            }
            b.WriteString("\ntype 'kit add <id>', 'kit remove <id>', or 'kit done'.\n")
            return Result{Output: b.String()}
        }
        if args[0] == "add" && len(args) >= 2 {
            id := args[1]
            for _, k := range c.Kit {
                if k.ID != id {
                    continue
                }
                if isPicked(id) {
                    return Result{Output: "already picked."}
                }
                if spent()+k.Cost > classes.KitBudget {
                    return Result{Output: fmt.Sprintf("not enough kit points (%d/%d).", spent(), classes.KitBudget)}
                }
                _ = gdb.SetFlag(ctx, "kit:"+id)
                return Result{Output: fmt.Sprintf("added %s. %d/%d points spent.", k.Name, spent(), classes.KitBudget)}
            }
            return Result{Output: "no such kit item."}
        }
        if args[0] == "remove" && len(args) >= 2 {
            id := args[1]
            if !isPicked(id) {
                return Result{Output: "not in your kit."}
            }
            _ = gdb.DelFlag(ctx, "kit:"+id) // see Step 3 below — add DelFlag if missing
            return Result{Output: fmt.Sprintf("removed. %d/%d points spent.", spent(), classes.KitBudget)}
        }
        if args[0] == "done" {
            // Always grant the free hook items (cost 0) automatically.
            for _, k := range c.Kit {
                if k.Cost == 0 {
                    _ = gdb.SetFlag(ctx, "kit:"+k.ID)
                }
            }
            // Materialize chosen kit into inventory.
            for _, k := range c.Kit {
                if isPicked(k.ID) {
                    _ = player.AddItem(gdb, k.ID, k.Name, k.Desc)
                }
            }
            _ = gdb.SetFlag(ctx, "kit_done")
            return Result{Output: "kit confirmed. items added to inventory.\n\nlook around — your mentor is here."}
        }
    }

    return Result{Output: "type 'kit list', 'kit add <id>', 'kit remove <id>', or 'kit done'."}
}
```

- [ ] **Step 3:** If `gamedb.DelFlag` doesn't already exist, add it next to `SetFlag`:

```go
func (g *GameDB) DelFlag(ctx context.Context, flag string) error {
    _, err := g.sqliteDB.Exec(`DELETE FROM player_flags WHERE flag=?`, flag)
    return err
}
```

- [ ] **Step 4:** In `internal/commands/commands.go`, find the top-level command dispatch entry point (the function that maps the raw command string to a handler). Add an early check:

```go
// At the very top of the dispatch function, before any other handler runs:
if res, consumed := CharacterIntercept(gdb, s, w, raw); consumed {
    return res
}
```

  Adapt the variable names to whatever the existing dispatch uses. The intercept must run *before* movement, look, inventory, etc., so the wizard has full control until the player finishes creation.

- [ ] **Step 5:** Run `go build ./...`. Expected: PASS.

- [ ] **Step 6:** Commit.

```bash
git add internal/commands/character.go internal/commands/commands.go internal/db/gamedb/gamedb.go
git commit -m "feat(character): wizard for name/class/kit creation"
```

---

## Task 9: World content — wakeup-camp, mentors, ghoul-intro, start_room

**Files:**
- Modify: `internal/world/defaults/mudout/world.yaml`

- [ ] **Step 1:** Change `start_room` near the top of the file (the spec has it as `dusthaven-0` currently per `2026-04-04-mudout-design.md`):

```yaml
start_room: wakeup-camp
```

- [ ] **Step 2:** Add the `wakeup-camp` room as the **first** room under `rooms:` (so it loads before `dusthaven-0`):

```yaml
  - id: wakeup-camp
    name: "WAKEUP CAMP"
    desc: |
      A ring of oil-drum fires under a sun-bleached billboard. Five drifters
      have made this their meeting spot — each one looking for a particular
      kind of recruit. Through the chain-link to the south, you can see the
      gates of Dusthaven.
    biome: settlement
    exits:
      south: dusthaven-0
    npcs:
      - id: old-cass
        name: "Old Cass"
        hp: 60
        attack: 10
        desc: "Retired gunslinger. Two fingers missing on the off hand. Drinks slow."
        dialogue:
          - trigger: always
            text: "You wear it on you, kid. The walk. Try 'inventory' — see what you came in with. Then 'look' around. Then 'examine' that crate at my feet. Then try 'fan' on the bottle. Then get to the Gate. Marta's hiring."
      - id: rust
        name: "Rust"
        hp: 50
        attack: 6
        desc: "One-eyed tinker hunched over a half-disassembled pipe rifle."
        dialogue:
          - trigger: always
            text: "Hands first. Type 'inventory'. Then 'look'. Then 'examine' that crate — schematics inside. Try 'hotwire' on the dead radio. When you can do all four, get to the Gate."
      - id: halftrack
        name: "Halftrack"
        hp: 45
        attack: 5
        desc: "One-legged trader with a satchel that bulges in suspicious places."
        dialogue:
          - trigger: always
            text: "Lesson one: 'inventory'. Lesson two: 'look'. Lesson three: 'examine' the crate — there's product in it. Lesson four: 'barter' with me. Then go talk to Marta at the Gate."
      - id: doc-vega
        name: "Doc Vega"
        hp: 50
        attack: 5
        desc: "Vault-issue lab coat, sleeves rolled, scalpel tucked behind the ear."
        dialogue:
          - trigger: always
            text: "Triage check. 'inventory'. 'look'. 'examine' the crate — chems inside. 'stim' yourself. Then south to the Gate."
      - id: mother-ash
        name: "Mother Ash"
        hp: 70
        attack: 7
        desc: "Ancient ghoul wrapped in tattered robes. Her eyes still see fine."
        dialogue:
          - trigger: "quest_not_active:ghoul-intro"
            text: "Old bones, child. There's a glowing thing in the Collapsed Mall — pre-war, brittle, mine. Bring it back to me and I'll know you. The Settlers won't help you. We will."
            quest_id: ghoul-intro
          - trigger: always
            text: "'inventory'. 'look'. 'examine' the crate. 'rad-feed' on the chunks. Then walk south. The world is older than the gate."
    items:
      - id: tutorial-crate
        name: "Battered Crate"
        desc: "A battered scrap crate with a faded stencil. Looks like it's been kicked open and shut a hundred times."
```

  *Note:* The `quest_id` trigger pattern is what the existing `settler-recruiter` uses to auto-grant quests via the talk command. Mother Ash uses it to grant `ghoul-intro` to any player who talks to her — but the rep gate from Task 6 will only let it through for Ghoul Collective rep ≥ 0. (Default is 0, Ghoul start is +10, so Ghouls always get it. Other classes start at 0 and also get it — that's fine; Ghoul-Collective alignment is intended to be open to everyone willing to take it.)

- [ ] **Step 3:** Add the `glowing-relic` item to `ruins-0` (Collapsed Mall). Find the `ruins-0` room in the YAML and add to its `items:` list:

```yaml
      - id: glowing-relic
        name: "Glowing Relic"
        desc: "A pre-war ceramic shard, faintly luminescent, warm to the touch."
```

- [ ] **Step 4:** Tag the existing `settlers-intro` and `settlers-defense` quests with `giver_faction: settlers` and `min_rep: 0`. Find the `quests:` block (line ~846) and add the two fields under each:

```yaml
  - id: settlers-intro
    title: "Prove Yourself"
    description: "Marta wants you to scavenge 3 scrap metal from the wasteland to prove you're useful to the Settlers."
    giver_npc_id: settler-recruiter
    giver_faction: settlers
    min_rep: 0
    obj_type: retrieve
    ...
```

  Same for `settlers-defense`.

- [ ] **Step 5:** Add the new `ghoul-intro` quest to the same `quests:` block:

```yaml
  - id: ghoul-intro
    title: "Old Bones"
    description: "Mother Ash wants you to retrieve a glowing relic from the Collapsed Mall."
    giver_npc_id: mother-ash
    giver_faction: ghoul-collective
    min_rep: 0
    obj_type: retrieve
    obj_target: glowing-relic
    obj_count: 1
    reward_credits: 50
    reward_xp_skill: scavenging
    reward_xp_amount: 25
    reward_rep_faction: ghoul-collective
    reward_rep_delta: 5
```

- [ ] **Step 6:** Run `go build ./...`. Expected: PASS.

- [ ] **Step 7:** Run `go test ./internal/world/...`. The world loader will validate the YAML parses and references resolve. If any test asserts the start room is `dusthaven-0`, update it to `wakeup-camp`. Expected: PASS.

- [ ] **Step 8:** Commit.

```bash
git add internal/world/defaults/mudout/world.yaml internal/world/world_test.go
git commit -m "feat(mudout): wakeup-camp room, 5 mentors, ghoul-intro quest"
```

---

## Task 10: End-to-end smoke test

**Files:**
- (no code changes; verification only)

- [ ] **Step 1:** Run the full build and test suite:

```bash
go build ./...
go test ./...
```

  Expected: PASS. Fix any incidental failures (test fixtures that hardcode column counts, mocks that need the new params, etc.).

- [ ] **Step 2:** Manual smoke test. Start the binary against a fresh sqlite DB:

```bash
rm -f /tmp/gl1tch-smoke.db
GL1TCH_DB=/tmp/gl1tch-smoke.db GL1TCH_WORLD=mudout go run . --offline
```

  (Adjust env vars / flags to match the project's actual entry point — check `main.go` if unsure.)

- [ ] **Step 3:** Walk the wizard end-to-end:
  1. `name Drifter` → name set
  2. `class list` → see all 5
  3. `class pick scavver` → confirms
  4. `kit list` → see 10 items + budget
  5. `kit add sawed-off` `kit add lockpicks-3` `kit add big-satchel` `kit add caps-50` `kit add lucky-charm` (totals to 10)
  6. `kit done` → kit materialized in inventory
  7. `look` → see wakeup-camp + 5 mentors
  8. `talk halftrack` → mentor dialogue prints
  9. `inventory` → see kit items
  10. `barter` → signature verb fires
  11. `south` → enter `dusthaven-0`
  12. `talk marta` → accepts `settlers-intro`

- [ ] **Step 4:** Repeat with `class pick ghoul`. Verify:
  - `talk marta` does NOT accept `settlers-intro` — Marta refuses with the rep-gate flavor line
  - `north` (or wherever Mother Ash is — she's in wakeup-camp, so `talk mother-ash` from there) accepts `ghoul-intro`
  - After completing ghoul-intro (drop the relic into inventory and `quest complete`), rep with `ghoul-collective` should be +15 (10 starting + 5 reward)

- [ ] **Step 5:** If everything works, final commit (only if there are uncommitted changes):

```bash
git status
# if anything stray, commit it
```

- [ ] **Step 6:** Done. Optionally bump a CHANGELOG if the project keeps one (search for `CHANGELOG.md`).

---

## Self-Review Notes

- **Spec coverage:** 5 classes ✓, point-buy 10pt kit ✓, wakeup-camp shared room ✓, 5 mentors with scripted dialogue ✓, signature verbs with skill-3 unlock ✓, rep gating on quests ✓, Ghoul rep penalty ✓, ghoul-intro quest ✓, levels 0→2 supported by existing quest XP + new ghoul quest ✓. The level math is data-only and handled by existing skills.Award.
- **Risk areas:** Task 8's "intercept all commands" wiring depends on finding the right dispatch point in commands.go — if the dispatch is fragmented, this may need adjustment. Worst case: gate the intercept inside the most common entry (the `Run` function in commands.go) and accept that movement commands also need a guard.
- **Out of scope, by design:** Full lift-roll mechanics, full hotwire mechanics, full fan multi-shot combat math, NPC dialogue lockout for non-class players talking to mismatched mentors (everyone can talk to everyone). These are deferred until the foundation is in place.
