# Kids Crafting UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a kids-mode-only crafting modal with Minecraft-style paint grid, toggleable recipe drawer, and workbench guidance for Blockhaven.

**Architecture:** New `#kids-craft-modal` added to `game.astro` alongside the existing `#craft-modal`; when `data-ui=kids` the action-grid Craft button routes to the kids modal instead of the original. All new logic lives in `mud.ts` in a `_kidscraft` state object and helper functions co-located with the existing craft section (lines 680+). The existing `matchRecipe()`, `sendCommand()`, `_recipes`, and `_lastState` are reused as-is.

**Tech Stack:** TypeScript (mud.ts), Astro (game.astro), Playwright (E2E tests), Blockhaven world YAML (no changes needed).

---

## File Map

| File | Change |
|------|--------|
| `web/src/lib/mud.ts` | Add `workbench?` to Recipe (line 42); add `KIDS_ITEM_EMOJI`, `kidsItemEmoji()`, `matchRecipeByIds()` (after line 714); add `_kidscraft` state (after line 689); add all kids craft functions (after line 858); modify action-grid handler (line 1254); wire new event handlers in initMUD |
| `web/src/pages/game.astro` | Add `#kids-craft-modal` HTML (after line 1247); add CSS in `<style>` block (after line 621) |
| `web/e2e/modals.spec.ts` | Update existing craft modal tests (lines 9-32) to target `#kids-craft-modal` |
| `web/e2e/kids-craft-modal.spec.ts` | New: drawer toggle, recipe cards, workbench badge |
| `web/e2e/kids-craft-painting.spec.ts` | New: paint mechanic, arm/eraser, craft button states |
| `web/e2e/kids-craft-flow.spec.ts` | New: E2E happy path, workbench message, eraser flow |

---

## Task 1: Foundation — extend Recipe type, add helpers

**Files:**
- Modify: `web/src/lib/mud.ts:35-42` (Recipe interface)
- Modify: `web/src/lib/mud.ts:712` (add after matchRecipe)

- [ ] **Step 1: Add `workbench` field to Recipe interface**

In `mud.ts` at line 42 (end of Recipe interface), add the `workbench` field:

```typescript
interface Recipe {
  id: string;
  name: string;
  ingredients: RecipeIngredient[];
  outputId: string;
  outputName: string;
  skillReq?: number;
  workbench?: string;   // room ID required to craft, e.g. "workbench", "furnace"
}
```

- [ ] **Step 2: Add KIDS_ITEM_EMOJI map and kidsItemEmoji() after the existing TIER_ICON block (around line 134)**

```typescript
// -- Kids crafting emoji map --------------------------------------------------

const KIDS_ITEM_EMOJI: Record<string, string> = {
  'wood-log':      '🪵',
  'stick':         '🪴',
  'stone':         '🪨',
  'iron-ore':      '⛏️',
  'iron-ingot':    '🔩',
  'frost-essence': '❄️',
  'diamond':       '💎',
  'obsidian':      '⬛',
  'dirt':          '🟫',
  'workbench':     '🔨',
  'furnace':       '🔥',
  'chest':         '📦',
};

function kidsItemEmoji(itemId: string): string {
  return KIDS_ITEM_EMOJI[itemId] ?? '📦';
}
```

- [ ] **Step 3: Add matchRecipeByIds() after the existing matchRecipe() function (after line 712)**

```typescript
/**
 * Like matchRecipe() but works with slot item IDs (string | null)[]
 * instead of full InvItem objects. Used by the kids craft modal.
 */
function matchRecipeByIds(slots: (string | null)[]): Recipe | null {
  const counts: Record<string, number> = {};
  for (const id of slots) {
    if (id) counts[id] = (counts[id] ?? 0) + 1;
  }
  const placedKeys = Object.keys(counts);
  if (placedKeys.length === 0) return null;

  for (const recipe of _recipes) {
    const req: Record<string, number> = {};
    for (const ing of recipe.ingredients) {
      req[ing.id] = (req[ing.id] ?? 0) + ing.count;
    }
    const reqKeys = Object.keys(req);
    if (reqKeys.length !== placedKeys.length) continue;
    if (reqKeys.every(k => counts[k] === req[k]) && placedKeys.every(k => req[k] === counts[k])) {
      return recipe;
    }
  }
  return null;
}
```

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/mud.ts
git commit -m "feat(kids-craft): extend Recipe type and add emoji/match helpers"
```

---

## Task 2: Modal HTML + CSS

**Files:**
- Modify: `web/src/pages/game.astro:1247` (add HTML after quest-kids-modal)
- Modify: `web/src/pages/game.astro` style block (add CSS after line 621)

- [ ] **Step 1: Write the failing test for modal presence**

Create `web/e2e/kids-craft-modal.spec.ts`:

```typescript
import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

