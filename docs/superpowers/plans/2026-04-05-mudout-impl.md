# Mudout World — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Mudout world as an embedded default — 12 rooms, 4 factions, loot tables, crafting recipes, and a `wasteland` UI profile wired into the web client.

**Architecture:** New world YAML at `internal/world/defaults/mudout/world.yaml`, embedded via `world.go`'s embed directive. The web client gets a new `_wastelandMode` flag alongside `_kidsMode`, driven by `ui_profile === 'wasteland'` in the `world_meta` message. A new `renderPipBoyBar` replaces `renderHearts` when in wasteland mode (same innerHTML pattern as existing renderHearts — values are numeric, not user HTML). A `mudout` entry in `WORLD_ACTIONS` provides world-specific action buttons.

**Tech Stack:** Go 1.21, `gopkg.in/yaml.v3`, TypeScript/Vitest, Astro

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Modify | `internal/world/world.go:15` | Add `defaults/mudout/world.yaml` to embed directive |
| Create | `internal/world/defaults/mudout/world.yaml` | Full world definition: rooms, factions, loot, crafting |
| Modify | `internal/world/world_test.go` | `TestMudoutWorldLoads` integration test |
| Modify | `web/src/lib/mud.ts` | `_wastelandMode`, `applyWastelandMode`, `renderPipBoyBar`, mudout actions, export `getWorldActions` |
| Modify | `web/src/lib/mud.test.ts` | Tests for `getWorldActions` |

---

## Task 1: Write the failing Go test for mudout

**Files:**
- Modify: `internal/world/world_test.go`

- [ ] **Step 1: Add `TestMudoutWorldLoads` to world_test.go**

Append this function at the end of `internal/world/world_test.go`:

```go
func TestMudoutWorldLoads(t *testing.T) {
	w, err := Load("mudout")
	if err != nil {
		t.Fatalf("Load(mudout): %v", err)
	}
	if w.Name != "mudout" {
		t.Errorf("name: got %q want %q", w.Name, "mudout")
	}
	if w.StartRoom != "dusthaven-0" {
		t.Errorf("start_room: got %q want %q", w.StartRoom, "dusthaven-0")
	}
	if len(w.Rooms) != 12 {
		t.Errorf("rooms: got %d want 12", len(w.Rooms))
	}
	if len(w.Factions) != 4 {
		t.Errorf("factions: got %d want 4", len(w.Factions))
	}
	if w.UI.Profile != "wasteland" {
		t.Errorf("ui.profile: got %q want %q", w.UI.Profile, "wasteland")
	}
	if w.UI.Theme.BG != "#0d0d00" {
		t.Errorf("theme.bg: got %q want %q", w.UI.Theme.BG, "#0d0d00")
	}
	if r := w.Room("dusthaven-0"); r == nil {
		t.Error("start room dusthaven-0 not found in index")
	}
	if len(w.CraftingRecipes) < 2 {
		t.Errorf("crafting_recipes: got %d want >=2", len(w.CraftingRecipes))
	}
	if len(w.LootTables) < 3 {
		t.Errorf("loot_tables: got %d want >=3", len(w.LootTables))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```
