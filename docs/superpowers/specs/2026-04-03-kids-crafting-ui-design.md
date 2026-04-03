# Kids Crafting UI Design

**Date:** 2026-04-03  
**Scope:** Blockhaven world only  
**Mode:** Kids UI (`data-ui=kids`)

---

## Overview

When a player in kids mode taps the 🔧 Craft button, they get a purpose-built crafting modal with:
- A Minecraft-style paint grid (click/drag to fill cells with selected items)
- A toggleable recipe help drawer showing all available patterns
- Workbench guidance: grayed-out recipes with friendly directional messages
- Full Playwright test coverage

The existing `#craft-modal` is left untouched. The new `#kids-craft-modal` is activated only when `data-ui=kids`.

---

## Architecture

### New modal: `#kids-craft-modal`

Added to `game.astro` alongside the existing `#craft-modal`. The kids Craft action button opens this modal instead of the original when `data-ui=kids`.

**Three panels:**

1. **Inventory picker** — item chips showing what the player currently holds; tapping arms the item (highlighted ring); tapping an armed item again switches to eraser mode
2. **3×3 paint grid** — mousedown fills the cell with the armed item and starts a drag; dragging fills subsequent cells; touch events (`touchstart`/`touchmove`/`touchend`) mirror mouse events for tablet support
3. **Recipe drawer** — triggered by `?` button; slides up as an overlay within the modal; tap `?` again (or press the close button) to dismiss

### State

A new `_kidscraft` object in `mud.ts` tracks:
- `armedItem: InvItem | null` — currently selected paint item
- `eraser: boolean` — whether eraser mode is active
- `slots: (string | null)[]` — 9-cell grid of item IDs (null = empty)

Existing shared helpers reused:
- `matchRecipe()` — runs on every cell change to detect a matching recipe
- `sendCommand()` — unchanged; sends `craft <recipe-id>` on success

---

## Recipe Drawer

Triggered by a `?` button in the top-right of the modal. Slides up as an overlay over the grid (does not navigate away).

Each recipe is shown as a card:

```
┌─────────────────────────────┐
│ 🪵🪵🪵                       │
│ 🪵_🪵  →  ⛏️ Wooden Pickaxe  │
│ _🪴_                        │
└─────────────────────────────┘
```

- 3×3 ingredient grid (emoji icons for items, blank cells for empty slots)
- Arrow → output item name + emoji
- `🔨 Needs Workbench` badge on recipes with a `workbench` field

Tapping a recipe card:
- Closes the drawer
- Auto-populates the grid with the recipe's ingredient pattern
- `matchRecipe()` fires immediately

Workbench recipes are visible and tappable but rendered grayed-out with the badge. They can be auto-populated, but tapping Craft shows a friendly message instead of sending the command (see Workbench Guidance below).

---

## Paint Mechanic

1. **Arm** — tap an inventory chip; it gets a highlight ring; player is now "holding" that item
2. **Paint** — mousedown on a grid cell fills it with the armed item; drag continues filling cells
3. **Replace** — painting over a cell already filled with a different item replaces it
4. **Eraser** — tap the armed item chip again to toggle eraser mode; drag clears cells; right-click a single cell also clears it
5. **Auto-populate** — tapping a recipe card fills all 9 cells to match the pattern

`matchRecipe()` runs on every cell change.

**Craft button states:**
- Match found: `🔧 Craft: Wooden Pickaxe` (green, enabled)
- Items placed, no match: `🔧 No matching recipe` (dimmed, disabled)
- Grid empty: `🔧 Open Recipe Guide` (enabled — opens the drawer)

---

## Workbench Guidance

Workbench recipes appear grayed-out with a `🔨 Needs Workbench` badge in the recipe drawer.

When a player completes a workbench recipe pattern and taps Craft:
- The `craft` command is **not** sent
- A friendly overlay message is shown: **"You need a Workbench! Head to [room name] 🔨"**
- Room name is derived from the recipe's `workbench` field (a room ID); the client looks up the display name from world data sent by the server at login, falling back to the raw room ID if not found

The server's `craft.failed` event with reason `"workbench_required"` is also intercepted client-side and shows this same message (handles cases where the player somehow bypasses the client-side check).

---

## UI Tests (Playwright)

Three new spec files in `web/tests/`:

### `kids-craft-modal.spec.ts` — modal behavior
- Kids mode activates `#kids-craft-modal`, not `#craft-modal`, when Craft is tapped
- Recipe drawer toggles open/closed with `?` button
- Recipe cards display correct ingredient patterns and output names
- Workbench recipes show `🔨 Needs Workbench` badge and are grayed out

### `kids-craft-painting.spec.ts` — paint mechanic
- Tapping an inventory chip arms it (shows highlight ring)
- Mousedown + drag fills cells with armed item
- Dragging over a filled cell replaces it
- Right-click clears a cell
- Tapping armed chip again enables eraser mode; dragging clears cells
- Auto-populate from recipe card fills grid with correct pattern
- `matchRecipe()` enables Craft button when pattern matches a recipe

### `kids-craft-flow.spec.ts` — end-to-end flows
- **Happy path:** arm item → paint recipe → tap Craft → modal closes → `craft <id>` sent → inventory updated
- **Missing ingredients:** partial pattern → Craft button stays disabled
- **Workbench required:** complete workbench recipe pattern → tap Craft → friendly message shown, no command sent
- **Eraser mode:** arm item → tap again → drag clears cells
- **Kids mode activation:** `data-ui=kids` is set, kids craft modal is present in DOM

---

## File Changes

| File | Change |
|------|--------|
| `web/src/pages/game.astro` | Add `#kids-craft-modal` HTML; CSS for drawer, paint grid, inventory chips, workbench badge |
| `web/src/lib/mud.ts` | Add `_kidscraft` state; paint event handlers; recipe drawer toggle; workbench intercept |
| `web/tests/kids-craft-modal.spec.ts` | New test file |
| `web/tests/kids-craft-painting.spec.ts` | New test file |
| `web/tests/kids-craft-flow.spec.ts` | New test file |

No changes to `internal/crafting/`, `internal/commands/`, or world YAML files.
