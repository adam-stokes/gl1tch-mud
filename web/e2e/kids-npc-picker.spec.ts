/**
 * E2E tests for the kids-mode NPC target picker.
 *
 * The picker appears when multiple NPCs in the room share the same capability
 * (talk/attack/trade). With a single matching NPC the command is sent directly
 * without showing the picker UI.
 *
 * State injection: addInitScript captures the WS instance so tests can dispatch
 * synthetic state.update messages without modifying world data.
 *
 * Command verification: sendCommand() echoes "> <cmd>" to #output before
 * sending over the wire, so we wait for that echo rather than intercepting
 * the WebSocket (which avoids tricky native-method-shadowing issues).
 */
import { test as base, expect, type Page } from '@playwright/test';

// ── helpers ──────────────────────────────────────────────────────────────────

function makeStateUpdate(roomNpcs: object[]) {
  return JSON.stringify({
    type: 'state.update',
    payload: {
      hp: 100, maxHp: 100,
      room_id: 'meadow-0', roomName: 'Test Room',
      exits: ['north'],
      inventory: [], credits: 0,
      room_npcs: roomNpcs,
    },
  });
}

/**
 * Fixture: gamePage with addInitScript that captures the live WS as __mudWS.
 */
const test = base.extend<{ gamePage: Page }>({
  gamePage: async ({ page }, use) => {
    await page.addInitScript(() => {
      const Orig = window.WebSocket;
      window.WebSocket = function (url: string, proto?: string | string[]) {
        const ws = new Orig(url, proto);
        (window as any).__mudWS = ws;
        return ws;
      } as typeof WebSocket;
      // Copy static constants so ws.readyState !== WebSocket.OPEN keeps working.
      (window as any).WebSocket.CONNECTING = Orig.CONNECTING;
      (window as any).WebSocket.OPEN       = Orig.OPEN;
      (window as any).WebSocket.CLOSING    = Orig.CLOSING;
      (window as any).WebSocket.CLOSED     = Orig.CLOSED;
      (window as any).WebSocket.prototype  = Orig.prototype;
    });

    await page.goto('/game?world=blockhaven');
    await page.fill('#player-name', 'tester');
    await page.click('#connect-btn');
    await page.waitForSelector('#hud-screen.active', { timeout: 10_000 });
    await page.waitForSelector('#action-grid .action-btn', { timeout: 10_000 });

    await use(page);
  },
});

/** Dispatches a synthetic state.update through the captured WS instance. */
async function injectState(page: Page, roomNpcs: object[]) {
  await page.evaluate((msg) => {
    const ws = (window as any).__mudWS as WebSocket;
    ws.dispatchEvent(new MessageEvent('message', { data: msg }));
  }, makeStateUpdate(roomNpcs));
}

/**
 * Waits for a specific command to appear in the #output element.
 * sendCommand() echoes "> <cmd>" to the terminal before sending over the wire.
 */
async function waitForCommandEcho(page: Page, cmd: string, timeout = 5_000) {
  await page.waitForFunction(
    (c: string) => (document.getElementById('output')?.textContent ?? '').includes('> ' + c),
    cmd,
    { timeout },
  );
}

// ── tests ────────────────────────────────────────────────────────────────────

