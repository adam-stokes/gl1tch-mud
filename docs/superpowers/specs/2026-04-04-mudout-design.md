# Mudout — World Design Spec

**Date:** 2026-04-04
**Sub-project:** 1 of 4 — World YAML + UI
**Status:** Approved

---

## Overview

Mudout is a Fallout 4-inspired open-world sandbox MUD added as a new world alongside the existing `cyberspace` and `blockhaven` worlds. It is loaded via the standard `world.Load("mudout")` path and ships as an embedded default at `internal/world/defaults/mudout/world.yaml`.

The world targets a wasteland-serious tone: dark, bleak, survival-focused, but not gratuitously gory. Death is "you go down hard", not permadeath. Players pick a faction and build permanent bases in an open, persistent world.

**This spec covers sub-project 1 only.** Subsequent sub-projects (extended mod slots, player bases, arena mini-games) build on the rooms, factions, and crafting vocabulary defined here.

---

## World Identity

```yaml
name: mudout
tagline: "the dust never settles."
ui:
  prompt: ">"
  profile: wasteland
  banner: |
    ███╗   ███╗██╗   ██╗██████╗  ██████╗ ██╗   ██╗████████╗
    ████╗ ████║██║   ██║██╔══██╗██╔═══██╗██║   ██║╚══██╔══╝
    ██╔████╔██║██║   ██║██║  ██║██║   ██║██║   ██║   ██║
    ██║╚██╔╝██║██║   ██║██║  ██║██║   ██║██║   ██║   ██║
    ██║ ╚═╝ ██║╚██████╔╝██████╔╝╚██████╔╝╚██████╔╝   ██║
  theme:
    bg:      "#0d0d00"
    fg:      "#d4a017"
    accent:  "#ff6600"
    dim:     "#5a4a00"
    border:  "#3a3000"
    error:   "#cc2200"
    success: "#7ab648"
narrator_model: claude-haiku-4-5-20251001
start_room: dusthaven-0
```

---

## UI Profile: `wasteland`

The `wasteland` profile is a peer to `kids` (Blockhaven) — same `ui_profile` field in `WorldMetaPayload`, different client-side skin.

| Element | kids (Blockhaven) | wasteland (Mudout) |
|---|---|---|
| Palette | warm parchment/gold | amber-on-black CRT |
| Prompt | `$` | `>` |
| Death text | "knocked out" | "you go down hard" |
| HP display | friendly bar | Pip-Boy-style bar: `[██████░░░░]` |
| Tone | encouraging, soft | terse, survival-focused |
| Font weight | rounded, readable | monospace, terminal |

**Client renders:**
- Status line: `RAD: 0  CAPS: 120  FACTION: Settlers  REP: +12` (RAD is a future mechanic; always shows 0 in sub-project 1)
- HP bar: green → yellow → red as HP drops
- Room header: all-caps, amber accent border
- Exits: compass-style `[ N: Ashfield | E: Burnt Overpass ]`
- Inventory grouped: `WEAPONS / ARMOR / JUNK / FOOD`
- Map: same BFS grid as Blockhaven, amber dots, current room `■`
- Error: red, prefixed `⚠`
- Success: green, prefixed `✓`

---

## Weather Table

```yaml
weather_table:
  - biome: settlement
    possible: [clear, overcast, ashfall]
  - biome: wasteland
    possible: [clear, ashstorm, radiation-fog, scorching]
  - biome: ruins
    possible: [clear, dead-calm, radiation-fog, tremor]
```

---

## Zones & Rooms

### Zone 1: Dusthaven Settlement (biome: settlement)

| Room ID | Name | Notes |
|---|---|---|
| `dusthaven-0` | The Gate | Start room. Faction recruiters, traders. |
| `dusthaven-1` | The Scrapyard | Workbench: weapons. Loot: gun parts, scrap. |
| `dusthaven-2` | The Workshop | Workbench: armor, structures. Crafting hub. |
| `dusthaven-3` | The Saloon | NPC quests, faction rep, info trading. |
| `dusthaven-4` | BASE-PLOT-STUB | Placeholder for player base system (sub-project 3). |

### Zone 2: The Barrens (biome: wasteland)

| Room ID | Name | Notes |
|---|---|---|
| `barrens-0` | The Ashfield | Open scavenging, raider patrols. |
| `barrens-1` | Burnt Overpass | Ambush zone, high loot risk/reward. |
| `barrens-2` | The Crater | Rare resource mine, faction conflict point. |
| `barrens-3` | ARENA-STUB | Placeholder for TDM/CTF arena (sub-project 4). |

### Zone 3: The Ruins (biome: ruins)

| Room ID | Name | Notes |
|---|---|---|
| `ruins-0` | Collapsed Mall | Heavy enemies, armor components. |
| `ruins-1` | The Vault Door | Locked, quest-gated, faction reward. |
| `ruins-2` | Vault Interior | Endgame room, faction resolution. |
| `ruins-3` | ARENA-STUB | Placeholder for tower defense arena (sub-project 4). |

