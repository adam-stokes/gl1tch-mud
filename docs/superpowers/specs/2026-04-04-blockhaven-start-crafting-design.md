# Blockhaven Start Journey — Crafting Enhancement Design Spec

**Date:** 2026-04-04
**Status:** Approved

---

## Context

The Blockhaven start journey currently drops the player in meadow-0 with a wooden pickaxe and wooden sword but no clear path to better gear before the first boss fight (Stoneling Chieftain, hp:60 attack:12). Key gaps:

- meadow-0 has no resources to gather or mine
- No stone-sword recipe — jump from wooden (5 atk) to iron sword (12 atk, workbench gated) is too steep
- No wooden-axe recipe — wood-log loop requires forest-1 which requires an axe
- No guided tutorial — Brix says "type craft" but gives no progression
- Assembly system exists but is undiscoverable without guidance

This spec adds: early-game resources in meadow-0, four new recipes, and a 4-step tutorial quest chain (`q-first-forge`) through Brix that teaches gather → craft → smelt → assembly before the player hits the main quest wall.

---

## Part 1: New Resources

### meadow-0 — Tree Stump

Add a `tree-stump` harvest resource to meadow-0. No tool required. Gives players a wood source at spawn before they have an axe.

```yaml
resources:
  - id: tree-stump
    type: harvest
    yields:
      - item_id: wood-log
        name: "Wood Log"
        desc: "A rough log pulled from an old stump."
        probability: 1.0
        count_min: 1
        count_max: 2
    tool_required: ""
    respawn_actions: 20
```

---

## Part 2: New Recipes

Added to `crafting_recipes` in world.yaml. All four are appended to the existing list.

| Recipe ID | Name | Ingredients | Workbench | Purpose |
|---|---|---|---|---|
| `wooden-axe` | Wooden Axe | wood-log x3 + stick x2 | none | Unlocks forest-1 ancient oak harvest |
| `stone-sword` | Stone Sword | stone x2 + stick x1 | none | Fills atk gap between wooden (5) and iron (12) sword |
| `stone-axe` | Stone Axe | stone x3 + stick x2 | none | Faster wood harvest than wooden-axe |
| `crude-iron-chest` | Crude Iron Chestplate | iron-ingot x4 | workbench | Light armor (3 defense); q-first-forge step 3 reward |

```yaml
  - id: wooden-axe
    name: "Wooden Axe"
    ingredients:
      - {id: wood-log, count: 3}
      - {id: stick, count: 2}
    output:
      id: wooden-axe
      name: "Wooden Axe"
      desc: "A basic axe. Can chop down trees for wood logs."
    skill_req: 0

  - id: stone-sword
    name: "Stone Sword"
    ingredients:
      - {id: stone, count: 2}
      - {id: stick, count: 1}
    output:
      id: stone-sword
      name: "Stone Sword"
      desc: "A stone sword. 8 attack."
    skill_req: 0

  - id: stone-axe
    name: "Stone Axe"
    ingredients:
      - {id: stone, count: 3}
      - {id: stick, count: 2}
    output:
      id: stone-axe
      name: "Stone Axe"
      desc: "A stone axe. Chops wood faster than a wooden axe."
    skill_req: 0

  - id: crude-iron-chest
    name: "Crude Iron Chestplate"
    ingredients:
      - {id: iron-ingot, count: 4}
    output:
      id: crude-iron-chest
      name: "Crude Iron Chestplate"
      desc: "A rough iron chestplate. 3 defense. Better than nothing."
    skill_req: 0
    workbench: workbench
```

---

## Part 3: Quest Chain `q-first-forge`

### Overview

A 4-step tutorial quest given by Apprentice Brix in meadow-1. Auto-accepted when the player first enters meadow-1 (trigger: `always`, quest_id: `q-first-forge`).

Teaches the three crafting systems in order: gather → craft → smelt → assembly.

### Brix Dialogue Updates

Brix gets four new dialogue entries (in addition to existing `always` and `skill_gte:mining:2`), each triggered by quest step completion:

```yaml
- trigger: quest_step:q-first-forge:0
  text: "The meadow's full of sticks if you look. Type 'gather' and bring me five. Oh — and there's an old stump outside in the square. You can pull logs off it by hand."
  quest_id: q-first-forge

- trigger: quest_step:q-first-forge:1
  text: "Nice haul. Now try crafting a stone sword — type 'craft stone-sword'. You'll need 2 stone and 1 stick. Mine the stone deposits here if you need more."

- trigger: quest_step:q-first-forge:2
  text: "Stone sword! Now you can actually fight back. But iron's where the real power is. Toss an iron-ore in that furnace with some coal — type 'smelt iron-ore'."

- trigger: quest_step:q-first-forge:3
  text: "There it is — iron! Go craft yourself a crude iron chestplate at the workbench. Then head up to the ruins workshop — it's got a scrap-forge. Take these parts and see what you can put together."
```

