## ADDED Requirements

### Requirement: Extended Room struct
`world.Room` SHALL gain:
- `Systems []System` — hackable systems in the room
- `Locks []Lock` — locked exits or containers

### Requirement: Extended NPC struct
`world.NPC` SHALL gain:
- `LootTableID string` — references a top-level loot table
- `Trades []TradeOffer`
- `Dialogue []DialogueLine`

### Requirement: Extended Item struct
`world.Item` SHALL gain:
- `IsDisguise bool` `yaml:"is_disguise,omitempty"` — marks item usable with `disguise` command

### Requirement: New top-level world structs
`world.World` SHALL gain:
- `CraftingRecipes []CraftingRecipe`
- `LootTables []LootTable`

### Requirement: All extensions are zero-value safe
A world YAML without any new fields SHALL parse without error. All new slice and map fields default to nil/empty.

#### Scenario: Existing world.yaml parses unchanged
- **WHEN** world.yaml has no `systems`, `locks`, `loot_tables`, `crafting_recipes`, `trades`, or `dialogue` fields
- **THEN** `world.Load()` succeeds and returns a fully valid World with nil slices for new fields

#### Scenario: Full extended world.yaml parses correctly
- **WHEN** world.yaml includes all new fields
- **THEN** all structs are populated with correct values from YAML

### Requirement: LootTable lookup helper
`world.World.FindLootTable(id string) *LootTable` SHALL return the matching table or nil.

### Requirement: CraftingRecipe lookup helper
`world.World.FindRecipe(id string) *CraftingRecipe` SHALL return the matching recipe or nil.