test.describe('kids craft modal -- presence', () => {
  test('kids-craft-modal exists in DOM', async ({ gamePage }) => {
    await expect(gamePage.locator('#kids-craft-modal')).toBeAttached();
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

```bash
cd web && npx playwright test e2e/kids-craft-modal.spec.ts --reporter=line
```

Expected: FAIL — `#kids-craft-modal` not found.

- [ ] **Step 3: Add the modal HTML to game.astro after line 1247 (after #quest-kids-modal closing div)**

```html
    <!-- Kids Craft Modal -->
    <div class="modal-overlay" id="kids-craft-modal">
      <div class="modal-box kids-craft-box">
        <button class="modal-close" id="kids-craft-close">&#x2715;</button>
        <div class="kids-craft-header">
          <div class="modal-title">crafting</div>
          <button class="kids-recipe-btn" id="kids-recipe-btn" title="Show recipes">?</button>
        </div>

        <div class="kids-inv-picker" id="kids-inv-picker"></div>

        <div class="kids-craft-layout">
          <div class="kids-craft-grid" id="kids-craft-grid"></div>
          <div class="craft-arrow">&#9654;</div>
          <div class="kids-craft-output" id="kids-craft-output">?</div>
        </div>

        <button id="kids-craft-do-btn" class="kids-craft-do-btn">&#x1F527; Open Recipe Guide</button>

        <div class="kids-recipe-drawer" id="kids-recipe-drawer">
          <div class="kids-recipe-drawer-header">
            <span>Crafting Recipes</span>
            <button class="kids-recipe-close" id="kids-recipe-close">&#x2715;</button>
          </div>
          <div class="kids-recipe-list" id="kids-recipe-list"></div>
        </div>

        <div class="kids-workbench-msg" id="kids-workbench-msg" hidden></div>
      </div>
    </div>
```

- [ ] **Step 4: Add CSS to the game.astro style block after the existing modal-close:hover rule (after line 621)**

```css
      .kids-workbench-msg[hidden] { display: none !important; }

      .kids-craft-box {
        width: min(380px, 95vw);
        max-height: 90vh;
        overflow: hidden;
        position: relative;
      }

      .kids-craft-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 0.75rem;
      }

      .kids-recipe-btn {
        background: #1a1a1a;
        border: 1px solid var(--comment);
        color: var(--green);
        font-size: 1rem;
        width: 28px;
        height: 28px;
        border-radius: 50%;
        cursor: pointer;
        line-height: 1;
        font-family: inherit;
      }

      .kids-inv-picker {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-bottom: 0.75rem;
        min-height: 36px;
      }

      .kids-inv-chip {
        background: #1a1a1a;
        border: 2px solid var(--comment);
        color: var(--fg);
        padding: 3px 10px;
        border-radius: 14px;
        cursor: pointer;
        font-size: 0.85rem;
        font-family: inherit;
        transition: border-color 0.1s, color 0.1s;
      }

      .kids-inv-chip.armed {
        border-color: var(--green);
        color: var(--green);
        box-shadow: 0 0 8px rgba(80,250,123,0.2);
      }

      .kids-inv-chip.eraser {
        border-color: var(--red);
        color: var(--red);
      }

      .kids-craft-layout {
        display: flex;
        align-items: center;
        gap: 1rem;
        justify-content: center;
        margin-bottom: 0.75rem;
      }

      .kids-craft-grid {
        display: grid;
        grid-template-columns: repeat(3, 52px);
        grid-template-rows: repeat(3, 52px);
        gap: 3px;
      }

      .kids-craft-cell {
        width: 52px;
        height: 52px;
        background: #1a1a1a;
        border: 2px solid #333;
        border-radius: 3px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 1.4rem;
        cursor: crosshair;
        user-select: none;
        color: var(--comment);
        transition: background 0.1s, border-color 0.1s;
      }

      .kids-craft-cell.filled {
        border-color: #555;
        background: #222;
        color: inherit;
      }

      .kids-craft-cell:hover { background: #2a2a2a; }

      .kids-craft-output {
        width: 60px;
        height: 60px;
        background: #1a1a1a;
        border: 2px solid #444;
        border-radius: 3px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 1.6rem;
        color: var(--comment);
      }

      .kids-craft-do-btn {
        width: 100%;
        padding: 0.6rem;
        background: #1a1a1a;
        border: 1px solid #444;
        color: #888;
        font-size: 0.9rem;
        font-family: inherit;
        border-radius: 3px;
        cursor: pointer;
        transition: all 0.15s;
      }

      .kids-craft-do-btn.ready {
        background: #0a1a0a;
        border-color: var(--green);
        color: var(--green);
        box-shadow: 0 0 10px rgba(80,250,123,0.15);
      }

      .kids-craft-do-btn:disabled {
        opacity: 0.45;
        cursor: not-allowed;
      }

      .kids-recipe-drawer {
        position: absolute;
        inset: 0;
        background: var(--bg-dark);
        border-radius: 4px;
        display: flex;
        flex-direction: column;
        transform: translateY(100%);
        transition: transform 0.22s ease;
        overflow: hidden;
        z-index: 10;
      }

      .kids-recipe-drawer.open { transform: translateY(0); }

      .kids-recipe-drawer-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem 1rem;
        border-bottom: 1px solid #333;
        color: var(--green);
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        flex-shrink: 0;
      }

      .kids-recipe-close {
        background: none;
        border: none;
        color: var(--comment);
        font-family: inherit;
        font-size: 1rem;
        cursor: pointer;
        line-height: 1;
      }

      .kids-recipe-close:hover { color: var(--red); }

      .kids-recipe-list {
        overflow-y: auto;
        flex: 1;
        padding: 0.75rem;
        display: flex;
        flex-direction: column;
        gap: 6px;
      }

      .kids-recipe-card {
        background: #1a1a1a;
        border: 1px solid #333;
        border-radius: 4px;
        padding: 8px 10px;
        cursor: pointer;
        display: flex;
        align-items: center;
        gap: 10px;
        transition: border-color 0.12s;
      }

      .kids-recipe-card:hover { border-color: #555; }

      .kids-recipe-card.needs-workbench { opacity: 0.5; }

      .kids-recipe-ing-grid {
        display: grid;
        grid-template-columns: repeat(3, 18px);
        gap: 1px;
        flex-shrink: 0;
      }

      .kids-recipe-ing-cell {
        width: 18px;
        height: 18px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.7rem;
        background: #111;
        border-radius: 2px;
      }

      .kids-recipe-arrow {
        color: var(--comment);
        font-size: 0.85rem;
        flex-shrink: 0;
      }

      .kids-recipe-output-label {
        color: var(--fg);
        font-size: 0.8rem;
        flex: 1;
      }

      .kids-workbench-badge {
        font-size: 0.65rem;
        background: #1a1000;
        border: 1px solid #664;
        color: #a86;
        padding: 2px 5px;
        border-radius: 8px;
        white-space: nowrap;
        flex-shrink: 0;
      }

      .kids-workbench-msg {
        position: absolute;
        inset: 0;
        background: rgba(0,0,0,0.88);
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 4px;
        padding: 1.5rem;
        text-align: center;
        color: #fc0;
        font-size: 1rem;
        line-height: 1.6;
        cursor: pointer;
        z-index: 20;
      }
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd web && npx playwright test e2e/kids-craft-modal.spec.ts --reporter=line
```

Expected: PASS — `#kids-craft-modal` is attached to DOM.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/game.astro web/e2e/kids-craft-modal.spec.ts
git commit -m "feat(kids-craft): add modal HTML and CSS"
```

---

## Task 3: Route craft button + open/close modal

**Files:**
- Modify: `web/src/lib/mud.ts:689` (add _kidscraft state after _matchedRecipe)
- Modify: `web/src/lib/mud.ts:1254` (update action-grid handler)
- Modify: `web/src/lib/mud.ts` (add openKidsCraftModal/closeKidsCraftModal after closeCraftModal at line 858)

- [ ] **Step 1: Write failing tests for open/close — add to kids-craft-modal.spec.ts**

```typescript
test.describe('kids craft modal -- open/close', () => {
  test('Craft button opens #kids-craft-modal in kids mode', async ({ gamePage }) => {
    await expect(gamePage.locator('body')).toHaveAttribute('data-ui', 'kids');
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('kids-craft-open') });
  });

  test('closes via close button', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await gamePage.click('#kids-craft-close');
    await expect(gamePage.locator('#kids-craft-modal')).not.toHaveClass(/open/);
  });

  test('closes via overlay click', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await gamePage.locator('#kids-craft-modal').click({ position: { x: 5, y: 5 } });
    await expect(gamePage.locator('#kids-craft-modal')).not.toHaveClass(/open/);
  });
});
```

- [ ] **Step 2: Run to verify tests fail**

```bash
cd web && npx playwright test e2e/kids-craft-modal.spec.ts --reporter=line
```

Expected: FAIL — clicking Craft still opens `#craft-modal`.

- [ ] **Step 3: Add _kidscraft state object after _matchedRecipe (line 689 of mud.ts)**

```typescript
// -- Kids craft modal state ---------------------------------------------------

interface KidsCraftState {
  armedItem: InvItem | null;
  eraser: boolean;
  slots: (string | null)[];  // 9 grid cells storing item IDs
  painting: boolean;          // true while mouse/touch button is held
}

const _kidscraft: KidsCraftState = {
  armedItem: null,
  eraser: false,
  slots: Array(9).fill(null),
  painting: false,
};
```

- [ ] **Step 4: Add stub functions + openKidsCraftModal/closeKidsCraftModal after closeCraftModal() (after line 858 of mud.ts)**

Add all of the following (stubs filled in later tasks):

```typescript
// -- Kids Craft Modal ---------------------------------------------------------

function renderKidsInvPicker() { /* implemented in Task 4 */ }
function wireKidsCraftCell(_cell: HTMLElement, _i: number) { /* implemented in Task 5 */ }
function refreshKidsCraftGrid() { /* implemented in Task 5 */ }
function renderKidsRecipeList() { /* implemented in Task 6 */ }

function openKidsCraftModal() {
  _kidscraft.armedItem = null;
  _kidscraft.eraser = false;
  _kidscraft.slots = Array(9).fill(null);
  _kidscraft.painting = false;

  renderKidsInvPicker();

  const grid = document.getElementById('kids-craft-grid')!;
  grid.replaceChildren();
  for (let i = 0; i < 9; i++) {
    const cell = document.createElement('div');
    cell.className = 'kids-craft-cell';
    cell.dataset.index = String(i);
    cell.textContent = '+';
    wireKidsCraftCell(cell, i);
    grid.appendChild(cell);
  }

  refreshKidsCraftGrid();
  document.getElementById('kids-craft-modal')!.classList.add('open');
}

function closeKidsCraftModal() {
  document.getElementById('kids-craft-modal')!.classList.remove('open');
  document.getElementById('kids-recipe-drawer')!.classList.remove('open');
  const msg = document.getElementById('kids-workbench-msg')!;
  msg.hidden = true;
}

function openKidsRecipeDrawer() {
  renderKidsRecipeList();
  document.getElementById('kids-recipe-drawer')!.classList.add('open');
}

function closeKidsRecipeDrawer() {
  document.getElementById('kids-recipe-drawer')!.classList.remove('open');
}

function showKidsWorkbenchMsg(text: string) {
  const el = document.getElementById('kids-workbench-msg')!;
  el.textContent = text + ' (tap to dismiss)';
  el.hidden = false;
}
```

- [ ] **Step 5: Modify action-grid handler at line 1254 of mud.ts**

Replace:
```typescript
    if (btn.dataset.special === 'craft') { openCraftModal(); return; }
```
With:
```typescript
    if (btn.dataset.special === 'craft') {
      if (document.body.dataset.ui === 'kids') {
        openKidsCraftModal();
      } else {
        openCraftModal();
      }
      return;
    }
```

- [ ] **Step 6: Wire all kids craft event handlers in initMUD (after the existing craft modal section, after line 1297 of mud.ts)**

```typescript
  // -- Kids Craft Modal event handlers ----------------------------------------

  document.getElementById('kids-craft-close')?.addEventListener('click', closeKidsCraftModal);

  document.getElementById('kids-craft-modal')?.addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeKidsCraftModal();
  });

  document.getElementById('kids-recipe-btn')?.addEventListener('click', openKidsRecipeDrawer);
  document.getElementById('kids-recipe-close')?.addEventListener('click', closeKidsRecipeDrawer);

  document.getElementById('kids-workbench-msg')?.addEventListener('click', () => {
    const el = document.getElementById('kids-workbench-msg')!;
    el.hidden = true;
  });

  document.getElementById('kids-craft-do-btn')?.addEventListener('click', () => {
    const matched = matchRecipeByIds(_kidscraft.slots);
    if (!matched) {
      openKidsRecipeDrawer();
      return;
    }
    if (matched.workbench) {
      const roomLabel = matched.workbench.replace(/-/g, ' ');
      showKidsWorkbenchMsg(
        `You need a ${roomLabel} to make this!\nFind one out in the world. 🔨`
      );
      return;
    }
    closeKidsCraftModal();
    sendCommand(`craft ${matched.id}`);
  });

  // Stop painting globally when mouse/touch is released
  document.addEventListener('mouseup', () => { _kidscraft.painting = false; });
  document.addEventListener('touchend', () => { _kidscraft.painting = false; });
```

- [ ] **Step 7: Update existing craft modal tests in modals.spec.ts (lines 9-32)**

Replace the existing `test.describe('craft modal', ...)` block:

```typescript
// -- Craft modal (kids mode routes to #kids-craft-modal) ----------------------

test.describe('craft modal', () => {
  test('Craft button opens #kids-craft-modal in kids mode', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('craft-open') });
  });

  test('closes via close button', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await gamePage.click('#kids-craft-close');
    await expect(gamePage.locator('#kids-craft-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('craft-closed-btn') });
  });

  test('closes via overlay click', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await gamePage.locator('#kids-craft-modal').click({ position: { x: 5, y: 5 } });
    await expect(gamePage.locator('#kids-craft-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('craft-closed-overlay') });
  });
});
```

- [ ] **Step 8: Run all tests**

```bash
cd web && npx playwright test --reporter=line
```

Expected: kids-craft-modal open/close tests PASS; modals.spec.ts craft tests PASS.

- [ ] **Step 9: Commit**

```bash
git add web/src/lib/mud.ts web/e2e/modals.spec.ts web/e2e/kids-craft-modal.spec.ts
git commit -m "feat(kids-craft): route craft button to kids modal, wire open/close"
```

---

## Task 4: Inventory picker

**Files:**
- Modify: `web/src/lib/mud.ts` (replace renderKidsInvPicker stub)

- [ ] **Step 1: Add failing inventory picker tests — add to kids-craft-modal.spec.ts**

```typescript
test.describe('kids craft modal -- inventory picker', () => {
  test('shows item chips when player has inventory', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    const chips = gamePage.locator('.kids-inv-chip');
    const count = await chips.count();
    if (count === 0) {
      test.skip(true, 'No inventory items -- cannot test picker chips');
      return;
    }
    await expect(chips.first()).toBeVisible();
    await gamePage.screenshot({ path: ss('kids-craft-inv-picker') });
  });

  test('tapping a chip arms it (.armed class added)', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    await chips.first().click();
    await expect(chips.first()).toHaveClass(/armed/);
  });

  test('tapping armed chip again switches to eraser mode (.eraser class)', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    const chip = chips.first();
    await chip.click();
    await expect(chip).toHaveClass(/armed/);
    await chip.click();
    await expect(chip).toHaveClass(/eraser/);
    await expect(chip).not.toHaveClass(/armed/);
  });
});
```

- [ ] **Step 2: Run to verify tests fail or skip**

```bash
cd web && npx playwright test e2e/kids-craft-modal.spec.ts --reporter=line
```

Expected: chip tests SKIP (no items) or FAIL (chips not rendered yet).

- [ ] **Step 3: Implement renderKidsInvPicker() — replace stub in mud.ts**

```typescript
function renderKidsInvPicker() {
  const picker = document.getElementById('kids-inv-picker')!;
  picker.replaceChildren();
  const inventory = _lastState?.inventory ?? [];
  const seen = new Set<string>();

  for (const item of inventory) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);

    const chip = document.createElement('button');
    chip.className = 'kids-inv-chip';
    chip.dataset.itemId = item.id;
    chip.textContent = kidsItemEmoji(item.id) + ' ' + item.name;

    chip.addEventListener('click', () => {
      if (_kidscraft.armedItem?.id === item.id && !_kidscraft.eraser) {
        // Already armed -- switch to eraser mode
        _kidscraft.eraser = true;
        _kidscraft.armedItem = null;
        document.querySelectorAll<HTMLElement>('.kids-inv-chip').forEach(c => {
          c.classList.remove('armed', 'eraser');
        });
        chip.classList.add('eraser');
      } else {
        // Arm this item
        _kidscraft.eraser = false;
        _kidscraft.armedItem = item;
        document.querySelectorAll<HTMLElement>('.kids-inv-chip').forEach(c => {
          c.classList.remove('armed', 'eraser');
        });
        chip.classList.add('armed');
      }
    });

    picker.appendChild(chip);
  }
}
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx playwright test e2e/kids-craft-modal.spec.ts --reporter=line
```

Expected: chip tests PASS (or SKIP if no inventory items in test world — acceptable).

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/mud.ts web/e2e/kids-craft-modal.spec.ts
git commit -m "feat(kids-craft): inventory picker -- arm/eraser toggle"
```

---

## Task 5: Paint mechanic

**Files:**
- Modify: `web/src/lib/mud.ts` (replace wireKidsCraftCell and refreshKidsCraftGrid stubs)

- [ ] **Step 1: Create kids-craft-painting.spec.ts with failing tests**

Create `web/e2e/kids-craft-painting.spec.ts`:

```typescript
import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

test.describe('kids craft -- paint mechanic', () => {
  test('grid renders 9 cells when modal opens', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await expect(gamePage.locator('.kids-craft-cell')).toHaveCount(9);
    await gamePage.screenshot({ path: ss('kids-craft-grid-empty') });
  });

  test('craft button shows "Open Recipe Guide" when grid is empty', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Open Recipe Guide');
    await expect(btn).not.toBeDisabled();
  });

  test('clicking armed item into empty cell marks cell as filled', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items to paint with');
      return;
    }
    await chips.first().click();
    const cell = gamePage.locator('.kids-craft-cell').first();
    await cell.click();
    await expect(cell).toHaveClass(/filled/);
    await expect(cell).not.toContainText('+');
    await gamePage.screenshot({ path: ss('kids-craft-cell-filled') });
  });

  test('right-click on filled cell clears it', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    await chips.first().click();
    const cell = gamePage.locator('.kids-craft-cell').first();
    await cell.click();
    await expect(cell).toHaveClass(/filled/);
    await cell.click({ button: 'right' });
    await expect(cell).not.toHaveClass(/filled/);
    await gamePage.screenshot({ path: ss('kids-craft-cell-cleared') });
  });

  test('craft button state reflects grid contents', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    await chips.first().click();
    await gamePage.locator('.kids-craft-cell').first().click();
    const btn = gamePage.locator('#kids-craft-do-btn');
    const text = await btn.textContent();
    // Should be "Craft: X" (match), "No matching recipe" (no match), not "Open Recipe Guide"
    expect(
      text?.includes('Craft:') || text?.includes('No matching recipe')
    ).toBe(true);
  });
});
```

- [ ] **Step 2: Run to verify tests fail**

```bash
cd web && npx playwright test e2e/kids-craft-painting.spec.ts --reporter=line
```

Expected: "grid renders 9 cells" FAILS because cells are not rendered (stub is empty). Others fail similarly.

- [ ] **Step 3: Implement wireKidsCraftCell() — replace stub in mud.ts**

```typescript
function wireKidsCraftCell(cell: HTMLElement, index: number) {
  const paint = () => {
    if (_kidscraft.eraser) {
      _kidscraft.slots[index] = null;
    } else if (_kidscraft.armedItem) {
      _kidscraft.slots[index] = _kidscraft.armedItem.id;
    }
    refreshKidsCraftGrid();
  };

  cell.addEventListener('mousedown', (e) => {
    if (e.button !== 0) return;
    e.preventDefault();
    _kidscraft.painting = true;
    paint();
  });

  cell.addEventListener('mouseover', () => {
    if (_kidscraft.painting) paint();
  });

  cell.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    _kidscraft.slots[index] = null;
    refreshKidsCraftGrid();
  });

  cell.addEventListener('touchstart', (e) => {
    e.preventDefault();
    _kidscraft.painting = true;
    paint();
  }, { passive: false });

  cell.addEventListener('touchmove', (e) => {
    e.preventDefault();
    const touch = e.touches[0];
    const el = document.elementFromPoint(touch.clientX, touch.clientY);
    const touchedCell = el?.closest<HTMLElement>('.kids-craft-cell');
    if (!touchedCell) return;
    const idx = parseInt(touchedCell.dataset.index ?? '-1', 10);
    if (idx < 0 || idx >= 9) return;
    if (_kidscraft.eraser) {
      _kidscraft.slots[idx] = null;
    } else if (_kidscraft.armedItem) {
      _kidscraft.slots[idx] = _kidscraft.armedItem.id;
    }
    refreshKidsCraftGrid();
  }, { passive: false });
}
```

- [ ] **Step 4: Implement refreshKidsCraftGrid() — replace stub in mud.ts**

```typescript
function refreshKidsCraftGrid() {
  document.querySelectorAll<HTMLElement>('.kids-craft-cell').forEach((cell, i) => {
    const itemId = _kidscraft.slots[i];
    cell.textContent = itemId ? kidsItemEmoji(itemId) : '+';
    cell.classList.toggle('filled', !!itemId);
  });

  const matched = matchRecipeByIds(_kidscraft.slots);
  const btn = document.getElementById('kids-craft-do-btn') as HTMLButtonElement | null;
  const output = document.getElementById('kids-craft-output');
  if (!btn || !output) return;

  const hasItems = _kidscraft.slots.some(s => s !== null);

  if (matched) {
    btn.textContent = '\u{1F527} Craft: ' + matched.name;
    btn.disabled = false;
    btn.classList.add('ready');
    output.textContent = kidsItemEmoji(matched.outputId);
  } else if (hasItems) {
    btn.textContent = '\u{1F527} No matching recipe';
    btn.disabled = true;
    btn.classList.remove('ready');
    output.textContent = '?';
  } else {
    btn.textContent = '\u{1F527} Open Recipe Guide';
    btn.disabled = false;
    btn.classList.remove('ready');
    output.textContent = '?';
  }
}
```

- [ ] **Step 5: Run tests**

```bash
cd web && npx playwright test e2e/kids-craft-painting.spec.ts --reporter=line
```

Expected: all tests PASS (inventory-dependent tests SKIP if no items in test world).

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/mud.ts web/e2e/kids-craft-painting.spec.ts
git commit -m "feat(kids-craft): paint mechanic -- fill, clear, drag, touch"
```

