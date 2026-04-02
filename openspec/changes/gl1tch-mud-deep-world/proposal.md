## Why

gl1tch-mud is a 4-room proof-of-concept with basic movement and combat but no depth — there is nothing to discover, acquire, build, steal, or exploit. A cyberpunk MUD without hacking, espionage, or a world that grows as you explore it is just a hallway. This change turns it into a game.

## What Changes

- **Crafting system**: recipe table in world.yaml, `craft` command, ingredient consumption from inventory, output item generation
- **Hacking system**: rooms and items gain `security_level`; player has a `hacking` skill; `hack` command runs a skill check and unlocks systems or retrieves loot on success; failed hacks raise `alert_level` and trigger NPC response
- **Lockpicking**: exits and containers gain optional `lock` declarations (difficulty + key IDs); `pick` and `unlock` commands; per-session lock state persisted in SQLite
- **Trading**: NPCs declare `trades` in world.yaml (wants/offers, faction requirement); `trade` and `offers` commands; faction reputation table drives NPC willingness
- **Looting**: NPCs declare `loot_table_id`; on NPC death the loot table is rolled and items are dropped into the room; items have rarity and count ranges
- **Espionage**: `hide`, `disguise`, and `talk` commands; stealth level affects NPC detection; NPC dialogue trees with condition triggers; NPC memory persisted per session
- **Ollama world evolution**: when a player uses `explore <direction>` on an unmapped exit, gl1tch-mud calls the local Ollama narrator model to generate a new room, NPC, and starter items, persists them to SQLite, and adds them to the live world graph — the world grows as you play
- **Extended world.yaml schema**: rooms gain `systems`, `locks`, `loot_tables`; NPCs gain `loot_table_id`, `dialogue`, `trades`; top-level `crafting_recipes` and `loot_tables` sections added
- **BUSD events**: all new actions emit typed events (`mud.hack.*`, `mud.craft.*`, `mud.trade.*`, `mud.stealth.*`, `mud.world.generated`) so the gl1tch companion can narrate them

## Capabilities

### New Capabilities
- `crafting-system`: Recipe DSL in world.yaml, `craft` command, ingredient validation, output item creation, BUSD event
- `hacking-system`: Per-room/item security levels, player skill checks, exploit progression, alert escalation, BUSD events
- `lockpicking-system`: Lock declarations on exits and containers, `pick`/`unlock` commands, key items, difficulty rolls, lock state persistence
- `trading-system`: NPC trade offers in world.yaml, `trade`/`offers` commands, faction reputation tracking
- `looting-system`: Loot tables with probability/rarity, loot roll on NPC death, item drops into room
- `espionage-system`: Stealth state, disguise items, `hide`/`disguise`/`talk` commands, NPC dialogue trees, NPC memory per session
- `world-evolution`: `explore` command, Ollama-driven room/NPC/item generation, dynamic world graph expansion, generation cache in SQLite
- `player-skills`: Skills table (hacking, stealth, lockpicking) with XP progression, level-up events
- `world-yaml-extensions`: Extended schema for rooms, NPCs, items, top-level recipes and loot tables

### Modified Capabilities
- `command-dispatcher`: Existing `attack` result wires into new loot roll on NPC death; existing `go` command checks lock state before allowing room transition

## Impact

- `internal/commands/commands.go` — new handlers registered: `craft`, `hack`, `exploit`, `pick`, `unlock`, `trade`, `offers`, `hide`, `disguise`, `talk`, `explore`; `attack` modified to trigger loot roll; `go` modified to check lock state
- `internal/db/schema.go` — 12 new tables: `player_skills`, `system_state`, `locks`, `lock_state`, `npc_trades`, `npc_trade_items`, `player_reputation`, `loot_tables`, `loot_entries`, `player_stealth`, `npc_memory`, `generated_content`
- `internal/world/world.go` — `Room`, `NPC`, `Item` structs extended; new `CraftingRecipe`, `LootTable`, `Lock`, `System`, `TradeOffer`, `Dialogue` types
- `internal/player/player.go` — `State` gains `Skills map[string]int`, `Reputation map[string]int`, `StealthLevel int`, `Disguise string`
- `internal/generation/` — new package: Ollama client, room/NPC/item prompt templates, response parser, SQLite persistence
- `worlds/cyberspace/world.yaml` — expanded with recipes, loot tables, lock declarations, NPC trades, NPC dialogue, 2-3 additional static rooms as anchors
- No new external Go dependencies (direct HTTP to Ollama, same as gl1tch)
