import { test, expect } from './helpers/auth';
import path from 'path';

const ss = (name: string) =>
  path.join('test-results', 'screenshots', `${name}.png`);

test.describe('kids craft modal -- presence', () => {
  test('kids-craft-modal exists in DOM', async ({ gamePage }) => {
    await expect(gamePage.locator('#kids-craft-modal')).toBeAttached();
  });
});
