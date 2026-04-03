# Blockhaven World — Design Spec

**Date:** 2026-04-02
**Status:** Approved, pending implementation

---

## Context

gl1tch-mud currently ships one world: `cyberspace`, a cyberpunk noir MUD with a strong narrative arc (ZERO/SIGNAL). The goal is to make gl1tch-mud a multi-world platform — players can switch between radically different world experiences from within the game.

**Blockhaven** is the second world: a Minecraft-flavored adventure MUD targeting players aged 8–12. The Crystal Core that powered Blockhaven's five biomes was shattered when a young dragon named Cinder accidentally crashed into it. The five Crystal Shards scattered across the biomes. Players mine, gather, craft, build, enchant, trade, and quest to restore the shards — and ultimately befriend Cinder to rebuild the Core.

This spec covers:
1. World switching infrastructure (new commands, per-world DB, main.go wiring)
2. Blockhaven world content (story bible, biomes, factions, core rooms, quests, events)
3. All new game mechanics (mining, harvesting, smelting, building, enchanting, weather, death/respawn)
4. DB schema additions

Idempotent generation pipelines for Blockhaven are a follow-on spec (Spec 2).

---

## Part 1: World Switching Infrastructure

### 1.1 New Commands

```
world list              — list installed worlds
world switch <name>     — switch to a different world
```

`world switch` hot-swaps the world (`w`) and database (`database`) in the game loop without restarting the process. Player state (room, HP, inventory, quests) is per-world.

### 1.2 Per-World Player Database

**Single-player path:** `~/.local/share/gl1tch-mud/worlds/<name>/player.db`

New function in `internal/db/db.go`:

```go
func OpenForWorld(worldName string) (*sql.DB, error)
```

Same schema as the existing `world.db`. When switching worlds, the old DB connection is closed and a new one is opened for the target world. This mirrors the existing `db.OpenForPlayer(playerID)` pattern.

**LAN multi-player path:** `~/.local/share/gl1tch-mud/players/<playerID>/<world>.db`
`db.OpenForPlayer` needs a world parameter: `db.OpenForPlayer(playerID, worldName)`.

### 1.3 main.go Changes

- `w` and `database` become variables that can be reassigned
- `commands.SetWorld(w)` and `commands.SetDB(db)` wiring (or pass via closure)
- On `world switch <name>`:
  1. Validate world exists (`world.Load` returns error if not found)
  2. Close existing DB
  3. Open new per-world DB via `db.OpenForWorld(name)`
  4. Load new world via `world.Load(name)`
  5. Update `s.World`, `s.RoomID` to new world's `start_room`
  6. Save state to new DB
  7. Print new room description

### 1.4 World YAML — New Fields

All new fields are zero-value safe (existing `cyberspace` world unaffected).

**On `Room`:**

```yaml
biome: meadow           # meadow | forest | desert | snow | caves
resources:
  - id: iron-vein
    type: mine          # mine | harvest
    yields:
      - item_id: iron-ore
        count_min: 1
        count_max: 3
    tool_required: pickaxe   # optional; omit for no-tool harvesting
    respawn_actions: 20      # re-appears after N player actions
```

**Top-level on `World`:**

```yaml
weather_table:
  - biome: meadow
    possible: [clear, rainy, windy, stormy]
  - biome: forest
    possible: [clear, rainy, foggy]
  - biome: desert
    possible: [clear, sandstorm, scorching]
  - biome: snow
    possible: [clear, light-snow, blizzard]
  - biome: caves
    possible: [clear, damp, tremor]
```

### 1.5 New DB Tables

Added to `internal/db/schema.go` (all additive, existing tables unchanged):

