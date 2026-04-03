## ADDED Requirements

### Requirement: player_id column on all player-state tables
All tables that store per-player state SHALL have a `player_id TEXT NOT NULL DEFAULT 'default'` column. The migration SHALL be applied at server startup via `ALTER TABLE ... ADD COLUMN` (idempotent: no-op if column already exists).

Affected tables: `player`, `inventory`, `player_skills`, `player_reputation`, `player_stealth`, `player_credits`, `player_actions`, `player_augments`, `player_faction`, `faction_members`, `npc_state`, `visited`, `system_state`, `lock_state`, `npc_memory`, `unlocked_recipes`, `item_mods`, `bounties`, `vuln_windows`, `quests`, `hideout_upgrades`.

#### Scenario: Migration on fresh DB
- **WHEN** `mudserver` starts with a DB that has no `player_id` columns
- **THEN** all affected tables gain the column with default `'default'`; existing rows get value `'default'`

#### Scenario: Migration idempotent
- **WHEN** `mudserver` starts with a DB where `player_id` columns already exist
- **THEN** startup completes without error; no duplicate columns are added

### Requirement: All player-state queries filtered by player_id
Every SQL query that reads or writes player-state tables SHALL include a `WHERE player_id = ?` clause (or equivalent in INSERT). The single-player CLI (`main.go`) SHALL continue to use `player_id = 'default'` implicitly via its existing DB functions.

#### Scenario: Player nova's inventory isolated
- **WHEN** player `nova` runs `inventory` command
- **THEN** only rows with `player_id = 'nova'` are returned; player `byte`'s items are not visible

#### Scenario: New player initialized with correct player_id
- **WHEN** player `byte` connects for the first time
- **THEN** all initialization INSERTs use `player_id = 'byte'`

### Requirement: Shared tables remain unscoped
Tables that represent shared world state SHALL NOT have a `player_id` column: `generated_content`, `world_events`. These are shared across all players.

#### Scenario: Generated rooms visible to all players
- **WHEN** player `nova` runs `explore` and a new room is generated and cached in `generated_content`
- **THEN** player `byte` can also enter that room and the generation is not repeated

### Requirement: player.Load and player.Save accept player_id parameter
The `player.Load(db, playerID string)` and `player.Save(db, playerID string, state State)` functions SHALL accept a `playerID` parameter. The existing CLI path SHALL pass `"default"` as the playerID.

#### Scenario: Load returns correct player
- **WHEN** `player.Load(db, "nova")` is called
- **THEN** it returns the State row where `player_id = 'nova'`, or default State if no row exists

#### Scenario: Save writes to correct player
- **WHEN** `player.Save(db, "nova", state)` is called
- **THEN** it upserts the row with `player_id = 'nova'` without affecting `player_id = 'byte'`
