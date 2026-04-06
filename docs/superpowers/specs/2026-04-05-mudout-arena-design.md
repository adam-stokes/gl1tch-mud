# Mudout — Arena Mini-Games Design

**Date:** 2026-04-05
**Sub-project:** 4 of 4 — Arena Mini-Games
**Status:** Approved

---

## Overview

Adds two single-player-vs-AI arena game modes to the Mudout world:

- **TDM (Team Deathmatch)** — `barrens-3`: kill 5 Ash Raiders before they kill you
- **Tower Defense** — `ruins-3`: survive 3 waves of raiders; your `dusthaven-4` base defense score fires as auto-turrets each wave

Builds on sub-project 3 (base defense score reused as turret damage in tower defense).

---

## Architecture

Three-layer change:

1. **DB** — new `arena_sessions` single-row-per-player table (only one active match at a time)
2. **`internal/arena` package** — `StartTDM`, `StartTowerDefense`, `GetActive`, `ProcessAttack`, `Quit`; all arena logic isolated here
3. **Commands** — `Arena` command (start/status/quit); `Attack` command checks for active arena match and routes to `arena.ProcessAttack` instead of room NPCs; `Result` struct gains `MoveRoom string` for arena-loss teleport; session.go handles `MoveRoom`

---

## DB Schema

```sql
CREATE TABLE IF NOT EXISTS arena_sessions (
    id              TEXT    PRIMARY KEY,
    game_type       TEXT    NOT NULL,
    phase           TEXT    NOT NULL DEFAULT 'fight',
    wave            INTEGER NOT NULL DEFAULT 0,
    enemies_json    TEXT    NOT NULL DEFAULT '[]',
    reward_credits  INTEGER NOT NULL DEFAULT 0,
    reward_item_id  TEXT    NOT NULL DEFAULT '',
    reward_item_name TEXT   NOT NULL DEFAULT '',
    reward_item_desc TEXT   NOT NULL DEFAULT '',
    status          TEXT    NOT NULL DEFAULT 'active',
    started_at      INTEGER NOT NULL DEFAULT 0
);
```

One row per active or recent match. Only `status='active'` rows are acted upon; old rows are overwritten when a new match starts (INSERT OR REPLACE).

---

## Arena Package

### Types

```go
type Enemy struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    HP     int    `json:"hp"`
    Attack int    `json:"attack"`
    Alive  bool   `json:"alive"`
}

type Match struct {
    ID              string
    GameType        string  // "tdm" | "tower-defense"
    Phase           string  // "fight" | "wave"
    Wave            int     // tower defense: 0-indexed current wave (0–2)
    Enemies         []Enemy
    RewardCredits   int
    RewardItemID    string
    RewardItemName  string
    RewardItemDesc  string
    Status          string  // "active" | "won" | "lost"
    StartedAt       int64
}

type AttackResult struct {
    Output  string
    Won     bool
    Lost    bool
}
```

### Functions

| Function | Description |
|---|---|
| `StartTDM(db) error` | Creates active TDM match: 5 raiders HP 30 attack 8, reward 200 caps |
| `StartTowerDefense(db) error` | Creates active TD match: wave 0, 3 raiders HP 25 attack 6, reward 300 caps + pre-war-circuitry |
| `GetActive(db) *Match` | Returns active match or nil |
| `ProcessAttack(db, w, s) AttackResult` | One attack tick; mutates enemies, applies turret auto-damage on wave start; saves match; returns output + won/lost flags |
| `Quit(db) string` | Marks match lost, returns confirmation message |
| `SpawnWave(wave int) []Enemy` | Returns 3 fresh raiders for TD wave (called internally) |

### ProcessAttack Logic

**TDM path:**
1. Player deals 15 damage to first living enemy
2. All living enemies counterattack: `max(1, enemy.Attack - s.Defense)` each
3. If all dead → mark won, deposit reward via `credits.Add`, add reward item to inventory
4. If player HP ≤ 0 → mark lost
5. Return output + won/lost

**Tower Defense path:**
1. If all current wave enemies are dead → clear wave, spawn next wave, apply turret auto-damage, heal player +15 HP, return wave-clear output
2. Otherwise: player deals 15 damage to first living enemy; surviving enemies counterattack
3. If all waves (0, 1, 2) cleared → mark won, deposit reward, add item to inventory
4. If player HP ≤ 0 → mark lost

**Turret auto-damage (tower defense, on wave spawn):**
`defScore := base.DefenseScore(db, w)` divided evenly across 3 enemies (integer division; remainder applied to enemy index 0). Applied immediately when wave starts — before any player action.

---

## Commands

### `Arena` command

Registered as `"arena"`.

Behaviour by context:

| Condition | Action |
|---|---|
| No active match, room = `barrens-3` | `arena.StartTDM(db)`, print match-started output |
| No active match, room = `ruins-3` | `arena.StartTowerDefense(db)`, print match-started output |
| No active match, other room | Return "find an arena entrance first." |
| Active match, args = `["quit"]` | `arena.Quit(db)`, teleport to `dusthaven-0`, heal to 50% max HP |
| Active match, no args | Show match status (game type, enemies remaining, wave if TD) |