```sql
-- Mineable/harvestable resource depletion tracking
CREATE TABLE IF NOT EXISTS room_resources (
    room_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    depleted INTEGER DEFAULT 0,
    depleted_at_action INTEGER DEFAULT 0,
    PRIMARY KEY (room_id, resource_id)
);

-- Current weather per biome
CREATE TABLE IF NOT EXISTS weather_state (
    biome TEXT PRIMARY KEY,
    condition TEXT NOT NULL DEFAULT 'clear',
    expires_action INTEGER DEFAULT 0
);

-- Item enchantments
CREATE TABLE IF NOT EXISTS enchants (
    item_id TEXT NOT NULL,
    enchant_id TEXT NOT NULL,
    level INTEGER DEFAULT 1,
    applied_at INTEGER DEFAULT 0,
    PRIMARY KEY (item_id, enchant_id)
);

-- Player-built structures in rooms
CREATE TABLE IF NOT EXISTS builds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id TEXT NOT NULL,
    build_id TEXT NOT NULL,
    name TEXT NOT NULL,
    desc TEXT,
    placed_at INTEGER DEFAULT 0
);

-- Crystal Shard collection progress
CREATE TABLE IF NOT EXISTS crystal_shards (
    shard_id TEXT PRIMARY KEY,
    biome TEXT NOT NULL,
    collected INTEGER DEFAULT 0,
    collected_at INTEGER DEFAULT 0
);

-- Death pile: items dropped on player death
CREATE TABLE IF NOT EXISTS death_pile (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_desc TEXT,
    died_at INTEGER DEFAULT 0
);

-- Enchanting XP (separate from skill XP)
CREATE TABLE IF NOT EXISTS enchanting_xp (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    xp INTEGER DEFAULT 0,
    level INTEGER DEFAULT 1
);

-- Planted crops
CREATE TABLE IF NOT EXISTS crops (
    room_id TEXT NOT NULL,
    slot INTEGER NOT NULL,
    seed_id TEXT NOT NULL,
    planted_at_action INTEGER NOT NULL,
    ready_at_action INTEGER NOT NULL,
    harvested INTEGER DEFAULT 0,
    PRIMARY KEY (room_id, slot)
);

-- Room chest storage (when player builds a chest)
CREATE TABLE IF NOT EXISTS chests (
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_desc TEXT,
    PRIMARY KEY (room_id, item_id)
);
```

---

## Part 2: Blockhaven World Content

### 2.1 Story Bible

**The World:** Blockhaven is a land of five biomes, each with its own terrain, creatures, and people. For generations, the Crystal Core floated above Meadow Town, radiating energy that kept the biomes alive and full of color.

**The Inciting Event:** A young dragon named Cinder was playing in the sky above Blockhaven and accidentally collided with the Crystal Core, shattering it into five Crystal Shards. Each shard flew to a different biome. Cinder, horrified and ashamed, fled to a hidden cave deep in the Snow Peaks.

**The Crisis:** Without the Core, the biomes are fading:
- The Meadow's flowers are wilting
- The Forest's ancient trees are going dormant
- The Desert's ruins are sinking deeper into the sand
- The Snow Peaks are crumbling
- The Deep Caves have gone dark

**The Player's Role:** A newcomer arrives in Meadow Town. Elder Mason of the Stoneguard explains what happened and asks for help. The player must travel to all five biomes, complete each faction's quest, and collect the Crystal Shards. Once all five are gathered, they must find Cinder and convince the dragon to help restore the Core.

**Narrative Chapters:**
1. `shard-hunt` — Collecting the five Crystal Shards (default start)
2. `ember-found` — Player has all shards, Cinder's hideaway is revealed
3. `core-restored` — Crystal Core rebuilt, world renewed (end state)

**Tone:** Adventure, discovery, cooperation. No faction is an enemy. Combat exists — mobs are territorial creatures, raiders are opportunists — but the resolution of the main story is about friendship and repair, not conquest.

---

### 2.2 Biomes, Factions, and Zones

