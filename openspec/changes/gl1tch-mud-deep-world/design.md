## Context

gl1tch-mud uses a one-shot stdin/stdout model: each command is a separate invocation with stdin carrying the user's input. The command dispatcher reads the world from YAML (embedded), player state from SQLite, runs the handler, prints output, publishes any BUSD event, and exits. All new systems must fit this pattern — no persistent server process.

## Goals / Non-Goals

**Goals:**
- All six new gameplay systems (crafting, hacking, lockpicking, trading, looting, espionage) are config-driven from world.yaml; adding content requires no Go changes
- Ollama world evolution generates rooms/NPCs/items via local model with zero external dependencies
- Every action emits a typed BUSD event for gl1tch companion narration
- SQLite schema is additive; existing world.db files migrate cleanly via `IF NOT EXISTS` DDL
- Player skills progress through XP; each system references the relevant skill

**Non-Goals:**
- Multiplayer (single-player SQLite model stays)
- Real-time server (one-shot invocation model stays)
- External LLM APIs (only Ollama at localhost:11434)
- Graphical output or TUI (text only, piped through gl1tch chat panel)

## Decisions

### 1. Skill checks use a single dice roll formula

All gated actions (hack, pick, trade reputation checks) use: `roll = rand(1,100) + skill_level - difficulty`. Success when `roll >= 50`. This is readable, testable, and consistent. No separate stats (INT, DEX) — one skill per domain avoids stat-bloat for a single-player terminal game.

**Alternative considered**: D20-style stat modifiers. Rejected — adds complexity with no meaningful tradeoff at single-player scale.

### 2. World evolution is prompt-templated, not free-form

`explore <direction>` sends a structured prompt to Ollama with: current room name/desc, player level, world theme. The response is parsed as a YAML fragment matching the existing Room schema. On parse failure the command returns "nothing there yet" — generation is optional, never blocking.

**Alternative considered**: Free-form prose + post-processing. Rejected — fragile; YAML template gives deterministic parsing and a consistent room format.

### 3. Lock state and stealth are session-scoped

Lock state (unlocked/picked), stealth level, and NPC memory reset each session (when `main()` starts). Faction reputation and player skills persist across sessions. This avoids a "permanently unlocked world" problem while giving meaningful skill progression.

**Alternative considered**: Fully persistent lock state. Rejected — would make the world trivially solvable after first playthrough; stealth/espionage lose tension.

### 4. Loot tables are declared in world.yaml, not per-NPC inline

A top-level `loot_tables:` section holds named tables; NPCs reference by `loot_table_id`. This allows multiple NPCs to share a table and makes tables reusable in generated rooms.

**Alternative considered**: Inline loot per NPC. Rejected — duplicates data when multiple NPCs of the same type exist.

### 5. Ollama generation cache keyed by prompt hash

Generated rooms are stored in `generated_content` with a SHA256 of the prompt as cache key. On subsequent `explore` to the same unmapped direction, the cached room is loaded from SQLite instead of re-calling Ollama. Avoids redundant generation and makes the world deterministic across restarts.

### 6. NPC dialogue uses a simple condition DSL, not Lua

Dialogue triggers use a micro-DSL: `has_item:<id>`, `rep_gte:<faction>:<n>`, `skill_gte:<skill>:<n>`, `always`. Evaluated in Go with a small switch — no embedded interpreter. Complex branching is handled by ordering multiple trigger conditions.

**Alternative considered**: Embedded Lua (gopher-lua). Rejected — overkill for terminal MUD; adds a large dependency for marginal expressiveness.

## Architecture

### New Packages

```
internal/crafting/      — recipe loading, ingredient check, item output
internal/hacking/       — system state, skill check, alert escalation
internal/locking/       — lock declaration, pick/unlock logic, key matching
internal/trading/       — NPC offer loading, reputation check, item swap
internal/looting/       — loot table rolling, item drop into room
internal/espionage/     — stealth state, disguise, NPC memory, dialogue eval
internal/generation/    — Ollama client, room/NPC/item prompt, YAML parser, cache
internal/skills/        — skill XP table, level-up thresholds, XP award
```

