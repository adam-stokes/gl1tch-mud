# World Selection Design

**Date:** 2026-04-03  
**Status:** Approved  

## Overview

Add a world selection lobby to both the text and web interfaces, replacing the hardcoded `"cyberspace"` world in `main.go`. Each world declares its own UI configuration (banner, prompt, tagline, theme colors) in `world.yaml`. A `--world <name>` flag lets users skip selection and go directly to a specific world.

---

## 1. World YAML Schema Extension

Each `world.yaml` gains a top-level `ui:` block:

```yaml
ui:
  banner: |
    ██████╗ ██╗      ██████╗  ██████╗██╗  ██╗
    ...
  prompt: ">"
  tagline: "jack in. ghost the gibson."
  theme:
    bg: "#282a36"
    fg: "#f8f8f2"
    accent: "#bd93f9"
    dim: "#6272a4"
    border: "#44475a"
    error: "#ff5555"
    success: "#50fa7b"
```

**Go changes (`internal/world/world.go`):**

- Add `WorldUI` struct with `Banner`, `Prompt`, `Tagline` string fields and a `Theme` sub-struct matching the existing `theme.Palette` shape.
- Add `UI WorldUI` field to the `World` struct.
- Add `WorldMeta` struct: `{Name, Tagline string, Theme WorldTheme}` — lightweight, no full world load required.
- Add `ListAvailable() []WorldMeta`: scans `~/.config/glitch/worlds/` for user-installed worlds, merges in embedded defaults (`cyberspace`, `blockhaven`). Deduplicates by name (user install wins).
- When loading a world, if `ui.theme` is set, it takes precedence over the active system theme from `internal/theme`.

**Both existing world YAMLs (`worlds/cyberspace/world.yaml`, `worlds/blockhaven/world.yaml`) get `ui:` blocks added.**

**Tests:**
- `WorldUI` YAML parsing (all fields present, partial fields, missing block falls back to defaults)
- `ListAvailable()` returns embedded worlds when no user dir exists
- `ListAvailable()` merges user worlds, user install wins on name collision
- `World.UI.Prompt` falls back to `">"` when unset
- `World.UI.Banner` falls back to empty string when unset

---

## 2. Text Interface

**`main.go` changes:**

- Add `--world` flag (`flag.String("world", "", "world to load")`).
- If `--world` is provided: skip selection, load that world directly.
- If `--world` is absent: call `selectWorld()` before `runGame()`.

**`selectWorld() string`** (new function in `main.go`):

```
  available worlds:

  [1] cyberspace  — jack in. ghost the gibson.
  [2] blockhaven  — the ruins remember everything.

  > 
```

- Uses `world.ListAvailable()` to populate the list.
- Accepts a number (`1`, `2`, ...) or world name as input.
- Invalid input re-prompts.
- Returns the selected world name.

**Game loop changes:**

- Banner printed from `w.UI.Banner` instead of the hardcoded ASCII art block.
- Tagline printed from `w.UI.Tagline`.
- Prompt uses `w.UI.Prompt + " "` instead of `"> "`.
- Mid-game `SwitchWorld` already updates `w`; the new prompt takes effect on next iteration automatically.

**`--serve` mode:** `--world` locks the server to that world. Without `--world`, serve mode loads all worlds (for multi-world web lobby support).

**Tests:**
- `selectWorld()` accepts valid number input
- `selectWorld()` accepts valid name input
- `selectWorld()` re-prompts on invalid input
- `--world` flag bypasses selector
- Prompt string uses `w.UI.Prompt`
- Banner output uses `w.UI.Banner`

---

## 3. Server & API

**`server.New()` signature change:**

```go
// Before
func New(w *world.World) *GameServer

// After
func New(worlds map[string]*world.World) *GameServer
```

The `GameServer` stores `map[string]*world.World`. Each `ClientSession` holds a `*world.World` reference for its chosen world.

**New endpoints:**

- **`GET /api/worlds`** — returns `[]WorldMeta` as JSON (name, tagline, theme). No auth. Used by the web lobby.
- **`GET /ws?world=<name>`** — WebSocket scoped to the named world. If `world` param is absent and the server was started with `--world` (single-world map), uses that world (preserves existing `--serve` behavior). If `world` param is absent on a multi-world server → HTTP 400 (client must select explicitly). Unknown world name → HTTP 400.

**WebSocket protocol addition:**

On connect, the server sends a `world_meta` message before the first `state` message:

```json
{
  "type": "world_meta",
  "payload": {
    "name": "cyberspace",
    "tagline": "jack in. ghost the gibson.",
    "theme": { "bg": "#282a36", "accent": "#bd93f9", ... }
  }
}
```