| Biome | Zone | Faction | Crystal Shard | Faction Role |
|---|---|---|---|---|
| Meadow | hub | The Stoneguard | Meadow Shard | Master builders, town keepers |
| Forest | A | The Thornwalkers | Forest Shard | Rangers, herbalists, nature guardians |
| Desert | B | The Dunekeepers | Desert Shard | Archaeologists, puzzle-solvers, relic hunters |
| Snow Peaks | C | The Frostborn | Mountain Shard | Hardy miners, smiths, mountain traders |
| Deep Caves | D | The Deepborn | Cave Shard | Ancient cave dwellers, Crystal Core lore keepers |

**Faction relationships:** No enemies. All factions are neutral-to-friendly. Higher reputation unlocks better trades and unique dialogue. Reputation earned via quests, trading, and world event participation.

---

### 2.3 Core Rooms (Hand-Authored)

11 rooms. These are the skeleton; pipelines grow the world from here.

```
meadow-0    Meadow Town Square         (start_room, Stoneguard HQ)
meadow-1    The Builder's Workshop     (workbench + furnace present)
forest-0    Forest Entrance            (Thornwalkers patrol here)
forest-1    Ancient Oak Grove          (rare seeds, harvestable ancient oak)
desert-0    Desert Gateway             (transition room, Dunekeeper checkpoint)
desert-1    Sandstone Ruins            (puzzle system: solve → Desert Shard)
snow-0      Mountain Pass              (weather: always cold, blizzard risk)
snow-1      Frost Village              (Frostborn traders, forge room)
caves-0     Cave Entrance              (danger warning, low-light desc)
caves-1     Crystal Cavern             (deepest room, Crystal Flame puzzle)
ember-0     Cinder's Hideaway          (locked until chapter=ember-found)
```

### 2.4 Starting Items

Player spawns with:
- Wooden Pickaxe (tool, allows basic mining)
- Wooden Sword (weapon, 5 attack)
- 3x Bread (consumable, restores 20 HP)
- Builder's Map (readable, explains the world and Crystal Core lore)

---

### 2.5 Key NPCs

| NPC | Location | Role |
|---|---|---|
| Elder Mason | meadow-0 | Quest giver (main arc), Stoneguard leader |
| Apprentice Brix | meadow-1 | Crafting tutor, gives first recipe quest |
| Warden Sylara | forest-0 | Thornwalkers leader, Forest Shard quest giver |
| Archivist Dunes | desert-1 | Dunekeepers leader, Desert Shard quest giver |
| Ironmaster Breck | snow-1 | Frostborn leader, Mountain Shard quest giver |
| Elder Voss | caves-1 | Deepborn leader, Cave Shard quest giver |
| Cinder | ember-0 | The Dragon; non-hostile; final quest NPC |

---

### 2.6 Mob Roster

| Mob | Biome | HP | Attack | Loot |
|---|---|---|---|---|
| Stoneling | Meadow | 20 | 5 | stone, flint |
| Wandering Raider | Meadow | 30 | 8 | iron-ore, bread |
| Thornsprite | Forest | 15 | 4 | seeds, leaves |
| Vine Creeper | Forest | 25 | 7 | vines, roots |
| Sand Golem | Desert | 40 | 10 | sand, sandstone, rare: gold-ore |
| Dune Stalker | Desert | 20 | 6 | leather, bones |
| Frost Wraith | Snow | 35 | 12 | frost-essence, ice-shard |
| Ice Crawler | Snow | 20 | 5 | ice, rare: diamond |
| Cave Lurker | Caves | 30 | 9 | coal, iron-ore |
| Crystal Shade | Caves | 50 | 15 | glowstone, crystal-fragment |

---

## Part 3: New Mechanics

### 3.1 Mining (`mine`)

```
mine              — list mineable resources in current room
mine <resource>   — mine a specific resource
```

- Checks inventory for required tool (e.g., pickaxe for stone/ore; any tool or none for softer materials)
- Rolls yield from resource's `yields` list using existing loot probability logic
- Records depletion in `room_resources` table
- Resource respawns when `player_actions.count >= depleted_at_action + respawn_actions`
- Enchant *Fortune* multiplies yield count; *Silk Touch* changes drop (e.g., stone instead of cobblestone)
- Earns enchanting XP on each mine action

