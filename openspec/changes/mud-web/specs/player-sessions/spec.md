## ADDED Requirements

### Requirement: Player identity from login
A player's identity SHALL be the `playerID` string provided in the `auth` message. The `playerID` is used as the key for all DB queries (see `player-id-scoping` spec). Player names SHALL be 2–20 characters, alphanumeric plus hyphens.

#### Scenario: Valid player name
- **WHEN** client sends `{"type":"auth","payload":{"playerID":"nova","passphrase":"secret"}}`
- **THEN** server accepts the name and loads or initializes state for player `nova`

#### Scenario: Invalid player name (too short)
- **WHEN** client sends playerID `"x"`
- **THEN** server responds with `auth.fail` reason `"playerID must be 2-20 alphanumeric characters"`

#### Scenario: New player first login
- **WHEN** a playerID with no existing DB record authenticates
- **THEN** server initializes default state (start room, full HP, zero skills, zero credits) and sends `auth.ok` with level 1

#### Scenario: Returning player login
- **WHEN** a playerID with existing DB records authenticates
- **THEN** server loads existing state (current room, HP, inventory, skills) and sends `auth.ok` with current level and XP

### Requirement: One connection per player name
At most one active WebSocket connection SHALL exist per `playerID`. If a second connection authenticates with the same name, the server SHALL reject it with `auth.fail`.

#### Scenario: Duplicate login rejected
- **WHEN** player `nova` is already connected and a second client sends auth with playerID `nova`
- **THEN** server responds with `{"type":"auth.fail","payload":{"reason":"player already connected"}}` and closes the new connection

### Requirement: Session cleanup on disconnect
When a session ends (disconnect or error), the server SHALL release the player's name so they can reconnect.

#### Scenario: Reconnect after disconnect
- **WHEN** player `nova` disconnects and then reconnects with the same name
- **THEN** the server accepts the new connection and loads state from DB

### Requirement: Idle timeout
Sessions with no messages for 30 minutes SHALL be closed by the server with a `{"type":"error","payload":{"message":"session timeout"}}` message before closing.

#### Scenario: Idle session closed
- **WHEN** no message has been received from a client for 30 minutes
- **THEN** server sends the timeout error message and closes the WebSocket
