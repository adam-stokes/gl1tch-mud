## 1. Schema & World Extensions

- [x] 1.1 Add `System`, `Lock`, `DialogueLine`, `TradeOffer`, `CraftingRecipe`, `LootTable`, `LootEntry` structs to `internal/world/world.go`
- [x] 1.2 Extend `Room` with `Systems []System` and `Locks []Lock`
- [x] 1.3 Extend `NPC` with `LootTableID`, `Trades`, `Dialogue`
- [x] 1.4 Extend `Item` with `IsDisguise bool`
- [x] 1.5 Extend `World` with `CraftingRecipes` and `LootTables` top-level slices
- [x] 1.6 Add `FindLootTable(id string)` and `FindRecipe(id string)` helpers to `World`
- [x] 1.7 Verify existing `worlds/cyberspace/world.yaml` still parses correctly (write a test)

## 2. Database Schema

- [x] 2.1 Add `player_skills` table (`skill TEXT PRIMARY KEY, level INT DEFAULT 0, xp INT DEFAULT 0`) to `internal/db/schema.go`
- [x] 2.2 Add `player_reputation` table (`faction TEXT PRIMARY KEY, value INT DEFAULT 0`)
- [x] 2.3 Add `system_state` table (`room_id TEXT, system_id TEXT, intrusion REAL, alert INT, PRIMARY KEY(room_id,system_id)`)
- [x] 2.4 Add `lock_state` table (`lock_id TEXT PRIMARY KEY, unlocked INT DEFAULT 0`)
- [x] 2.5 Add `npc_memory` table (`npc_id TEXT, action TEXT, ts INT, PRIMARY KEY(npc_id,action)`)
- [x] 2.6 Add `player_stealth` table (`id INT PRIMARY KEY CHECK(id=1), level INT DEFAULT 50, disguise TEXT DEFAULT 'none'`)
- [x] 2.7 Add `generated_content` table (`prompt_hash TEXT PRIMARY KEY, type TEXT, yaml_blob TEXT, created_at INT`)
- [x] 2.8 Write schema migration test: open existing DB, apply new schema, confirm all tables exist

## 3. Player Skills Package

- [x] 3.1 Create `internal/skills/skills.go` with `Load(db)`, `Award(db, skill, xp)`, `Level(db, skill)`, `XP(db, skill)` functions
- [x] 3.2 Implement XP threshold table and level-up detection in `Award`
- [x] 3.3 Add `skills` command to `commands.Registry` — prints all skill levels and XP
- [x] 3.4 Write unit tests: award XP, level up at threshold, persist across calls

## 4. Looting System

- [x] 4.1 Create `internal/looting/looting.go` with `Roll(w *world.World, npcID string) []world.Item`
- [x] 4.2 Implement probability roll per loot entry using `math/rand`
- [x] 4.3 Modify `commands.Attack` to call `looting.Roll` on NPC death and add items to room
- [x] 4.4 Print loot summary in attack result output
- [x] 4.5 Emit `mud.loot.dropped` BUSD event on NPC death with loot list
- [x] 4.6 Write unit tests: 0.0 probability never drops, 1.0 always drops, count range respected

## 5. Hacking System

- [x] 5.1 Create `internal/hacking/hacking.go` with `Hack(db, room, systemID, hackingSkill) Result`
- [x] 5.2 Implement skill roll formula: `rand(1,100) + skill - security_level*10 >= 50`
- [x] 5.3 Implement alert escalation in `system_state` table; at alert=3 mark NPCs hostile
- [x] 5.4 Add `hack` command to `commands.Registry`
- [x] 5.5 On success: deliver reward item, award hacking XP via skills package, emit `mud.hack.success`
- [x] 5.6 On failure: increment alert, emit `mud.hack.alert`
- [x] 5.7 Write unit tests: success path, failure/alert path, already-hacked guard

## 6. Lockpicking System

