## ADDED Requirements

### Requirement: Stealth state
The player has a `stealth_level` (integer 0–100, default 50, session-scoped) and a `disguise` (string, default "none"). Higher stealth reduces NPC detection; disguise affects dialogue triggers.

### Requirement: hide command
`hide` SHALL attempt to raise stealth_level by `rand(5,15)` capped at 100. Prints current stealth level. Emits `mud.stealth.hidden` if level increases above 70.

### Requirement: disguise command
`disguise <item-id>` SHALL:
1. Check player has the item and it is tagged as a disguise in world.yaml
2. Set `disguise` field on player stealth state
3. Print confirmation
4. Items used as disguises are NOT consumed (they are worn, not spent)

### Requirement: talk command
`talk <npc-id>` SHALL:
1. Find the NPC in the current room
2. Evaluate NPC dialogue triggers in order; use the first matching trigger's text
3. Trigger conditions: `always`, `has_item:<item-id>`, `rep_gte:<faction>:<n>`, `skill_gte:<skill>:<n>`, `disguise:<value>`
4. Record the interaction in `npc_memory` (npc_id, action="talked", timestamp)
5. Emit `mud.espionage.talked` with npc_id and matched trigger

#### Scenario: always trigger matches
- **WHEN** player talks to NPC and the first trigger is `always`
- **THEN** that line of dialogue is shown

#### Scenario: has_item trigger matches
- **WHEN** player has the specified item in inventory
- **THEN** that dialogue line is shown

#### Scenario: rep_gte trigger matches
- **WHEN** player's faction reputation meets the threshold
- **THEN** that dialogue line is shown

#### Scenario: NPC has no dialogue
- **WHEN** NPC has no `dialogue:` block
- **THEN** generic "they don't seem interested in talking" message

### Requirement: NPC detection of player
When a player with stealth_level < 30 enters a room with a hostile NPC, the NPC SHOULD automatically attack. Print a detection message and emit `mud.stealth.broken`.

#### Scenario: Low stealth triggers auto-combat
- **WHEN** player moves into a room with a hostile NPC and stealth_level < 30
- **THEN** NPC attacks first, message printed, event fires
