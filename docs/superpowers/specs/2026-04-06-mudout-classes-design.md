# Mudout — Class System & Onboarding Design

**Date:** 2026-04-06
**World:** mudout
**Status:** Approved (brainstorm)

---

## Overview

Mudout currently drops new players into `dusthaven-0` with no class, no starting kit, and no guided introduction. This spec adds:

1. A **5-class system** themed to the wasteland (Gunslinger, Mechanic, Scavver, Medic, Ghoul)
2. A **point-buy starting kit** (10 kit points per class) so the player can customize their loadout
3. A **guided onboarding journey** that teaches the basic verbs, hands the player to the existing Settler quest chain, and is balanced to take a fresh player from level 0 to level 2 in their signature skill

The class system is intentionally light-mechanical: each class gives a starting skill bonus, a unique signature verb, a curated kit list, and a mentor NPC. Classes are *not* gated skill trees — a Mechanic can still level combat, a Gunslinger can still level crafting. Class is a head start on flavor and competence, not a permanent box.

---

## The Five Classes

| Class | Archetype | Signature Verb | Skill Bonus (start) | Faction Lean |
|---|---|---|---|---|
| **Gunslinger** | Dark Tower hero, old-world iron in a chrome world | `fan` — multi-shot attack, costs 2 ammo, hits twice | +1 combat | neutral |
| **Mechanic** | Tinker who keeps dead tech alive | `hotwire` — bypass an electronic lock or system if you have parts in inventory | +1 crafting | neutral |
| **Scavver** | The "gets things" class — barter, lift, find | `barter` — better trade prices at vendors; `lift` — steal a single item from an NPC's inventory (skill check) | +1 scavenging, +1 trading | neutral |
| **Medic** | Chems, bandages, ex-Vault medic | `stim` — heal 25 HP and gain temporary +1 damage for 5 actions | +1 survival | neutral |
| **Ghoul** | Irradiated wastelander, eats junk, immune to rads | `rad-feed` — consume an irradiated junk item to heal HP; immune to radiation hazards in all biomes | +1 scavenging | **+10 Ghoul Collective, −10 Settlers** at start |

### Signature verb unlocks

Signature verbs are class-only at character creation, but **any class can unlock any signature verb by reaching level 3 in the matching gating skill**:

| Verb | Gating Skill |
|---|---|
| `fan` | combat |
| `hotwire` | crafting |
| `barter` | trading |
| `lift` | scavenging |
| `stim` | survival |
| `rad-feed` | scavenging |

This preserves long-term build freedom while making the first hour feel distinct.

### Faction-rep starting penalty (Ghoul only)

Ghoul characters start with `+10` rep with `ghoul-collective` and `-10` rep with `settlers`. Rep gating ships in this spec (see **Faction Rep Gating** below), and the Ghoul's `-10` Settler rep is what makes the Settler path harder: it bars them from a few low-tier Settler quests until they grind back to neutral, while opening Ghoul Collective quests immediately. Other classes start at 0 rep with all factions.

---

## Starting Kit: 10 Kit Points

Each class has a curated list of 8–10 items. The player spends up to 10 points during character creation. Every class list includes one **free 0-point quest hook item** — a memento that ties into a future quest line (not implemented in this spec).

### Gunslinger kit list

| Item | Cost | Effect |
|---|---|---|
| Worn revolver | 3 | 1d6 damage, sidearm |
| Lever rifle | 5 | 1d8 damage, two-handed |
| Speedloader x2 | 1 | Faster reload |
| Patched duster | 2 | +2 defense |
| Tin star badge | 1 | Bluffs Settler NPCs |
| Whiskey flask | 1 | Heal 10 HP, 3 charges |
| Bone-handle knife | 2 | 1d4 backup melee |
| Trail rations | 1 | Heal 5 HP, 5 charges |
| Lucky bullet | 4 | One guaranteed crit |
| **Faded photograph** | 0 | Quest hook (free) |

### Mechanic kit list

| Item | Cost | Effect |
|---|---|---|
| Pipe pistol | 3 | 1d6 damage, sidearm |
| Wrench | 2 | 1d6 melee, doubles as crafting tool |
| Welding mask | 2 | +1 defense, immune to flash effects |
| Toolkit | 3 | Required for `hotwire` on advanced systems |
| Scrap metal x5 | 1 | Crafting reagent |
| Pipe parts x3 | 1 | Crafting reagent |
| Spare battery | 2 | Powers `hotwire` once without consuming parts |
| Goggles | 1 | +1 perception in dust storms |
| Jury-rig kit | 4 | One-time field repair (restore broken weapon) |
| **Burnt service medal** | 0 | Quest hook (free) |