---

## Factions

### The Settlers (`settlers`)
- **Desc:** Survivors who pooled resources to build Dusthaven. They want walls, clean water, and to be left alone.
- **Agenda:** Fortify the settlement, push raiders out of the Barrens.
- **Territory:** `dusthaven-0` — `dusthaven-3`
- **Allies:** `ironclad` | **Enemies:** `ash-raiders`

### The Ash Raiders (`ash-raiders`)
- **Desc:** Nomadic scavengers who claim the Barrens by force. They don't build — they take.
- **Agenda:** Raid Dusthaven, control the Crater, burn the Vault.
- **Territory:** `barrens-0`, `barrens-1`
- **Allies:** none | **Enemies:** `settlers`, `ironclad`

### The Ironclad (`ironclad`)
- **Desc:** Ex-military remnant, obsessed with reclaiming pre-war tech from the Vault. Rigid, disciplined, ruthless.
- **Agenda:** Open the Vault, recover the pre-war weapons cache, establish martial law.
- **Territory:** `barrens-2`, `ruins-0` — `ruins-2`
- **Allies:** `settlers` (uneasy) | **Enemies:** `ash-raiders`, `ghoul-collective`

### The Ghoul Collective (`ghoul-collective`)
- **Desc:** Irradiated survivors living in the deep ruins. Misunderstood, ancient, sitting on secrets.
- **Agenda:** Keep outsiders out of the Vault — it's not a weapons cache, it's their home.
- **Territory:** `ruins-1`, `ruins-2`
- **Allies:** none | **Enemies:** `ironclad`

**Faction join:** Player picks a faction at `dusthaven-0` (or stays neutral at rep cost). Faction rep gates trades, quests, and eventually the Vault door.

**Contested territory:** `ruins-1` and `ruins-2` are claimed by both Ironclad and Ghoul Collective. Both factions have NPCs present; combat between them is active. This is intentional — the Vault door is the conflict point.

---

## Crafting: Assembly Slot System

Extends the existing `assembly` recipe type to cover all three item categories.

### Weapons (6 slots)

| Slot ID | Required | Tag | Notes |
|---|---|---|---|
| `frame` | yes | `gun-frame` | existing |
| `barrel` | yes | `gun-barrel` | existing |
| `receiver` | yes | `gun-receiver` | new |
| `stock` | no | `gun-stock` | existing |
| `grip` | no | `gun-grip` | new |
| `scope` | no | `gun-scope` | new |
| `muzzle` | no | `gun-muzzle` | new |

### Armor (4 slots, new category)

| Slot ID | Required | Tag | Stat |
|---|---|---|---|
| `base` | yes | `armor-base` | damage resist base |
| `lining` | no | `armor-lining` | radiation resist, weight |
| `plating` | no | `armor-plating` | damage resist bonus |
| `pockets` | no | `armor-pockets` | carry weight |

### Base Structures (stub — sub-project 3)

| Slot ID | Required | Tag |
|---|---|---|
| `foundation` | yes | `structure-foundation` |
| `walls` | no | `structure-walls` |
| `roof` | no | `structure-roof` |
| `upgrade` | no | `structure-upgrade` (turret, generator, storage) |

---

## Loot Tables

| Table ID | Zone | Contents |
|---|---|---|
| `settlement-loot` | dusthaven | food, scrap metal, cloth, pipe parts |
| `wasteland-loot` | barrens | gun components, ammo casings, radaway, junk |
| `ruins-loot` | ruins | pre-war tech, armor plates, rare receivers, vault keys |

## Resources (minable)

| Room | Resource | Tool |
|---|---|---|
| `barrens-2` | `scrap-iron`, `copper-wire`, `radiation-crystal` (rare) | pickaxe |
| `ruins-0` | `polymer-sheet`, `pre-war-circuitry` | salvage tool |

---

## Subsequent Sub-Projects

| # | Name | Depends On |
|---|---|---|
| 2 | Extended mod slots (armor + structure upgrades in engine) | sub-project 1 |
| 3 | Player base system (permanent plots, persistent rooms in DB) | sub-projects 1, 2 |
| 4 | Arena mini-games (tick-based TDM, CTF, tower defense, hide & seek) | sub-project 1 |

### Arena mini-game design notes (for sub-project 4)
- Rooms `barrens-3` and `ruins-3` are arena entry points
- Tick-based resolution: players submit actions, tick resolves all simultaneously
- Turn-based hybrid: each tick is a "round" with a countdown
- Game types: TDM, CTF, tower defense, hide & seek
- AI NPCs fill out teams to minimum player counts
- Win conditions vary per game type; rewards are loot + faction rep

### Player base design notes (for sub-project 3)
- `dusthaven-4` is the base plot zone
- Each player owns one plot (permanent, DB-persisted)
- Bases built via structure assembly slots defined in sub-project 2
- Structures: walls, roof, storage, generator, turret
- Base can be upgraded over time; not destructible by other players (PvE-only threat: raider events)
