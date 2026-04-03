# Kids Map Design

**Date:** 2026-04-03
**Scope:** Block Haven only (`ui_profile: kids`)
**Goal:** Always-visible map panel showing all rooms, current player location, and other online players' positions in real time — so kids can find each other and parents can find kids.

---

## Architecture

Two server changes, one new UI panel. No new WebSocket message types — all data flows through the existing `world_meta` and `state.update` messages.

### Server: BFS Grid Layout (`internal/world/world.go`)

At world load, run a single BFS from `start_room`. For each cardinal exit, assign a grid offset to the neighbor:

```
north → (0, -1)   south → (0, +1)
east  → (+1, 0)   west  → (-1, 0)
```

Each room gets computed `GridX, GridY int` fields (not in YAML, derived at load). If two rooms would land on the same cell, log a warning and nudge the second room to the nearest free cell. Block Haven's graph is small enough this won't occur in practice.

### Protocol: `world_meta` gains `map_rooms`

Sent once on connect. Client stores as a static lookup table.

```json
{
  "type": "world_meta",
  "map_rooms": [
    { "id": "meadow-0", "name": "Meadow Town Square", "biome": "meadow", "x": 0, "y": 0 },
    { "id": "forest-0",  "name": "Forest Edge",       "biome": "forest",  "x": 1, "y": 0 }
  ]
}
```

### Protocol: `state.update` gains `online_players`

Sent on every `state.update`. Server iterates all active sessions and emits each player's current room. Sessions without a known room (pre-first-look) are omitted. The current player is excluded — their position is already conveyed by the existing `room` field in `state.update` and shown with `★` on the map.

```json
{
  "online_players": [
    { "name": "dad",  "room_id": "meadow-0" },
    { "name": "kid1", "room_id": "forest-0" }
  ]
}
```

Both new fields are `omitempty` — old clients and non-kids worlds ignore them gracefully.

---

## Frontend Map Panel

### Layout

Always-visible panel in kids mode, below the existing room context panel (WHO'S HERE / EXITS / YOUR STUFF) in the right sidebar.

```
┌─────────────────────────────────────────────┐
│  [Block Haven]   Room Name   ♥♥♥  💰 12     │
├──────────────────┬──────────────────────────┤
│                  │  WHO'S HERE              │
│  game output     │  • Elder Mason 💬        │
│  (scrolling)     ├──────────────────────────┤
│                  │  EXITS                   │
│                  │  [↑ N] [→ E] [↓ S]      │
│                  ├──────────────────────────┤
│                  │  MAP          [⊞][⊡][⊠] │
│                  │  ┌─────────────────────┐ │
│                  │  │  · [F] ·            │ │
│                  │  │  · [★] [D] ·        │ │
│                  │  │  · · [Sn] ·         │ │
│                  │  └─────────────────────┘ │
├──────────────────┴──────────────────────────┤
│  [👁 Look] [💬 Talk] [⚒️ Craft]             │
└─────────────────────────────────────────────┘
```

`★` = current player's room. Player initials float as badges over their room cell.

### Rendering

Pure CSS grid — no canvas, no SVG, no library. Each room is a `<div>` positioned via:

```css
grid-column: calc(var(--room-x) + 1);
grid-row:    calc(var(--room-y) + 1);
```

Empty grid cells are invisible gaps. The grid is sized to the bounding box of all room coordinates.

Room cells show:
- Biome color background (meadow=green, forest=dark green, desert=tan, snow=white, caves=grey, ember=red-orange)
- 2–3 letter abbreviation of the room name
- `★` if it is the current player's room
- Initials badge for each other online player present in that room

### Zoom Levels

Three buttons in the map header toggle `data-map-zoom` on the map container:

| Zoom | Attribute | Cell size | Viewport |
|------|-----------|-----------|----------|
| World | `world` | 18px | All rooms |
| Area | `area` | 32px | Current room ± 2 hops, centered |
| Room | `room` | 56px | Current room + immediate neighbors |

All three use the same DOM — zoom only changes CSS cell size and scroll position. `area` and `room` translate the grid to keep the current room centered.

Default zoom: `world`.

---

## Player Presence Updates

When `state.update` arrives:
1. Clear all player badges from map cells
2. Re-render initials badges onto the correct cells from `online_players`
3. Move `★` marker to current player's room (from existing state fields)

### Edge Cases

| Situation | Behavior |
|-----------|----------|
| Player in unknown room ID | Badge omitted, `console.warn` |
| Two players in same room | Both initials shown as stacked badges |
| `map_rooms` not yet received | Map panel shows "loading…", renders once data arrives |
| World has no cardinal exits | Map panel hidden; exits buttons still work |
| Room coordinate collision | Server logs warning, second room nudged to nearest free cell |

---

## Scoping

All map panel HTML, CSS, and JS are gated on `[data-ui="kids"]`. The cyberspace interface is unaffected. New protocol fields are additive and `omitempty`.

---

## Testing

### Backend (Go)

- BFS assigns correct coordinates for a small test graph
- Collision nudges second room to nearest free cell
- World with no cardinal exits produces empty `map_rooms` (no panic)
- `StateUpdatePayload` serializes `online_players` correctly
- Sessions without a current room are excluded from `online_players`
- `world_meta` serializes `map_rooms` correctly

### Frontend (Playwright — `web/e2e/kids-map.spec.ts`)

- Map panel present in DOM in kids mode
- Map panel absent in non-kids mode
- Room cells render with correct biome CSS classes
- Current player's room has `★` marker
- Zoom buttons toggle `data-map-zoom` attribute
- Second mock player in `online_players` renders initials badge on correct room cell

---

## Out of Scope

- Map for non-kids worlds
- Player avatars or icons beyond initials
- Animated movement transitions
- Mobile-specific map layout
- Map persistence (last zoom level) across sessions
