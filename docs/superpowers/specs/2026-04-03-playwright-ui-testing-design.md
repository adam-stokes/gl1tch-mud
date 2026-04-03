# Playwright UI Testing Design

## Goal

Add a Playwright E2E test suite focused on modal behaviour, plus a Claude skill (`/ui-test`) that runs the tests, reads failure screenshots, and reports plain-English results.

## Architecture

```
web/
  e2e/
    modals.spec.ts          — modal open/close tests (item, craft, quest)
    helpers/
      auth.ts               — shared login fixture
  playwright.config.ts      — webServer + browser config

.claude/
  skills/
    ui-test.md              — Claude skill: run tests, interpret results
```

## Tech Stack

- **Playwright** (`@playwright/test`) — browser automation and screenshot capture
- **Go server** — started by Playwright `webServer` config on port 8091 before tests run
- **World** — blockhaven (sends `ui_profile: "kids"`, has all three modals)
- **Reporter** — JSON (`--reporter=json`) for machine-readable results + HTML for human browsing

---

## Playwright Config (`web/playwright.config.ts`)

- `webServer.command`: `go run . -serve -world blockhaven -port 8091`
- `webServer.cwd`: `../` (repo root, relative to `web/` — where `main.go` lives)
- `webServer.url`: `http://localhost:8091`
- `webServer.reuseExistingServer`: `false` (always fresh)
- `testDir`: `./e2e`
- `outputDir`: `./test-results`
- Browser: Chromium only (sufficient for modal testing)
- Screenshots: `on` (captured on every step, not just failure)
- Screenshot dir: `./test-results/screenshots`

---

## Auth Fixture (`web/e2e/helpers/auth.ts`)

A Playwright `test.extend` fixture named `gamePage` that:

1. Navigates to `http://localhost:8091/game?world=blockhaven`
2. Fills `#player-name` with a test handle (e.g. `tester`)
3. Leaves `#passphrase` blank (server has no passphrase in test mode)
4. Clicks `#connect-btn`
5. Waits for `#hud-screen` to have class `active`
6. Waits for `output.done` signal (input enabled) — detected by `#cmd-input` not being disabled
7. Returns the `page` object ready for game interaction

---

## Test Scope (`web/e2e/modals.spec.ts`)

Three test groups, all using the `gamePage` fixture.

### Group 1: Item Modal

**Prerequisite:** Player must have an item in inventory. The `look` command is sent after login; if `#inv-grid` has no `.occupied` slots, the test is skipped with a note.

**Tests:**
- `item modal opens when inventory item is clicked` — click first `.inv-slot.occupied`, assert `#item-modal` has class `open`, take screenshot
- `item modal closes via close button` — click `#item-modal-close`, assert `open` class removed, take screenshot
- `item modal closes via overlay click` — reopen modal, click the overlay (the `#item-modal` element itself, not `.modal-box`), assert `open` class removed

### Group 2: Craft Modal

**Prerequisite:** None — Craft button always present in blockhaven.

**Tests:**
- `craft modal opens when Craft button is clicked` — click `.action-btn[data-kids-action="craft"]`, assert `#craft-modal` has class `open`, take screenshot
- `craft modal closes via close button` — click `#craft-modal-close`, assert `open` class removed, take screenshot
- `craft modal closes via overlay click` — reopen modal, click `#craft-modal` overlay, assert `open` removed

### Group 3: Quest Modal (kids mode)

**Prerequisite:** `data-ui="kids"` on `<body>` — confirmed by blockhaven's `ui_profile`. Quests button present via kids action grid.

**Tests:**
- `quest modal opens when Quests button is clicked` — click `.action-btn[data-kids-action="quests"]`, assert `#quest-kids-modal` has class `open`, take screenshot
- `quest modal closes via close button` — click `#quest-kids-modal-close`, assert `open` removed, take screenshot
- `quest modal closes via overlay click` — reopen modal, click `#quest-kids-modal` overlay, assert `open` removed

---

## Screenshot Strategy

- Every test that touches a modal takes two screenshots: one at open state, one at closed state
- Named: `{test-group}-{test-name}-{open|closed}.png`
- Stored in `web/test-results/screenshots/`
- On failure, Playwright also auto-captures a `failure-{test-name}.png`

---

## Claude Skill (`/ui-test`)

**File:** `.claude/skills/ui-test.md`

**Invocation:** `/ui-test` in any conversation

**Behaviour:**

1. Run `cd /path/to/repo/web && npx playwright test --reporter=json 2>&1`
2. Read `web/test-results/results.json`
3. For each failed test: read the failure screenshot(s) using the Read tool and display them inline
4. Report summary:
   - Pass/fail counts
   - For failures: test name, assertion that failed, screenshot at point of failure, likely cause
5. Does not auto-fix — diagnoses only

**Example output format:**
```
3/4 tests passed.

FAILED: craft modal closes on overlay click
  Assertion: expected #craft-modal to not have class "open"
  Screenshot: [inline image]
  Likely cause: click event landed on .modal-box child rather than overlay element.
  Suggestion: use page.locator('#craft-modal').click({ position: { x: 5, y: 5 } })
  to force click on overlay edge.
```

---

## npm Script

Add to `web/package.json`:
```json
"test:e2e": "playwright test",
"test:e2e:ui": "playwright test --ui"
```

---

## Out of Scope

- Testing navigation (compass buttons, exit buttons) — covered by manual play
- Chat functionality — not modal-related
- Mobile/Safari browsers — Chromium only for now
- Per-test server reset / world state seeding — can be added later if flakiness appears