### 3.2 Harvesting (`harvest`)

```
harvest           — list harvestable resources in current room
harvest <resource> — harvest plants, trees, mushrooms, crops
```

- Same mechanics as `mine` but `type: harvest` resources
- Trees: yield wood logs, sticks, rare: apple
- Plants/mushrooms: yield seeds, flowers, food items
- Farmland crops: harvested via `harvest` after planting with `plant`

### 3.3 Gathering (`gather`)

```
gather            — ambient gather from the environment
```

- No tool required; always available
- Yields 1-2 random ambient items based on biome:
  - Meadow: flint, stick, wildflower
  - Forest: stick, berry, leaf
  - Desert: sand, flint, bone
  - Snow: ice, pebble, snowball
  - Caves: gravel, coal, moss
- 20-action cooldown (tracked in `player_actions`)

### 3.4 Smelting (`smelt`)

```
smelt <item-id>   — smelt an ore into an ingot
```

- Requires furnace in room (built structure or furnace present in world YAML)
- Consumes 1 fuel item (wood, coal, charcoal) per smelt
- Recipes defined in world YAML `crafting_recipes` with `workbench: furnace`
- Examples: iron-ore → iron-ingot, gold-ore → gold-ingot, sand → glass

### 3.5 Planting (`plant`, `harvest`)

```
plant <seed-id>   — plant a seed in a farmland room
```

- Only in rooms with `biome: meadow` or rooms that have a `garden-plot` build
- Seed entry created in `crops` table with `ready_at_action = current_action + grow_time`
- `harvest` in the same room after the ready time yields the crop and removes the entry
- Grow times defined in the world YAML `resources` list on farmland rooms as a `grow_actions` field on the seed resource entry (e.g., `grow_actions: 15`). The `plant` command looks up the resource by seed item ID.

### 3.6 Building (`build`)

```
build             — list available build recipes
build <recipe-id> — consume materials and place a structure
```

- Build recipes defined in world YAML `crafting_recipes` with `type: build`
- Consumes items from inventory
- Creates entry in `builds` table for current room
- Built structures enable mechanics:
  - `workbench` → unlocks advanced crafting recipes
  - `furnace` → enables `smelt` command in this room
  - `enchanting-table` → enables `enchant` command in this room
  - `chest` → enables `stash`/`unstash` commands in this room
  - `garden-plot` → enables `plant` command in this room
- `look` output lists built structures in the room

### 3.7 Enchanting (`enchant`)

```
enchant           — show current enchanting XP and available enchants
enchant <item-id> — enchant an item
```

- Requires enchanting table in room
- Enchanting XP earned via: mining, combat kills, quest completion
- Enchanting XP levels (1–30), higher level = more powerful enchant options
- Enchant applied is chosen at random from item-type-appropriate options (with player confirmation)
- Enchants stored in `enchants` table; displayed on `inventory` and `examine <item>`

**Enchant catalog:**

| Enchant | Applies To | Effect |
|---|---|---|
| Sharpness I/II/III | Swords, axes | +5/10/15 attack |
| Fortune I/II/III | Pickaxes, axes | +1/2/3 to mine/harvest yield |
| Swift Feet I/II | Boots | Flavor: move faster description |
| Flame Touch | Swords | Fire damage flavor, +5 attack vs ice mobs |
| Silk Touch | Pickaxes | Alternate drops (stone instead of cobblestone) |
| Feather Fall | Boots | Cave fall flavor, no mechanical penalty |
| Frost Edge | Swords | +8 attack vs desert mobs |
| Diamond Luck | Any | +2% rare item chance on all yield rolls |

### 3.8 Weather (`weather`)

```
weather           — show current weather in your biome
```

