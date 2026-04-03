# Playwright UI Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Playwright E2E test suite for modal open/close behaviour plus a Claude skill (`/ui-test`) that runs the tests and reports failures with screenshots.

**Architecture:** Playwright config in `web/playwright.config.ts` uses the `webServer` option to start the Go server on port 8091 before tests. A shared `gamePage` fixture handles WebSocket login. Three test groups cover craft, item, and quest modals. A Claude skill at `~/.claude/skills/ui-test.md` invokes the tests and interprets results.

**Tech Stack:** `@playwright/test`, Chromium, Bun, Go (`main.go -serve`), Astro frontend

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `web/playwright.config.ts` | Create | Playwright config: webServer, browser, reporters |
| `web/e2e/helpers/auth.ts` | Create | Shared `gamePage` fixture: login + wait for HUD |
| `web/e2e/modals.spec.ts` | Create | All modal tests (craft, item, quest) |
| `web/package.json` | Modify | Add `test:e2e` and `test:e2e:headed` scripts |
| `~/.claude/skills/ui-test.md` | Create | Claude skill: run tests, show screenshots, diagnose |

---

### Task 1: Install Playwright and add scripts

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install `@playwright/test`**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun add -D @playwright/test
```

Expected: `@playwright/test` appears in `devDependencies` in `package.json`.

- [ ] **Step 2: Install Chromium browser**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx playwright install chromium
```

Expected: Chromium downloaded, no errors.

- [ ] **Step 3: Add test scripts to `web/package.json`**

Open `web/package.json`. In the `"scripts"` block, add two entries after `"test:watch"`:

```json
{
  "scripts": {
    "dev": "astro dev",
    "build": "astro build",
    "preview": "astro preview",
    "astro": "astro",
    "test": "vitest run",
    "test:watch": "vitest",
    "test:e2e": "playwright test",
    "test:e2e:headed": "playwright test --headed"
  }
}
```

- [ ] **Step 4: Verify Playwright CLI works**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx playwright --version
```

Expected: prints version like `Version 1.x.x`

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/package.json web/bun.lockb && git commit -m "chore(web): install playwright and add test:e2e scripts"
```

---

### Task 2: Create Playwright config

**Files:**
- Create: `web/playwright.config.ts`

- [ ] **Step 1: Create `web/playwright.config.ts`**

```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  outputDir: './test-results',
  use: {
    baseURL: 'http://localhost:8091',
    screenshot: 'on',
    viewport: { width: 1280, height: 800 },
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
  webServer: {
    command: 'go run . -serve -world blockhaven -port 8091',
    cwd: '../',
    url: 'http://localhost:8091',
    reuseExistingServer: false,
    timeout: 30_000,
  },
  reporter: [
    ['json', { outputFile: './test-results/results.json' }],
    ['html', { outputFolder: './test-results/html', open: 'never' }],
  ],
});
```

- [ ] **Step 2: Create the test-results directory with a .gitkeep**

```bash
mkdir -p /Users/stokes/Projects/gl1tch-mud/web/test-results && touch /Users/stokes/Projects/gl1tch-mud/web/test-results/.gitkeep
```

- [ ] **Step 3: Add test-results output to .gitignore**