test.describe('kids NPC target picker', () => {
  test('single talkable NPC — Talk sends command directly, no picker', async ({ gamePage }) => {
    await injectState(gamePage, [
      { id: 'elder-mason', name: 'Elder Mason', can_talk: true, can_trade: false, attackable: false },
    ]);
    await gamePage.waitForSelector('[data-kids-action="talk"]', { timeout: 5_000 });

    await gamePage.click('[data-kids-action="talk"]');

    // sendCommand echoes "> talk elder-mason" to #output before sending.
    await waitForCommandEcho(gamePage, 'talk elder-mason');
    await expect(gamePage.locator('#target-picker')).not.toHaveClass(/open/);
  });

  test('two talkable NPCs — Talk opens picker with both names', async ({ gamePage }) => {
    await injectState(gamePage, [
      { id: 'npc-a', name: 'NPC Alpha', can_talk: true,  can_trade: false, attackable: false },
      { id: 'npc-b', name: 'NPC Beta',  can_talk: true,  can_trade: false, attackable: false },
    ]);
    await gamePage.waitForSelector('[data-kids-action="talk"]', { timeout: 5_000 });

    await gamePage.click('[data-kids-action="talk"]');

    await expect(gamePage.locator('#target-picker')).toHaveClass(/open/);
    const btns = gamePage.locator('.target-btn');
    await expect(btns).toHaveCount(2);
    await expect(btns.nth(0)).toContainText('NPC Alpha');
    await expect(btns.nth(1)).toContainText('NPC Beta');
  });

  test('two talkable NPCs — picking second target sends correct talk command', async ({ gamePage }) => {
    await injectState(gamePage, [
      { id: 'npc-a', name: 'NPC Alpha', can_talk: true, can_trade: false, attackable: false },
      { id: 'npc-b', name: 'NPC Beta',  can_talk: true, can_trade: false, attackable: false },
    ]);
    await gamePage.waitForSelector('[data-kids-action="talk"]', { timeout: 5_000 });
    await gamePage.click('[data-kids-action="talk"]');
    await gamePage.waitForSelector('#target-picker.open', { timeout: 3_000 });

    await gamePage.locator('.target-btn', { hasText: 'NPC Beta' }).click();

    await waitForCommandEcho(gamePage, 'talk npc-b');
    await expect(gamePage.locator('#target-picker')).not.toHaveClass(/open/);
  });

  test('single attackable NPC — Attack sends command directly, no picker', async ({ gamePage }) => {
    await injectState(gamePage, [
      { id: 'stoneling-chieftain', name: 'Stoneling Chieftain', can_talk: false, can_trade: false, attackable: true },
    ]);
    await gamePage.waitForSelector('[data-kids-action="attack"]', { timeout: 5_000 });

    await gamePage.click('[data-kids-action="attack"]');

    await waitForCommandEcho(gamePage, 'attack stoneling-chieftain');
    await expect(gamePage.locator('#target-picker')).not.toHaveClass(/open/);
  });

  test('two attackable NPCs — Attack opens picker with both names', async ({ gamePage }) => {
    await injectState(gamePage, [
      { id: 'enemy-a', name: 'Rock Raider',  can_talk: false, can_trade: false, attackable: true },
      { id: 'enemy-b', name: 'Vine Creeper', can_talk: false, can_trade: false, attackable: true },
    ]);
    await gamePage.waitForSelector('[data-kids-action="attack"]', { timeout: 5_000 });

    await gamePage.click('[data-kids-action="attack"]');

    await expect(gamePage.locator('#target-picker')).toHaveClass(/open/);
    const btns = gamePage.locator('.target-btn');
    await expect(btns).toHaveCount(2);
    await expect(btns.nth(0)).toContainText('Rock Raider');
    await expect(btns.nth(1)).toContainText('Vine Creeper');
  });

  test('picker closes when clicking outside it', async ({ gamePage }) => {
    await injectState(gamePage, [
      { id: 'npc-a', name: 'NPC Alpha', can_talk: true, can_trade: false, attackable: false },
      { id: 'npc-b', name: 'NPC Beta',  can_talk: true, can_trade: false, attackable: false },
    ]);
    await gamePage.waitForSelector('[data-kids-action="talk"]', { timeout: 5_000 });
    await gamePage.click('[data-kids-action="talk"]');
    await gamePage.waitForSelector('#target-picker.open', { timeout: 3_000 });

    // Click somewhere neutral — outside picker and action-grid.
    await gamePage.click('#room-name', { force: true });

    await expect(gamePage.locator('#target-picker')).not.toHaveClass(/open/);
  });
});
