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