### `Attack` command modification

At the top of `Attack`, before the existing NPC lookup:

```go
if match := arena.GetActive(db); match != nil {
    res := arena.ProcessAttack(db, w, s)
    if res.Lost {
        s.HP = s.MaxHP / 2
        s.RoomID = "dusthaven-0"
        player.Save(db, s) //nolint:errcheck
        return Result{Output: res.Output + "\nyou've been knocked out. back to dusthaven."}
    }
    return Result{Output: res.Output}
}
// ... existing NPC attack code follows unchanged
```

### `Result` struct — new `MoveRoom` field

```go
type Result struct {
    Output           string
    Event            *ResultEvent
    SwitchWorld      string
    MoveRoom         string  // NEW: if non-empty, session moves player to this room ID
    PendingRequestID string
    PendingPlayer    string
}
```

### Session.go — handle MoveRoom

After `result := handler(...)`, add alongside the existing `SwitchWorld` check:

```go
if result.MoveRoom != "" {
    s.state.RoomID = result.MoveRoom
    _ = player.Save(s.database, s.state)
}
```

---

## Sample Output

**TDM start:**
```
COMBAT ZONE — TDM
5 Ash Raiders. Kill them all.
Reward: 200 caps.
Type 'attack' to engage. 'arena quit' to forfeit.
```

**TDM attack tick:**
```
you fire at Ash Raider. [15 dmg → 15 HP]
Ash Raider retaliates for 7. your HP: 63/100.
Ash Raider retaliates for 7. your HP: 56/100.
--- 5 enemies remaining ---
```

**TDM win:**
```
you fire at Ash Raider. [15 dmg → dead]
--- all enemies down. match won. ---
+200 caps deposited.
```

**Tower Defense wave start (defense score 6):**
```
WAVE 1 — turrets fire.
  Ash Raider A: -2 [23 HP]
  Ash Raider B: -2 [23 HP]
  Ash Raider C: -2 [23 HP]
3 enemies remaining. type 'attack' to engage.
```

**Tower Defense wave clear:**
```
Wave 1 cleared. +15 HP. [HP: 86/100]
--- Wave 2 incoming ---
```

**Tower Defense win:**
```
Wave 3 cleared.
--- all waves survived. match won. ---
+300 caps deposited.
pre-war-circuitry added to inventory.
```

**Loss:**
```
you've been knocked out. back to dusthaven.
```

---

## World YAML changes

`barrens-3` and `ruins-3` desc updated to remove "coming soon" and describe the active arena. Items (flyers/manuals) remain for flavour. No structural room changes needed.

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Modify | `internal/db/schema.go` | Add `arena_sessions` table |
| Modify | `internal/db/schema_test.go` | Add `arena_sessions` to table list |
| Create | `internal/arena/arena.go` | All arena logic |
| Create | `internal/arena/arena_test.go` | Full arena test suite |
| Modify | `internal/commands/commands.go` | `Arena` command; `Attack` arena intercept; `MoveRoom` on Result; registry wiring |
| Create | `internal/commands/arena_test.go` | Arena and Attack-intercept tests |
| Modify | `internal/server/session.go` | Handle `result.MoveRoom` |
| Modify | `internal/world/defaults/mudout/world.yaml` | Update barrens-3 and ruins-3 desc |

---

## Testing

- `TestStartTDM` — match created with 5 alive enemies, status active
- `TestStartTowerDefense` — match created, wave=0, 3 enemies, phase=wave
- `TestGetActive_none` — returns nil when no active match
- `TestProcessAttack_TDM_damagesEnemy` — first enemy HP reduced by 15
- `TestProcessAttack_TDM_enemiesCounterattack` — player HP reduced by surviving enemy attacks
- `TestProcessAttack_TDM_win` — all enemies at 1 HP; one attack → won=true, credits deposited
- `TestProcessAttack_TDM_loss` — player HP=1, enemy attack > 1 → lost=true
- `TestProcessAttack_TD_turretsDamageOnWaveStart` — defense score applied to enemy HP at wave start
- `TestProcessAttack_TD_waveClear` — kill 3 enemies → wave increments, new enemies spawned, player +15 HP
- `TestProcessAttack_TD_win` — survive 3 waves → won=true, credits + item deposited
- `TestQuit` — match marked lost, message returned
- `TestArenaCommand_startTDM` — in barrens-3, no match → TDM started
- `TestArenaCommand_startTD` — in ruins-3, no match → TD started
- `TestArenaCommand_showStatus` — active match → status output
- `TestAttackIntercept` — active match → arena.ProcessAttack called, not room NPC

---

## Constraints

- Player damage is flat 15 (no weapon stats involved — arena is self-contained)
- Only one active arena match at a time per player
- `arena` command only starts matches at `barrens-3` or `ruins-3`; usable from any room to check status or quit
- Tower defense auto-turret damage uses `base.DefenseScore(db, w)` — always reads `dusthaven-4` builds
- Max turret auto-damage per enemy is uncapped (a maxed base score of 11 with 3 raiders = 3–4 damage each, not a one-shot)
- Rewards use existing `credits.Add` and `player.AddItem` functions
