## ADDED Requirements

### Requirement: Skills table in SQLite
Player skills SHALL be stored in `player_skills` table with columns: `skill TEXT PRIMARY KEY`, `level INTEGER DEFAULT 0`, `xp INTEGER DEFAULT 0`. Skills persist across sessions.

Supported skills for v1: `hacking`, `lockpicking`.

### Requirement: XP award on skill use
Each successful use of a skill-gated action SHALL award XP:
- Successful hack: +20 hacking XP
- Successful pick: +15 lockpicking XP

XP thresholds for level-up: level 1 = 50 XP, level 2 = 150 XP, level 3 = 300 XP, level 4 = 600 XP, level 5 = 1000 XP. No cap beyond level 5 for v1.

### Requirement: Level-up notification
When XP crosses a level threshold, gl1tch-mud SHALL print a level-up message and emit `mud.skill.levelup` with skill name and new level.

#### Scenario: XP award after successful hack
- **WHEN** hack succeeds
- **THEN** hacking XP increases by 20; if threshold crossed, level-up message shown

#### Scenario: Skills persist across sessions
- **WHEN** player exits and restarts gl1tch-mud
- **THEN** skill levels and XP are loaded from SQLite unchanged

#### Scenario: skills command
- **WHEN** player types `skills`
- **THEN** current level and XP for each skill is printed in a formatted list