- [x] 6.1 Create `internal/locking/locking.go` with `IsLocked(db, lockID)`, `Unlock(db, lockID)`, `Pick(db, lockID, difficulty, skill) bool`
- [x] 6.2 Modify `commands.Go` to call `locking.IsLocked` for the exit before allowing movement
- [x] 6.3 Add `pick` command: roll formula, update lock_state on success, award lockpicking XP
- [x] 6.4 Add `unlock` command: check inventory for key item, unlock without roll
- [x] 6.5 Emit `mud.lock.picked` on successful pick
- [x] 6.6 Write unit tests: locked exit blocks movement, key bypasses roll, pick succeeds/fails

## 7. Trading System

- [x] 7.1 Create `internal/trading/trading.go` with `ListOffers(w, npcID, rep map) []TradeOffer` and `Execute(db, w, npcID, tradeID, inventory, rep) error`
- [x] 7.2 Add `offers` command to `commands.Registry`
- [x] 7.3 Add `trade` command: check rep, check inventory, swap items, increment faction rep, emit `mud.trade.completed`
- [x] 7.4 Write unit tests: missing items refused, faction gate enforced, successful swap

## 8. Crafting System

- [x] 8.1 Create `internal/crafting/crafting.go` with `Craft(db, w, recipeID, inventory, hackingSkill) Result`
- [x] 8.2 Add `craft` command to `commands.Registry`
- [x] 8.3 Implement ingredient check, consumption, output creation, skill gate
- [x] 8.4 Emit `mud.craft.completed` on success, `mud.craft.failed` on missing ingredients
- [x] 8.5 Write unit tests: missing ingredients listed, skill gate blocks, successful craft produces item

## 9. Espionage System

- [x] 9.1 Create `internal/espionage/espionage.go` with stealth load/save, dialogue evaluator, NPC memory record
- [x] 9.2 Implement `EvalDialogue(triggers []DialogueLine, player state) string`
- [x] 9.3 Add `hide` command: roll stealth increase, persist to `player_stealth`
- [x] 9.4 Add `disguise` command: check `is_disguise` on item, update `player_stealth.disguise`
- [x] 9.5 Add `talk` command: evaluate dialogue triggers, record in npc_memory, emit `mud.espionage.talked`
- [x] 9.6 Add auto-combat trigger in `commands.Go` / `commands.Look`: if stealth < 30 and hostile NPC present, print detection message, emit `mud.stealth.broken`
- [x] 9.7 Write unit tests: trigger evaluation order, always-matches, has_item match, rep_gte match

## 10. World Evolution (Ollama Generation)

- [x] 10.1 Create `internal/generation/generation.go` with `Generator` struct, `Generate(ctx, currentRoom, direction) (*world.Room, error)`
- [x] 10.2 Implement SHA256 prompt hash and cache lookup/write in `generated_content`
- [x] 10.3 Implement Ollama POST with 5s timeout, parse response as YAML Room fragment
- [x] 10.4 Add bidirectional exit wiring between current room and generated room in live world graph
- [x] 10.5 Add `explore` command to `commands.Registry`; fall through to `go` if exit already exists
- [x] 10.6 Emit `mud.world.generated` on successful generation
- [x] 10.7 Write unit tests: cache hit skips Ollama, malformed response returns gracefully, timeout returns gracefully (mock HTTP)

## 11. World Content Expansion

- [x] 11.1 Add `crafting_recipes:` section to `worlds/cyberspace/world.yaml` with 3 recipes
- [x] 11.2 Add `loot_tables:` section with tables for each NPC type
- [x] 11.3 Add `systems:` to at least 2 rooms with `security_level` and reward items
- [x] 11.4 Add `locks:` to at least 1 exit with a key item placed elsewhere
- [x] 11.5 Add `trades:` to at least 1 NPC with faction requirement
- [x] 11.6 Add `dialogue:` to at least 2 NPCs with `always` and at least one conditional trigger
- [x] 11.7 Add 2 new static rooms to the world graph to give the evolution system anchor points
- [x] 11.8 Tag at least 1 item as `is_disguise: true`

## 12. Integration & Build

- [x] 12.1 Wire all new packages into `main.go` (pass db + world to new command handlers)
- [x] 12.2 Run `go build ./...` — all green
- [x] 12.3 Run `go test ./...` — all green
- [x] 12.4 Manual smoke test: `/mud` → craft, hack, pick, trade, loot, talk, explore
