import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

test.describe('kids craft -- paint mechanic', () => {
  test('grid renders 9 cells when modal opens', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await expect(gamePage.locator('#kids-craft-modal')).toHaveClass(/open/);
    await expect(gamePage.locator('.kids-craft-cell')).toHaveCount(9);
    await gamePage.screenshot({ path: ss('kids-craft-grid-empty') });
  });

  test('craft button shows "Open Recipe Guide" when grid is empty', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Open Recipe Guide');
    await expect(btn).not.toBeDisabled();
  });

  test('clicking armed item into empty cell marks cell as filled', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items to paint with');
      return;
    }
    await chips.first().click();
    const cell = gamePage.locator('.kids-craft-cell').first();
    await cell.click();
    await expect(cell).toHaveClass(/filled/);
    await expect(cell).not.toContainText('+');
    await gamePage.screenshot({ path: ss('kids-craft-cell-filled') });
  });

  test('right-click on filled cell clears it', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    await chips.first().click();
    const cell = gamePage.locator('.kids-craft-cell').first();
    await cell.click();
    await expect(cell).toHaveClass(/filled/);
    await cell.click({ button: 'right' });
    await expect(cell).not.toHaveClass(/filled/);
    await gamePage.screenshot({ path: ss('kids-craft-cell-cleared') });
  });

  test('craft button state reflects grid contents', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    const chips = gamePage.locator('.kids-inv-chip');
    if (await chips.count() === 0) {
      test.skip(true, 'No inventory items');
      return;
    }
    await chips.first().click();
    await gamePage.locator('.kids-craft-cell').first().click();
    const btn = gamePage.locator('#kids-craft-do-btn');
    const text = await btn.textContent();
    expect(
      text?.includes('Craft:') || text?.includes('No matching recipe')
    ).toBe(true);
  });
});

test.describe('kids craft -- workbench guidance', () => {
  test('workbench recipe shows message and does not send craft command', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const workbenchCard = gamePage.locator('.kids-recipe-card.needs-workbench').first();
    const visible = await workbenchCard.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No workbench recipe cards visible');
      return;
    }
    await workbenchCard.click();
    const btn = gamePage.locator('#kids-craft-do-btn');
    await expect(btn).toContainText('Craft:');

    await gamePage.evaluate(() => {
      (window as any).__wsCraftCmds = [];
      const orig = WebSocket.prototype.send;
      WebSocket.prototype.send = function(data: unknown) {
        try {
          const msg = JSON.parse(data as string);
          if (msg.type === 'input' && typeof msg.payload?.text === 'string') {
            (window as any).__wsCraftCmds.push(msg.payload.text);
          }
        } catch { /* not JSON */ }
        return orig.call(this, data);
      };
    });

    await btn.click();
    await expect(gamePage.locator('#kids-workbench-msg')).not.toHaveAttribute('hidden', '');
    await expect(gamePage.locator('#kids-workbench-msg')).toContainText('\u{1F528}');

    const cmds: string[] = await gamePage.evaluate(() => (window as any).__wsCraftCmds ?? []);
    expect(cmds.filter((c: string) => c.startsWith('craft '))).toHaveLength(0);
    await gamePage.screenshot({ path: ss('kids-craft-workbench-msg') });
  });

  test('tapping workbench message dismisses it', async ({ gamePage }) => {
    await gamePage.click('[data-kids-action="craft"]');
    await gamePage.click('#kids-recipe-btn');
    const workbenchCard = gamePage.locator('.kids-recipe-card.needs-workbench').first();
    const visible = await workbenchCard.isVisible({ timeout: 3_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, 'No workbench recipe cards visible');
      return;
    }
    await workbenchCard.click();
    await gamePage.locator('#kids-craft-do-btn').click();
    await expect(gamePage.locator('#kids-workbench-msg')).not.toHaveAttribute('hidden', '');
    await gamePage.locator('#kids-workbench-msg').click();
    await expect(gamePage.locator('#kids-workbench-msg')).toHaveAttribute('hidden', '');
  });
});
