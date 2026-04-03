## Why

gl1tch-mud is a single-player CLI game with no network layer. The author wants to host a LAN session for their kids from within the game itself — type `/lan`, get a URL, share it, and kids connect via browser without installing anything.

## What Changes

- New `/lan` command in `commands/`: starts the embedded HTTP/WebSocket server as a background goroutine and prints the LAN URL. `/lan stop` shuts it down. `/lan status` shows connected players.
- New `internal/server` package: session manager, WebSocket protocol types, broadcast hub, auth middleware.
- New `web/` directory: Astro static site with xterm.js terminal. Built output is embedded in the gl1tch-mud binary via `//go:embed`.
- `internal/db`: schema gains `player_id` scoping on all player-state tables (non-breaking; defaults to `"default"` for the CLI host player).
- `internal/player`: Load/Save parameterized by player ID.
- `go.mod`: add WebSocket dependency (`nhooyr.io/websocket`).
- `Makefile`: add `web-build` target (builds Astro; must run before `make build`).

## Capabilities

### New Capabilities

- `lan-command`: `/lan`, `/lan stop`, `/lan status` commands that control the embedded multiplayer server from the CLI game loop.
- `websocket-server`: HTTP/WebSocket server that accepts browser player connections, authenticates via shared passphrase, and routes game commands to the existing handler dispatch.
- `player-sessions`: Per-player identity, state isolation, and lifecycle management (connect, disconnect, timeout). Browser players share the same world as the CLI host.
- `web-terminal`: Astro + xterm.js browser client. Login screen → full-page terminal. Streams server output token-by-token. No framework state — the terminal is the UI.
- `player-id-scoping`: Database schema and query layer scoped by `player_id` so concurrent players have isolated inventories, skills, reputation, and progress.

### Modified Capabilities

None — existing single-player CLI game loop is unchanged. The host plays in the terminal as before; `/lan` is just another command.

## Impact

- **No new binary:** server runs inside the existing `gl1tch-mud` process as a goroutine
- **DB schema:** additive `player_id` columns; existing host player data uses `player_id = "default"`
- **Embedded frontend:** `web/dist/` embedded in the binary via `//go:embed`; `make web-build` must run before `make build`
- **Dependencies:** `nhooyr.io/websocket` added to `go.mod`; Node.js + Astro for build only (not a runtime dependency)
- **No changes to:** game logic packages, world loading, pipeline YAMLs, main.go game loop