- Weather changes every 50 actions (checked in `weather_state` table)
- `ExpireWeather(db, currentAction, biome)` called on each action, rolls new condition if expired
- Weather effects applied at yield-roll time in `mine`, `harvest`, `gather`

| Weather | Biome | Effect |
|---|---|---|
| Clear | Any | +10% yield |
| Rainy | Meadow/Forest | +seed/clay drop chance |
| Stormy | Meadow | Rare: lightning event reveals buried loot item in room |
| Sandstorm | Desert | Ruins entrances uncover: unlocks hidden exits for 20 actions |
| Blizzard | Snow | Movement cost flavor; +gem drop chance from mining |
| Tremor | Caves | Random ore vein revealed: +1 mine resource in room for 10 actions |

### 3.9 Death and Respawn

**On player death (HP reaches 0):**
1. All inventory items moved to `death_pile` table with current `room_id`
2. Player HP restored to `max_hp`
3. Player `room_id` set to `meadow-0`
4. Output: death flavor message + location of death pile

**`deathpile` command:**
```
deathpile         — show where your last death pile is and what's in it
```
Items in the death pile can be recovered by returning to the death room and using `take death-pile`. The `look` command queries the `death_pile` table for the current room and renders a virtual "Your Death Pile" item in the items list if any entries exist. `take death-pile` moves all matching rows back to `inventory` and deletes them from `death_pile`.

Death pile is cleared when all items are retrieved, or after 100 actions (items lost permanently).

---

## Part 4: Quest Arc

### 4.1 Main Quest Arc — "The Five Shards"

Quests unlock when player first enters the corresponding biome. Each awards a Crystal Shard item + reputation with that faction.

| Quest ID | Title | Giver | Objective | Reward |
|---|---|---|---|---|
| `q-meadow-shard` | The Wilting Meadow | Elder Mason | Kill Stoneling Chieftain (hp:60), retrieve Meadow Shard | 150 credits + Meadow Shard + Stoneguard rep +5 |
| `q-forest-shard` | Root of the Problem | Warden Sylara | Mine the Ancient Root Vein in forest-1 | 120 credits + Forest Shard + Thornwalkers rep +5 |
| `q-desert-shard` | Lost to the Sand | Archivist Dunes | Solve the Sandstone Ruins puzzle (hack the ruin-terminal, level 2) | 200 credits + Desert Shard + Dunekeepers rep +5 |
| `q-mountain-shard` | The Frozen Forge | Ironmaster Breck | Smelt a Frostcore Ingot (frost-essence + iron-ingot in furnace) | 175 credits + Mountain Shard + Frostborn rep +5 |
| `q-cave-shard` | Into the Dark | Elder Voss | Reach Crystal Cavern (caves-1) and re-light the Crystal Flame (hack the flame-terminal) | 250 credits + Cave Shard + Deepborn rep +5 |

### 4.2 Final Quest — "Cinder's Return"

Unlocked when all 5 Crystal Shards are in inventory.

1. Elder Mason's dialogue trigger: `has_all_shards` (new trigger type — see below). He says: "A glow has appeared in the snow. I think you can find Cinder now." This also auto-accepts quest `q-cinder-return`.
2. Passage to `ember-0` unlocks: the lock on `snow-0`'s hidden exit uses `keys: [crystal-key]`. Quest `q-cinder-return` grants `crystal-key` as a reward item on acceptance. This reuses the existing key/lock system cleanly.
3. Talk to Cinder — dialogue evolves based on `has_all_shards` trigger
4. Final dialogue trigger: `has_all_shards` (same new trigger type)
5. Quest completes, chapter advances to `core-restored`
6. Rewards: `dragon-scale-chestplate` (best armor), `crystal-warden` title item, +50 rep all factions

**New dialogue trigger: `has_all_shards`**
Evaluates to true when all 5 rows in `crystal_shards` table have `collected = 1`. Implemented in the existing dialogue trigger evaluation logic in `commands.go`.

---