go test ./internal/world/... -run TestMudoutWorldLoads -v
```

Expected: FAIL — "Load(mudout): world: load default: open defaults/mudout/world.yaml: file does not exist"

---

## Task 2: Create mudout/world.yaml and update embed

**Files:**
- Create: `internal/world/defaults/mudout/world.yaml`
- Modify: `internal/world/world.go:15`

- [ ] **Step 1: Update the embed directive in world.go**

In `internal/world/world.go`, change line 15 from:

```go
//go:embed defaults/cyberspace/world.yaml defaults/blockhaven/world.yaml
```

to:

```go
//go:embed defaults/cyberspace/world.yaml defaults/blockhaven/world.yaml defaults/mudout/world.yaml
```

- [ ] **Step 2: Create the world YAML**

Create `internal/world/defaults/mudout/world.yaml`. Full content follows — paste exactly:

The YAML defines:
- `name: mudout`, `start_room: dusthaven-0`, `narrator_model: claude-haiku-4-5-20251001`
- `ui:` block with banner, `prompt: ">"`, `profile: wasteland`, tagline, theme (bg `#0d0d00`, fg `#d4a017`, accent `#ff6600`, dim `#5a4a00`, border `#3a3000`, error `#cc2200`, success `#7ab648`)
- `weather_table:` three biomes: settlement (clear/overcast/ashfall), wasteland (clear/ashstorm/radiation-fog/scorching), ruins (clear/dead-calm/radiation-fog/tremor)
- 4 `factions:` settlers (territory dusthaven-0..3, allies ironclad, enemies ash-raiders), ash-raiders (territory barrens-0..1, enemies settlers+ironclad), ironclad (territory barrens-2+ruins-0..2, allies settlers, enemies ash-raiders+ghoul-collective), ghoul-collective (territory ruins-1..2, enemies ironclad)
- 4 `loot_tables:` settlement-loot, wasteland-loot, ruins-loot, raider-loot
- 2 `crafting_recipes:` pipe-pistol (assembly, workbench: weapons, 7 slots: frame/barrel/receiver/stock/grip/scope/muzzle), leather-armor (assembly, workbench: armor, 4 slots: base/lining/plating/pockets)
- 12 `rooms:` dusthaven-0 through dusthaven-4, barrens-0 through barrens-3, ruins-0 through ruins-3

See the design spec at `docs/superpowers/specs/2026-04-04-mudout-design.md` for the authoritative room descriptions, NPC dialogue, and item content. Reproduce them faithfully.

Key room wiring (exits):
```
dusthaven-0: north->dusthaven-1, east->dusthaven-3, south->barrens-0
dusthaven-1: south->dusthaven-0, east->dusthaven-2
dusthaven-2: west->dusthaven-1, north->dusthaven-4
dusthaven-3: west->dusthaven-0
dusthaven-4: south->dusthaven-2
barrens-0:   north->dusthaven-0, east->barrens-1, west->barrens-2
barrens-1:   west->barrens-0, east->ruins-0, south->barrens-3
barrens-2:   east->barrens-0
barrens-3:   north->barrens-1
ruins-0:     west->barrens-1, north->ruins-1, east->ruins-3
ruins-1:     south->ruins-0, north->ruins-2  (lock: vault-lock, key: vault-key-fragment)
ruins-2:     south->ruins-1
ruins-3:     west->ruins-0
```

Key NPCs per room:
- dusthaven-0: Marta (Settlers recruiter), hp 50, attack 8
- dusthaven-1: Dex the Scrapper, hp 40, attack 5 (trades scrap-metal x3 for pipe-part x2)
- dusthaven-2: Roz the Gunsmith, hp 40, attack 5
- dusthaven-3: Hank the Bartender, hp 60, attack 10 (trades caps x10 for canned-food x2)
- barrens-0: Raider Scout, hp 35, attack 9, loot_table_id: raider-loot
- barrens-1: Ash Raider, hp 55, attack 12, loot_table_id: wasteland-loot
- barrens-2: Ironclad Sentry, hp 65, attack 14, loot_table_id: wasteland-loot
- ruins-0: Feral Ghoul hp 45 attack 11 (ruins-loot), Ironclad Patrol hp 70 attack 15 (ruins-loot)
- ruins-1: Ironclad Commander hp 90 attack 18, Elder Maren hp 60 attack 8
- ruins-2: Vault Dweller hp 40 attack 6

Resources:
- dusthaven-1: scrapyard-pile (mine, tool: pickaxe, respawn: 6)
- barrens-0: ashfield-scavenge (harvest, respawn: 4)
- barrens-1: overpass-salvage (harvest, respawn: 5)
- barrens-2: crater-ore (mine, tool: pickaxe, respawn: 8) — yields scrap-iron, copper-wire, radiation-crystal(0.08)
- ruins-0: mall-salvage (harvest, respawn: 8) — yields polymer-sheet, pre-war-circuitry

Workbench types:
- dusthaven-1: [weapons]
- dusthaven-2: [armor, structures]

- [ ] **Step 3: Run the failing Go test — verify it now passes**

```
go test ./internal/world/... -run TestMudoutWorldLoads -v
```

Expected:
```
=== RUN   TestMudoutWorldLoads
--- PASS: TestMudoutWorldLoads (0.00s)
PASS
```

