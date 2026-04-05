# Mudout Sub-Project 2 — Equip System Design

**Date:** 2026-04-05
**Sub-project:** 2 of 4 — Armor equip system
**Status:** Approved

---

## Overview

Crafted armor (assembled via the `leather-armor` recipe and future armor recipes) does nothing until equipped. Sub-project 2 adds the equip layer: a single armor slot, persisted to DB, that reduces incoming combat damage by the item's `damage_resist` stat.

This sub-project does NOT cover base/structure slots — those belong to sub-project 3.

---

## Architecture

Three changes to three layers:

1. **DB** — new `equipped_armor` table (single-row)
2. **Engine** — `wear`/`unwear`/`equipment` commands; combat damage reduction; `player.State.Defense`
3. **Protocol** — `StateUpdatePayload.EquippedArmor` sent to client each tick

---

## Section 1: Database

New table added to `internal/db/schema.go`:

```sql
CREATE TABLE IF NOT EXISTS equipped_armor (
    id        INTEGER PRIMARY KEY CHECK(id = 1),
    item_id   TEXT    NOT NULL,
    item_name TEXT    NOT NULL,
    defense   INTEGER NOT NULL DEFAULT 0
);
```

Single-row table (id always 1). Armor is equipped by upsert; unequipped by delete. No migration needed — existing schema init creates it on first run.

---

## Section 2: Player State

`internal/player/player.go` — add `Defense int` to `State`:

```go
type State struct {
    PlayerID string
    Name     string
    RoomID   string
    HP       int
    MaxHP    int
    World    string
    Defense  int  // summed from equipped armor damage_resist; 0 if nothing equipped
}
```

New functions in `player.go`:

```go
// EquipArmor upserts the equipped armor record.
func EquipArmor(db *sql.DB, itemID, itemName string, defense int) error

// UnequipArmor removes the equipped armor record.
func UnequipArmor(db *sql.DB) error

// LoadDefense reads the current equipped armor defense value into state.
// Called after LoadForWorld to populate state.Defense.
func LoadDefense(db *sql.DB, s *State)

// EquippedArmorItem returns the currently equipped armor item, or nil.
type EquippedArmorRecord struct {
    ItemID   string
    ItemName string
    Defense  int
}
func GetEquippedArmor(db *sql.DB) (*EquippedArmorRecord, error)
```

---

## Section 3: Combat

In `internal/commands/commands.go`, the `Attack` function currently does:

```go
s.HP -= npc.Attack
```

Change to:

```go
dmg := npc.Attack - s.Defense
if dmg < 1 {
    dmg = 1
}
s.HP -= dmg
```

Armor can never reduce damage below 1 — no invincibility.

---

## Section 4: Commands

Three new commands added to `internal/commands/commands.go`:

### `wear <item-id>` / `equip <item-id>`
- Finds item in player inventory by ID
- Item must have `armor` tag (checked via world item lookup)
- If armor already equipped: return old item to inventory first
- Upsert new armor into `equipped_armor`, using item's `Stats["damage_resist"]` as defense
- Update `s.Defense`
- Output: `"you put on the Leather Armor. [DEF +3]"`

### `remove armor` / `unwear`
- Reads `equipped_armor` record
- Returns item to inventory (`AddItem`)
- Deletes `equipped_armor` record
- Sets `s.Defense = 0`
- Output: `"you remove the Leather Armor."`

### `equipment` / `eq`
- Reads `equipped_armor` record
- Output: `"ARMOR: Leather Armor [DEF 3]"` or `"nothing equipped."`

---

## Section 5: Protocol

`internal/server/protocol.go` — add to `StateUpdatePayload`:

```go
EquippedArmor *EquippedArmorInfo `json:"equipped_armor,omitempty"`
```

```go
type EquippedArmorInfo struct {
    ItemID   string `json:"item_id"`
    ItemName string `json:"item_name"`
    Defense  int    `json:"defense"`
}
```

Populated in `sendStateUpdate` from `player.GetEquippedArmor`.

The web client (`mud.ts`) reads `state.equipped_armor` and displays it in the wasteland status line: `DEF: 3`.

---

## Section 6: Client (mud.ts)

Add `equipped_armor` to the `StateUpdate` interface:

```typescript
interface EquippedArmorInfo {
  item_id: string;
  item_name: string;
  defense: number;
}

interface StateUpdate {
  // ... existing fields ...
  equipped_armor?: EquippedArmorInfo;
}
```

In wasteland mode, display defense in the status line alongside credits:

```
CAPS: 120  DEF: 3
```

If nothing equipped: `DEF: 0`.

---

## Commands Wired

```
"wear":      Wear,
"equip":     Wear,
"unwear":    Unwear,
"remove":    Unwear,   // args checked: must be "armor"
"equipment": Equipment,
"eq":        Equipment,
```

---

## Scope Boundary

- Structure slots (foundation/walls/roof/upgrade) → sub-project 3
- Multiple armor slots (helmet, legs, arms) → future enhancement
- Weapon equip (active weapon affecting attack damage) → not in this sub-project
