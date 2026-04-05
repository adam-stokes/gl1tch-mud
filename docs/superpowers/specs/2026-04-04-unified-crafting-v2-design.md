# Unified Crafting v2 — Slot Assembly System

**Date:** 2026-04-04  
**Scope:** Extend the crafting engine with a generic slot-assembly recipe type; add Blockhaven gun, armor, and item content

---

## Overview

The current crafting engine supports one recipe type: ingredient-list → output. This spec adds a second type — **assembly** — where a base item is built by filling named slots with component items. The slot schema is world-agnostic: any game hosted on gl1tch-mud can define its own slots and component tags.

The immediate use case is Blockhaven: scavenged-tech gun crafting and faction armor crafting. A future use case is gl1tch cyberspace: cyberdeck assembly with slots like `cpu`, `ram`, `antenna`, `exploit`.

---

## Data Model

### Recipe type field

`CraftingRecipe` gains a `Type` field. Existing recipes omit it and default to `ingredient`, so no existing YAML breaks.

```go
type CraftingRecipeType string

const (
    RecipeTypeIngredient CraftingRecipeType = "ingredient"
    RecipeTypeAssembly   CraftingRecipeType = "assembly"
)
```

### CraftingSlot

```go
type CraftingSlot struct {
    ID         string         // world-defined: "barrel", "cpu", "chest"
    Name       string         // display name shown in UI
    Required   bool           // if true, blocks crafting when empty
    AcceptsTag string         // item tag filter e.g. "gun-barrel"
    StatMods   map[string]int // stat contributions when slot is filled e.g. {damage: 2, range: 1}
}
```

### Updated CraftingRecipe

```go
type CraftingRecipe struct {
    // All existing fields unchanged
    ID             string
    Name           string
    Ingredients    []CraftingIngredient
    Output         Item
    SkillReq       int
    Workbench      string
    TierThresholds []int
    TierNames      []string

    // New fields
    Type  CraftingRecipeType // defaults to "ingredient" if omitted
    Slots []CraftingSlot     // only used for assembly type
}
```

### Item tags and stats

Items gain two new optional fields:

```go
type Item struct {
    // All existing fields unchanged
    // New fields:
    Tags     []string       // e.g. ["gun-barrel", "component"]
    Stats    map[string]int // base stats e.g. {damage: 5, range: 3}
    StatMods map[string]int // contributions when used as a slot component
    Quality  string         // "crude" | "standard" | "refined" — set at drop time
}
```

Quality is cosmetic metadata on the item itself; stat contributions are baked into `StatMods` at generation time (a refined drop has higher values, not a multiplier applied at craft time).

---

## Backend: Crafting Engine

### Dispatch

`internal/crafting/crafting.go` exposes a single entry point:

```go
func Craft(player *player.State, recipe *world.CraftingRecipe, slots map[string]string, w *world.World) (world.Item, error)
```

`slots` maps `slotID → itemID`. Empty/nil for ingredient recipes.

Internal dispatch:

```go
switch recipe.Type {
case world.RecipeTypeAssembly:
    return craftAssemble(player, recipe, slots, w)
default:
    return craftIngredient(player, recipe, w)
}
```

The existing ingredient logic moves into `craftIngredient` — no behavior change.

### craftAssemble

1. Skill gate — same check as ingredient
2. Workbench check — same check as ingredient
3. Validate required slots are all filled (return `ErrMissingSlot` if not)
4. Validate each filled slot's item exists in player inventory
5. Validate each item's `Tags` contains the slot's `AcceptsTag` (return `ErrWrongComponent` if not)
6. Consume all slot items from inventory
7. Sum `StatMods` from all filled slots onto base output item's `Stats`
8. Return assembled item

### Error types

```go
var (
    ErrMissingSlot    = errors.New("required slot not filled")
    ErrWrongComponent = errors.New("item does not fit this slot")
)
```

### Command handler

The `craft` command handler passes the slot map from the client message payload. For ingredient recipes, `slots` is nil. Protocol:

```json
{
  "command": "craft",
  "recipe_id": "pipe-pistol",
  "slots": {
    "barrel": "item-uuid-abc",
    "grip": "item-uuid-def"
  }
}
```

---

## Frontend: Kids Assembly Modal

### Routing

`mud.ts` `Recipe` interface gains `type` and `slots` fields. When the craft button is tapped:

- `recipe.type === 'assembly'` → open `AssemblyModal`
- otherwise → open existing paint grid modal (unchanged)

### AssemblyModal layout

Two-column layout inside the existing modal shell:

**Left column:** decorative SVG silhouette of the item category (gun, chest armor, etc.). Swaps based on recipe output category tag. Pure visual — no interaction.

**Right column:** slot panel. One row per slot:
- Slot name label
- Drop target area (tap to open inventory picker filtered to `AcceptsTag`)
- When filled: item name chip + stat contribution chips (`+2 DMG`, `+3 RNG`)
- Required slots glow red outline until filled

**Bottom bar:**
- Live stat preview bars (damage ████░░, range ███░░░) — updates as slots are filled
- "FORGE IT" button — disabled until all required slots are filled

### Inventory picker

Reuses the existing inventory picker component. Filtered to items matching the slot's `AcceptsTag`. Items show quality badge (crude / standard / refined) and their stat mods.

### What is unchanged

- Paint grid modal and all its E2E tests
- Recipe drawer / recipe cards
- Workbench guidance
- All existing `kids-craft-*` E2E test files

---

## Blockhaven Content

### Workbenches

| ID | Name | Location |
|----|------|----------|
| `scrap-forge` | Scrap Forge | ruins-workshop room (new room, all biomes have one) |
| `anvil` | Ancient Anvil | same room as scrap-forge |

### Gun recipes (type: assembly, workbench: scrap-forge)

| ID | Name | Required Slots | Optional Slots | Skill |
|----|------|----------------|----------------|-------|
| `pipe-pistol` | Pipe Pistol | frame, barrel | grip | 1 |
| `bolt-rifle` | Bolt Rifle | frame, barrel, stock | sight | 3 |
| `scatter-cannon` | Scatter Cannon | frame, barrel | choke, brace | 3 |
| `flare-launcher` | Flare Launcher | frame, barrel, igniter | — | 4 |
| `twin-barrel` | Twin Barrel | frame, barrel-left, barrel-right | grip, trigger | 4 |
| `bone-sniper` | Bone Sniper | frame, barrel, stock, sight | stabilizer | 5 |
| `vault-repeater` | Vault Repeater | frame, barrel, drum, stock | sight | 7 |

### Armor recipes (type: assembly, workbench: anvil)

| ID | Name | Required Slots | Optional Slots | Skill |
|----|------|----------------|----------------|-------|
| `scrap-vest` | Scrap Vest | chest | shoulder, lining | 1 |
| `plate-coat` | Plate Coat | chest, shoulder | gauntlets, boots | 2 |
| `thornwalker-leathers` | Thornwalker Leathers | chest, lining | shoulder, hood | 3 |
| `stoneguard-shell` | Stoneguard Shell | chest, shoulder, gauntlets, boots | — | 4 |
| `dunekeepers-wrap` | Dunekeepers Wrap | chest, lining | veil | 3 |
| `frostborn-parka` | Frostborn Parka | chest, shoulder, lining | boots | 4 |
| `deepborn-suit` | Deepborn Suit | chest, shoulder, lining | helm | 5 |
| `ruin-exosuit` | Ruin Exosuit | chest, shoulder, gauntlets, boots, helm | — | 8 |

### Item recipes (type: ingredient, paint grid — existing mechanic, new entries)

| ID | Name | Ingredients |
|----|------|-------------|
| `medkit` | Medkit | spider-silk + ember-cloth + cave-moss |
| `smoke-bomb` | Smoke Bomb | coal-dust + woven-moss + copper-tube |
| `lockpick-set` | Lockpick Set | wire + bone-shard + copper |
| `grapple-hook` | Grapple Hook | iron-chain + carved-handle + rope |
| `faction-token` | Faction Token | faction-ore×3 + deepstone-shard |
| `ancient-battery` | Ancient Battery | copper + iron + crystal-lens |

`ancient-battery` is a key item that unlocks gun assembly recipes when crafted. It sets a `gun_recipes_unlocked` boolean on `player.State` (new field). The server filters gun recipes out of the `recipes` state update array until this flag is true.

### Component items

**Gun components** (tags: `component`, `gun-<slot>`):