### world.yaml Extensions

```yaml
# Top-level additions
crafting_recipes:
  - id: packet-sniffer
    name: "Packet Sniffer"
    ingredients: [{id: raw-silicon, count: 2}, {id: copper-wire, count: 1}]
    output: {id: packet-sniffer, name: "Packet Sniffer", desc: "..."}
    skill_req: 3  # hacking skill level required

loot_tables:
  - id: netrunner-loot
    entries:
      - {item_id: data-chip, probability: 0.7, count_min: 1, count_max: 2}
      - {item_id: credits, probability: 1.0, count_min: 10, count_max: 50}

# Per-room additions
rooms:
  - id: net-1
    systems:
      - {id: ice-wall, security_level: 4, reward_item: root-key}
    locks:
      - {id: vault-exit, exit: north, difficulty: 6, keys: [root-key]}

# Per-NPC additions
npcs:
  - id: netrunner-0
    loot_table_id: netrunner-loot
    trades:
      - id: trade-001
        wants: [{id: data-chip, count: 1}]
        offers: [{id: encryption-key, count: 1}]
        faction_req: ""   # empty = anyone
    dialogue:
      - {trigger: "always", text: "System's hot. Watch yourself."}
      - {trigger: "has_item:root-key", text: "Where'd you get that key?"}
      - {trigger: "rep_gte:netrunners:10", text: "One of us. Good."}
```

### Database Schema Additions (all additive)

```sql
CREATE TABLE IF NOT EXISTS player_skills (
    skill TEXT PRIMARY KEY, level INTEGER DEFAULT 0, xp INTEGER DEFAULT 0);
CREATE TABLE IF NOT EXISTS player_reputation (
    faction TEXT PRIMARY KEY, value INTEGER DEFAULT 0);
CREATE TABLE IF NOT EXISTS system_state (
    room_id TEXT, system_id TEXT, intrusion REAL DEFAULT 0, alert INTEGER DEFAULT 0,
    PRIMARY KEY (room_id, system_id));
CREATE TABLE IF NOT EXISTS lock_state (
    lock_id TEXT PRIMARY KEY, unlocked INTEGER DEFAULT 0);
CREATE TABLE IF NOT EXISTS npc_memory (
    npc_id TEXT, action TEXT, ts INTEGER, PRIMARY KEY (npc_id, action));
CREATE TABLE IF NOT EXISTS player_stealth (
    id INTEGER PRIMARY KEY CHECK (id=1), level INTEGER DEFAULT 50, disguise TEXT DEFAULT 'none');
CREATE TABLE IF NOT EXISTS generated_content (
    prompt_hash TEXT PRIMARY KEY, type TEXT, yaml_blob TEXT, created_at INTEGER);
```

### BUSD Event Map

| Action | Topic | Key Payload Fields |
|--------|-------|--------------------|
| Craft success | `mud.craft.completed` | recipe_id, output_item |
| Craft fail (missing ingredients) | `mud.craft.failed` | recipe_id, missing |
| Hack success | `mud.hack.success` | system_id, reward |
| Hack fail / alert raised | `mud.hack.alert` | system_id, alert_level |
| Lock picked | `mud.lock.picked` | lock_id, skill_used |
| Trade completed | `mud.trade.completed` | npc_id, gave, received |
| Loot dropped | `mud.loot.dropped` | npc_id, items |
| Stealth broken | `mud.stealth.broken` | room_id, by_npc |
| Room generated | `mud.world.generated` | room_id, direction, model |

## Migration Plan

1. Extend `internal/db/schema.go` with new `IF NOT EXISTS` tables — existing DBs auto-migrate
2. Extend world.yaml types in `internal/world/world.go`
3. Implement each new package independently, unit-testable in isolation
4. Register new commands in `commands.Registry`
5. Modify `attack` to call `looting.Roll()` on NPC death
6. Modify `go` to call `locking.CheckExit()` before room transition
7. Expand `worlds/cyberspace/world.yaml` with content for all new systems
8. Wire generation package into `explore` command

No changes to the one-shot binary model, BUSD client, or player.Load/Save.
