## 1. Dependencies & Project Setup

- [x] 1.1 Add `nhooyr.io/websocket` to `go.mod` / `go.sum`
- [x] 1.2 Create `internal/server/` package skeleton (server.go, session.go, auth.go, protocol.go, static.go)
- [x] 1.3 Initialize `web/` as an Astro project (`npm create astro@latest web -- --template minimal --no-install`, then `cd web && npm install`)
- [x] 1.4 Add `@xterm/xterm` and `@xterm/addon-fit` to `web/package.json`
- [x] 1.5 Add `web-build` target to `Makefile`; make `build` depend on `web-build`

## 2. Database: player_id Scoping

- [x] 2.1 Add `db.OpenForPlayer(playerID string)` — per-player DB at `players/<id>/world.db` (same schema, file-level isolation; avoids player_id column migration)
- [x] 2.2 No migration needed — CLI host uses existing `world.db`; web players each get their own DB file
- [x] 2.3 `player.Load` unchanged for CLI; web sessions pass their own DB connection
- [x] 2.4 `player.Save` unchanged for CLI; web sessions pass their own DB connection
- [x] 2.5 `player.Init` unchanged for CLI; web sessions pass their own DB connection
- [x] 2.6 No SQL audit needed — per-player DB means all existing queries are naturally isolated
- [x] 2.7 `main.go` unchanged — CLI continues using `db.Open()` and original player functions
- [x] 2.8 Run `make build && echo "look" | ./gl1tch-mud` to confirm no CLI regression

## 3. WebSocket Protocol Types

- [x] 3.1 Define `ClientMsg` struct in `internal/server/protocol.go`: `Type string`, `Payload json.RawMessage`
- [x] 3.2 Define `ServerMsg` and payload types: `AuthOKPayload`, `AuthFailPayload`, `OutputTokenPayload`, `ErrorPayload`
- [x] 3.3 Write `writeMsg(ctx, conn, msg)` helper in `protocol.go` — marshals and sends a `ServerMsg`

## 4. Auth & Session Management

- [x] 4.1 Implement `internal/server/auth.go`: `ValidatePlayerID(id string) error` (2–20 alphanumeric+hyphen) and `ValidatePassphrase(given, expected string) bool`
- [x] 4.2 Implement `ClientSession` struct in `internal/server/session.go`: `playerID`, `conn`, `db`, `playerState`, `world`, `cancel`, `lastActivity time.Time`
- [x] 4.3 Implement `SessionRegistry` in `internal/server/server.go`: `Register(playerID) (*ClientSession, error)`, `Unregister(playerID)`, `List() []string` with `sync.RWMutex`
- [x] 4.4 Implement idle timeout: reset `lastActivity` on each message; background goroutine closes sessions idle >30 minutes with error message

## 5. GameServer & WebSocket Handler

- [x] 5.1 Implement `GameServer` struct in `internal/server/server.go`: `world *world.World`, `worldMu sync.RWMutex`, `registry *SessionRegistry`, `passphrase string`, `httpServer *http.Server`
- [x] 5.2 Implement `GameServer.Start(port int) (lanURL string, err error)`: detect LAN IP, register routes (`/ws` + static files), launch `http.ListenAndServe` in goroutine, return URL
- [x] 5.3 Implement `GameServer.Stop()`: call `httpServer.Shutdown()`, close all sessions
- [x] 5.4 Implement `/ws` handler: upgrade, auth handshake (first message must be `auth`; validate passphrase + playerID format + no duplicate), call `session.Handle()`
- [x] 5.5 Implement `ClientSession.Handle()`: open per-player DB, load/init player state, send welcome `output.token`, enter read loop dispatching to `routeMessage()`
- [x] 5.6 Implement `routeMessage()`: `input` → dispatch command; `interrupt` → cancel context and send `output.done`; unknown → send error
- [x] 5.7 Implement command dispatch: parse verb+args, look up `commands.Registry`, call handler under world read-lock, stream `Result.Output` as `output.token` chunks, send `output.done`
- [x] 5.8 Implement `session.Close()`: deferred — cancels context, closes DB, calls `registry.Unregister()`

## 6. Static File Embedding

- [x] 6.1 `//go:embed all:web/dist` in `main.go`; `server.SetFS()` wires the sub-FS into the server (embed paths must be relative to main.go at repo root)
- [x] 6.2 In `GameServer.Start()`, serve static files via `FileHandler()` for all non-`/ws` routes

## 7. /lan Command

- [x] 7.1 Create `internal/commands/lan.go`: register `lan` verb via `init()`
- [x] 7.2 Implement `lan` handler: subcommands `""` (start), `stop`, `status`
- [x] 7.3 `start` subcommand: if server already running print status; else call `server.Start()`, print URL + passphrase reminder
- [x] 7.4 `stop` subcommand: call `server.Stop()`, print confirmation or "not running"
- [x] 7.5 `status` subcommand: print URL + `registry.List()` player names
- [x] 7.6 `LANServer` interface in commands breaks import cycle; `SetLANServer()` called from `main.go`

## 8. Astro + xterm.js Frontend

- [x] 8.1 Write `web/src/pages/index.astro`: login card + hidden terminal div
- [x] 8.2 Write `web/src/lib/mud.ts`: xterm.js with Dracula theme, FitAddon, all WS logic
- [x] 8.3 Implement WebSocket connect on form submit with `auth` message
- [x] 8.4 Handle `auth.ok`: hide login, show terminal, write welcome banner
- [x] 8.5 Handle `auth.fail`: show inline error on login form
- [x] 8.6 Implement line-buffered input: accumulate keystrokes, Enter submits, Backspace edits, Ctrl-C sends interrupt
- [x] 8.7 Implement output: `output.token` → `term.write()`; `output.done` → write prompt, re-enable input
- [x] 8.8 Implement FitAddon resize on window resize + send resize message
- [x] 8.9 Style login page: Dracula palette, monospace font, centered card
- [x] 8.10 `npm run build` passes clean

## 9. Integration & Smoke Test

- [x] 9.1 Run `make web-build && make build` — verify binary compiles with embedded frontend
- [x] 9.2 Start `./gl1tch-mud`, type `/lan test123` — verify URL printed with LAN IP
- [ ] 9.3 Open two browser tabs with different names — verify each sees isolated inventory/state
- [ ] 9.4 Test wrong passphrase in browser → auth.fail shown on login
- [ ] 9.5 Test `/lan stop` → browser tabs disconnected gracefully
- [ ] 9.6 Test `/lan status` while players connected → names listed
- [ ] 9.7 Disconnect a browser tab and reconnect with same name → state preserved from DB
- [x] 9.8 Confirm existing CLI still works normally without `/lan` (`echo "look" | ./gl1tch-mud` passes)
