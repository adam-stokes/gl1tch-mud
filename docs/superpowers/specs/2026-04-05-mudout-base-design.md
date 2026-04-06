# Mudout — Player Base System Design

**Date:** 2026-04-05
**Sub-project:** 3 of 4 — Player Base System
**Status:** Approved

---

## Overview

Adds a permanent player base at `dusthaven-4` (The Base Plots). Players build structures using wasteland materials, accruing a defense score. While offline, Ash Raider events periodically threaten the base; outcomes are computed from the defense score and reported on next login.

Builds on sub-projects 1 (world YAML) and 2 (armor equip/assembly). No client-side changes required.

---

## Architecture

Three-layer change:

1. **World YAML** — six build recipes added to `mudout/world.yaml` with `workbench: "build"`. Defense values stored in recipe output `stats: {defense: N}`.
2. **`internal/base` package** — `MaybeSpawnRaid`, `ResolvePendingRaids`, `DefenseScore`. Pure functions over `*sql.DB` and `*world.World`.
3. **Session integration** — `session.go` calls `ResolvePendingRaids` on mudout session start (shows login raid report) and `MaybeSpawnRaid` after each mudout command.

Reuses existing infrastructure: `builds` table, `world_events` table, `chests` table, `stash`/`unstash` commands, `world_events` CRUD in `internal/events`.

---

## Structure Recipes

All recipes use `workbench: "build"` and `type: ingredient`. Ingredients are sourced from existing wasteland loot tables.

| Recipe ID | Name | Ingredients | `stats.defense` |
|---|---|---|---|
| `foundation` | Base Foundation | 5× scrap-iron, 3× polymer-sheet | 0 |
| `base-walls` | Reinforced Walls | 4× scrap-iron, 2× polymer-sheet | 3 |
| `base-roof` | Corrugated Roof | 3× polymer-sheet, 2× scrap-iron | 1 |
| `chest` | Storage Locker | 3× scrap-iron, 2× copper-wire | 0 |
| `base-generator` | Diesel Generator | 4× copper-wire, 2× pre-war-circuitry | 2 |
| `base-turret` | Sentry Turret | 5× scrap-iron, 3× copper-wire, 2× pre-war-circuitry | 5 |

**Max defense score:** 11 (all structures built).

`chest` uses the existing ID so `stash`/`unstash` commands work at `dusthaven-4` immediately after building — no new code needed for storage.

---

## Raid System

### Spawning

`base.MaybeSpawnRaid(db *sql.DB, w *world.World)` is called from `session.go` after every command in the mudout world. It spawns a raid event when all three conditions hold:

1. `actionCount(db) % 30 == 0`
2. No active `world_events` row with `type='base-raid'` and `target_room='dusthaven-4'` exists
3. At least one structure is built: `SELECT COUNT(*) FROM builds WHERE room_id='dusthaven-4'` > 0

Raid event inserted into `world_events`: `type='base-raid'`, `target_room='dusthaven-4'`, `faction='ash-raiders'`, `expires_actions=30`, `status='active'`.

### Resolution

`base.ResolvePendingRaids(db *sql.DB, w *world.World) string` is called at mudout session start, before the first `sendStateUpdate`. It:

1. Queries all `world_events` where `type='base-raid'` AND `target_room='dusthaven-4'` AND `status='active'` AND `expires_actions + created_actions <= actionCount(db)`.
2. Calls `base.DefenseScore(db, w)` — queries `builds WHERE room_id='dusthaven-4'`, looks up each `build_id` in `w.CraftingRecipes`, sums `recipe.Output.Stats["defense"]`.
3. Rolls `raidStrength` as `rand.Intn(15) + 1` (range 1–15).
4. If `DefenseScore >= raidStrength`: raid repelled — no item loss.
5. If `DefenseScore < raidStrength`: raid breaks through — deletes up to 3 random rows from `chests WHERE room_id='dusthaven-4'`, records lost item names.
6. Marks each event `status='resolved'`.
7. Returns a narrative report string (empty string if no raids were pending).

### Sample Report Output

Repelled:
```
RAID REPORT: Ash Raiders hit your base while you were gone.
Raid strength: 8  |  Your defense: 11
Your defenses held. Nothing was taken.
```

Broken through:
```
RAID REPORT: Ash Raiders hit your base while you were gone.
Raid strength: 12  |  Your defense: 4
Raiders broke through. Lost: Canned Food, Scrap Iron.
```

No structures built: raids do not spawn (condition 3).

---

## Commands

### `baseinfo` / `mybase`

Shows structures built at `dusthaven-4`, total defense score, chest item count, and actions until next raid check.

```
BASE STATUS — The Base Plots
───────────────────────────
  foundation     Base Foundation       [DEF  0]
  base-walls     Reinforced Walls      [DEF  3]
  chest          Storage Locker        [DEF  0]
───────────────────────────
  DEFENSE SCORE: 3 / 11 max
  CHEST ITEMS: 2
  Next raid check: ~27 actions
```

If nothing is built:
```
BASE STATUS — The Base Plots
No structures built. Head to dusthaven-4 and use 'build' to start.
```

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Create | `internal/base/base.go` | `DefenseScore`, `MaybeSpawnRaid`, `ResolvePendingRaids` |
| Create | `internal/base/base_test.go` | Tests for all three functions |
| Modify | `internal/commands/commands.go` | `BaseInfo` command + registry wiring |
| Create | `internal/commands/baseinfo_test.go` | Tests for `BaseInfo` command |
| Modify | `internal/server/session.go` | Call `ResolvePendingRaids` on mudout start; call `MaybeSpawnRaid` after each mudout command |
| Modify | `internal/world/defaults/mudout/world.yaml` | 6 build recipes; update `dusthaven-4` desc to remove stub note |

---

## Testing

- `TestDefenseScore` — builds table with known structures, verifies sum
- `TestDefenseScoreEmpty` — no structures, score = 0
- `TestMaybeSpawnRaid_spawns` — action count multiple of 30, structure exists, no active raid → event inserted
- `TestMaybeSpawnRaid_noSpawnWithoutStructures` — no structures → no event inserted
- `TestMaybeSpawnRaid_noSpawnIfActiveRaid` — active raid exists → no duplicate inserted
- `TestResolvePendingRaids_repelled` — defense >= raid strength → no chest items removed, report contains "held"
- `TestResolvePendingRaids_broken` — defense < raid strength → chest items removed, report contains "broke through"
- `TestResolvePendingRaids_noPending` — no expired raids → empty string returned
- `TestBaseInfoCommand` — builds in dusthaven-4, verify output contains defense score

---

## Constraints

- Raids only spawn if the player has started building (at least 1 structure in `dusthaven-4`). No harassment of new players.
- Maximum 3 chest items lost per raid (bounded loss).
- Raid strength range 1–15 vs max defense 11 means a fully-built base cannot be 100% guaranteed safe — a strength-12+ raid will break through even with everything built. This is intentional; the turret (DEF 5) is the biggest single investment.
- No client changes required. Raid report delivered as a plain server message string.