**`--world` in serve mode:** server is initialized with a single-entry map `{name: world}`. The lobby is bypassed; the web client connects directly to the game page for that world.

**`main.go` startup (serve mode):**

```
if --world set:
  load single world → map{name: world} → server.New()
else:
  ListAvailable() → load all → map → server.New()
```

**Tests:**
- `GET /api/worlds` returns correct WorldMeta for each loaded world
- `GET /ws?world=cyberspace` opens session with correct world
- `GET /ws?world=unknown` returns 400
- `GET /ws` (no param) defaults to the locked world in single-world map, returns 400 on multi-world map
- `world_meta` message sent on connect with correct fields
- Players in different worlds are isolated (no cross-world broadcast)
- `server.New()` with single-entry map suppresses lobby behavior

---

## 4. Web Interface

**Directory structure:**

```
web/src/
  components/
    Terminal.astro       # scrolling output pane
    Inventory.astro      # inventory panel
    HUD.astro            # hp/credits/room bar
    WorldCard.astro      # lobby world selection card
  layouts/
    GameLayout.astro     # base game chrome; injects theme CSS vars from world_meta
  pages/
    index.astro          # lobby: fetches /api/worlds, renders WorldCard list
    cyberspace.astro     # cyberspace game page; composes shared components
    blockhaven.astro     # blockhaven game page; composes shared components
  lib/
    mud.ts               # updated: connect(world: string) passes ?world= param
    theme.ts             # new: applyTheme(meta: WorldMeta) sets CSS custom properties
```

**Lobby (`index.astro`):**

- On load, fetches `/api/worlds`.
- Renders one `WorldCard` per world: name, tagline, accent color.
- Clicking a card navigates to `/<worldname>`.
- If server was started with `--world`, redirects directly to `/<worldname>`.

**Game pages (`cyberspace.astro`, `blockhaven.astro`):**

- Call `mud.connect(worldName)` on mount.
- On `world_meta` message: call `theme.applyTheme(meta)` to set CSS vars (`--bg`, `--accent`, etc.).
- Compose `Terminal`, `Inventory`, `HUD` components — world pages can add/remove/reorder panels.
- All game logic stays in shared components; pages only control layout.

**`mud.ts` change:**

```typescript
// Before
function connect() { new WebSocket('/ws') }

// After
function connect(world: string) { new WebSocket(`/ws?world=${world}`) }
```

**`theme.ts` (new):**

```typescript
function applyTheme(meta: WorldMeta): void {
  const root = document.documentElement;
  Object.entries(meta.theme).forEach(([k, v]) => {
    root.style.setProperty(`--${k}`, v);
  });
}
```

**Tests (TypeScript):**
- `mud.connect(world)` constructs correct WebSocket URL
- `theme.applyTheme()` sets expected CSS custom properties on document root
- `WorldCard` renders name, tagline, and applies accent color
- Lobby fetches `/api/worlds` and renders correct number of cards
- Unknown world name in URL shows error state

---

## Data Flow Summary

```
gl1tch-mud (no --world)
  │
  ├── text: selectWorld() → numbered menu → world name
  │         runGame(worldName)
  │
  └── web:  server loads all worlds
            GET /api/worlds → lobby renders WorldCards
            click card → /<worldname>
            WebSocket /ws?world=<name>
            server sends world_meta → theme.applyTheme()
            game runs in world-scoped session

gl1tch-mud --world blockhaven
  │
  ├── text: skip selector → runGame("blockhaven")
  └── web:  single-world server → lobby skipped → direct to /blockhaven
```

---

## Files Changed

| File | Change |
|------|--------|
| `main.go` | `--world` flag, `selectWorld()`, parameterize world name, banner/prompt from world |
| `internal/world/world.go` | `WorldUI`, `WorldMeta`, `ListAvailable()`, `UI` field on `World` |
| `internal/server/server.go` | `New(worlds map)`, `/api/worlds`, `/ws?world=` routing, `world_meta` message |
| `worlds/cyberspace/world.yaml` | Add `ui:` block |
| `worlds/blockhaven/world.yaml` | Add `ui:` block |
| `web/src/pages/index.astro` | Lobby page |
| `web/src/pages/cyberspace.astro` | New: cyberspace game page |
| `web/src/pages/blockhaven.astro` | New: blockhaven game page |
| `web/src/layouts/GameLayout.astro` | New: shared game chrome |
| `web/src/components/Terminal.astro` | Extracted from index.astro |
| `web/src/components/Inventory.astro` | Extracted from index.astro |
| `web/src/components/HUD.astro` | Extracted from index.astro |
| `web/src/components/WorldCard.astro` | New: lobby card |
| `web/src/lib/mud.ts` | `connect(world)` param |
| `web/src/lib/theme.ts` | New: `applyTheme()` |