## Part 5: World Events

Same engine as cyberspace (`internal/events/`). Blockhaven-specific templates:

| Event | Type | Effect | Duration |
|---|---|---|---|
| Raider Siege | `raid` | Raiders spawn in meadow-0; defeat them for Stoneguard rep + loot | 30 actions |
| Wandering Merchant | `merchant` | Rare trader NPC appears in meadow-0 with unique items | 25 actions |
| Crystal Pulse | `pulse` | Mining yield 2x in all biomes | 20 actions |
| Cave-In Warning | `cave_in` | caves-0 exit blocked; alternate path via snow-0 | 15 actions |
| Dragon Sighting | `sighting` | Cinder spotted; clue item revealed in a random biome room | 10 actions |

---

## Part 6: Critical Files to Modify / Create

### Modify
- `internal/db/schema.go` — add 9 new tables
- `internal/db/db.go` — add `OpenForWorld(worldName)`, update `OpenForPlayer` signature
- `internal/world/world.go` — add `Biome` to Room, `Resources []Resource` to Room, `WeatherTable` to World
- `internal/player/player.go` — death handling, death pile, enchanting XP helpers
- `internal/commands/commands.go` — add `mine`, `harvest`, `gather`, `smelt`, `plant`, `build`, `enchant`, `weather`, `deathpile`, `world`, `stash`, `unstash`
- `main.go` — world switching wiring, `db.OpenForWorld` on startup

### Create
- `worlds/blockhaven/world.yaml` — full Blockhaven world (11 core rooms + content)
- `worlds/blockhaven/story-bible.md` — canonical narrative reference
- `worlds/blockhaven/world-state.yaml` — idempotency tracker for pipelines
- `internal/weather/weather.go` — weather tick logic
- `internal/enchanting/enchanting.go` — enchant application logic

### Reuse (no changes needed)
- `internal/crafting/crafting.go` — build recipes use the same `craft` engine with `type: build`
- `internal/trading/trading.go` — villager trades use existing system
- `internal/quests/quests.go` — all 5 shard quests use existing quest engine
- `internal/events/events.go` — world events use existing engine with new templates
- `internal/combat` patterns in `commands.go` — mob combat works as-is

---

## Part 7: Verification

**World switching:**
```
world list              → shows [cyberspace, blockhaven]
world switch blockhaven → loads blockhaven, shows meadow-0 description
world switch cyberspace → loads cyberspace, shows net-0 description
```

**New mechanics:**
```
mine                    → lists iron-vein, coal-seam in room
mine iron-vein          → "You swing your pickaxe... [2x iron-ore]"
mine iron-vein          → (depleted) "The vein is exhausted."
smelt iron-ore          → (with furnace + coal) "You smelt 1x iron-ingot"
build workbench         → consumes 4x wood-plank, "You assemble a sturdy workbench."
enchant iron-sword      → "Your enchanting table glows... Sharpness I applied."
weather                 → "The meadow is clear. Yields are slightly improved."
gather                  → "You pick through the area... 1x flint, 1x stick."
plant wheat-seed        → "You press the seed into the soil. Check back in 15 actions."
```

**Death and respawn:**
```
(HP → 0 in combat)      → "You've been defeated! Your items lie at [room-name]. You wake up at Meadow Town Square."
deathpile               → "Your death pile is in: forest-1 (Ancient Oak Grove). Items: iron-sword, 3x iron-ore"
(travel to forest-1)    → room shows "Your Death Pile" as an item
take death-pile         → items restored to inventory
```

**Main quest:**
```
talk elder-mason        → quest q-meadow-shard auto-accepted
(kill Stoneling Chieftain, take Meadow Shard)
quest complete q-meadow-shard → 150 credits, rep +5
(collect all 5 shards)
talk elder-mason        → "The snow glows. Find Cinder."
(travel to ember-0 via snow-0 unlocked exit)
talk cinder             → final dialogue, quest completes
```
