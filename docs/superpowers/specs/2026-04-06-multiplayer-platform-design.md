# gl1tch-mud Multiplayer Platform Design

**Date:** 2026-04-06
**Status:** Approved
**Approach:** C — Postgres for accounts + shared worlds, SQLite for solo worlds

## Overview

Turn gl1tch-mud into a self-contained multiplayer platform for family and friends. Postgres (Docker) handles user accounts, auth, and shared world state. Solo worlds keep existing per-player SQLite. Deployed on a home LAN server.

## Deployment

- **Target:** Home LAN (no public internet, no TLS)
- **Runtime:** Docker Compose — Postgres 17 + gl1tch-mud binary
- **CLI mode preserved:** Without `DATABASE_URL`, game runs solo-only (no Postgres needed)

## 1. User Accounts & Auth

### Postgres Schema (`auth` schema)

```sql
CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,  -- bcrypt
    role TEXT NOT NULL DEFAULT 'player',  -- 'admin' or 'player'
    banned BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,  -- crypto/rand hex
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Account Management (CLI)

Parent-managed. Admin creates all accounts from the terminal:

```bash
gl1tch-mud useradd --username kai --password hunter2 --role player
gl1tch-mud useradd --username stokes --password xxx --role admin
gl1tch-mud userdel --username kai
gl1tch-mud usermod --username kai --password newpass
gl1tch-mud usermod --username kai --ban
gl1tch-mud usermod --username kai --unban
gl1tch-mud userlist
```

Requires `DATABASE_URL` to be set.

### Auth Flow

1. Client opens WebSocket, sends `{type: "login", username, password}`
2. Server validates credentials (bcrypt compare), checks not banned
3. On success: creates session row, returns `{type: "auth_ok", token, account_id}`
4. Client stores token in localStorage
5. On reconnect: client sends `{type: "resume", token}` — server validates session, skips login
6. Sessions expire after 7 days of inactivity
7. CLI single-player mode bypasses auth entirely

### Admin Commands (In-Game)

Only `role=admin` accounts. Players see "unknown command."

| Command | Description |
|---------|-------------|
| `kick <player>` | Disconnect a player |
| `ban <player>` | Disable account, disconnect |
| `unban <player>` | Re-enable account |
| `announce <msg>` | Server-wide broadcast |
| `who` | All connected players + locations |
| `teleport <player> <room>` | Move a player |
| `give <player> <item>` | Grant items |
| `reset <player>` | Wipe player progress (with confirmation) |

## 2. Shared vs Solo Worlds

### World YAML Meta

```yaml
meta:
  name: mudout
  mode: shared        # "shared" or "solo"
  description: "The wasteland awaits..."
```

- **cyberspace** — solo
- **blockhaven** — solo
- **mudout** — shared

### Solo Worlds (mode: solo)

Current behavior, unchanged:
- Per-player SQLite DB at `~/.local/share/gl1tch-mud/players/<accountID>/<worldName>.db`
- All state personal (NPCs, quests, loot, locks, resources)
- No player visibility — you're alone
- Path uses account UUID instead of old playerID string

### Shared Worlds (mode: shared)

State lives in Postgres, keyed by world name:
- NPCs, locks, resources, systems — shared by all players
- Inventory, skills, credits, position — per-player in the shared world
- When someone kills an NPC — dead for everyone
- When someone hacks a system — hacked for everyone
- When a resource is mined — depleted for everyone

#### NPC Respawn

Shared worlds need respawn so the world doesn't get permanently emptied:

```yaml
meta:
  name: mudout
  mode: shared
  respawn_minutes: 30  # NPCs/resources respawn after 30 min (default: 30)
```

### Postgres Schema (`shared` schema)

```sql
CREATE SCHEMA IF NOT EXISTS shared;

-- Per-player state in shared worlds
CREATE TABLE shared.player_state (
    account_id UUID NOT NULL REFERENCES auth.accounts(id),
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    hp INTEGER NOT NULL DEFAULT 100,
    max_hp INTEGER NOT NULL DEFAULT 100,
    credits INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared.inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES auth.accounts(id),
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_desc TEXT NOT NULL DEFAULT ''
);

CREATE TABLE shared.player_skills (
    account_id UUID NOT NULL REFERENCES auth.accounts(id),
    world_id TEXT NOT NULL,
    skill TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 1,
    xp INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, skill)
);

CREATE TABLE shared.equipped_armor (
    account_id UUID NOT NULL REFERENCES auth.accounts(id),
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    defense INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

-- Shared world state (same for all players)
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
    account_id UUID NOT NULL REFERENCES auth.accounts(id),
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
    placed_by UUID REFERENCES auth.accounts(id),
    placed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (world_id, room_id, build_id)
);