---

## Task 6: Recipe drawer

**Files:**
- Modify: `web/src/lib/mud.ts` (replace renderKidsRecipeList stub)

- [ ] **Step 1: Add failing recipe drawer tests — add to kids-craft-modal.spec.ts**

```typescript
test.describe('kids craft modal -- recipe drawer', () => {
  test('? button opens recipe drawer', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-recipe-drawer')).not.toHaveClass(/open/);
    await gamePage.click('#kids-recipe-btn');
    await expect(gamePage.locator('#kids-recipe-drawer')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('kids-recipe-drawer-open') });
  });

  test('close button dismisses recipe drawer', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    await expect(gamePage.locator('#kids-recipe-drawer')).toHaveClass(/open/);
    await gamePage.click('#kids-recipe-close');
    await expect(gamePage.locator('#kids-recipe-drawer')).not.toHaveClass(/open/);
  });

  test('recipe cards are shown in drawer', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const cards = gamePage.locator('.kids-recipe-card');
    await expect(cards.first()).toBeVisible({ timeout: 5_000 });
    expect(await cards.count()).toBeGreaterThan(0);
    await gamePage.screenshot({ path: ss('kids-recipe-cards') });
  });

  test('workbench recipes show badge and have needs-workbench class', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const badges = gamePage.locator('.kids-workbench-badge');
    await expect(badges.first()).toBeVisible({ timeout: 5_000 });
    expect(await badges.count()).toBeGreaterThan(0);
    const card = badges.first().locator('xpath=ancestor::div[contains(@class,"kids-recipe-card")]');
    await expect(card).toHaveClass(/needs-workbench/);
    await gamePage.screenshot({ path: ss('kids-recipe-workbench-badge') });
  });

  test('tapping recipe card closes drawer and populates grid', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const nonWorkbench = gamePage.locator('.kids-recipe-card:not(.needs-workbench)').first();
    const visible = await nonWorkbench.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No non-workbench recipe cards visible');
      return;
    }
    await nonWorkbench.click();
    await expect(gamePage.locator('#kids-recipe-drawer')).not.toHaveClass(/open/);
    await expect(gamePage.locator('.kids-craft-cell.filled').first()).toBeVisible();
    await gamePage.screenshot({ path: ss('kids-recipe-card-autofill') });
  });
});
```