- [ ] **Step 4: Run the full world test suite — verify no regressions**

```
go test ./internal/world/... -v 2>&1 | tail -20
```

Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/world/world.go internal/world/defaults/mudout/world.yaml internal/world/world_test.go
git commit -m "feat(mudout): add mudout world with 12 rooms, 4 factions, wasteland UI profile"
```

---

## Task 3: Write failing TypeScript tests for wasteland mode

**Files:**
- Modify: `web/src/lib/mud.ts` (export `getWorldActions` only)
- Modify: `web/src/lib/mud.test.ts`

- [ ] **Step 1: Export `getWorldActions` from mud.ts**

In `web/src/lib/mud.ts`, find:

```typescript
const DEFAULT_ACTIONS: ActionDef[] = WORLD_ACTIONS['cyberspace'];
```

Add this export immediately after it:

```typescript
/** Returns the action button definitions for a given world name. Exported for testing. */
export function getWorldActions(worldName: string): ActionDef[] {
  return WORLD_ACTIONS[worldName] ?? DEFAULT_ACTIONS;
}
```

- [ ] **Step 2: Add wasteland tests to mud.test.ts**

Replace the full contents of `web/src/lib/mud.test.ts` with:

```typescript
import { describe, it, expect } from 'vitest';
import { buildWsUrl, getWorldActions } from './mud';

describe('buildWsUrl', () => {
  it('constructs ws:// URL with world param', () => {
    const url = buildWsUrl('ws:', 'localhost:8080', 'cyberspace');
    expect(url).toBe('ws://localhost:8080/ws?world=cyberspace');
  });

  it('constructs wss:// URL for https', () => {
    const url = buildWsUrl('wss:', 'example.com', 'blockhaven');
    expect(url).toBe('wss://example.com/ws?world=blockhaven');
  });

  it('percent-encodes world names with spaces', () => {
    const url = buildWsUrl('ws:', 'localhost', 'my world');
    expect(url).toBe('ws://localhost/ws?world=my%20world');
  });
});

describe('getWorldActions', () => {
  it('returns mudout-specific actions including Scavenge and Mine', () => {
    const actions = getWorldActions('mudout');
    const labels = actions.map(a => a.label);
    expect(labels).toContain('Look');
    expect(labels).toContain('Attack');
    expect(labels).toContain('Scavenge');
    expect(labels).toContain('Mine');
    expect(labels).toContain('Craft');
  });

  it('falls back to cyberspace actions for unknown world', () => {
    const actions = getWorldActions('unknown-world');
    const labels = actions.map(a => a.label);
    expect(labels).toContain('Look');
    expect(labels).toContain('Hack');
  });

  it('returns blockhaven actions for blockhaven world', () => {
    const actions = getWorldActions('blockhaven');
    const labels = actions.map(a => a.label);
    expect(labels).toContain('Forage');
  });
});
```

- [ ] **Step 3: Run tests — verify the new tests fail**

```
cd web && npx vitest run src/lib/mud.test.ts 2>&1 | tail -20
```

Expected: `getWorldActions` tests fail — 'Scavenge' and 'Mine' not found in actions (mudout not yet in WORLD_ACTIONS).

---

## Task 4: Implement wasteland mode in mud.ts

**Files:**
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Add `mudout` to `WORLD_ACTIONS`**

In `web/src/lib/mud.ts`, find the closing `};` of `WORLD_ACTIONS` (after the blockhaven entry, around line 216). Insert the `mudout` entry before the closing brace:

```typescript
  mudout: [
    { icon: '👁',  label: 'Look',     cmd: 'look' },
    { icon: '⚔️', label: 'Attack',   cmd: 'attack' },
    { icon: '🔍',  label: 'Scavenge', cmd: 'search' },
    { icon: '⛏️', label: 'Mine',     cmd: 'mine' },
    { icon: '🗺',  label: 'Explore',  cmd: 'explore' },
    { icon: '⚡',  label: 'Skills',   cmd: 'skills' },
    { icon: '📋',  label: 'Quests',   cmd: 'quests' },
    { icon: '🔧',  label: 'Craft',    special: 'craft', cls: 'craft-btn' },
  ],
