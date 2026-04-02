## ADDED Requirements

### Requirement: Loot table declarations in world.yaml
A world YAML MAY declare a top-level `loot_tables:` list. Each entry SHALL have:
- `id` (string, required): unique table identifier
- `entries` (list, required): each has `item_id`, `probability` (0.0–1.0), `count_min`, `count_max`

NPCs MAY reference a loot table via `loot_table_id` (string).

### Requirement: Loot roll on NPC death
When the `attack` command kills an NPC (NPC HP reaches 0), gl1tch-mud SHALL:
1. Look up the NPC's `loot_table_id` in the world
2. For each entry in the table, roll `rand(0.0, 1.0)`; if roll ≤ probability, generate `rand(count_min, count_max)` copies of that item
3. Add all rolled items to the current room's item list
4. Print a loot summary ("the netrunner drops: data-chip x2, credits x15")
5. Emit `mud.loot.dropped` with the NPC ID and item list

#### Scenario: NPC with loot table drops items on death
- **WHEN** player kills an NPC that has a loot_table_id
- **THEN** loot is rolled, items appear in the room, message is printed, event fires

#### Scenario: NPC with no loot table drops nothing
- **WHEN** player kills an NPC with no loot_table_id
- **THEN** no loot message, no items dropped, no event

#### Scenario: Zero probability item never drops
- **WHEN** a loot entry has probability 0.0
- **THEN** that item is never added to the room

#### Scenario: 1.0 probability item always drops
- **WHEN** a loot entry has probability 1.0
- **THEN** that item is always added to the room after the NPC dies