- [ ] **Step 2: Run to verify tests fail**

```bash
cd web && npx playwright test e2e/kids-craft-modal.spec.ts --reporter=line
```

Expected: drawer open/close PASS (wired in Task 3); recipe card tests FAIL (no cards rendered yet).

- [ ] **Step 3: Implement renderKidsRecipeList() — replace stub in mud.ts**

```typescript
function renderKidsRecipeList() {
  const list = document.getElementById('kids-recipe-list')!;
  list.replaceChildren();

  for (const recipe of _recipes) {
    const card = document.createElement('div');
    card.className = 'kids-recipe-card';
    if (recipe.workbench) card.classList.add('needs-workbench');

    // Build flat ingredient slot array: fill cells in ingredient order
    const ingSlots: (string | null)[] = Array(9).fill(null);
    let slotIdx = 0;
    for (const ing of recipe.ingredients) {
      for (let n = 0; n < ing.count && slotIdx < 9; n++, slotIdx++) {
        ingSlots[slotIdx] = ing.id;
      }
    }

    const ingGrid = document.createElement('div');
    ingGrid.className = 'kids-recipe-ing-grid';
    for (const id of ingSlots) {
      const cell = document.createElement('span');
      cell.className = 'kids-recipe-ing-cell';
      cell.textContent = id ? kidsItemEmoji(id) : '';
      ingGrid.appendChild(cell);
    }

    const arrow = document.createElement('span');
    arrow.className = 'kids-recipe-arrow';
    arrow.textContent = '\u2192';

    const outputLabel = document.createElement('span');
    outputLabel.className = 'kids-recipe-output-label';
    outputLabel.textContent = kidsItemEmoji(recipe.outputId) + ' ' + recipe.name;

    if (recipe.workbench) {
      const badge = document.createElement('span');
      badge.className = 'kids-workbench-badge';
      badge.textContent = '\u{1F528} Needs Workbench';
      card.appendChild(badge);
    }

    card.appendChild(ingGrid);
    card.appendChild(arrow);
    card.appendChild(outputLabel);

    // Auto-populate grid on click
    card.addEventListener('click', () => {
      _kidscraft.slots = [...ingSlots];
      closeKidsRecipeDrawer();
      refreshKidsCraftGrid();
    });

    list.appendChild(card);
  }
}
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx playwright test e2e/kids-craft-modal.spec.ts --reporter=line
```