CREATE TABLE shared.chests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    stored_by UUID REFERENCES auth.accounts(id)
);
```

## 3. Chat System

Three tiers, all via WebSocket. Only works in shared worlds + LAN mode.

| Command | Scope | Output |
|---------|-------|--------|
| `say <msg>` | Same room | `[kai] hey check this out` |
| `shout <msg>` | Same world | `[SHOUT] kai: anyone found the keycard?` |
| `whisper <player> <msg>` | Direct | `[to kai] meet at ruins-3` / `[from stokes] meet at ruins-3` |

### Implementation

- Chat commands registered in command registry like any other command
- `Result` struct gains `ChatMessages []ChatMessage` field:
  ```go
  type ChatMessage struct {
      Type   string // "say", "shout", "whisper"
      Sender string
      Target string // whisper target username, empty for say/shout
      Body   string
  }
  ```
- Session loop checks for chat messages in result, routes via `SessionRegistry`:
  - `say` → broadcast to all sessions in same room + same world
  - `shout` → broadcast to all sessions in same world
  - `whisper` → send to target player's session
- No chat history persistence. Fire-and-forget over WebSocket.

## 4. Database & Storage Architecture

### Postgres (Docker)

- Single `gl1tch_mud` database with `auth` and `shared` schemas
- Migrations: numbered SQL files embedded in binary via `internal/pgdb/`
- Connection: `DATABASE_URL` env var, defaults to `postgres://gl1tch:gl1tch@localhost:5432/gl1tch_mud`
- Driver: `pgx/v5` (pure Go, no CGO)
- Query layer: `sqlc` for type-safe generated Go from SQL

### SQLite (solo worlds, unchanged)

- Path: `~/.local/share/gl1tch-mud/players/<accountID>/<worldName>.db`
- Account UUID replaces old playerID string
- Same schema, same `modernc.org/sqlite` driver

### Query Layer: sqlc

All database access uses **sqlc** — write SQL, generate type-safe Go code. No ORM, no runtime reflection.

- `internal/db/sqlc/` — generated code lives here
- `internal/db/queries/` — `.sql` query files (source of truth)
- `internal/db/schema/` — schema `.sql` files for sqlc to parse
- Two sqlc targets: `sqlite` (solo worlds) and `postgres` (auth + shared worlds)
- Command handlers receive a `*db.Queries` (generated by sqlc) instead of raw `*sql.DB`
- Both SQLite and Postgres queries generate to the same Go interface where possible
- `sqlc.yaml` config at project root

**Migration from raw SQL:** Existing `db.Query/QueryRow/Exec` calls across command handlers and `internal/player/` get replaced with generated sqlc methods. This is the bulk of the migration work but pays off in compile-time safety.

**Dialect handling:** sqlc generates separate code per engine. Shared query signatures where SQL is compatible; engine-specific where not (e.g., `INSERT OR REPLACE` vs `ON CONFLICT DO UPDATE`).

### Docker Compose

```yaml
services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: gl1tch_mud
      POSTGRES_USER: gl1tch
      POSTGRES_PASSWORD: gl1tch
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  gl1tch-mud:
    build: .
    depends_on: [postgres]
    environment:
      DATABASE_URL: postgres://gl1tch:gl1tch@postgres:5432/gl1tch_mud
    ports:
      - "8080:8080"
    volumes:
      - muddata:/data
    command: ["--serve", "--port", "8080"]

volumes:
  pgdata:
  muddata:
```

## 5. Web UI Changes

### Login Screen

- Username + password form, "Connect" button
- On success, stores session token in localStorage
- Auto-reconnects with stored token on refresh/disconnect

### World Selector

- Shows after login, before entering game
- Lists available worlds with mode badge: `cyberspace [solo]`, `mudout [shared]`
- Shared worlds show online count: `mudout [shared] (3 online)`
- Player picks world, WebSocket connects: `?world=mudout&token=<session_token>`

### In-Game

- Chat messages render with distinct styling (color/prefix) from game output
- `/who` output shows online players in current world
- Shared world indicator in status area
- No graphical overhaul — stays terminal-style

## 6. Code Changes

### Packages That Change

| Package | Change |
|---------|--------|
| `internal/db/` | Migrate to sqlc-generated queries |
| `internal/commands/` | Handler signature: `*sql.DB` → sqlc `*Queries` |
| `internal/server/` | Auth handshake, session registry keyed by account UUID, chat routing |
| `internal/session/` | World mode awareness, store injection |
| `main.go` | Postgres setup, migration runner, `useradd`/`userdel`/`usermod`/`userlist` subcommands |

### New Packages

| Package | Purpose |
|---------|---------|
| `internal/db/queries/` | sqlc SQL query files (SQLite + Postgres) |
| `internal/pgdb/` | Postgres connection, embedded migrations |
| `internal/auth/` | Account CRUD, bcrypt hashing, session tokens |
| `internal/chat/` | say/shout/whisper handlers + message routing |

### What Doesn't Change

- Game logic packages (arena, craft, hacking, locking, skills, etc.)
- World YAML format (just adding `mode` to meta)
- CLI single-player mode (no Postgres needed)
- Pipeline system, BUSD integration
- Build/release process (adding Dockerfile + compose)

## 7. Migration Strategy

- No data migration. Fresh Postgres = fresh shared worlds.
- Existing solo SQLite DBs keep working — path changes from playerID to accountID.
- Players start fresh on shared mudout (new mode, expected).
- Future: solo worlds can migrate to Postgres if desired (schema is compatible).