Add to `web/.gitignore` (create if it doesn't exist):

```
test-results/html/
test-results/results.json
test-results/screenshots/
test-results/*.png
```

Note: keep `.gitkeep` tracked so the directory exists for CI.

- [ ] **Step 4: Verify config is valid**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx playwright test --list 2>&1 | head -5
```

Expected: either "No tests found" or a config parse error. A parse error means fix the config. "No tests found" is correct at this stage.

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/playwright.config.ts web/test-results/.gitkeep web/.gitignore && git commit -m "chore(web): add playwright config with webServer pointing at Go server"
```

---

### Task 3: Create auth fixture

**Files:**
- Create: `web/e2e/helpers/auth.ts`

- [ ] **Step 1: Create the directory**

```bash
mkdir -p /Users/stokes/Projects/gl1tch-mud/web/e2e/helpers
```

- [ ] **Step 2: Create `web/e2e/helpers/auth.ts`**

```typescript
import { test as base, expect, type Page } from '@playwright/test';

/**
 * gamePage: a Page already authenticated and inside the game HUD.
 * Navigates to /game?world=blockhaven, logs in as "tester" with no passphrase,
 * and waits until the HUD is visible and input is enabled.
 */
export const test = base.extend<{ gamePage: Page }>({
  gamePage: async ({ page }, use) => {
    await page.goto('/game?world=blockhaven');

    // Fill login form
    await page.fill('#player-name', 'tester');
    // passphrase left blank — server has no passphrase in test config

    await page.click('#connect-btn');

    // Wait for HUD to be shown (auth.ok received → showHUD() called)
    await page.waitForSelector('#hud-screen.active', { timeout: 10_000 });

    // Wait for first output.done — cmd-input becomes enabled
    await page.waitForSelector('#cmd-input:not([disabled])', { timeout: 10_000 });

    // Wait for state.update — action-grid should have buttons
    await page.waitForSelector('#action-grid .action-btn', { timeout: 10_000 });

    await use(page);
  },
});

export { expect };
```

- [ ] **Step 3: Verify the file parses (TypeScript check)**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx tsc --noEmit --skipLibCheck e2e/helpers/auth.ts 2>&1 | head -20
```

Expected: no output (clean) or only missing-module warnings for `@playwright/test` which will be resolved at runtime.

- [ ] **Step 4: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/e2e/helpers/auth.ts && git commit -m "test(web): add playwright auth fixture for game login"
```

---

### Task 4: Craft modal tests

**Files:**
- Create: `web/e2e/modals.spec.ts`

The craft modal is the simplest to test — no prerequisite state needed. The Craft button is always present in the kids-mode action grid (`data-kids-action="craft"`). We open it, screenshot it, close it via button, close it via overlay click.

The overlay click must land on the `.modal-overlay` element itself (not on `.modal-box`). Use `{ position: { x: 5, y: 5 } }` to click the top-left corner of the overlay, which is guaranteed to be outside `.modal-box`.

- [ ] **Step 1: Create `web/e2e/modals.spec.ts` with craft tests**

```typescript
import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

// ── Craft modal ───────────────────────────────────────────────────────────────

test.describe('craft modal', () => {
  test('opens when Craft button is clicked', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#craft-modal')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('craft-open') });
  });

  test('closes via close button', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#craft-modal')).toHaveClass(/open/);
    await gamePage.click('#craft-modal-close');
    await expect(gamePage.locator('#craft-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('craft-closed-btn') });
  });

  test('closes via overlay click', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#craft-modal')).toHaveClass(/open/);
    // Click top-left corner of overlay — always outside .modal-box
    await gamePage.locator('#craft-modal').click({ position: { x: 5, y: 5 } });
    await expect(gamePage.locator('#craft-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('craft-closed-overlay') });
  });
});
```

- [ ] **Step 2: Create the screenshots directory**

```bash
mkdir -p /Users/stokes/Projects/gl1tch-mud/web/test-results/screenshots
```

- [ ] **Step 3: Run just the craft tests to see them fail (proves tests are wired correctly)**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx playwright test --grep "craft modal" 2>&1 | tail -30
```

Expected: tests run (server starts), some may fail if modal is broken — that's the point. If you see "browser not found" errors, re-run `bunx playwright install chromium`. If you see TypeScript errors, fix them before continuing.

- [ ] **Step 4: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/e2e/modals.spec.ts web/test-results/screenshots/.gitkeep && git commit -m "test(web): add craft modal open/close E2E tests"
```

---

### Task 5: Item modal tests

**Files:**
- Modify: `web/e2e/modals.spec.ts`

The item modal requires a `.inv-slot.occupied` element to click. Block Haven seeds starting items for new players (`world.SeedStartingItems`). After `state.update` is received, `renderInventory` populates the grid. Wait up to 5 seconds for an occupied slot; skip the test group with a message if none appear.

- [ ] **Step 1: Append item modal tests to `web/e2e/modals.spec.ts`**

Add after the closing `});` of the craft modal describe block:

```typescript
// ── Item modal ────────────────────────────────────────────────────────────────

test.describe('item modal', () => {
  test('opens when inventory item is clicked', async ({ gamePage }) => {
    const slot = gamePage.locator('.inv-slot.occupied').first();
    const hasItem = await slot.isVisible({ timeout: 5_000 }).catch(() => false);
    if (!hasItem) {
      test.skip(true, 'No inventory items seeded — cannot test item modal');
      return;
    }
    await slot.click();
    await expect(gamePage.locator('#item-modal')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('item-open') });
  });

  test('closes via close button', async ({ gamePage }) => {
    const slot = gamePage.locator('.inv-slot.occupied').first();
    const hasItem = await slot.isVisible({ timeout: 5_000 }).catch(() => false);
    if (!hasItem) {
      test.skip(true, 'No inventory items seeded — cannot test item modal');
      return;
    }
    await slot.click();
    await expect(gamePage.locator('#item-modal')).toHaveClass(/open/);
    await gamePage.click('#item-modal-close');
    await expect(gamePage.locator('#item-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('item-closed-btn') });
  });

  test('closes via overlay click', async ({ gamePage }) => {
    const slot = gamePage.locator('.inv-slot.occupied').first();
    const hasItem = await slot.isVisible({ timeout: 5_000 }).catch(() => false);
    if (!hasItem) {
      test.skip(true, 'No inventory items seeded — cannot test item modal');
      return;
    }
    await slot.click();
    await expect(gamePage.locator('#item-modal')).toHaveClass(/open/);
    await gamePage.locator('#item-modal').click({ position: { x: 5, y: 5 } });
    await expect(gamePage.locator('#item-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('item-closed-overlay') });
  });
});
```

- [ ] **Step 2: Run item modal tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx playwright test --grep "item modal" 2>&1 | tail -30
```

Expected: tests run; skipped if no inventory items, or fail/pass based on modal state.

- [ ] **Step 3: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/e2e/modals.spec.ts && git commit -m "test(web): add item modal open/close E2E tests"
```

---

### Task 6: Quest modal tests

**Files:**
- Modify: `web/e2e/modals.spec.ts`

The quest modal is kids-mode only. Block Haven sends `ui_profile: "kids"` in `world_meta`, which triggers `applyKidsMode()` and sets `data-ui="kids"` on `<body>`. The Quests button (`data-kids-action="quests"`) is always rendered in the kids action grid. The modal always opens (even with no quests — it shows "No active quests yet.").

- [ ] **Step 1: Append quest modal tests to `web/e2e/modals.spec.ts`**

Add after the closing `});` of the item modal describe block:

```typescript
// ── Quest modal (kids mode) ───────────────────────────────────────────────────

test.describe('quest modal', () => {
  test('body has data-ui=kids (kids mode active)', async ({ gamePage }) => {
    // Confirms applyKidsMode() ran — prerequisite for all quest modal tests
    await expect(gamePage.locator('body')).toHaveAttribute('data-ui', 'kids');
  });

  test('opens when Quests button is clicked', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="quests"]');
    await expect(gamePage.locator('#quest-kids-modal')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('quest-open') });
  });

  test('closes via close button', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="quests"]');
    await expect(gamePage.locator('#quest-kids-modal')).toHaveClass(/open/);
    await gamePage.click('#quest-kids-modal-close');
    await expect(gamePage.locator('#quest-kids-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('quest-closed-btn') });
  });

  test('closes via overlay click', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="quests"]');
    await expect(gamePage.locator('#quest-kids-modal')).toHaveClass(/open/);
    await gamePage.locator('#quest-kids-modal').click({ position: { x: 5, y: 5 } });
    await expect(gamePage.locator('#quest-kids-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('quest-closed-overlay') });
  });
});
```

- [ ] **Step 2: Run the full test suite**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx playwright test 2>&1 | tail -40
```

Expected: all 10 tests run (3 craft + 3 item + 4 quest). Some will fail if modals are broken — that's expected and the point of the tests.

- [ ] **Step 3: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/e2e/modals.spec.ts && git commit -m "test(web): add quest modal E2E tests and kids-mode activation check"
```

---

### Task 7: Create `/ui-test` Claude skill

**Files:**
- Create: `~/.claude/skills/ui-test.md`

This skill is invoked with `/ui-test` in any conversation. It runs Playwright, reads the JSON results, reads failure screenshots, and reports plain-English findings.

- [ ] **Step 1: Create `~/.claude/skills/ui-test.md`**

```markdown
---
name: ui-test
description: Run Playwright E2E tests for the gl1tch-mud web UI, show screenshots for failures, and diagnose what's broken.
invocation: /ui-test
---

# UI Test Runner

Run the Playwright modal test suite, interpret results, and show inline screenshots for any failures.

## Steps

1. **Run the tests:**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bunx playwright test --reporter=json 2>&1
```

The `webServer` config starts the Go server automatically. Tests take ~30 seconds (server startup + 10 tests).

2. **Read `web/test-results/results.json`** to get structured pass/fail data.

3. **For each failed test:**
   - Read the screenshot file from `web/test-results/screenshots/` that matches the test name
   - Also check `web/test-results/` for any `*-failed-*.png` Playwright auto-captures
   - Show the screenshot inline using the Read tool
   - State the assertion that failed
   - Give a likely cause based on the test name:
     - "closes via overlay click" failures → click probably landed on `.modal-box` not the overlay; suggest `click({ position: { x: 5, y: 5 } })` already in use — look at whether `e.target === e.currentTarget` check in the handler fires
     - "closes via close button" failures → close button click handler not wired, or `classList.remove('open')` not called
     - "opens when ... clicked" failures → action button click not reaching the modal open function; check `handleKidsAction` routing

4. **Report format:**

```
{pass_count}/{total_count} tests passed.

PASSED: craft modal opens when Craft button is clicked
PASSED: ...

FAILED: craft modal closes via overlay click
  Assertion: #craft-modal should not have class "open"
  Screenshot: [inline image]
  Likely cause: ...
  File to check: web/src/lib/mud.ts — closeCraftModal() or overlay click handler
```

5. If all tests pass, report: "All {n} modal tests passed. Screenshots saved to `web/test-results/screenshots/`."
```

- [ ] **Step 2: Verify skill is discoverable**

```bash
ls ~/.claude/skills/ui-test.md
```

Expected: file exists.

- [ ] **Step 3: Run `/ui-test` in this conversation to verify it works end-to-end**

Type `/ui-test` in the conversation. Expected: skill loads, tests run, results reported with screenshots.

- [ ] **Step 4: Commit the plan files (skill is outside the repo)**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add docs/superpowers/plans/2026-04-03-playwright-ui-testing.md && git commit -m "docs: add playwright UI testing implementation plan"
```

---

## End-to-End Verification

After all tasks complete:

1. `cd web && bunx playwright test` — all 10 tests run, Go server starts and stops automatically
2. `web/test-results/screenshots/` contains PNG files for each modal state
3. `web/test-results/html/index.html` shows a visual test report
4. `/ui-test` in conversation runs everything and reports failures with inline screenshots

## Notes on Expected Failures

The user reports modals are not working. Likely causes to investigate after tests confirm the failures:

- **Overlay click not working:** The `#craft-modal` overlay click handler calls `closeCraftModal()` when `e.target === e.currentTarget`. If `pointer-events` CSS on `.modal-box` intercepts the click, `e.target` will never equal `e.currentTarget`. Check `pointer-events` on `.modal-box` in game.astro.
- **Quest modal close button:** The `#quest-kids-modal-close` handler was wired in the last commit of the kids-mode feature — verify it's present in `mud.ts` around line 1255.
- **Craft modal:** `closeCraftModal()` should work — it's been in the codebase longest. If it's failing, check for a JS error before the handler fires.
