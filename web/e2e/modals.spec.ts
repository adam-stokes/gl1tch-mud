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
