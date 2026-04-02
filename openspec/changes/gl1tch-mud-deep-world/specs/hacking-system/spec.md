## ADDED Requirements

### Requirement: System declarations on rooms
A room in world.yaml MAY declare a `systems:` list. Each entry SHALL have:
- `id` (string, required): unique system identifier within the room
- `security_level` (integer 1–10, required): difficulty of the hack
- `reward_item` (string, optional): item ID added to player inventory on successful hack
- `reward_text` (string, optional): text printed on success

### Requirement: Player hacking skill
The player state SHALL include a `hacking` skill (integer, default 0). Skill level affects hack success probability.

### Requirement: hack command
`hack <system-id>` SHALL:
1. Check the current room has a system with that ID
2. Roll `rand(1,100) + hacking_skill - (security_level * 10)`. Success when roll ≥ 50
3. On success: deliver `reward_item` to inventory (if declared), print success text, emit `mud.hack.success`, award skill XP
4. On failure: increment `alert_level` for the system by 1; if alert_level ≥ 3, trigger NPC aggro in the room; emit `mud.hack.alert`

#### Scenario: Successful hack
- **WHEN** roll succeeds
- **THEN** reward delivered, XP awarded, event fires

#### Scenario: Failed hack raises alert
- **WHEN** roll fails
- **THEN** alert_level increments; at threshold 3 NPCs in room become hostile, event fires

#### Scenario: System already hacked
- **WHEN** player attempts to hack a system they already successfully hacked this session
- **THEN** command prints "already compromised" and returns

#### Scenario: No system in room
- **WHEN** player types `hack terminal-99` in a room with no systems
- **THEN** command prints "no hackable systems here"
