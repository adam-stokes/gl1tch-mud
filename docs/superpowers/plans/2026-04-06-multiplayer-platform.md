# Multiplayer Platform Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn gl1tch-mud into a self-contained multiplayer platform with Postgres-backed user accounts, shared worlds, and chat — deployed via Docker Compose on a home LAN.

**Architecture:** Postgres (Docker) for auth + shared world state, SQLite for solo worlds. sqlc generates type-safe Go from SQL. Command handlers migrate from raw `*sql.DB` to sqlc `*Queries`. Web UI gains login form + world selector with mode badges.

**Tech Stack:** Go 1.25, sqlc, pgx/v5, Postgres 17, Docker Compose, bcrypt, Astro/TypeScript

**Spec:** `docs/superpowers/specs/2026-04-06-multiplayer-platform-design.md`

---

## Phase 1: sqlc Setup + SQLite Query Migration

Migrate existing raw SQL to sqlc-generated code. This is the foundation — everything else builds on it.

### Task 1: sqlc Configuration and Schema

**Files:**
- Create: `sqlc.yaml`
- Create: `internal/db/schema/sqlite/schema.sql`
- Create: `internal/db/schema/postgres/auth.sql`
- Create: `internal/db/schema/postgres/shared.sql`

- [ ] **Step 1: Install sqlc**

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Verify: `sqlc version` prints a version string.

- [ ] **Step 2: Create sqlc.yaml config**

```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "internal/db/queries/sqlite/"
    schema: "internal/db/schema/sqlite/"
    gen:
      go:
        package: "sqliteq"
        out: "internal/db/sqliteq"
        emit_json_tags: true
        emit_empty_slices: true

  - engine: "postgresql"
    queries: "internal/db/queries/postgres/"
    schema: "internal/db/schema/postgres/"
    gen:
      go:
        package: "pgq"
        out: "internal/db/pgq"
        emit_json_tags: true
        emit_empty_slices: true
        sql_package: "pgx/v5"
```

- [ ] **Step 3: Create SQLite schema file**

Copy the existing schema from `internal/db/schema.go` into `internal/db/schema/sqlite/schema.sql`. This is a direct copy of the CREATE TABLE statements — no changes to table structure.

- [ ] **Step 4: Create Postgres auth schema file**

Write `internal/db/schema/postgres/auth.sql`:

```sql
CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'player',
    banned BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 5: Create Postgres shared world schema file**

Write `internal/db/schema/postgres/shared.sql` with all shared world tables from the spec (shared.player_state, shared.inventory, shared.npc_state, etc.).

```sql
CREATE SCHEMA IF NOT EXISTS shared;

CREATE TABLE shared.player_state (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    hp INTEGER NOT NULL DEFAULT 100,
    max_hp INTEGER NOT NULL DEFAULT 100,
    credits INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared.inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_desc TEXT NOT NULL DEFAULT ''
);

CREATE TABLE shared.player_skills (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    skill TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 1,
    xp INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, skill)
);

CREATE TABLE shared.equipped_armor (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    defense INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared.npc_state (
    world_id TEXT NOT NULL,
    npc_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    hp INTEGER NOT NULL,
    alive BOOLEAN NOT NULL DEFAULT TRUE,
    respawn_at TIMESTAMPTZ,
    PRIMARY KEY (world_id, npc_id)
);

CREATE TABLE shared.lock_state (
    world_id TEXT NOT NULL,
    lock_id TEXT NOT NULL,
    unlocked BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (world_id, lock_id)
);

CREATE TABLE shared.resources (
    world_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    depleted BOOLEAN NOT NULL DEFAULT FALSE,
    respawn_at TIMESTAMPTZ,
    PRIMARY KEY (world_id, resource_id)
);

CREATE TABLE shared.system_state (
    world_id TEXT NOT NULL,
    system_id TEXT NOT NULL,
    hacked BOOLEAN NOT NULL DEFAULT FALSE,
    intrusion_level INTEGER NOT NULL DEFAULT 0,
    alert_level INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, system_id)
);

CREATE TABLE shared.death_pile (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_desc TEXT NOT NULL DEFAULT '',
    dropped_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shared.arena_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    game_type TEXT NOT NULL,
    wave INTEGER NOT NULL DEFAULT 0,
    enemies TEXT NOT NULL DEFAULT '[]',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shared.builds (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    build_id TEXT NOT NULL,
    placed_by UUID,
    placed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (world_id, room_id, build_id)
);

CREATE TABLE shared.chests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    stored_by UUID
);

CREATE TABLE shared.player_reputation (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    faction TEXT NOT NULL,
    value INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, faction)
);

CREATE TABLE shared.player_stealth (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 50,
    disguise TEXT NOT NULL DEFAULT 'none',
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared.player_flags (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    flag TEXT NOT NULL,
    PRIMARY KEY (account_id, world_id, flag)
);

CREATE TABLE shared.player_actions (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared.quests (
    id TEXT NOT NULL,
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    obj_type TEXT NOT NULL,
    obj_target TEXT NOT NULL,
    obj_room TEXT,
    obj_count INTEGER NOT NULL DEFAULT 1,
    obj_progress INTEGER NOT NULL DEFAULT 0,
    reward_credits INTEGER NOT NULL DEFAULT 0,
    reward_xp_skill TEXT,
    reward_xp_amount INTEGER NOT NULL DEFAULT 0,
    reward_item_id TEXT,
    reward_item_name TEXT,
    reward_item_desc TEXT,
    giver_npc_id TEXT,
    accepted_at INTEGER NOT NULL,
    next_quest_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (id, account_id, world_id)
);

CREATE TABLE shared.world_events (
    id TEXT PRIMARY KEY,
    world_id TEXT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    target_room TEXT NOT NULL,
    faction TEXT,
    payout_credits INTEGER NOT NULL DEFAULT 0,
    payout_item_id TEXT,
    payout_item_name TEXT,
    payout_item_desc TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    expires_actions INTEGER NOT NULL DEFAULT 20,
    created_actions INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE TABLE shared.player_faction (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    faction_id TEXT NOT NULL,
    faction_name TEXT NOT NULL,
    agenda TEXT,
    hideout_room_id TEXT,
    credits INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared.faction_members (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    npc_id TEXT PRIMARY KEY,
    npc_name TEXT NOT NULL,
    npc_desc TEXT,
    role TEXT NOT NULL DEFAULT 'associate',
    stationed_room TEXT,
    loyalty INTEGER NOT NULL DEFAULT 50,
    recruited_at INTEGER NOT NULL
);

CREATE TABLE shared.hideout_upgrades (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    upgrade_id TEXT NOT NULL,
    installed_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, world_id, upgrade_id)
);

CREATE TABLE shared.unlocked_recipes (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    recipe_id TEXT NOT NULL,
    unlocked_at INTEGER,
    PRIMARY KEY (account_id, world_id, recipe_id)
);

CREATE TABLE shared.npc_memory (
    world_id TEXT NOT NULL,
    npc_id TEXT NOT NULL,
    action TEXT NOT NULL,
    ts INTEGER NOT NULL,
    PRIMARY KEY (world_id, npc_id, action)
);

CREATE TABLE shared.generated_content (
    world_id TEXT NOT NULL,
    prompt_hash TEXT NOT NULL,
    type TEXT NOT NULL,
    yaml_blob TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (world_id, prompt_hash)
);

CREATE TABLE shared.weather_state (
    world_id TEXT NOT NULL,
    biome TEXT NOT NULL,
    condition TEXT NOT NULL DEFAULT 'clear',
    expires_action INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, biome)
);

CREATE TABLE shared.enchants (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    enchant_id TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 1,
    applied_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, item_id, enchant_id)
);

CREATE TABLE shared.enchanting_xp (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    xp INTEGER NOT NULL DEFAULT 0,
    level INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared.crystal_shards (
    world_id TEXT NOT NULL,
    shard_id TEXT NOT NULL,
    biome TEXT NOT NULL,
    collected BOOLEAN NOT NULL DEFAULT FALSE,
    collected_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, shard_id)
);

CREATE TABLE shared.crops (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    slot INTEGER NOT NULL,
    seed_id TEXT NOT NULL,
    planted_at_action INTEGER NOT NULL,
    ready_at_action INTEGER NOT NULL,
    harvested BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (world_id, room_id, slot)
);

CREATE TABLE shared.visited (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    PRIMARY KEY (account_id, world_id, room_id)
);

CREATE TABLE shared.player_augments (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    skill TEXT NOT NULL,
    bonus INTEGER NOT NULL,
    installed_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, world_id, skill)
);

