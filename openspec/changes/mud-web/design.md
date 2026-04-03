## Context

gl1tch-mud is a complete single-player CLI MUD. Its command handlers are pure functions — `HandlerFunc(db *sql.DB, p player.State, w *world.World, args []string) Result` — with no I/O coupling. The game loop in `main.go` is stdin → parse → dispatch → print. This decoupling makes it straightforward to start an embedded server from within the game.

The host player types `/lan` while playing in their terminal. The server boots as a goroutine in the same process, sharing the already-loaded world. Kids visit the printed URL in a browser and join.

## Goals / Non-Goals

**Goals:**
- `/lan` command starts the embedded HTTP/WebSocket server and prints the LAN URL
- Browser players connect with xterm.js terminal and share the same world as the host
- Each player (host + kids) has isolated game state (inventory, skills, HP, reputation)
- Existing CLI game loop is completely unchanged — host plays exactly as before

**Non-Goals:**
- Players seeing or interacting with each other in v1 (no "X enters the room" broadcasts)
- TLS / HTTPS (LAN only)
- User registration or persistent accounts (player name chosen at login)
- Separate binary or daemon

## Decisions

### D1: Server runs as goroutine inside `gl1tch-mud` process

`/lan` calls `server.Start()` which launches the HTTP listener in a goroutine and returns immediately, printing the URL. The CLI game loop continues unblocked. `/lan stop` calls `server.Stop()`.

This means the server shares the already-initialized world instance — no reload needed.

Alternatives considered: separate `cmd/mudserver` binary. Rejected because it requires a separate install step, can't share the live world instance, and breaks the "just type `/lan`" UX.

### D2: Host player is `player_id = "default"`, browser players use their chosen name

The existing CLI path is untouched. All player-state queries already use a fixed player row; after the `player_id` migration they implicitly use `"default"`. Browser players supply their name at login; it becomes their `player_id`.

### D3: `nhooyr.io/websocket` over `gorilla/websocket`

Context-aware, actively maintained, safe for concurrent writes without external locking. gorilla/websocket requires a mutex for concurrent writes from multiple goroutines.

### D4: Single shared `*world.World` with `sync.RWMutex`

World YAML is loaded once at startup. All sessions (CLI + WebSocket) read from the same instance. `explore`-generated rooms use `world.AddRoom()`, the only write path, protected by a write lock. Generated rooms are visible to all players.

### D5: Per-player SQLite connection

Each `ClientSession` opens its own `*sql.DB`. SQLite WAL mode handles concurrent access. Each player's writes are naturally serialized by their connection. The CLI host already holds its own `*sql.DB` opened in `main.go`.

### D6: LAN URL detection

`/lan` detects the machine's non-loopback IPv4 address (first from `net.InterfaceAddrs()`) and prints `http://<LAN-IP>:<port>`. Falls back to `localhost` if detection fails.

### D7: Passphrase via `/lan` argument or env var

`/lan secret123` sets the session passphrase. If no argument, falls back to `MUDSERVER_PASSPHRASE` env var. If neither is set, the server starts with no auth (local LAN, family use — acceptable). Passphrase is stored in the `GameServer` struct for the session lifetime.

### D8: Astro build embedded with `//go:embed`

`go:embed web/dist` in a new file `internal/server/static.go` embeds the Astro build. `make web-build` runs `npm run build` in `web/`. The embedded FS is served for all non-`/ws` routes.

## Risks / Trade-offs

- **SQLite concurrent writes** → WAL mode + per-player connections mitigates contention. Acceptable at family-LAN scale.
- **World mutation during `explore`** → `sync.RWMutex` protects concurrent reads. Write lock held briefly during `AddRoom()`.
- **No reconnection resumption** → State committed to DB is preserved on disconnect; in-flight command is lost. Player reconnects with same name and continues.
- **`make web-build` prerequisite** → Astro dist must exist before `make build` or the `//go:embed` directive will fail. Makefile `build` target will depend on `web-build`.

## Migration Plan

1. `make web-build` — build Astro frontend into `web/dist/`
2. `make build` — compiles `gl1tch-mud` with embedded frontend
3. Run `gl1tch-mud` as normal; type `/lan [passphrase]` to start server
4. Share printed URL with kids

Rollback: `/lan stop`. CLI game unaffected throughout.

## Open Questions

- Should players in the same room see each other listed in room descriptions? (Deferred to v2.)
- Should `/lan` show a QR code in the terminal? (Nice-to-have; out of scope for v1.)
