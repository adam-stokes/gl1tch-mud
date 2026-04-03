import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

test.describe('kids craft -- E2E flows', () => {
  test('kids mode is active', async ({ gamePage }) => {
    await expect(gamePage.locator('body')).toHaveAttribute('data-ui', 'kids');
  });

  test('kids-craft-modal and sub-elements are in DOM', async ({ gamePage }) => {
    await expect(gamePage.locator('#kids-craft-modal')).toBeAttached();
    await expect(gamePage.locator('#kids-craft-grid')).toBeAttached();
    await expect(gamePage.locator('#kids-recipe-drawer')).toBeAttached();
    await expect(gamePage.locator('#kids-inv-picker')).toBeAttached();
  });

  test('empty grid + craft button opens recipe drawer', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Open Recipe Guide');
    await btn.click();
    await expect(gamePage.locator('#kids-recipe-drawer')).toHaveClass(/open/);
    await gamePage.screenshot({ path: ss('kids-craft-flow-empty-opens-drawer') });
  });

  test('arm item, paint cells, verify button state updates', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    await chips.first().click();
    await expect(chips.first()).toHaveClass(/armed/);
    const cells = gamePage.locator('.kids-craft-cell');
    await cells.nth(0).click();
    await cells.nth(1).click();
    await cells.nth(2).click();
    await expect(cells.nth(0)).toHaveClass(/filled/);
    await expect(cells.nth(1)).toHaveClass(/filled/);
    await expect(cells.nth(2)).toHaveClass(/filled/);
    const btn = gamePage.locator('#kids-craft-do-btn');
    const text = await btn.textContent();
    expect(text?.includes('Craft:') || text?.includes('No matching recipe')).toBe(true);
    await gamePage.screenshot({ path: ss('kids-craft-flow-painted') });
  });

  test('recipe card auto-fills grid', async ({ gamePage }) => {
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
    await gamePage.screenshot({ path: ss('kids-craft-flow-autofill') });
  });

  test('eraser mode clears painted cells', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    const chip = chips.first();
    await chip.click();
    const cell = gamePage.locator('.kids-craft-cell').first();
    await cell.click();
    await expect(cell).toHaveClass(/filled/);
    await chip.click();
    await expect(chip).toHaveClass(/eraser/);
    await cell.click();
    await expect(cell).not.toHaveClass(/filled/);
    await gamePage.screenshot({ path: ss('kids-craft-flow-eraser') });
  });

  test('happy path: auto-fill recipe, craft, modal closes, command sent', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const nonWorkbench = gamePage.locator('.kids-recipe-card:not(.needs-workbench)').first();
    const visible = await nonWorkbench.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No non-workbench recipe cards visible');
      return;
    }

    await gamePage.evaluate(() => {
      (window as any).__wsCraftCmds = [];
      const orig = WebSocket.prototype.send;
      WebSocket.prototype.send = function(data: unknown) {
        try {
          const msg = JSON.parse(data as string);
          if (msg.type === 'input' && (msg.payload?.text as string).startsWith('craft ')) {
            (window as any).__wsCraftCmds.push(msg.payload.text);
          }
        } catch { /* not JSON */ }
        return orig.call(this, data);
      };
    });

    await nonWorkbench.click();
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Craft:');
    await btn.click();
    await expect(gamePage.locator('#kids-craft-modal')).not.toHaveClass(/open/);
    const cmds: string[] = await gamePage.evaluate(() => (window as any).__wsCraftCmds ?? []);
    expect(cmds.length).toBeGreaterThan(0);
    expect(cmds[0]).toMatch(/^craft /);
    await gamePage.screenshot({ path: ss('kids-craft-flow-happy-path') });
  });
});
