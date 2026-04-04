# Gamification System Design

**Date:** 2026-04-03  
**Scope:** `adam-stokes/gl1tch-gamification` plugin + gl1tch-mud integration  
**Status:** Approved

---

## Overview

A game-agnostic gamification plugin (`glitch-gamification`) that acts as the single source of truth for achievements, leaderboards, and player/agent stats. Any game — starting with gl1tch-mud — publishes standardized events to it via BUSD. gl1tch-mud remains fully independent; gamification is optional and fails soft.

Three user-visible features:
1. **Achievements** — unlocked by in-game actions, announced in chat as a glitch message
2. **Game Top** — faction-grouped leaderboard showing players and NPC agents, rendered in chat via `top` command
3. **NPC Agent Activity** — glitch AI, pipelines, and injected NPCs appear as agents in the leaderboard alongside human players

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  Games (gl1tch-mud, future games)                   │
│  • publish game.action events                        │
│  • register achievement catalogs on startup          │
│  • listen for game.achievement.unlocked              │
│  • render top/achievements in their own UI           │
└────────────────────┬────────────────────────────────┘
                     │ BUSD
┌────────────────────▼────────────────────────────────┐
│  glitch-gamification (daemon)                        │
│  • owns leaderboard, faction scores, unlock history  │
│  • evaluates achievements against incoming actions   │
│  • publishes unlocks and top responses back to bus   │
│  • game-agnostic: knows events, not games            │
└────────────────────┬────────────────────────────────┘
                     │ reads (no writes)
┌────────────────────▼────────────────────────────────┐
│  gl1tch core (glitch.db)                            │
│  • existing XP, personal bests, score_events         │
│  • gamification may read these; never duplicates     │
└─────────────────────────────────────────────────────┘
```

**Key constraints:**
- gl1tch-mud has no compile-time dependency on glitch-gamification
- All integration is via BUSD; if the bus or daemon is down, the game runs normally
- NPC agents are protocol-level equals to human players (`agent: true` flag distinguishes them in display only)
- World is metadata on events, not a leaderboard dimension — one global top across both worlds

---

## Plugin Details

**Repo:** `adam-stokes/gl1tch-gamification`  
**Binary:** `glitch-gamification`  
**Manifest:** `glitch-plugin.yaml` (follows gl1tch plugin conventions)  
**Storage:** `~/.local/share/glitch/gamification.db` (own SQLite, WAL mode)  
**Mode:** daemon (`glitch-gamification daemon`)

---

## Event Protocol

### Published by games → gamification

**`game.catalog.register`** — sent once on game startup:
```json
{
  "source": "gl1tch-mud",
  "version": "1.0.0",
  "achievements": [
    {
      "id": "first_blood",
      "name": "First Blood",
      "description": "Win your first combat",
      "trigger": { "action": "combat.won", "count": 1 },
      "xp": 50
    }
  ]
}
```

Catalogs are held in memory; re-registered on daemon restart. Multiple sources can register independently.

**`game.action`** — published on every meaningful player or agent action:
```json
{
  "source": "gl1tch-mud",
  "player": "stokes",
  "faction": "robots",
  "agent": false,
  "action": "combat.won",
  "value": 1,
  "meta": {
    "world": "neon-city",
    "npc": "Stoneling Chieftain",
    "room": "server-core"
  }
}
```

For NPC agents: `"agent": true`, `"player": "glitch"` or `"player": "pipeline:mud-npc-inject"`. Faction defaults to the cwd of the publishing process.

**`game.top.request`** — sent by a player command:
```json
{
  "request_id": "abc123",
  "player": "stokes"
}
```

**`game.achievements.request`** — sent by the `achievements` command:
```json
{
  "request_id": "def456",
  "player": "stokes",
  "source": "gl1tch-mud"
}
```

### Published by gamification → games

**`game.achievement.unlocked`**:
```json
{
  "player": "stokes",
  "achievement_id": "first_blood",
  "name": "First Blood",
  "description": "Win your first combat",
  "xp": 50,
  "source": "gl1tch-mud"
}
```

**`game.top.reply`**:
```json
{
  "request_id": "abc123",
  "entries": [
    {
      "rank": 1,
      "faction": "robots",
      "faction_score": 4821,
      "members": [
        { "name": "stokes", "score": 3100, "agent": false },
        { "name": "glitch", "score": 1721, "agent": true }
      ]
    }
  ]
}
```

**`game.achievements.reply`**:
```json
{
  "request_id": "def456",
  "player": "stokes",
  "unlocked": [
    { "id": "first_blood", "name": "First Blood", "description": "Win your first combat" }
  ],
  "in_progress": [
    { "id": "cartographer", "name": "Cartographer", "description": "Visit 50 rooms", "progress": 12, "total": 50 }
  ]
}
```

In-progress entries are achievements whose `action_counts[player][trigger.action] > 0` but have not yet met the threshold.

---

## glitch-gamification Schema

```sql
-- One row per player/agent, updated on every action
CREATE TABLE players (
    id           TEXT PRIMARY KEY,
    faction      TEXT,
    score        INTEGER DEFAULT 0,
    action_count INTEGER DEFAULT 0,
    is_agent     INTEGER DEFAULT 0,  -- 0=human, 1=NPC/pipeline/agent
    last_seen    INTEGER              -- unix timestamp
);

