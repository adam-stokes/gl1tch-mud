# Block Haven Kids UI Design

**Date:** 2026-04-03
**Scope:** Block Haven world only (`ui_profile: kids`)
**Audience:** Middle school players (~ages 11–14)

## Problem

The gl1tch-mud web interface was designed for older players comfortable with MUD conventions — typing commands, reading text output, guessing available actions. For Block Haven's younger audience this creates friction: kids don't know what to type, don't know who they can interact with, and are confronted with walls of text when they need clear visual context.

## Goal

Make the Block Haven web interface fully playable by a middle schooler who has never used a text MUD, without dumbing down the game itself. All changes are isolated to Block Haven via a `ui_profile` flag — the cyberspace world and any future adult-oriented worlds are unaffected.

---

## Architecture

### World Config Flag

`worlds/blockhaven/world.yaml` — add to the existing `ui` section:

```yaml
ui:
  profile: kids
  banner: |
    ...
  prompt: "$"
  tagline: "the ruins remember everything."
```

### Protocol Change

`world_meta` WebSocket message gains a `ui_profile` string field. The frontend reads this on connect.

### State Update Expansion

`state.update` must include structured room presence data so the frontend can render context without parsing text output:

```json
{
  "room_npcs": [
    { "id": "elder-mason", "name": "Elder Mason", "can_talk": true, "can_trade": false, "attackable": false },
    { "id": "stone-golem", "name": "Stone Golem", "can_talk": false, "can_trade": false, "attackable": true }
  ],
  "room_items": [
    { "id": "stone-shard", "name": "Stone Shard", "takeable": true }
  ],
  "room_resources": [
    { "id": "limestone-vein", "name": "Limestone Vein", "action": "mine" }
  ]
}
```

The server already holds this data when building the room view — it just needs to be serialized into the structured payload. No new round trips or commands.

### Frontend Detection

```
mud.ts: if (meta.ui_profile === 'kids') → applyKidsMode()
```

`applyKidsMode()` sets `data-ui="kids"` on `<body>`. All kids-mode CSS rules are scoped to `[data-ui="kids"]`. Zero impact on other worlds.

---

## Layout

```
┌─────────────────────────────────────────┐
│  [Block Haven]   Room Name   ♥♥♥  💰 12 │  top bar (unchanged)
├──────────────────┬──────────────────────┤
│                  │  WHO'S HERE          │
│  game output     │  • Elder Mason 💬🛒  │  room context panel
│  (scrolling      │  • Stone Golem ⚔️    │
│   text)          │                      │
│                  │  EXITS               │
│                  │  [↑ N] [→ E] [↓ S]  │  clickable exit buttons
│                  ├──────────────────────┤
│                  │  YOUR STUFF          │
│                  │  [🪨 Stone] [🗡️ Axe]│  visual inventory chips
├──────────────────┴──────────────────────┤
│  [👁 Look] [💬 Talk] [⚔️ Attack]        │  context-sensitive buttons
│  [🌿 Forage] [🔍 Search] [⚒️ Craft]    │  (hidden when N/A)
├─────────────────────────────────────────┤
│  [⌨️]  ___________________________  [↵] │  text input (collapsed)
└─────────────────────────────────────────┘
```

---

## Components

### Room Context Panel (replaces sidebar top)

Displays who and what is currently in the room using structured data from `state.update`:

- **NPCs** listed by name with icon badges: 💬 (can talk), 🛒 (can trade), ⚔️ (attackable)
- **Resources** listed with their action type (mine, forage, harvest)
- Items on the ground listed as takeable

### Exit Buttons

Clickable directional buttons rendered from the exits map in `state.update`. Only directions that exist appear. Clicking sends `go north` etc. directly — no typing required.

```
[↑ North]  [→ East]  [↓ South]
```

### Visual Inventory

Items in the player's inventory rendered as chip components (icon + name) instead of a text list. Tapping a chip opens the existing item detail modal.

### Context-Sensitive Action Buttons

Buttons are hidden when they have no valid targets in the current room.

| Button | Visible when |
|--------|-------------|
| Talk | ≥1 NPC with `can_talk: true` in room |
| Attack | ≥1 NPC with `attackable: true` in room |
| Trade | ≥1 NPC with `can_trade: true` in room |
| Forage | ≥1 resource in `room_resources` |
| Search | Always |
| Look | Always |
| Craft | Always |
| Skills / Quests | Always |

### Inline Target Picker

When a context-sensitive button is clicked:

- **0 targets** — button hidden (never reached)
- **1 target** — action fires immediately, no picker
- **2+ targets** — pill row appears above action buttons

```
╭─────────────────────────────╮
│  Who do you want to talk to? │
│  [Elder Mason]  [Town Guard] │
╰─────────────────────────────╯
```

Picker dismisses on tap-outside or re-clicking the button. Sends the standard command to the server (e.g. `talk elder-mason`) — no backend command changes needed.

### Text Input (Collapsed)

The text input bar is collapsed by default. A keyboard icon `⌨️` in the left corner of the input bar expands it. State persists in `localStorage` so kids who prefer typing aren't re-collapsed each session.

### Quest Tracker Modal

The Quests button opens the existing modal but renders quests as cards with progress bars:

```
╭──────────────────────────────────╮
│ 🧱 Rebuild the Watchtower         │
│ ████████░░░░  3 of 5 stones       │
│                                   │
│ 🗺️ Find the Lost Map              │
│ ░░░░░░░░░░░░  Not started         │
╰──────────────────────────────────╯
```

Progress is derived from existing server-side quest state. Quests that track a numeric objective (e.g. collect 5 stones) render a fill bar; quests with only complete/incomplete state render as "In progress" or "Not started" with an empty/full bar.

---

## Onboarding Hints

First-time contextual tooltips tracked in `localStorage` under `bh_hints_seen`. Each hint fires once and never repeats. Hints appear as a dismissible banner above the action buttons, auto-dismiss after 5 seconds.

| Key | Trigger | Message |
|-----|---------|---------|
| `first_login` | First Block Haven session | "Click the buttons below to do things. They change based on what's around you!" |
| `first_npc` | First time NPC appears in room | "Someone's here! Click Talk to chat with them." |
| `first_item` | First item picked up | "Check Your Stuff to see what you're carrying." |
| `first_quest` | First quest accepted | "A new quest! Tap Quests to track your progress." |
| `first_type` | Text input first expanded | "You can also type commands if you want." |

---

## Data Flow Summary

```
Server                          Frontend (kids mode)
──────                          ────────────────────
world_meta {ui_profile:"kids"}  → applyKidsMode() → sets data-ui="kids"

state.update {                  → rebuildRoomContext()  (WHO'S HERE panel)
  room_npcs,                    → rebuildExitButtons()  (EXITS)
  room_items,                   → rebuildInventoryChips() (YOUR STUFF)
  room_resources,               → rebuildActionButtons()  (hide/show by context)
  exits,
  inventory,
  quests
}

user clicks Talk (2 NPCs)       → showTargetPicker(["Elder Mason","Town Guard"])
user clicks "Elder Mason"       → sendCommand("talk elder-mason")
server streams output           → output pane (unchanged)
```

---

## Out of Scope

- Changes to cyberspace or any other world's UI
- Backend command changes (same commands, same syntax)
- Mobile-specific layout (responsive is nice-to-have, not required)
- Sound effects or animations
