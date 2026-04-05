import { test, expect } from '@playwright/test';

// Assumes: test server with ui_profile=kids, recipe "pipe-pistol" (assembly, required: frame+barrel),
// player inventory contains pipe-frame-crude (gun-frame) and copper-tube-crude (gun-barrel).

test.describe('Kids Assembly Modal', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('[data-ui="kids"]');
  });

  test('assembly modal opens for assembly recipe', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');

    await expect(page.locator('#kids-assembly-modal')).toHaveClass(/open/);
    await expect(page.locator('#kids-craft-modal')).not.toHaveClass(/open/);
  });

  test('required slots show red outline initially', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await expect(page.locator('.kids-assembly-slot-row[data-slot-id="frame"]')).toHaveClass(/required-empty/);
    await expect(page.locator('.kids-assembly-slot-row[data-slot-id="barrel"]')).toHaveClass(/required-empty/);
  });

  test('FORGE IT button is disabled until required slots filled', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await expect(page.locator('#kids-assembly-forge-btn')).toBeDisabled();
  });

  test('filling slots enables FORGE IT and shows stat bars', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    // Fill frame
    await page.click('.kids-assembly-slot-row[data-slot-id="frame"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    // Fill barrel
    await page.click('.kids-assembly-slot-row[data-slot-id="barrel"]');
    await page.waitForSelector('#kids-assembly-inv-picker:not([hidden])');
    await page.locator('.kids-assembly-inv-item').first().click();

    await expect(page.locator('#kids-assembly-forge-btn')).toBeEnabled();
    await expect(page.locator('.kids-stat-bar-row')).toHaveCount({ min: 1 });
  });

  test('FORGE IT sends craft command with slot args and closes modal', async ({ page }) => {
    const sentMessages: string[] = [];
    page.on('websocket', ws => {
      ws.on('framesent', frame => {
        if (typeof frame.payload === 'string') sentMessages.push(frame.payload);
      });
    });

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

    await expect(page.locator('#kids-assembly-modal')).not.toHaveClass(/open/);

    const craftMsg = sentMessages.find(m => m.includes('craft pipe-pistol') && m.includes('frame=') && m.includes('barrel='));
    expect(craftMsg).toBeTruthy();
  });

  test('close button dismisses modal', async ({ page }) => {
    await page.click('.action-btn[data-special="craft"]');
    await page.waitForSelector('#kids-recipe-drawer.open');
    await page.click('[data-recipe-id="pipe-pistol"]');
    await page.waitForSelector('#kids-assembly-modal.open');

    await page.click('#kids-assembly-close');
    await expect(page.locator('#kids-assembly-modal')).not.toHaveClass(/open/);
  });
});
