## ADDED Requirements

### Requirement: Lock declarations on room exits
A room exit in world.yaml MAY include a `lock` object with:
- `id` (string, required): unique lock identifier
- `difficulty` (integer 1–10, required): pick difficulty
- `keys` (list of strings, optional): item IDs that unlock without a skill check

### Requirement: Lock state persists per session in SQLite
Lock state (locked/unlocked) is stored in `lock_state` table. All locks start locked at session start. Unlocked state persists until the session ends (process exits).

### Requirement: go command checks lock state
Before moving through an exit, the `go` command SHALL check whether the exit has a lock and whether that lock is unlocked. If the exit is locked the player cannot pass and receives a message indicating the lock.

#### Scenario: Locked exit blocks movement
- **WHEN** player types `go north` and the north exit has an unresolved lock
- **THEN** movement is blocked and player sees the lock description

### Requirement: pick command
`pick <lock-id>` SHALL:
1. Check player has a `lockpicking` skill entry
2. Roll `rand(1,100) + lockpicking_skill - (difficulty * 10)`. Success when roll ≥ 50
3. On success: set lock state to unlocked in SQLite, print success message, emit `mud.lock.picked`, award lockpicking XP
4. On failure: print failure message with hint, no state change

### Requirement: unlock command
`unlock <lock-id>` with a matching key item in inventory SHALL set the lock to unlocked without a skill check.

#### Scenario: Key in inventory bypasses skill check
- **WHEN** player has a key item whose ID matches the lock's `keys` list
- **THEN** lock opens immediately

#### Scenario: pick succeeds
- **WHEN** skill roll succeeds
- **THEN** lock state is unlocked, exit becomes passable, XP awarded

#### Scenario: pick fails
- **WHEN** skill roll fails
- **THEN** lock remains locked, no state change, failure message printed
