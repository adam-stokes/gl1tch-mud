## ADDED Requirements

### Requirement: Crafting recipes in world.yaml
A world YAML MAY declare a top-level `crafting_recipes:` list. Each entry SHALL declare:
- `id` (string, required): unique recipe identifier
- `name` (string, required): display name of the output
- `ingredients` (list, required): each entry has `id` (item ID) and `count` (integer ≥ 1)
- `output` (object, required): `id`, `name`, `desc` of the crafted item
- `skill_req` (integer, optional, default 0): minimum `hacking` skill level required

#### Scenario: Recipe loaded from world.yaml
- **WHEN** world.yaml declares `crafting_recipes:` with one or more entries
- **THEN** the entries are accessible via `world.CraftingRecipes`

#### Scenario: Missing crafting_recipes is zero-value safe
- **WHEN** world.yaml omits `crafting_recipes:`
- **THEN** `world.CraftingRecipes` is nil and `craft` command returns "no recipes known"

### Requirement: craft command
`craft <recipe-id>` SHALL:
1. Look up the recipe by ID in `world.CraftingRecipes`
2. Check player skill meets `skill_req`; if not, print skill-gate message and return
3. Check player inventory contains all required ingredients in sufficient count
4. If any ingredient missing, list what is missing and return without consuming anything
5. Remove ingredients from inventory, add output item to inventory
6. Print a confirmation message and emit `mud.craft.completed` BUSD event

#### Scenario: Successful craft
- **WHEN** player has all ingredients and meets skill requirement
- **THEN** ingredients are consumed, output item appears in inventory, event fires

#### Scenario: Missing ingredients
- **WHEN** player lacks one or more required ingredients
- **THEN** command lists the missing items, no inventory change, no event

#### Scenario: Skill gate
- **WHEN** player's hacking skill is below `skill_req`
- **THEN** command prints "skill too low" with required vs current level, no craft attempt

#### Scenario: Unknown recipe
- **WHEN** player types `craft unknown-id`
- **THEN** command prints "unknown recipe" and lists available recipe names
