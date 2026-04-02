## ADDED Requirements

### Requirement: Trade declarations on NPCs
An NPC in world.yaml MAY declare a `trades:` list. Each entry SHALL have:
- `id` (string, required): unique trade identifier
- `wants` (list, required): items the NPC accepts; each has `id` and `count`
- `offers` (list, required): items the NPC gives in return; each has `id`, `name`, `desc`, `count`
- `faction_req` (string, optional): faction name and minimum rep required (format `"faction:minRep"`)

### Requirement: Faction reputation persists in SQLite
Player reputation per faction is stored in `player_reputation`. Default value is 0. Reputation is awarded by completing trades, quests (future), or certain dialogue outcomes.

### Requirement: offers command
`offers <npc-id>` SHALL list all trades the NPC is willing to make given the player's current faction reputation. Trades with unmet `faction_req` are omitted.

### Requirement: trade command
`trade <trade-id>` SHALL:
1. Look up the trade on the NPC in the current room
2. Check faction reputation requirement is met
3. Check player inventory contains all `wants` items in required count
4. Remove wants items, add offers items, increment faction reputation by 1
5. Print confirmation, emit `mud.trade.completed`

#### Scenario: Successful trade
- **WHEN** player has required items and meets faction rep
- **THEN** items swapped, rep increments, event fires

#### Scenario: Missing wanted items
- **WHEN** player lacks required items
- **THEN** trade refused, list of missing items shown

#### Scenario: Faction requirement not met
- **WHEN** player's rep is below faction_req threshold
- **THEN** NPC refuses: "we don't do business with strangers"

#### Scenario: NPC has no trades
- **WHEN** player types `offers <npc-id>` for an NPC with no trade declarations
- **THEN** prints "nothing to offer"
