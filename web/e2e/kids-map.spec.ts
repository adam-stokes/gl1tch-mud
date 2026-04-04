import { test, expect } from './helpers/auth';

test.describe('kids map', () => {
  test('map panel is present in kids mode', async ({ gamePage }) => {
    await expect(gamePage.locator('#kids-map-panel')).toBeAttached();
    await expect(gamePage.locator('#kids-map-grid')).toBeAttached();
  });

  test('map panel is hidden when data-ui is not kids', async ({ gamePage }) => {
    // Directly test the CSS gate: removing data-ui="kids" should hide the panel
    await gamePage.evaluate(() => document.body.removeAttribute('data-ui'));
    await expect(gamePage.locator('#kids-map-panel')).toHaveCSS('display', 'none');
  });

  test('map grid has at least one room cell after state.update', async ({ gamePage }) => {
    // Wait for map cells to be rendered (they appear after state.update)
    await gamePage.waitForSelector('.kids-map-cell', { timeout: 10_000 });
    const cells = gamePage.locator('.kids-map-cell');
    expect(await cells.count()).toBeGreaterThan(0);
  });

  test('room cells have biome data attributes', async ({ gamePage }) => {
    await gamePage.waitForSelector('.kids-map-cell', { timeout: 10_000 });
    const firstCell = gamePage.locator('.kids-map-cell').first();
    const biome = await firstCell.getAttribute('data-biome');
    expect(biome).not.toBeNull();
  });

  test('current room cell has star marker', async ({ gamePage }) => {
    await gamePage.waitForSelector('.kids-map-cell.current-room', { timeout: 10_000 });
    const star = gamePage.locator('.kids-map-cell.current-room .kids-map-star');
    await expect(star).toBeAttached();
    await expect(star).toContainText('★');
  });

  test('zoom buttons toggle data-map-zoom attribute', async ({ gamePage }) => {
    await gamePage.waitForSelector('.kids-map-zoom-btn', { timeout: 10_000 });
    const grid = gamePage.locator('#kids-map-grid');

    // Default is world
    await expect(grid).toHaveAttribute('data-map-zoom', 'world');

    // Click area zoom
    await gamePage.click('[data-zoom="area"]');
    await expect(grid).toHaveAttribute('data-map-zoom', 'area');

    // Click room zoom
    await gamePage.click('[data-zoom="room"]');
    await expect(grid).toHaveAttribute('data-map-zoom', 'room');

    // Back to world
    await gamePage.click('[data-zoom="world"]');
    await expect(grid).toHaveAttribute('data-map-zoom', 'world');
  });

  test('active zoom button has active class', async ({ gamePage }) => {
    await gamePage.waitForSelector('.kids-map-zoom-btn', { timeout: 10_000 });

    await gamePage.click('[data-zoom="area"]');
    await expect(gamePage.locator('[data-zoom="area"]')).toHaveClass(/active/);
    await expect(gamePage.locator('[data-zoom="world"]')).not.toHaveClass(/active/);
  });
});