Step 4 seeds a `pipe-frame-crude` and `copper-tube-crude` into inventory as part of its reward, so the player has the components to attempt a pipe-pistol assembly.

### Quest Definition

```yaml
- id: q-first-forge
  name: "First Forge"
  desc: "Apprentice Brix wants to teach you the basics of crafting, smelting, and assembly."
  giver: apprentice-brix
  steps:
    - id: step-gather
      desc: "Gather 5 sticks from the meadow."
      objective:
        type: gather_item
        item_id: stick
        count: 5
      reward:
        items:
          - {id: wood-log, name: "Wood Log", desc: "A rough log.", count: 2}
        xp: 10

    - id: step-craft-sword
      desc: "Craft a stone sword."
      objective:
        type: craft_item
        item_id: stone-sword
        count: 1
      reward:
        items:
          - {id: stone, name: "Stone", desc: "A rough stone block.", count: 3}
          - {id: coal, name: "Coal", desc: "A lump of coal. Burns in a furnace.", count: 1}
        rep:
          - {faction: stoneguard, amount: 1}

    - id: step-smelt
      desc: "Smelt 1 iron-ingot at the furnace in the Builder's Workshop."
      objective:
        type: smelt_item
        item_id: iron-ingot
        count: 1
      reward:
        items:
          - {id: iron-ingot, name: "Iron Ingot", desc: "A bar of smelted iron.", count: 3}
          - {id: crude-iron-chest, name: "Crude Iron Chestplate", desc: "A rough iron chestplate. 3 defense.", count: 1}
        rep:
          - {faction: stoneguard, amount: 1}

    - id: step-assembly
      desc: "Assemble a pipe-pistol at the ruins workshop scrap-forge."
      # Gun components (pipe-frame-crude, copper-tube-crude) are seeded into inventory
      # when step-smelt completes and Brix's step 3 dialogue fires — before the player
      # heads to the ruins-workshop. They are listed as reward here for record-keeping
      # but the implementation seeds them on step 3 dialogue trigger, not on assembly complete.
      objective:
        type: assemble_item
        item_id: pipe-pistol
        count: 1
      reward:
        credits: 100
        items:
          - {id: pipe-frame-crude, name: "Pipe Frame", desc: "A rough iron pipe frame.", count: 1, seed_on_step_start: true}
          - {id: copper-tube-crude, name: "Copper Tube", desc: "A salvaged copper pipe.", count: 1, seed_on_step_start: true}
        rep:
          - {faction: stoneguard, amount: 2}
        unlock_dialogue:
          npc: elder-mason
          trigger: completed_first_forge
          text: "You've got the hang of it already. The ruins up above — the Stoneguard built those before the Core shattered. Worth exploring."
```

---

## Part 4: Files to Modify

All changes are in YAML world files only — no Go code changes required.

| File | Change |
|---|---|
| `internal/world/defaults/blockhaven/world.yaml` | Add tree-stump to meadow-0 resources; add 4 new crafting_recipes; add q-first-forge to quests; update Brix dialogue |
| `worlds/blockhaven/world.yaml` | Mirror all changes above (kept in sync with defaults) |

---

## Part 5: Verification

```
(start in meadow-0)
harvest tree-stump      → "You wrench a log from the old stump... +1x Wood Log"
gather                  → "you gather from the surroundings... +2x Stick, +1x Flint"
(go north to meadow-1)
talk apprentice-brix    → quest q-first-forge auto-accepted; step 1 active
gather                  → accumulate 5 sticks
(quest step 1 complete → +2x wood-log seeded)
craft stone-sword       → (with 2 stone from mining + 1 stick) "You craft a Stone Sword."
(quest step 2 complete → +3 stone, +1 coal, stoneguard rep +1)
smelt iron-ore          → (with coal + iron-ore) "You smelt 1x Iron Ingot."
(quest step 3 complete → +3 iron-ingot, +crude-iron-chest, stoneguard rep +1)
craft crude-iron-chest  → via workbench (or received as reward — either path)
(go up from meadow-0 to ruins-workshop-meadow)
assemble pipe-pistol    → slot frame + barrel at scrap-forge
(quest step 4 complete → 100 credits, stoneguard rep +2, Elder Mason new dialogue)
```
