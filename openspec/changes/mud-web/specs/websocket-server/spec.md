## ADDED Requirements

### Requirement: HTTP server binds and serves
The server SHALL start an HTTP listener on a configurable port (default 8080, overridable via `--port` flag or `MUDSERVER_PORT` env var). It SHALL serve the embedded Astro static files at `/` and upgrade WebSocket connections at `/ws`.

#### Scenario: Server starts on default port
- **WHEN** `mudserver` is launched with no port flag
- **THEN** it listens on `0.0.0.0:8080` and logs the address to stdout

#### Scenario: Server starts on custom port
- **WHEN** `mudserver --port 9000` is launched
- **THEN** it listens on `0.0.0.0:9000`

#### Scenario: Static files are served
- **WHEN** a browser requests `GET /`
- **THEN** the server responds with the embedded `index.html` from the Astro build

### Requirement: WebSocket upgrade at /ws
The server SHALL upgrade HTTP connections at `/ws` to WebSocket. Non-WebSocket requests to `/ws` SHALL receive HTTP 400.

#### Scenario: Valid WebSocket upgrade
- **WHEN** a client sends a valid WebSocket upgrade request to `/ws`
- **THEN** the server completes the handshake and the connection enters the auth-pending state

#### Scenario: Non-WebSocket request to /ws
- **WHEN** a plain HTTP GET is sent to `/ws`
- **THEN** the server responds with HTTP 400 and closes the connection

### Requirement: Authentication handshake
After WebSocket connection is established, the server SHALL wait for an `auth` message. If the first message is not `auth`, or if the passphrase does not match `MUDSERVER_PASSPHRASE`, the server SHALL send `auth.fail` and close the connection. On success, the server SHALL send `auth.ok` with the player's current level, title, and XP.

#### Scenario: Correct passphrase
- **WHEN** client sends `{"type":"auth","payload":{"playerID":"nova","passphrase":"secret"}}`
- **THEN** server responds with `{"type":"auth.ok","payload":{"playerID":"nova","level":1,"title":"Script Kiddie","xp":0}}`

#### Scenario: Wrong passphrase
- **WHEN** client sends an auth message with an incorrect passphrase
- **THEN** server responds with `{"type":"auth.fail","payload":{"reason":"invalid passphrase"}}` and closes the connection

#### Scenario: First message is not auth
- **WHEN** client sends any non-auth message before authenticating
- **THEN** server responds with `{"type":"auth.fail","payload":{"reason":"auth required"}}` and closes the connection

### Requirement: Command routing
After authentication, each `input` message SHALL be parsed as a game command (verb + optional args), dispatched to the matching handler in `commands.Registry`, and the result streamed back as `output.token` messages followed by `output.done`.

#### Scenario: Known command
- **WHEN** authenticated client sends `{"type":"input","payload":{"text":"look"}}`
- **THEN** server dispatches to the `look` handler and sends the room description as one or more `output.token` messages, then `output.done`

#### Scenario: Unknown command
- **WHEN** authenticated client sends `{"type":"input","payload":{"text":"frobnicate"}}`
- **THEN** server sends `{"type":"output.token","payload":{"token":"Unknown command: frobnicate\n"}}` then `output.done`

#### Scenario: Empty input
- **WHEN** authenticated client sends `{"type":"input","payload":{"text":""}}`
- **THEN** server sends only `output.done` with no output tokens

### Requirement: Interrupt handling
The server SHALL cancel any in-progress command execution when it receives an `interrupt` message from the client, and respond with `output.done`.

#### Scenario: Interrupt during slow command
- **WHEN** a long-running command (e.g., `explore`) is in progress and the client sends `{"type":"interrupt"}`
- **THEN** the server cancels the context for that command and sends `output.done`

### Requirement: Graceful disconnect
When a WebSocket connection closes (client disconnect, timeout, or error), the server SHALL cancel any in-progress command for that session and remove the session from the active registry.

#### Scenario: Client disconnects mid-command
- **WHEN** the WebSocket connection drops while a command is executing
- **THEN** the command context is cancelled and the session is removed; DB state already committed is preserved
