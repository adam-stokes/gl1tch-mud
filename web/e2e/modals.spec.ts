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

// ── Skills modal (kids mode) ──────────────────────────────────────────────────

test.describe('skills modal', () => {
  test('opens when Skills button is clicked', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="skills"]');
    await expect(gamePage.locator('#skills-kids-modal')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('skills-open') });
  });

  test('closes via close button', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="skills"]');
    await expect(gamePage.locator('#skills-kids-modal')).toHaveClass(/open/);
    await gamePage.click('#skills-kids-modal-close');
    await expect(gamePage.locator('#skills-kids-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('skills-closed-btn') });
  });

  test('closes via overlay click', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="skills"]');
    await expect(gamePage.locator('#skills-kids-modal')).toHaveClass(/open/);
    await gamePage.locator('#skills-kids-modal').click({ position: { x: 5, y: 5 } });
    await expect(gamePage.locator('#skills-kids-modal')).not.toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('skills-closed-overlay') });
  });
});

// ── Ground items (kids mode) ──────────────────────────────────────────────────

test.describe('ground items', () => {
  test('room-ground-list shows items on the floor', async ({ gamePage }) => {
    // Meadow Town Square always has a Builder's Map on the ground
    await expect(gamePage.locator('#room-ground-list')).toBeVisible();
    const rows = gamePage.locator('#room-ground-list .room-ground-row');
    await expect(rows).not.toHaveCount(0);
    await gamePage.screenshot({ path: ss('ground-items') });
  });

  test('Take button picks up the item', async ({ gamePage }) => {
    const takeBtn = gamePage.locator('#room-ground-list .kids-take-btn').first();
    const hasTakeBtn = await takeBtn.isVisible({ timeout: 5_000 }).catch(() => false);
    if (!hasTakeBtn) {
      test.skip(true, 'No takeable items on ground');
      return;
    }
    await takeBtn.click();
    // After taking, the item row should eventually disappear from the ground list
    // (state.update fires after output.done)
    await expect(gamePage.locator('#room-ground-list .kids-take-btn').first()).not.toBeVisible({ timeout: 8_000 });
    await gamePage.screenshot({ path: ss('ground-after-take') });
  });
});

// ── Quest details (kids mode) ─────────────────────────────────────────────────

test.describe('quest details', () => {
  test('quest modal shows description when quest has one', async ({ gamePage }) => {
    // Accept the starting quest by talking to Elder Mason
    await gamePage.click('[data-kids-action="talk"]');
    // Elder Mason is the only NPC who can talk in the start room — no target picker needed
    // Wait for quest to be accepted (state.update fires)
    await gamePage.waitForTimeout(2_000);
    await gamePage.click('[data-kids-action="quests"]');
    await expect(gamePage.locator('#quest-kids-modal')).toHaveClass(/open/);
    const desc = gamePage.locator('.quest-kids-desc').first();
    const hasDesc = await desc.isVisible({ timeout: 3_000 }).catch(() => false);
    if (hasDesc) {
      await expect(desc).not.toBeEmpty();
    }
    await gamePage.screenshot({ path: ss('quest-with-desc') });
  });
});
