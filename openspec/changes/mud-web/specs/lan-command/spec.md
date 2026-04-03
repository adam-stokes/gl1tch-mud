## ADDED Requirements

### Requirement: /lan starts the embedded server
The `/lan [passphrase]` command SHALL start the HTTP/WebSocket server as a background goroutine within the running `gl1tch-mud` process and print the LAN URL and optional passphrase reminder. If the server is already running, the command SHALL print the existing URL and connected player count instead of starting a second instance.

#### Scenario: /lan starts server and prints URL
- **WHEN** user types `/lan` and no server is running
- **THEN** the server starts on a default port (8080) and output includes the LAN URL: `LAN session started: http://192.168.1.x:8080` and `Share this URL with your players.`

#### Scenario: /lan with passphrase
- **WHEN** user types `/lan secret123`
- **THEN** server starts with passphrase `secret123` and output includes `Passphrase: secret123`

#### Scenario: /lan when already running
- **WHEN** server is already active and user types `/lan`
- **THEN** output shows `LAN session already active: http://192.168.1.x:8080 (2 players connected)`

### Requirement: /lan stop shuts down the server
`/lan stop` SHALL gracefully shut down the embedded server — closing all active WebSocket sessions with a disconnect message — and print confirmation.

#### Scenario: /lan stop with active sessions
- **WHEN** server is running with connected players and user types `/lan stop`
- **THEN** all WebSocket connections receive `{"type":"error","payload":{"message":"server shutting down"}}` and are closed; output prints `LAN session stopped.`

#### Scenario: /lan stop when not running
- **WHEN** no server is running and user types `/lan stop`
- **THEN** output prints `No LAN session is active.`

### Requirement: /lan status shows connected players
`/lan status` SHALL print the server URL and a list of currently connected player names.

#### Scenario: /lan status with players
- **WHEN** server is running with players `nova` and `byte` connected
- **THEN** output shows:
  ```
  LAN session: http://192.168.1.x:8080
  Connected players: nova, byte
  ```

#### Scenario: /lan status with no players
- **WHEN** server is running but no browser players are connected
- **THEN** output shows `LAN session: http://... (no players connected)`

### Requirement: LAN IP detection
The server SHALL detect the machine's primary non-loopback IPv4 address for the URL. If detection fails, it SHALL fall back to `localhost`.

#### Scenario: LAN IP shown
- **WHEN** machine has a non-loopback IPv4 address (e.g., 192.168.1.5)
- **THEN** URL uses that address, not `127.0.0.1`

#### Scenario: Detection fallback
- **WHEN** no non-loopback IPv4 address is found
- **THEN** URL uses `localhost`
