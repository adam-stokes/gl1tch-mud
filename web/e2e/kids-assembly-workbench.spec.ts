import { test, expect } from '@playwright/test';

// Assumes: player starts outside a ruins-workshop room (no scrap-forge present).

test.describe('Kids Assembly Workbench Gate', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('[data-ui="kids"]');
  });

  test('server rejects forge attempt when not at scrap-forge', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await page.click('.kids-assembly-slot-row[data-slot-id="frame"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await page.click('.kids-assembly-slot-row[data-slot-id="barrel"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await page.click('#kids-assembly-forge-btn');

    await expect(page.locator('#output')).toContainText('requires a scrap-forge', { timeout: 3000 });
  });
});