### Scavver kit list

| Item | Cost | Effect |
|---|---|---|
| Sawed-off shotgun | 4 | 1d8 damage, point-blank only |
| Switchblade | 2 | 1d4 melee, concealable (+1 to `lift` checks) |
| Lockpicks x3 | 1 | Required for picking physical locks |
| Fake creds | 2 | Bluffs Ironclad NPCs once |
| Lucky charm | 2 | +1 to first `lift` per day |
| Big satchel | 1 | +5 inventory slots |
| Rad-x x2 | 1 | Temporary rad resistance |
| Bottle caps x50 | 2 | Starting currency |
| Smoke pellet | 3 | Auto-escape one combat encounter |
| **Pawned wedding ring** | 0 | Quest hook (free) |

### Medic kit list

| Item | Cost | Effect |
|---|---|---|
| Service pistol | 3 | 1d6 damage, sidearm |
| Scalpel | 1 | 1d4 melee, also crafting tool for chems |
| Lab coat | 1 | +1 defense, marks player as medic to NPCs |
| Stimpak x3 | 2 | Heal 25 HP each |
| Bandages x5 | 1 | Heal 10 HP, stops bleeding |
| Med-X | 2 | Temporary +2 damage resistance, 10 actions |
| Buffout | 2 | Temporary +20 max HP, 10 actions |
| Empty syringes x5 | 1 | Crafting reagent for new chems |
| Vault-Tec ID | 2 | Bluffs Vault-related NPCs once |
| **Pre-war photograph (family)** | 0 | Quest hook (free) |

### Ghoul kit list

| Item | Cost | Effect |
|---|---|---|
| Bone club | 2 | 1d6 melee |
| Salvaged pistol | 3 | 1d6 damage, sidearm |
| Tattered hood | 1 | +1 defense, hides ghoul features (lessens Settler hostility) |
| Glowing trinket | 2 | Lights dark rooms, marks player as Ghoul Collective |
| Rad-chunks x5 | 1 | `rad-feed` reagent — heal 15 HP each |
| Junk food x5 | 1 | Heal 5 HP each (irradiated; non-ghouls take damage) |
| Old-world coin pouch | 2 | 30 bottle caps starting currency |
| Reinforced wrappings | 2 | +1 defense, +1 fire resistance |
| Ancient grudge | 4 | One-time +5 damage on first attack vs Ironclad NPCs |
| **Pre-war locket** | 0 | Quest hook (free) |

The 10-point budget is tuned so a typical loadout = 1 weapon + 1 armor + 2–3 consumables + the free hook. This is enough to clear the first 2–3 XP levels of content comfortably without trivializing them.

---

## The Onboarding Journey

### New room: `wakeup-camp`

A scavenger camp tucked under a collapsed billboard just outside the Gate. **All five mentors share this one room.** This is simpler than five class-flavored wake rooms and makes the world feel populated on first impression.

```
WAKEUP CAMP
A ring of oil-drum fires under a sun-bleached billboard. Five drifters
have made this their meeting spot — each one looking for a particular
kind of recruit. Through the chain-link to the south, you can see the
gates of Dusthaven.

Here: Old Cass (gunslinger), Rust (mechanic), Halftrack (scavver),
      Doc Vega (medic), Mother Ash (ghoul)
Exits: [ S: The Gate ]
```

The room intro narration is class-aware: the line "Your mentor looks up as you approach" names the matching mentor based on the player's class.

### The five mentors

| Class | Mentor | Voice |
|---|---|---|
| Gunslinger | **Old Cass** | Retired gunslinger, drawl, missing two fingers |
| Mechanic | **Rust** | One-eyed tinker, mutters in schematics |
| Scavver | **Halftrack** | One-legged trader, talks fast, never lies (technically) |
| Medic | **Doc Vega** | Former Vault medic, clinical, dry humor |
| Ghoul | **Mother Ash** | Ancient ghoul, speaks in riddles, the only mentor who calls Dusthaven "the new town" |

Each mentor is an NPC with a scripted dialogue tree (not LLM-generated) so the tutorial is deterministic and testable.

### Character creation flow

When a new player loads (no row in `player` table), they are spawned into `wakeup-camp` and the **character wizard** takes over the session. The wizard is a finite-state machine, not free-form chat:

1. **Name** — `name <text>`
2. **Class** — `class list` shows all 5 with flavor + bonuses + signature verb; `class pick <id>` confirms
3. **Kit** — `kit list` shows the class's items with costs and remaining points; `kit add <item>` / `kit remove <item>` mutate the loadout; `kit done` confirms (only if total ≤ 10)
4. **Confirm** — `start` finalizes the character

On `start`:
- Class is written to `player.class`
- Kit items are added to inventory
- Skill bonuses are applied (`scavenging`, `combat`, `crafting`, `trading`, or `survival`)
- Faction rep deltas are applied (Ghoul only)
- The matching mentor's dialogue tree begins automatically

### Mentor handoff (~3 minutes of guided play)

Mentor dialogue is scripted to teach four commands and the signature verb in order. The tutorial is room-local and cannot be skipped on first playthrough (a `tutorial_complete` flag prevents it from re-firing on subsequent logins).

| Step | Mentor line | Teaches |
|---|---|---|
| 1 | "Take stock of yourself, runner." | `inventory` |
| 2 | "Have a look around." | `look` |
| 3 | "That crate by my feet — give it a once-over." | `examine` |
| 4 | "Try this on for size: `<signature verb>`." | the class signature verb on a safe target in the room |
| 5 | "Now get to The Gate. Marta's hiring." | sets `tutorial_complete = true`, points south |

## Faction Rep Gating

The existing `player_reputation` table (`faction TEXT, value INTEGER`) already tracks per-faction rep, and `gamedb.GetReputation` / `gamedb.IncrementReputation` exist. This spec extends that system so quests can require a minimum rep with their giving faction.

### Schema additions

No new tables. One new gamedb helper:

- `AdjustReputation(faction string, delta int) error` — adds an arbitrary signed delta (positive or negative). Used by character creation to apply Ghoul's `-10 settlers / +10 ghoul-collective`, and by future quest rewards that grant or revoke rep.

(`IncrementReputation` stays as-is for callers that only need `+1`.)

### Quest definition additions

The quest YAML schema gains two optional fields:

```yaml
- id: settlers-intro
  title: "Prove Yourself"
  giver_npc_id: settler-recruiter
  giver_faction: settlers      # NEW — which faction this NPC speaks for
  min_rep: 0                   # NEW — minimum rep with giver_faction to receive this quest
  ...
```

If `min_rep` is omitted, the quest is offered to anyone (current behavior). If `giver_faction` is omitted, no rep check happens.

### Quest accept logic

In the quest acceptance code path (where Marta currently hands out `settlers-intro`):

1. If `quest.giver_faction` is empty → offer normally.
2. Otherwise, fetch `gamedb.GetReputation(quest.giver_faction)`.
3. If `current_rep < quest.min_rep` → the NPC refuses with a flavor line: `"Marta eyes you and shakes her head. 'I don't deal with your kind. Not yet.'"`
4. Otherwise → offer normally.

### Mudout quest rep tags

For this spec, the two existing Settler quests get `giver_faction: settlers, min_rep: 0`. That means a baseline player (rep 0) can take them, but a Ghoul (rep -10) cannot until they grind to 0. To give Ghoul players an immediate path forward, this spec adds **one new starter quest** for the Ghoul Collective:

```yaml
- id: ghoul-intro
  title: "Old Bones"
  description: "Mother Ash wants you to retrieve a glowing relic from the Collapsed Mall — an old-world thing the Ironclad would kill to own."
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

This introduces a third optional field on quests: `reward_rep_faction` + `reward_rep_delta`, applied via `AdjustReputation` on quest completion. (Existing quests can adopt it later; not required for this spec.)

Mother Ash already exists in `wakeup-camp` as the Ghoul mentor, so she doubles as the quest giver. The `glowing-relic` item lives in `ruins-0` (Collapsed Mall) — just a single new YAML item, no new room.

### Levels 0 → 2 math (Ghoul path)

| Source | Skill | XP |
|---|---|---|
| `ghoul-intro` reward | scavenging | 25 |
| 2× raider grunt encounters (any biome) | combat | 60 |
| 1× scrap container | scavenging | 20 |
| 1× rad-feed exchange (use signature verb on rad-chunks) | scavenging | 30 |
| Kills inside Collapsed Mall while retrieving the relic | combat | 40 |

Total: ~175 XP. Hits level 2 in scavenging cleanly without needing the Settler chain.

### First real quests (already exist)

The player walks south from `wakeup-camp` into `dusthaven-0` and meets **Marta** (the existing `settler-recruiter` NPC), who hands out the existing quest chain:

- `settlers-intro` — scavenge 3 scrap metal in the Barrens (+25 scavenging XP)
- `settlers-defense` — kill 2 raider scouts (+50 combat XP)

No new quest content is required for this spec.

### Levels 0 → 2 math

Current XP thresholds (`internal/skills/skills.go`): level 1 = 50 XP, level 2 = 150 XP, level 3 = 300 XP.

| Source | Skill | XP |
|---|---|---|
| `settlers-intro` reward | scavenging | 25 |
| `settlers-defense` reward | combat | 50 |
| 2× raider grunt encounters in Barrens | combat | 60 |
| 2× scrap containers in Barrens | scavenging | 40 |
| 1× hotwired terminal OR lifted item OR similar class-flavored interaction | varies | 30 |

Total: roughly 205 XP spread across 1–2 signature skills. Any class will comfortably hit **level 2** in their signature skill by completing the Settler chain plus normal exploration of the Barrens. Level 3 (300 XP) is reachable for players who push a little further.

---

## Engine Changes

| Component | Path | Notes |
|---|---|---|
| `Class` field on `player.State` | `internal/player/player.go` | string, persisted in DB |
| `class` and `tutorial_complete` columns on `player` table | new migration under `internal/db/migrations/` | both default to empty/false |
| Class registry | `internal/classes/classes.go` (new package) | declares all 5 classes, their kit lists, signature verbs, skill bonuses, and starting faction-rep deltas |
| Character creation wizard | `internal/commands/character.go` (new) | session-bound finite-state machine; commands `name`, `class`, `kit`, `start` |
| `kit` command UI | same file | `kit list`, `kit add <item>`, `kit remove <item>`, `kit done` |
| `wakeup-camp` room | `internal/world/defaults/mudout/world.yaml` | one new room, five mentor NPCs, scripted dialogue tree, exit south to `dusthaven-0` |
| `start_room: wakeup-camp` | `internal/world/defaults/mudout/world.yaml` | new players only; existing players keep their saved `room_id` from the DB |
| Signature verbs: `fan`, `hotwire`, `barter`, `lift`, `stim`, `rad-feed` | `internal/commands/commands.go` | each verb checks `class == X OR skills.Level(gatingSkill) >= 3` |
| Mentor dialogue trees | `internal/world/defaults/mudout/world.yaml` (NPC dialogue field) or new `internal/dialogue/` package if YAML grows unwieldy | scripted, deterministic, testable |
| `AdjustReputation(faction, delta)` helper | `internal/db/gamedb/gamedb.go` | signed delta; used by char creation and quest rewards |
| Quest YAML fields: `giver_faction`, `min_rep`, `reward_rep_faction`, `reward_rep_delta` | `internal/quests/quests.go` (struct) + loader + accept/complete logic | all four optional, backward-compatible |
| Quest accept rep check | `internal/commands/commands.go` (or wherever quest accept is wired) | refuses with NPC flavor line if rep too low |
| Quest reward rep apply | quest completion path | calls `AdjustReputation` if reward fields set |
| `ghoul-intro` quest + `glowing-relic` item | `internal/world/defaults/mudout/world.yaml` | new quest, new item in `ruins-0` |
| Two existing Settler quests tagged with `giver_faction: settlers, min_rep: 0` | `internal/world/defaults/mudout/world.yaml` | minimal edit |

---

## Out of Scope

The following are deliberately excluded from this spec to keep it shippable:

- Balancing exact weapon damage values across all kit items
- Expanding kit lists beyond ~10 items per class
- The risk roll mechanics for `lift` (will use a simple `scavenging vs target-level` check; full design deferred)
- Class-specific quest content beyond the existing Settler chain
- Multiclass / class-change mechanics (single class for life in v1)
- Class-specific death penalties or respawn rules
- The future quest lines that consume the free hook items (faded photograph, burnt service medal, etc.)

---

## Open Decisions Resolved

| Question | Decision |
|---|---|
| One shared `wakeup-camp` room or five class-flavored wake rooms? | **One shared room** with five mentors |
| Does Ghoul start with a Settler rep penalty? | **Yes** (`-10` Settlers, `+10` Ghoul Collective) |
| Are signature verbs class-locked forever? | **No** — unlockable for any class at skill level 3 in the gating skill |
| Mechanical depth of class system? | **Light mechanical tilt** — skill bonus, signature verb, kit, mentor; no exclusive skill trees |
| Number of classes? | **Five** |
| Customization model? | **Point-buy, 10 kit points per class** |
| Faction rep gating? | **In scope** — quest YAML gains `giver_faction`/`min_rep`/`reward_rep_*` fields, plus a Ghoul-friendly starter quest so the rep penalty is meaningful from action one |
