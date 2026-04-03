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