-- Faction leaderboard, derived from player faction fields
CREATE TABLE factions (
    id           TEXT PRIMARY KEY,
    score        INTEGER DEFAULT 0,
    member_count INTEGER DEFAULT 0,
    last_active  INTEGER
);

-- Unlocked achievements, idempotent inserts
CREATE TABLE unlocked (
    player         TEXT NOT NULL,
    achievement_id TEXT NOT NULL,
    source         TEXT NOT NULL,
    unlocked_at    INTEGER,
    PRIMARY KEY (player, achievement_id)
);

-- Action counts per player per action type, drives achievement evaluation
CREATE TABLE action_counts (
    player TEXT NOT NULL,
    action TEXT NOT NULL,
    count  INTEGER DEFAULT 0,
    PRIMARY KEY (player, action)
);
```

### Daemon processing loop

On `game.action`:
1. Upsert `players` (score += value, action_count++)
2. Upsert `factions` (score += value)
3. Upsert `action_counts` (count++)
4. For each registered achievement from this source: if `action_counts[player][trigger.action] >= trigger.count` and not already in `unlocked` → insert `unlocked`, publish `game.achievement.unlocked`

On `game.top.request`:
1. Query top 10 factions by score
2. For each faction, query members ordered by score
3. Publish `game.top.reply` with `request_id`

Achievement evaluation is count-based only. No complex rule expressions in v1.

---

## gl1tch-mud Changes

### 1. Event adapter (`internal/busd/gamification.go`)

Thin translation layer, publishes `game.action` after existing mud events:

| Mud event | game.action |
|---|---|
| `mud.combat.ended` (outcome=won) | `combat.won` |
| `mud.combat.ended` (outcome=lost) | `combat.lost` |
| `mud.hack.success` | `hack.success` |
| `mud.trade.completed` | `trade.completed` |
| `mud.craft.completed` | `craft.completed` |
| `mud.lock.picked` | `lock.picked` |
| `mud.room.entered` (first=true) | `room.explored` |
| `mud.player.died` | `player.died` |

Faction resolved from player's current faction in DB; falls back to `"unaffiliated"`.

### 2. Achievement catalog (`achievements.yaml`)

Defines gl1tch-mud's achievements. Published via `game.catalog.register` on `GameServer.Start()`. Silent no-op if bus unavailable.

### 3. New BUSD subscriptions in `GameServer`

Alongside existing `mud.chat.reply` subscription:

- **`game.achievement.unlocked`** → broadcast to the relevant player as a glitch chat message
- **`game.top.reply`** → route to the player who issued the `top` command (matched by request_id)
- **`game.achievements.reply`** → route to the player who issued the `achievements` command (matched by request_id)

### 4. New commands

**`top`** — publishes `game.top.request`, waits up to 2s for reply, renders in chat. Prints `"gamification offline"` if no reply.

**`achievements`** — publishes `game.achievements.request` (new topic), waits for `game.achievements.reply`, renders unlocked + in-progress.

No structural changes to command registry or session handling beyond adding these two commands.

---

## UI Rendering (in chat window)

**Achievement unlock** (pushed to player immediately):
```
gl1tch  » achievement unlocked: First Blood
         win your first combat · +50xp
```

**`top` command output**:
```
gl1tch  » ── game top ──────────────────
           # FACTION        SCORE  MEMBERS
           1 robots          4821    2 (1 agent)
             · stokes        3100
             · glitch †      1721
           2 observability   2340    1
             · stokes        2340
           3 unaffiliated     890    1
             · pipeline †     890
           † = agent
```

**`achievements` command output**:
```
gl1tch  » ── your achievements ─────────
           ✓ First Blood    — first combat win
           ✓ Ghost          — 10 successful hacks
             Cartographer   — visit 50 rooms (12/50)
```

Locked achievements with count triggers show progress. All output fits standard terminal width.

---

## NPC Agent Integration

NPC agents (glitch AI, pipeline runs, injected NPCs) publish `game.action` with `agent: true`. They are:

- Indistinguishable from humans at the protocol level
- Displayed with `†` suffix in game top
- Counted in faction member totals with `(N agent)` annotation
- Eligible for achievements (glitch can unlock "First Blood" too)

The glitch AI publishes when it responds to a chat mention. Pipelines publish when they complete a run. No changes to the gamification daemon are required — they're just another source.

---

## Extensibility

Any future text-based game:
1. Implements event adapter (publishes `game.action`)
2. Provides `achievements.yaml`
3. Subscribes to `game.achievement.unlocked` and `game.top.reply`

No changes to `glitch-gamification` required. The daemon is fully game-agnostic.