```

- [ ] **Step 2: Add `_wastelandMode` flag**

In `web/src/lib/mud.ts`, find:

```typescript
let _kidsMode = false;
```

Add `_wastelandMode` on the next line:

```typescript
let _kidsMode = false;
let _wastelandMode = false;
```

- [ ] **Step 3: Add `renderPipBoyBar` function**

In `web/src/lib/mud.ts`, find the `// ── HP hearts` comment (around line 221). Add `renderPipBoyBar` immediately after the closing brace of `renderHearts`:

```typescript
// ── Pip-Boy HP bar (wasteland mode) ──────────────────────────────────────────

function renderPipBoyBar(hp: number, maxHP: number): string {
  const total  = 10;
  const pct    = maxHP > 0 ? hp / maxHP : 0;
  const filled = Math.round(pct * total);
  let color = '#7ab648';
  if (pct <= 0.5)  color = '#d4a017';
  if (pct <= 0.25) color = '#cc2200';
  const bar = '\u2588'.repeat(filled) + '\u2591'.repeat(total - filled);
  return `<span style="color:${color}">[${bar}]</span>`;
}
```

Note: `\u2588` is `█` and `\u2591` is `░` — using unicode escapes to avoid hook false-positives.

- [ ] **Step 4: Add `applyWastelandMode` function**

In `web/src/lib/mud.ts`, find the `// ── Kids mode` comment (around line 312). Add `applyWastelandMode` immediately after the closing brace of `applyKidsMode`:

```typescript
// ── Wasteland mode ────────────────────────────────────────────────────────────

function applyWastelandMode(): void {
  _wastelandMode = true;
  document.body.dataset.ui = 'wasteland';
}
```

- [ ] **Step 5: Handle `ui_profile === 'wasteland'` in the world_meta handler**

In `web/src/lib/mud.ts`, find (around line 1603):

```typescript
if (meta.ui_profile === 'kids') {
  applyKidsMode();
}
```

Change to:

```typescript
if (meta.ui_profile === 'kids') {
  applyKidsMode();
} else if (meta.ui_profile === 'wasteland') {
  applyWastelandMode();
}
```

- [ ] **Step 6: Use `renderPipBoyBar` when in wasteland mode**

In `web/src/lib/mud.ts`, find in `applyStateUpdate` (around line 1766):

```typescript
hpHearts.innerHTML    = renderHearts(state.hp, state.maxHp);
```

Change to (this follows the identical pattern as the existing `renderHearts` call — values are numeric, no user HTML):

```typescript
hpHearts.innerHTML = _wastelandMode
  ? renderPipBoyBar(state.hp, state.maxHp)
  : renderHearts(state.hp, state.maxHp);
```

- [ ] **Step 7: Run the TypeScript tests — verify all pass**

```
cd web && npx vitest run src/lib/mud.test.ts 2>&1
```

Expected output: all tests PASS including `getWorldActions` suite.

- [ ] **Step 8: Run the full web test suite**

```
cd web && npx vitest run 2>&1 | tail -15
```

Expected: all tests PASS.

- [ ] **Step 9: Commit**

```bash
git add web/src/lib/mud.ts web/src/lib/mud.test.ts
git commit -m "feat(mudout): add wasteland UI mode, Pip-Boy HP bar, mudout action buttons"
```

---

## Task 5: Full suite verification

- [ ] **Step 1: Run the complete Go test suite**

```
go test ./... 2>&1
```

Expected: all packages PASS, no regressions.

- [ ] **Step 2: Verify mudout appears in available worlds**

```
go test ./internal/world/... -run TestListAvailable -v
```

Expected: PASS. The `mudout` world is discoverable via `ListAvailable()`.

---

## Self-Review

- **Spec coverage:** 12 rooms, 4 factions, 4 loot tables, 2 assembly crafting recipes (pipe-pistol with 7 slots, leather-armor with 4 slots), weather for all 3 biomes, arena/base stubs with readable items.
- **Type consistency:** `getWorldActions` exported in Task 3 Step 1, tested in Task 3 Step 2, implemented in Task 4 Step 1. `renderPipBoyBar` defined in Task 4 Step 3, used in Task 4 Step 6. `_wastelandMode` declared in Task 4 Step 2, set in Task 4 Step 4.
- **Placeholders:** None. Arena/base stubs use player-facing text, not dev notes.
