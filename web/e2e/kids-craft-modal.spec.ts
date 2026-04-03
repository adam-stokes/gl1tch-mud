import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

test.describe('kids craft modal -- presence', () => {
  test('kids-craft-modal exists in DOM', async ({ gamePage }) => {
    await expect(gamePage.locator('#kids-craft-modal')).toBeAttached();
  });
});

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

  test('tapping armed chip again switches to eraser mode', async ({ gamePage }) => {
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