| Item | Tag | Quality Tiers | Drops From |
|------|-----|---------------|------------|
| Pipe Frame | gun-frame | crude/standard | surface enemies |
| Iron Receiver | gun-frame | standard/refined | dungeon chests |
| Copper Tube | gun-barrel | crude/standard | mining nodes |
| Rifled Pipe | gun-barrel | standard | workshop loot |
| Reinforced Barrel | gun-barrel | refined | boss drops |
| Wrapped Grip | gun-stock | crude | common drops |
| Carved Handle | gun-stock | standard | forest enemies |
| Iron Sights | gun-sight | crude/standard | scrap piles |
| Crystal Lens | gun-sight | refined | cave crystals |
| Coal Igniter | gun-igniter | standard | deepborn enemies |
| Drum Cylinder | gun-drum | standard/refined | vault enemies |
| Bone Stabilizer | gun-stabilizer | refined | rare drops |

**Armor components** (tags: `component`, `armor-<slot>`):

| Item | Tag | Quality Tiers | Drops From |
|------|-----|---------------|------------|
| Leather Plate | armor-chest | crude/standard | common enemies |
| Iron Sheet | armor-chest | standard | stoneguard enemies |
| Deepstone Slab | armor-chest | refined | cave bosses |
| Hide Pad | armor-shoulder | crude | wolves/beasts |
| Chain Links | armor-shoulder | standard | ruins chests |
| Carved Pauldron | armor-shoulder | refined | stoneguard elites |
| Woven Moss | armor-lining | crude | forest forage |
| Spider Silk | armor-lining | standard | cave spiders |
| Ember Cloth | armor-lining | refined | dunekeepers traders |
| Bone Helm | armor-helm | standard | deepborn |
| Fur Boots | armor-boots | crude/standard | frostborn drops |
| Iron Gauntlets | armor-gauntlets | standard/refined | stoneguard |
| Desert Veil | armor-veil | standard | dunekeeper loot |
| Shadow Hood | armor-hood | refined | thornwalker elites |

### Component stat ranges by quality

Each component item is defined three times in YAML (one entry per quality tier) with explicit `stat_mods` values. There is no runtime multiplier. The looting system picks the appropriate tier entry based on the drop table.

Example (Copper Tube defined three times):
```yaml
- id: copper-tube-crude
  name: Copper Tube
  quality: crude
  tags: [component, gun-barrel]
  stat_mods: {damage: 1, range: 1}

- id: copper-tube-standard
  name: Copper Tube
  quality: standard
  tags: [component, gun-barrel]
  stat_mods: {damage: 2, range: 2}

- id: copper-tube-refined
  name: Copper Tube
  quality: refined
  tags: [component, gun-barrel]
  stat_mods: {damage: 3, range: 3}
```

---

## Progression Arc

```
Start → find Pipe Frame + Copper Tube → craft Ancient Battery (paint grid)
      → gun_recipes_unlocked → craft Pipe Pistol at Scrap Forge
      → explore harder biomes → find refined components
      → unlock faction armor recipes (skill gates 3-5)
      → endgame: Bone Sniper + Ruin Exosuit (skill 7-8)
```

Each biome has a ruins-workshop room containing both the scrap-forge and anvil. Faction enemies in that biome drop faction-themed components, so the Stoneguard Shield requires Stoneguard Shell components from the meadow/fortress area.

---

## Cross-World Reusability

The assembly system is fully generic. To add cyberdeck assembly in gl1tch:

```yaml
crafting_recipes:
  - id: basic-deck
    name: Basic Cyberdeck
    type: assembly
    workbench: workshop
    skill_req: 2
    output:
      id: basic-deck
      name: Basic Cyberdeck
    slots:
      - id: cpu
        name: Processor
        required: true
        accepts_tag: deck-cpu
        stat_mods: {hack_speed: 2}
      - id: ram
        name: Memory
        required: true
        accepts_tag: deck-ram
        stat_mods: {program_slots: 1}
      - id: antenna
        name: Antenna
        required: false
        accepts_tag: deck-antenna
        stat_mods: {range: 3}
```

No backend changes needed — just YAML content.

---

## Testing

- `internal/crafting/crafting_test.go`: add assembly test cases covering required slot missing, wrong component tag, successful assemble with stat accumulation
- `web/e2e/kids-assembly-modal.spec.ts`: new E2E covering slot fill → stat preview update → forge
- `web/e2e/kids-assembly-workbench.spec.ts`: workbench gate check
- All existing `kids-craft-*` tests must continue passing unchanged