CREATE TABLE shared.item_mods (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_instance TEXT NOT NULL,
    mod_id TEXT,
    applied_at INTEGER,
    PRIMARY KEY (account_id, world_id, item_instance)
);

CREATE TABLE shared.bounties (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    npc_id TEXT,
    created_at INTEGER,
    PRIMARY KEY (world_id, room_id)
);

CREATE TABLE shared.vuln_windows (
    world_id TEXT NOT NULL,
    system_id TEXT NOT NULL,
    bonus INTEGER,
    expires_action INTEGER,
    PRIMARY KEY (world_id, system_id)
);

CREATE TABLE shared.player_credits (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    credits INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);
```

- [ ] **Step 6: Run sqlc generate to validate schemas**

```bash
sqlc generate
```

Expected: creates empty `internal/db/sqliteq/` and `internal/db/pgq/` directories with `db.go`, `models.go` files (no query methods yet since no `.sql` query files exist).

- [ ] **Step 7: Commit**

```bash
git add sqlc.yaml internal/db/schema/ internal/db/sqliteq/ internal/db/pgq/
git commit -m "feat: add sqlc config and database schemas for SQLite + Postgres"
```

---

### Task 2: SQLite Player Queries (sqlc)

Migrate the `internal/player/` package queries to sqlc.

**Files:**
- Create: `internal/db/queries/sqlite/player.sql`
- Modify: `internal/player/player.go`

- [ ] **Step 1: Write SQLite player query file**

Create `internal/db/queries/sqlite/player.sql`:

```sql
-- name: GetPlayer :one
SELECT name, room_id, hp, max_hp, world FROM player WHERE id = 1;

-- name: SeedPlayer :exec
INSERT INTO player (id, name, room_id, hp, max_hp, world) VALUES (1, ?, ?, ?, ?, ?);

-- name: SavePlayer :exec
UPDATE player SET room_id = ?, hp = ?, max_hp = ?, world = ? WHERE id = 1;

-- name: ListInventory :many
SELECT item_id, item_name, item_desc FROM inventory;

-- name: AddItem :exec
INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?, ?, ?);

-- name: RemoveItem :execresult
DELETE FROM inventory WHERE item_id = ?;

-- name: GetNPCState :one
SELECT hp, alive FROM npc_state WHERE room_id = ? AND npc_id = ?;

-- name: UpsertNPCDead :exec
INSERT OR REPLACE INTO npc_state (room_id, npc_id, hp, alive) VALUES (?, ?, ?, 0);

-- name: UpsertNPCAlive :exec
INSERT OR REPLACE INTO npc_state (room_id, npc_id, hp, alive) VALUES (?, ?, ?, 1);

-- name: MarkVisited :exec
INSERT OR IGNORE INTO visited (room_id) VALUES (?);

-- name: HasVisited :one
SELECT room_id FROM visited WHERE room_id = ?;

-- name: ClearInventory :exec
DELETE FROM inventory;

-- name: InsertDeathPile :exec
INSERT INTO death_pile (room_id, item_id, item_name, item_desc, died_at) VALUES (?, ?, ?, ?, ?);

-- name: GetDeathPile :many
SELECT item_id, item_name, item_desc FROM death_pile WHERE room_id = ?;

-- name: DeleteDeathPile :exec
DELETE FROM death_pile WHERE room_id = ?;

-- name: AnyDeathPile :one
SELECT room_id, COUNT(*) as count FROM death_pile GROUP BY room_id ORDER BY MAX(died_at) DESC, MAX(id) DESC LIMIT 1;

-- name: MarkShardCollected :exec
UPDATE crystal_shards SET collected = 1, collected_at = ? WHERE shard_id = ?;

-- name: GetActionCount :one
SELECT count FROM player_actions WHERE id = 1;

-- name: EquipArmor :exec
INSERT OR REPLACE INTO equipped_armor (id, item_id, item_name, defense) VALUES (1, ?, ?, ?);

-- name: UnequipArmor :exec
DELETE FROM equipped_armor WHERE id = 1;

-- name: GetEquippedArmor :one
SELECT item_id, item_name, defense FROM equipped_armor WHERE id = 1;
```

- [ ] **Step 2: Run sqlc generate**

```bash
sqlc generate
```

Expected: `internal/db/sqliteq/player.sql.go` generated with typed Go methods.

- [ ] **Step 3: Update internal/player/player.go to use sqlc**

Replace raw `db.Query/Exec` calls with generated `sqliteq.New(db).MethodName()` calls. Keep the same public API (`Load`, `Save`, `Inventory`, etc.) but the internals now use sqlc.

Example for `LoadForWorld`:

```go
package player

import (
    "database/sql"
    "fmt"

    "github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
)