Expected: all recipe drawer tests PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/mud.ts web/e2e/kids-craft-modal.spec.ts
git commit -m "feat(kids-craft): recipe drawer with cards, workbench badge, auto-fill"
```

---

## Task 7: Workbench guidance tests

**Files:**
- Modify: `web/e2e/kids-craft-painting.spec.ts` (add workbench tests)

The workbench logic is already wired in the `kids-craft-do-btn` click handler from Task 3. This task adds the test to verify it.

- [ ] **Step 1: Add workbench guidance tests to kids-craft-painting.spec.ts**

```typescript
test.describe('kids craft -- workbench guidance', () => {
  test('workbench recipe shows message and does not send craft command', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const workbenchCard = gamePage.locator('.kids-recipe-card.needs-workbench').first();
    const visible = await workbenchCard.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No workbench recipe cards visible');
      return;
    }
    await workbenchCard.click();
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Craft:');

    // Intercept WebSocket sends before clicking
    await gamePage.evaluate(() => {
      (window as any).__wsCraftCmds = [];
      const orig = WebSocket.prototype.send;
      WebSocket.prototype.send = function(data: unknown) {
        try {
          const msg = JSON.parse(data as string);
          if (msg.type === 'input' && typeof msg.payload?.text === 'string') {
            (window as any).__wsCraftCmds.push(msg.payload.text);
          }
        } catch { /* not JSON */ }
        return orig.call(this, data);
      };
    });

    await btn.click();

    // Message should be visible
    await expect(gamePage.locator('#kids-workbench-msg')).not.toHaveAttribute('hidden', '');
    await expect(gamePage.locator('#kids-workbench-msg')).toContainText('\u{1F528}');

    // No craft command should have been sent
    const cmds: string[] = await gamePage.evaluate(() => (window as any).__wsCraftCmds ?? []);
    expect(cmds.filter(c => c.startsWith('craft '))).toHaveLength(0);

    await gamePage.screenshot({ path: ss('kids-craft-workbench-msg') });
  });

  test('tapping workbench message dismisses it', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const workbenchCard = gamePage.locator('.kids-recipe-card.needs-workbench').first();
    const visible = await workbenchCard.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No workbench recipe cards visible');
      return;
    }
    await workbenchCard.click();
    await gamePage.locator('#kids-craft-do-btn').click();
    await expect(gamePage.locator('#kids-workbench-msg')).not.toHaveAttribute('hidden', '');
    await gamePage.locator('#kids-workbench-msg').click();
    await expect(gamePage.locator('#kids-workbench-msg')).toHaveAttribute('hidden', '');
  });
});
```

- [ ] **Step 2: Run all tests**

```bash
cd web && npx playwright test --reporter=line
```

Expected: all tests PASS or SKIP — no failures.

- [ ] **Step 3: Commit**

```bash
git add web/e2e/kids-craft-painting.spec.ts
git commit -m "test(kids-craft): workbench guidance -- message shown, no command sent"
```

---

## Task 8: End-to-end flow tests

**Files:**
- Create: `web/e2e/kids-craft-flow.spec.ts`

- [ ] **Step 1: Write all flow tests**

Create `web/e2e/kids-craft-flow.spec.ts`:

```typescript
import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

test.describe('kids craft -- E2E flows', () => {
  test('kids mode is active', async ({ gamePage }) => {
    await expect(gamePage.locator('body')).toHaveAttribute('data-ui', 'kids');
  });

  test('kids-craft-modal and sub-elements are in DOM', async ({ gamePage }) => {
    await expect(gamePage.locator('#kids-craft-modal')).toBeAttached();
    await expect(gamePage.locator('#kids-craft-grid')).toBeAttached();
    await expect(gamePage.locator('#kids-recipe-drawer')).toBeAttached();
    await expect(gamePage.locator('#kids-inv-picker')).toBeAttached();
  });

  test('empty grid + craft button opens recipe drawer', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Open Recipe Guide');
    await btn.click();
    await expect(gamePage.locator('#kids-recipe-drawer')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('kids-craft-flow-empty-opens-drawer') });
  });

  test('arm item, paint cells, verify button state updates', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    await chips.first().click();
    await expect(chips.first()).toHaveClass(/armed/);

    const cells = gamePage.locator('.kids-craft-cell');
    await cells.nth(0).click();
    await cells.nth(1).click();
    await cells.nth(2).click();

    await expect(cells.nth(0)).toHaveClass(/filled/);
    await expect(cells.nth(1)).toHaveClass(/filled/);
    await expect(cells.nth(2)).toHaveClass(/filled/);

    const btn = gamePage.locator('#kids-craft-do-btn');
    const text = await btn.textContent();
    expect(text?.includes('Craft:') || text?.includes('No matching recipe')).toBe(true);
    await gamePage.screenshot({ path: ss('kids-craft-flow-painted') });
  });

  test('recipe card auto-fills grid and enables craft when matched', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const nonWorkbench = gamePage.locator('.kids-recipe-card:not(.needs-workbench)').first();
    const visible = await nonWorkbench.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No non-workbench recipe cards visible');
      return;
    }
    await nonWorkbench.click();
    await expect(gamePage.locator('#kids-recipe-drawer')).not.toHaveClass(/open/);
    await expect(gamePage.locator('.kids-craft-cell.filled').first()).toBeVisible();
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Craft:');
    await expect(btn).not.toBeDisabled();
    await gamePage.screenshot({ path: ss('kids-craft-flow-autofill-ready') });
  });

  test('eraser mode clears painted cells', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    const chip = chips.first();
    await chip.click();
    const cell = gamePage.locator('.kids-craft-cell').first();
    await cell.click();
    await expect(cell).toHaveClass(/filled/);
    await chip.click();
    await expect(chip).toHaveClass(/eraser/);
    await cell.click();
    await expect(cell).not.toHaveClass(/filled/);
    await gamePage.screenshot({ path: ss('kids-craft-flow-eraser') });
  });

  test('happy path: auto-fill non-workbench recipe, craft, modal closes, command sent', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const nonWorkbench = gamePage.locator('.kids-recipe-card:not(.needs-workbench)').first();
    const visible = await nonWorkbench.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No non-workbench recipe cards visible');
      return;
    }

    await gamePage.evaluate(() => {
      (window as any).__wsCraftCmds = [];
      const orig = WebSocket.prototype.send;
      WebSocket.prototype.send = function(data: unknown) {
        try {
          const msg = JSON.parse(data as string);
          if (msg.type === 'input' && (msg.payload?.text as string).startsWith('craft ')) {
            (window as any).__wsCraftCmds.push(msg.payload.text);
          }
        } catch { /* not JSON */ }
        return orig.call(this, data);
      };
    });

    await nonWorkbench.click();
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Craft:');
    await btn.click();

    await expect(gamePage.locator('#kids-craft-modal')).not.toHaveClass(/open/);

    const cmds: string[] = await gamePage.evaluate(() => (window as any).__wsCraftCmds ?? []);
    expect(cmds.length).toBeGreaterThan(0);
    expect(cmds[0]).toMatch(/^craft /);

    await gamePage.screenshot({ path: ss('kids-craft-flow-happy-path') });
  });
});
```

- [ ] **Step 2: Run to verify tests pass**

```bash
cd web && npx playwright test e2e/kids-craft-flow.spec.ts --reporter=line
```

Expected: structural tests PASS; item-dependent tests SKIP or PASS.

- [ ] **Step 3: Run full test suite**

```bash
cd web && npx playwright test --reporter=line
```

Expected: all tests PASS or SKIP — zero failures.

- [ ] **Step 4: Commit**

```bash
git add web/e2e/kids-craft-flow.spec.ts
git commit -m "test(kids-craft): E2E flow tests -- happy path, workbench, eraser, auto-fill"
```

---

## Self-Review

**Spec coverage:**
- Recipe help drawer (toggleable ?) — Task 6
- Minecraft-style paint grid — Task 5
- Workbench grayed-out with badge — Task 6 (renderKidsRecipeList)
- Workbench guidance message on craft attempt — Task 3 (click handler) + Task 7 (tests)
- Auto-populate from recipe card — Task 6
- Arm/eraser toggle — Task 4
- Touch support — Task 5 (wireKidsCraftCell touchmove)
- Existing craft modal tests updated — Task 3
- Full Playwright coverage (modal, painting, flow) — Tasks 2/5/6/7/8

**Placeholder scan:** No TBDs. All stubs named and filled in their respective tasks.

**Type consistency:**
- `_kidscraft.slots: (string | null)[]` passed to `matchRecipeByIds(slots: (string | null)[])` — consistent.
- `_kidscraft.armedItem: InvItem | null` used in `renderKidsInvPicker` and `wireKidsCraftCell` — consistent.
- `Recipe.workbench?: string` added in Task 1, read in Task 3 click handler and Task 6 renderKidsRecipeList — consistent.
- `_lastState?.inventory` used in renderKidsInvPicker — `_lastState` is assigned in `applyStateUpdate` in mud.ts; declared at module scope as `let _lastState: StateUpdate | null = null` (verify the exact declaration before Task 4).