func LoadForWorld(db *sql.DB, worldName, startRoom string) (*State, error) {
    q := sqliteq.New(db)
    row, err := q.GetPlayer(context.Background())
    if err == sql.ErrNoRows {
        s := &State{Name: "hacker", RoomID: startRoom, HP: 100, MaxHP: 100, World: worldName}
        if err := q.SeedPlayer(context.Background(), sqliteq.SeedPlayerParams{
            Name: s.Name, RoomID: s.RoomID, Hp: int64(s.HP), MaxHp: int64(s.MaxHP), World: s.World,
        }); err != nil {
            return nil, fmt.Errorf("player: seed: %w", err)
        }
        return s, nil
    }
    if err != nil {
        return nil, fmt.Errorf("player: load: %w", err)
    }
    return &State{
        Name: row.Name, RoomID: row.RoomID,
        HP: int(row.Hp), MaxHP: int(row.MaxHp), World: row.World,
    }, nil
}
```

Repeat for all methods: `Save`, `Inventory`, `AddItem`, `RemoveItem`, `NPCAlive`, `KillNPC`, `NPCCurrentHP`, `SetNPCHP`, `MarkVisited`, `HasVisited`, `DumpToDeathPile`, `GetDeathPile`, `ClaimDeathPile`, `AnyDeathPile`, `MarkShardCollected`, `EquipArmor`, `UnequipArmor`, `GetEquippedArmor`, `LoadDefense`.

- [ ] **Step 4: Run existing tests**

```bash
go test ./internal/player/... -v
```

Expected: all existing tests pass with sqlc-generated code.

- [ ] **Step 5: Commit**

```bash
git add internal/db/queries/sqlite/player.sql internal/db/sqliteq/ internal/player/
git commit -m "refactor: migrate player package to sqlc-generated queries"
```

---

### Task 3: SQLite Command Queries (sqlc)

Migrate the inline SQL in command handlers to sqlc.

**Files:**
- Create: `internal/db/queries/sqlite/commands.sql`
- Create: `internal/db/queries/sqlite/skills.sql`
- Create: `internal/db/queries/sqlite/credits.sql`
- Create: `internal/db/queries/sqlite/crafting.sql`
- Create: `internal/db/queries/sqlite/quests.sql`
- Create: `internal/db/queries/sqlite/factions.sql`
- Create: `internal/db/queries/sqlite/mining.sql`
- Create: `internal/db/queries/sqlite/building.sql`
- Create: `internal/db/queries/sqlite/enchanting.sql`
- Create: `internal/db/queries/sqlite/stealth.sql`
- Create: `internal/db/queries/sqlite/weather.sql`
- Create: `internal/db/queries/sqlite/arena.sql`
- Modify: all files in `internal/commands/`
- Modify: `internal/skills/`, `internal/credits/`, `internal/crafting/`, `internal/quests/`, `internal/factions/`, `internal/enchanting/`, `internal/weather/`, `internal/base/`, `internal/arena/`

This is a large task. Work through it one query file at a time:

1. Read the source package (e.g., `internal/commands/mining.go`)
2. Extract every SQL statement into the corresponding `.sql` query file
3. Run `sqlc generate`
4. Update the Go source to use generated methods
5. Run tests for that package
6. Commit

- [ ] **Step 1: Extract all SQL from command handlers into query files**

For each command handler file, grep for `db.Query`, `db.QueryRow`, `db.Exec` and move each SQL statement to the appropriate query `.sql` file with a `-- name:` annotation.

Example from `mining.go`:

```sql
-- internal/db/queries/sqlite/mining.sql

-- name: BumpActions :exec
INSERT INTO player_actions (id, count) VALUES (1, 1) ON CONFLICT(id) DO UPDATE SET count = count + 1;

-- name: GetResourceState :one
SELECT depleted, depleted_at_action FROM room_resources WHERE room_id = ? AND resource_id = ?;

-- name: DepleteResource :exec
INSERT INTO room_resources (room_id, resource_id, depleted, depleted_at_action) VALUES (?, ?, 1, ?)
ON CONFLICT(room_id, resource_id) DO UPDATE SET depleted = 1, depleted_at_action = excluded.depleted_at_action;

-- name: UndepleteResource :exec
UPDATE room_resources SET depleted = 0 WHERE room_id = ? AND resource_id = ?;

-- name: GetReadyCrops :many
SELECT seed_id FROM crops WHERE room_id = ? AND ready_at_action <= ? AND harvested = 0;

-- name: HarvestCrops :exec
UPDATE crops SET harvested = 1 WHERE room_id = ? AND seed_id = ? AND ready_at_action <= ? AND harvested = 0;

-- name: CountCrops :one
SELECT COUNT(*) FROM crops WHERE room_id = ? AND seed_id = ? AND ready_at_action <= ? AND harvested = 0;

-- name: GetUsedSlots :many
SELECT slot FROM crops WHERE room_id = ? AND harvested = 0;

-- name: PlantCrop :exec
INSERT INTO crops (room_id, slot, seed_id, planted_at_action, ready_at_action) VALUES (?, ?, ?, ?, ?);

-- name: CountBuilds :one
SELECT COUNT(*) FROM builds WHERE room_id = ? AND build_id = ?;
```

Repeat for every package that has SQL. Read each file, extract queries, annotate.

- [ ] **Step 2: Run sqlc generate**

```bash
sqlc generate
```

Expected: all query files compile. Fix any SQL syntax errors.

- [ ] **Step 3: Update command handlers to use sqlc**

Change the `HandlerFunc` signature in `internal/commands/commands.go`:

```go
// Before:
type HandlerFunc func(db *sql.DB, s *player.State, w *world.World, args []string) Result

// After (temporary — Task 9 will change this to *gamedb.GameDB):
type HandlerFunc func(q *sqliteq.Queries, s *player.State, w *world.World, args []string) Result
```

Update every handler to use `q.MethodName(ctx, params)` instead of raw SQL.

**Important:** This changes the handler signature, so also update:
- `internal/server/session.go:264` — where handlers are called
- `main.go` — where handlers are called in the CLI game loop

Both call sites construct the `*sqliteq.Queries` from their `*sql.DB` and pass it in.

- [ ] **Step 4: Update all helper packages**

Update `internal/skills/`, `internal/credits/`, `internal/crafting/`, `internal/quests/`, `internal/factions/`, `internal/enchanting/`, `internal/weather/`, `internal/base/`, `internal/arena/` to accept `*sqliteq.Queries` instead of `*sql.DB`.

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/db/queries/sqlite/ internal/db/sqliteq/ internal/commands/ internal/server/ internal/player/ internal/skills/ internal/credits/ internal/crafting/ internal/quests/ internal/factions/ internal/enchanting/ internal/weather/ internal/base/ internal/arena/ main.go
git commit -m "refactor: migrate all SQL to sqlc-generated queries"
```

---

## Phase 2: Auth System

### Task 4: Postgres Auth Queries + Auth Package

**Files:**
- Create: `internal/db/queries/postgres/auth.sql`
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

- [ ] **Step 1: Write Postgres auth query file**

Create `internal/db/queries/postgres/auth.sql`:

```sql
-- name: CreateAccount :one
INSERT INTO auth.accounts (username, password_hash, role)
VALUES ($1, $2, $3)
RETURNING id, username, role, banned, created_at;

-- name: GetAccountByUsername :one
SELECT id, username, password_hash, role, banned, created_at, updated_at
FROM auth.accounts WHERE username = $1;

-- name: GetAccountByID :one
SELECT id, username, password_hash, role, banned, created_at, updated_at
FROM auth.accounts WHERE id = $1;

-- name: DeleteAccount :exec
DELETE FROM auth.accounts WHERE username = $1;

-- name: UpdatePassword :exec
UPDATE auth.accounts SET password_hash = $1, updated_at = now() WHERE username = $2;

-- name: SetBanned :exec
UPDATE auth.accounts SET banned = $1, updated_at = now() WHERE username = $2;

-- name: ListAccounts :many
SELECT id, username, role, banned, created_at FROM auth.accounts ORDER BY username;

-- name: CreateSession :one
INSERT INTO auth.sessions (account_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING id, token, expires_at;

-- name: GetSession :one
SELECT s.id, s.account_id, s.token, s.expires_at, a.username, a.role, a.banned
FROM auth.sessions s
JOIN auth.accounts a ON a.id = s.account_id
WHERE s.token = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM auth.sessions WHERE token = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM auth.sessions WHERE expires_at <= now();

-- name: TouchSession :exec
UPDATE auth.sessions SET expires_at = $1 WHERE token = $2;
```

- [ ] **Step 2: Run sqlc generate**

```bash
sqlc generate
```

Expected: `internal/db/pgq/auth.sql.go` generated.

- [ ] **Step 3: Write failing auth test**

Create `internal/auth/auth_test.go`:

```go
package auth_test

import (
    "context"
    "testing"

    "github.com/adam-stokes/gl1tch-mud/internal/auth"
)

func TestHashAndVerify(t *testing.T) {
    hash, err := auth.HashPassword("hunter2")
    if err != nil {
        t.Fatal(err)
    }
    if !auth.CheckPassword("hunter2", hash) {
        t.Error("expected password to match")
    }
    if auth.CheckPassword("wrong", hash) {
        t.Error("expected wrong password to not match")
    }
}

func TestGenerateToken(t *testing.T) {
    tok, err := auth.GenerateToken()
    if err != nil {
        t.Fatal(err)
    }
    if len(tok) != 64 { // 32 bytes hex-encoded
        t.Errorf("expected 64 char token, got %d", len(tok))
    }
}
```

Run: `go test ./internal/auth/ -v`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 4: Implement auth package**

Create `internal/auth/auth.go`:

```go
package auth

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"

    "golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", fmt.Errorf("auth: hash: %w", err)
    }
    return string(hash), nil
}

func CheckPassword(password, hash string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("auth: token: %w", err)
    }
    return hex.EncodeToString(b), nil
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/auth/ -v
```

Expected: PASS.

- [ ] **Step 6: Add go dependencies**

```bash
go get golang.org/x/crypto/bcrypt
go get github.com/jackc/pgx/v5
```

- [ ] **Step 7: Commit**

```bash
git add internal/db/queries/postgres/auth.sql internal/db/pgq/ internal/auth/ go.mod go.sum
git commit -m "feat: add auth package with bcrypt + session tokens, Postgres auth queries"
```

---

### Task 5: CLI Account Management Subcommands

**Files:**
- Create: `internal/auth/cli.go`
- Create: `internal/auth/cli_test.go`
- Modify: `main.go`

- [ ] **Step 1: Write failing test for account creation**

Create `internal/auth/cli_test.go`:

```go
package auth_test

import (
    "context"
    "testing"

    "github.com/adam-stokes/gl1tch-mud/internal/auth"
)

func TestCreateAccountValidation(t *testing.T) {
    tests := []struct {
        name     string
        username string
        password string
        role     string
        wantErr  bool
    }{
        {"valid", "kai", "hunter2", "player", false},
        {"empty username", "", "hunter2", "player", true},
        {"empty password", "kai", "", "player", true},
        {"invalid role", "kai", "hunter2", "superadmin", true},
        {"short password", "kai", "ab", "player", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := auth.ValidateNewAccount(tt.username, tt.password, tt.role)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateNewAccount() err = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

Run: `go test ./internal/auth/ -run TestCreateAccount -v`
Expected: FAIL.

- [ ] **Step 2: Implement validation**

Add to `internal/auth/auth.go`:

```go
func ValidateNewAccount(username, password, role string) error {
    if username == "" {
        return fmt.Errorf("username required")
    }
    if len(password) < 3 {
        return fmt.Errorf("password must be at least 3 characters")
    }
    if role != "admin" && role != "player" {
        return fmt.Errorf("role must be 'admin' or 'player'")
    }
    return nil
}
```

- [ ] **Step 3: Run test**

```bash
go test ./internal/auth/ -v
```

Expected: PASS.

- [ ] **Step 4: Add CLI subcommands to main.go**

Add subcommand handling before the serve/game branch in `main.go`:

```go
if len(os.Args) > 1 {
    switch os.Args[1] {
    case "useradd":
        // Parse flags: --username, --password, --role
        // Validate, hash password, insert via pgq
        runUserAdd(os.Args[2:])
        return
    case "userdel":
        runUserDel(os.Args[2:])
        return
    case "usermod":
        runUserMod(os.Args[2:])
        return
    case "userlist":
        runUserList()
        return
    }
}
```

Implement each function using `pgq.New(pgPool).CreateAccount(...)` etc.

Example `runUserAdd`:

```go
func runUserAdd(args []string) {
    fs := flag.NewFlagSet("useradd", flag.ExitOnError)
    username := fs.String("username", "", "account username")
    password := fs.String("password", "", "account password")
    role := fs.String("role", "player", "account role (admin/player)")
    fs.Parse(args)

    if err := auth.ValidateNewAccount(*username, *password, *role); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }

    pool := mustConnectPG()
    defer pool.Close()

    hash, err := auth.HashPassword(*password)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }

    q := pgq.New(pool)
    acct, err := q.CreateAccount(context.Background(), pgq.CreateAccountParams{
        Username:     *username,
        PasswordHash: hash,
        Role:         *role,
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("created account %q (id: %s, role: %s)\n", acct.Username, acct.ID, acct.Role)
}
```

- [ ] **Step 5: Add Postgres connection helper**

Add to `main.go`:

```go
func mustConnectPG() *pgxpool.Pool {
    url := os.Getenv("DATABASE_URL")
    if url == "" {
        url = "postgres://gl1tch:gl1tch@localhost:5432/gl1tch_mud"
    }
    pool, err := pgxpool.New(context.Background(), url)
    if err != nil {
        fmt.Fprintf(os.Stderr, "postgres: %v\n", err)
        os.Exit(1)
    }
    return pool
}
```

- [ ] **Step 6: Build and verify**

```bash
go build -o gl1tch-mud .
./gl1tch-mud useradd --username test --password test123 --role player
./gl1tch-mud userlist
./gl1tch-mud userdel --username test
```

Expected: accounts created, listed, deleted in Postgres.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/ main.go
git commit -m "feat: add CLI account management (useradd/userdel/usermod/userlist)"
```

---

## Phase 3: Server Auth + World Mode

### Task 6: World Mode Field

**Files:**
- Modify: `internal/world/world.go` — add Mode field to World struct
- Modify: `worlds/mudout/world.yaml` — set `mode: shared`
- Modify: `worlds/cyberspace/world.yaml` — set `mode: solo`
- Modify: `worlds/blockhaven/world.yaml` — set `mode: solo`

- [ ] **Step 1: Add Mode field to World struct**

In `internal/world/world.go`, add to the World struct:

```go
type World struct {
    Name            string           `yaml:"name"`
    Mode            string           `yaml:"mode,omitempty"` // "solo" or "shared", defaults to "solo"
    StartRoom       string           `yaml:"start_room"`
    // ... rest unchanged
}

func (w *World) IsShared() bool {
    return w.Mode == "shared"
}
```

- [ ] **Step 2: Update world YAML files**

Add `mode: shared` to mudout's world.yaml (under the top-level name field).
Add `mode: solo` to cyberspace and blockhaven.

- [ ] **Step 3: Update WorldMeta to include mode**

In the WorldMeta struct and `ListAvailable()`, include mode so the web UI can display it.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/world/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/world/ worlds/
git commit -m "feat: add world mode field (solo/shared), mark mudout as shared"
```

---

### Task 7: Server Auth Handshake

Replace the current playerID+passphrase auth with username+password login and session token resume.

**Files:**
- Modify: `internal/server/protocol.go` — update auth payload types
- Modify: `internal/server/server.go` — add pgPool field, update handleWS
- Modify: `internal/server/session.go` — key by account UUID

- [ ] **Step 1: Update protocol types**

In `internal/server/protocol.go`:

```go
// Replace AuthPayload:
type LoginPayload struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type ResumePayload struct {
    Token string `json:"token"`
}

// Update AuthOKPayload:
type AuthOKPayload struct {
    AccountID string `json:"accountID"`
    Username  string `json:"username"`
    Token     string `json:"token"`
    Role      string `json:"role"`
}
```

- [ ] **Step 2: Update GameServer to hold pgPool**

In `internal/server/server.go`:

```go
type GameServer struct {
    worlds      map[string]*world.World
    lockedWorld string
    registry    *SessionRegistry
    pgPool      *pgxpool.Pool // nil if no Postgres
    httpServer  *http.Server
    // ... rest unchanged
}

func New(worlds map[string]*world.World, lockedWorld string, pgPool *pgxpool.Pool) *GameServer {
    // ...
}
```

- [ ] **Step 3: Update handleWS auth flow**

In `handleWS`, replace the current passphrase check with:

```go
switch first.Type {
case "login":
    var login LoginPayload
    json.Unmarshal(first.Payload, &login)
    // Look up account via pgq.GetAccountByUsername
    // Check banned
    // bcrypt verify password
    // Create session token
    // Return auth.ok with token

case "resume":
    var resume ResumePayload
    json.Unmarshal(first.Payload, &resume)
    // Look up session via pgq.GetSession
    // Check not expired, not banned
    // Touch session expiry
    // Return auth.ok
}
```

- [ ] **Step 4: Update SessionRegistry key**

Change `sessions map[string]*ClientSession` to be keyed by account UUID instead of playerID. Update `ClientSession` to carry `accountID` and `username` fields.

- [ ] **Step 5: Update session.Handle() DB opening**

In `session.go`, the DB opening depends on world mode:

```go
func (s *ClientSession) Handle(ctx context.Context) {
    if s.world.IsShared() {
        // Use Postgres — s.pgPool is set by GameServer
        // s.queries = pgq.New(s.pgPool)
    } else {
        // Use SQLite — existing behavior
        s.database, _ = db.OpenForPlayer(s.accountID, s.worldName)
        // s.queries = sqliteq.New(s.database)
    }
    // ... rest of handle loop
}
```

- [ ] **Step 6: Build and test manually**

Start Postgres via Docker, create an account, start server, connect via web UI.

```bash
docker run -d --name gl1tch-pg -e POSTGRES_DB=gl1tch_mud -e POSTGRES_USER=gl1tch -e POSTGRES_PASSWORD=gl1tch -p 5432:5432 postgres:17
DATABASE_URL=postgres://gl1tch:gl1tch@localhost:5432/gl1tch_mud ./gl1tch-mud useradd --username stokes --password test --role admin
DATABASE_URL=postgres://gl1tch:gl1tch@localhost:5432/gl1tch_mud ./gl1tch-mud --serve --port 8080
```

Open browser to `http://localhost:8080`, login with stokes/test.

- [ ] **Step 7: Commit**

```bash
git add internal/server/ main.go
git commit -m "feat: replace passphrase auth with account login + session tokens"
```

---

### Task 8: Postgres Shared World Queries

**Files:**
- Create: `internal/db/queries/postgres/shared.sql`

- [ ] **Step 1: Write shared world query file**

Create `internal/db/queries/postgres/shared.sql` with queries that mirror the SQLite ones but use Postgres syntax and `$1` parameters, with `account_id` and `world_id` columns.

```sql
-- name: GetSharedPlayerState :one
SELECT room_id, hp, max_hp, credits FROM shared.player_state
WHERE account_id = $1 AND world_id = $2;

-- name: UpsertSharedPlayerState :exec
INSERT INTO shared.player_state (account_id, world_id, room_id, hp, max_hp, credits)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (account_id, world_id)
DO UPDATE SET room_id = $3, hp = $4, max_hp = $5, credits = $6;

-- name: ListSharedInventory :many
SELECT item_id, item_name, item_desc FROM shared.inventory
WHERE account_id = $1 AND world_id = $2;

-- name: AddSharedItem :exec
INSERT INTO shared.inventory (account_id, world_id, item_id, item_name, item_desc)
VALUES ($1, $2, $3, $4, $5);

-- name: RemoveSharedItem :execresult
DELETE FROM shared.inventory
WHERE account_id = $1 AND world_id = $2 AND item_id = $3;

-- name: GetSharedNPCState :one
SELECT hp, alive, respawn_at FROM shared.npc_state
WHERE world_id = $1 AND npc_id = $2;

-- name: UpsertSharedNPCDead :exec
INSERT INTO shared.npc_state (world_id, npc_id, room_id, hp, alive, respawn_at)
VALUES ($1, $2, $3, $4, FALSE, $5)
ON CONFLICT (world_id, npc_id)
DO UPDATE SET hp = $4, alive = FALSE, respawn_at = $5;

-- name: UpsertSharedNPCAlive :exec
INSERT INTO shared.npc_state (world_id, npc_id, room_id, hp, alive)
VALUES ($1, $2, $3, $4, TRUE)
ON CONFLICT (world_id, npc_id)
DO UPDATE SET hp = $4, alive = TRUE, respawn_at = NULL;

-- name: RespawnExpiredNPCs :exec
UPDATE shared.npc_state SET alive = TRUE, respawn_at = NULL
WHERE world_id = $1 AND alive = FALSE AND respawn_at IS NOT NULL AND respawn_at <= now();

-- (Continue for all shared tables — locks, resources, systems, etc.)
-- Follow the same pattern: mirror SQLite queries but with account_id/world_id and $N params.
```

Continue adding queries for every shared table: lock_state, resources, system_state, death_pile, arena_sessions, builds, chests, skills, equipped_armor, quests, world_events, factions, etc.

- [ ] **Step 2: Run sqlc generate**

```bash
sqlc generate
```

Expected: `internal/db/pgq/shared.sql.go` generated with all shared world methods.

- [ ] **Step 3: Commit**

```bash
git add internal/db/queries/postgres/shared.sql internal/db/pgq/
git commit -m "feat: add sqlc queries for Postgres shared world state"
```

---

### Task 9: Unified Query Interface

Create an interface that both sqliteq and pgq satisfy, so command handlers work with either backend.

**Files:**
- Create: `internal/db/gamedb/gamedb.go`

- [ ] **Step 1: Define the interface**

Create `internal/db/gamedb/gamedb.go`:

```go
package gamedb

import "context"

// GameDB is the interface that both sqliteq.Queries and pgq.Queries satisfy
// for game operations. Command handlers accept this instead of a concrete type.
type GameDB interface {
    // Player
    GetPlayer(ctx context.Context) (PlayerRow, error)
    SavePlayer(ctx context.Context, params SavePlayerParams) error
    // ... etc
}
```

The exact shape depends on what sqlc generates. The key: both generated packages expose methods with compatible signatures (same param/return types modulo the generated param structs).

**Alternative approach:** If the generated types diverge too much, use a thin adapter pattern:

```go
type GameDB struct {
    sqlite *sqliteq.Queries // non-nil for solo
    pg     *pgq.Queries     // non-nil for shared
    acctID string           // set for shared worlds
    worldID string
}
```

With wrapper methods that delegate to the right backend. This is more pragmatic than a full interface if sqlc generates different param struct types.

- [ ] **Step 2: Update HandlerFunc signature**

```go
type HandlerFunc func(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result
```

- [ ] **Step 3: Update all call sites**

Session creates the right `GameDB` based on world mode, passes it to handlers.

- [ ] **Step 4: Run all tests**

```bash
go test ./... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/gamedb/ internal/commands/ internal/server/ main.go
git commit -m "feat: unified GameDB interface — handlers work with SQLite or Postgres"
```

---

## Phase 4: Chat System

### Task 10: Chat Commands

**Files:**
- Create: `internal/chat/chat.go`
- Create: `internal/chat/chat_test.go`
- Modify: `internal/commands/commands.go` — register say/shout/whisper
- Modify: `internal/commands/commands.go` — add ChatMessages to Result
- Modify: `internal/server/session.go` — route chat messages

- [ ] **Step 1: Add ChatMessage to Result**

In `internal/commands/commands.go`:

```go
type ChatMessage struct {
    Type   string // "say", "shout", "whisper"
    Sender string
    Target string // whisper target, empty for say/shout
    Body   string
}

type Result struct {
    Output           string
    Event            *Event
    SwitchWorld      string
    MoveRoom         string
    ChatMessages     []ChatMessage // new
    PendingRequestID string
    PendingPlayer    string
}
```

- [ ] **Step 2: Write failing chat test**

Create `internal/chat/chat_test.go`:

```go
package chat_test

import (
    "testing"

    "github.com/adam-stokes/gl1tch-mud/internal/chat"
    "github.com/adam-stokes/gl1tch-mud/internal/commands"
)

func TestSayCommand(t *testing.T) {
    result := chat.Say("testplayer", []string{"hello", "world"})
    if len(result.ChatMessages) != 1 {
        t.Fatalf("expected 1 chat message, got %d", len(result.ChatMessages))
    }
    msg := result.ChatMessages[0]
    if msg.Type != "say" {
        t.Errorf("expected type 'say', got %q", msg.Type)
    }
    if msg.Body != "hello world" {
        t.Errorf("expected body 'hello world', got %q", msg.Body)
    }
    if msg.Sender != "testplayer" {
        t.Errorf("expected sender 'testplayer', got %q", msg.Sender)
    }
}

func TestWhisperNoTarget(t *testing.T) {
    result := chat.Whisper("testplayer", nil)
    if result.Output == "" {
        t.Error("expected usage message")
    }
    if len(result.ChatMessages) != 0 {
        t.Error("expected no chat messages")
    }
}

func TestShoutEmpty(t *testing.T) {
    result := chat.Shout("testplayer", nil)
    if result.Output == "" {
        t.Error("expected usage message for empty shout")
    }
}
```

Run: `go test ./internal/chat/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement chat commands**

Create `internal/chat/chat.go`:

```go
package chat

import (
    "strings"

    "github.com/adam-stokes/gl1tch-mud/internal/commands"
)

func Say(sender string, args []string) commands.Result {
    text := strings.Join(args, " ")
    if text == "" {
        return commands.Result{Output: "say <message>"}
    }
    return commands.Result{
        ChatMessages: []commands.ChatMessage{{
            Type: "say", Sender: sender, Body: text,
        }},
    }
}

func Shout(sender string, args []string) commands.Result {
    text := strings.Join(args, " ")
    if text == "" {
        return commands.Result{Output: "shout <message>"}
    }
    return commands.Result{
        ChatMessages: []commands.ChatMessage{{
            Type: "shout", Sender: sender, Body: text,
        }},
    }
}

func Whisper(sender string, args []string) commands.Result {
    if len(args) < 2 {
        return commands.Result{Output: "whisper <player> <message>"}
    }
    target := args[0]
    text := strings.Join(args[1:], " ")
    return commands.Result{
        Output: "[to " + target + "] " + text,
        ChatMessages: []commands.ChatMessage{{
            Type: "whisper", Sender: sender, Target: target, Body: text,
        }},
    }
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/chat/ -v
```

Expected: PASS.

- [ ] **Step 5: Register chat commands and route messages**

Register in `commands.go` registry. Note: chat commands need the playerID, so they'll be special-cased in the session dispatch (similar to how `say` and `goto` are already handled in `session.go`).

Update `session.go` `dispatchCommand` to check for chat messages in the result and route them:

```go
for _, cm := range result.ChatMessages {
    switch cm.Type {
    case "say":
        // Broadcast to all sessions in same room + same world
        s.registry.BroadcastToRoomInWorld(s.worldName, s.state.RoomID, ServerMsg{
            Type:    "chat.message",
            Payload: ChatMessagePayload{From: cm.Sender, Text: cm.Body},
        })
    case "shout":
        s.registry.BroadcastToWorld(s.worldName, ServerMsg{
            Type:    "chat.message",
            Payload: ChatMessagePayload{From: "[SHOUT] " + cm.Sender, Text: cm.Body},
        })
    case "whisper":
        s.registry.SendToPlayer(cm.Target, ServerMsg{
            Type:    "chat.message",
            Payload: ChatMessagePayload{From: "[from " + cm.Sender + "]", Text: cm.Body},
        })
    }
}
```

- [ ] **Step 6: Add BroadcastToRoomInWorld to SessionRegistry**

```go
func (r *SessionRegistry) BroadcastToRoomInWorld(worldName, roomID string, msg ServerMsg) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for _, s := range r.sessions {
        if s.worldName == worldName && s.state != nil && s.state.RoomID == roomID {
            _ = writeMsg(context.Background(), s.conn, msg)
        }
    }
}
```

- [ ] **Step 7: Commit**

```bash
git add internal/chat/ internal/commands/commands.go internal/server/
git commit -m "feat: add say/shout/whisper chat commands with room/world/DM routing"
```

---

## Phase 5: Admin Commands

### Task 11: Admin Commands

**Files:**
- Create: `internal/commands/admin.go`
- Modify: `internal/server/session.go` — admin command dispatch

- [ ] **Step 1: Implement admin commands**

Create `internal/commands/admin.go`:

```go
package commands

func init() {
    Registry["kick"]      = adminOnly(Kick)
    Registry["ban"]       = adminOnly(Ban)
    Registry["unban"]     = adminOnly(Unban)
    Registry["announce"]  = adminOnly(Announce)
    Registry["who"]       = Who  // available to all, but shows more for admin
    Registry["teleport"]  = adminOnly(Teleport)
    Registry["give"]      = adminOnly(Give)
    Registry["reset"]     = adminOnly(Reset)
}

func adminOnly(h HandlerFunc) HandlerFunc {
    return func(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
        if s.Role != "admin" {
            return Result{Output: "unknown command."}
        }
        return h(gdb, s, w, args)
    }
}
```

Each admin command returns a Result. Server-side actions (kick, ban) use a new `AdminAction` field on Result that the session handler interprets:

```go
type AdminAction struct {
    Type   string // "kick", "ban", "unban", "announce", "teleport", "give", "reset"
    Target string // target username
    Data   string // additional data (item ID, room ID, message, etc.)
}
```

- [ ] **Step 2: Handle admin actions in session dispatch**

In `session.go`, after executing a command, check `result.AdminAction` and perform the server-side operation (disconnect player, update DB, broadcast, etc.).

- [ ] **Step 3: Add Role field to player.State**

```go
type State struct {
    PlayerID  string
    AccountID string
    Role      string // "admin" or "player"
    Name      string
    // ... rest unchanged
}
```

Set from auth during session creation.

- [ ] **Step 4: Test admin commands manually**

Login as admin, run `who`, `kick`, `announce`. Login as player, verify admin commands show "unknown command."

- [ ] **Step 5: Commit**

```bash
git add internal/commands/admin.go internal/server/ internal/player/
git commit -m "feat: add admin commands (kick/ban/who/teleport/give/announce/reset)"
```

---

## Phase 6: Web UI Updates

### Task 12: Login Screen

**Files:**
- Modify: `web/src/lib/mud.ts` — replace playerID login with username/password
- Modify: `web/src/pages/game.astro` — update login form HTML

- [ ] **Step 1: Update login form in game.astro**

Replace the playerID input with username + password fields:

```html
<div id="login-card">
    <h2>gl1tch-mud</h2>
    <input type="text" id="username" placeholder="username" autocomplete="username" />
    <input type="password" id="password" placeholder="password" autocomplete="current-password" />
    <button id="login-btn">Connect</button>
    <p id="login-error" class="error"></p>
</div>
```

- [ ] **Step 2: Update WebSocket auth in mud.ts**

Replace the auth message:

```typescript
// Before:
ws.send(JSON.stringify({ type: 'auth', payload: { playerID, passphrase } }));

// After — login:
ws.send(JSON.stringify({ type: 'login', payload: { username, password } }));

// Or resume with stored token:
const token = localStorage.getItem('gl1tch-token');
if (token) {
    ws.send(JSON.stringify({ type: 'resume', payload: { token } }));
} else {
    ws.send(JSON.stringify({ type: 'login', payload: { username, password } }));
}
```

- [ ] **Step 3: Handle auth.ok with token storage**

```typescript
case 'auth.ok':
    localStorage.setItem('gl1tch-token', msg.payload.token);
    localStorage.setItem('gl1tch-username', msg.payload.username);
    _myPlayerID = msg.payload.username;
    // Show game HUD
    break;

case 'auth.fail':
    localStorage.removeItem('gl1tch-token');
    // Show error, show login form
    break;
```

- [ ] **Step 4: Build and test**

```bash
cd web && npm run build && cd ..
go build -o gl1tch-mud .
```

Test login flow in browser.

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -m "feat: update web UI login to username/password with token resume"
```

---

### Task 13: World Selector with Mode Badges

**Files:**
- Modify: `web/src/pages/index.astro` — show mode badges and online count
- Modify: `internal/server/server.go` — add mode to /api/worlds response

- [ ] **Step 1: Update /api/worlds response**

In `server.go` `handleWorlds`, include mode and online count:

```go
type WorldListEntry struct {
    Name    string          `json:"name"`
    Tagline string          `json:"tagline"`
    Theme   world.WorldTheme `json:"theme"`
    Mode    string          `json:"mode"`    // "solo" or "shared"
    Online  int             `json:"online"`  // connected player count
}
```

- [ ] **Step 2: Update index.astro to show badges**

```typescript
worlds.forEach((w, i) => {
    const badge = w.mode === 'shared' ? '[shared]' : '[solo]';
    const online = w.mode === 'shared' && w.online > 0 ? ` (${w.online} online)` : '';
    // Render: "1. cyberspace [solo]" or "2. mudout [shared] (3 online)"
});
```

- [ ] **Step 3: Build and test**

```bash
cd web && npm run build && cd ..
```

Verify world selector shows mode badges.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/index.astro internal/server/
git commit -m "feat: world selector shows mode badges and online count"
```

---

## Phase 7: Docker Deployment

### Task 14: Dockerfile + Docker Compose

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `internal/pgdb/migrate.go`

- [ ] **Step 1: Create Dockerfile**

```dockerfile
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/web/dist web/dist
RUN CGO_ENABLED=0 go build -o gl1tch-mud .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /app/gl1tch-mud /usr/local/bin/
ENTRYPOINT ["gl1tch-mud"]
CMD ["--serve", "--port", "8080"]
```

- [ ] **Step 2: Create docker-compose.yml**

```yaml
services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: gl1tch_mud
      POSTGRES_USER: gl1tch
      POSTGRES_PASSWORD: gl1tch
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U gl1tch -d gl1tch_mud"]
      interval: 5s
      timeout: 5s
      retries: 5

  gl1tch-mud:
    build: .
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://gl1tch:gl1tch@postgres:5432/gl1tch_mud
    ports:
      - "8080:8080"
    volumes:
      - muddata:/root/.local/share/gl1tch-mud

volumes:
  pgdata:
  muddata:
```

- [ ] **Step 3: Create migration runner**

Create `internal/pgdb/migrate.go`:

```go
package pgdb

import (
    "context"
    "embed"
    "fmt"
    "sort"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
    // Create migrations tracking table
    _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version TEXT PRIMARY KEY,
        applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`)
    if err != nil {
        return fmt.Errorf("pgdb: create migrations table: %w", err)
    }

    // Read and sort migration files
    entries, err := migrations.ReadDir("migrations")
    if err != nil {
        return fmt.Errorf("pgdb: read migrations: %w", err)
    }
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].Name() < entries[j].Name()
    })

    for _, entry := range entries {
        name := entry.Name()
        if !strings.HasSuffix(name, ".sql") {
            continue
        }

        // Check if already applied
        var applied bool
        pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied)
        if applied {
            continue
        }

        // Apply migration
        data, _ := migrations.ReadFile("migrations/" + name)
        if _, err := pool.Exec(ctx, string(data)); err != nil {
            return fmt.Errorf("pgdb: migrate %s: %w", name, err)
        }
        pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name)
    }
    return nil
}
```

- [ ] **Step 4: Create initial migration file**

Create `internal/pgdb/migrations/001_init.sql` combining the auth and shared schema SQL files.

- [ ] **Step 5: Wire migration into server startup**

In `main.go`, when `DATABASE_URL` is set, run migrations before starting the server:

```go
if pgPool != nil {
    if err := pgdb.Migrate(context.Background(), pgPool); err != nil {
        log.Fatalf("migration failed: %v", err)
    }
}
```

- [ ] **Step 6: Test Docker Compose**

```bash
docker compose up --build
```

In another terminal:

```bash
docker compose exec gl1tch-mud gl1tch-mud useradd --username stokes --password test --role admin
```

Open browser to `http://localhost:8080`.

- [ ] **Step 7: Commit**

```bash
git add Dockerfile docker-compose.yml internal/pgdb/
git commit -m "feat: add Docker deployment with Postgres migrations"
```

---

## Phase 8: NPC Respawn (Shared Worlds)

### Task 15: Shared World NPC/Resource Respawn

**Files:**
- Modify: `internal/world/world.go` — add RespawnMinutes field
- Create: `internal/server/respawn.go`

- [ ] **Step 1: Add RespawnMinutes to World**

```go
type World struct {
    // ...
    Mode            string `yaml:"mode,omitempty"`
    RespawnMinutes  int    `yaml:"respawn_minutes,omitempty"` // default 30
    // ...
}
```

- [ ] **Step 2: Create respawn ticker**

Create `internal/server/respawn.go`:

```go
package server

import (
    "context"
    "time"
)

func (gs *GameServer) respawnTicker() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        if !gs.IsRunning() || gs.pgPool == nil {
            return
        }
        ctx := context.Background()
        q := pgq.New(gs.pgPool)
        for name, w := range gs.worlds {
            if !w.IsShared() {
                continue
            }
            q.RespawnExpiredNPCs(ctx, name)
            // Similar for resources
        }
    }
}
```

- [ ] **Step 3: Start ticker on server launch**

In `GameServer.Start()`, add `go gs.respawnTicker()`.

- [ ] **Step 4: Set respawn_at on NPC death**

When an NPC is killed in a shared world, set `respawn_at = now() + respawn_minutes`:

```sql
-- In shared.sql:
-- name: UpsertSharedNPCDead :exec
INSERT INTO shared.npc_state (world_id, npc_id, room_id, hp, alive, respawn_at)
VALUES ($1, $2, $3, $4, FALSE, now() + make_interval(mins => $5))
ON CONFLICT (world_id, npc_id)
DO UPDATE SET hp = $4, alive = FALSE, respawn_at = now() + make_interval(mins => $5);
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/respawn.go internal/world/ internal/db/queries/postgres/
git commit -m "feat: NPC/resource respawn ticker for shared worlds"
```

---

## Phase 9: Integration Test + Final Polish

### Task 16: End-to-End Integration Test

**Files:**
- Create: `integration_test.go`

- [ ] **Step 1: Write integration test**

```go
//go:build integration

package main_test

import (
    "context"
    "encoding/json"
    "net/http/httptest"
    "testing"
    "time"

    "nhooyr.io/websocket"
)

func TestFullLoginFlow(t *testing.T) {
    // 1. Start Postgres testcontainer
    // 2. Run migrations
    // 3. Create test account
    // 4. Start GameServer
    // 5. WebSocket connect
    // 6. Send login message
    // 7. Assert auth.ok response with token
    // 8. Send "look" command
    // 9. Assert output.token response
    // 10. Disconnect
    // 11. Reconnect with token (resume)
    // 12. Assert auth.ok
}

func TestSharedWorldMultiplayer(t *testing.T) {
    // 1. Two accounts, two WebSocket connections to mudout (shared)
    // 2. Player A kills NPC
    // 3. Player B checks NPC — should be dead
    // 4. Player A says "hello"
    // 5. Player B receives chat message
}
```

- [ ] **Step 2: Run integration tests**

```bash
go test -tags integration -v -timeout 60s
```

- [ ] **Step 3: Commit**

```bash
git add integration_test.go
git commit -m "test: add integration tests for login flow and shared world multiplayer"
```

---

### Task 17: CLI Solo Mode Preservation

Verify that CLI single-player mode still works without Postgres.

**Files:**
- Modify: `main.go` — ensure no Postgres panic when DATABASE_URL is unset

- [ ] **Step 1: Test CLI mode without Postgres**

```bash
unset DATABASE_URL
go build -o gl1tch-mud .
./gl1tch-mud
```

Select cyberspace, play normally. Verify no Postgres errors.

- [ ] **Step 2: Test shared world rejection without Postgres**

If a player tries to switch to mudout (shared) without Postgres, show a clear message:

```
"mudout is a shared world and requires the multiplayer server. start with --serve to enable."
```

- [ ] **Step 3: Commit if changes needed**

```bash
git add main.go
git commit -m "fix: graceful handling when shared world accessed without Postgres"
```

---

## Summary

| Phase | Tasks | What It Delivers |
|-------|-------|-----------------|
| 1 | 1-3 | sqlc foundation, all existing SQL migrated |
| 2 | 4-5 | Auth package, CLI account management |
| 3 | 6-9 | World mode, server auth, shared world queries, unified DB interface |
| 4 | 10 | Chat (say/shout/whisper) |
| 5 | 11 | Admin commands |
| 6 | 12-13 | Web UI login + world selector |
| 7 | 14 | Docker Compose deployment |
| 8 | 15 | NPC respawn for shared worlds |
| 9 | 16-17 | Integration tests, CLI preservation |

Each phase produces working, testable software. Phase 1 is the biggest lift (sqlc migration) but doesn't change behavior — just the query layer. Phases 2-7 add new capabilities incrementally.
