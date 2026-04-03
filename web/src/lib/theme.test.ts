import { describe, it, expect, beforeEach } from 'vitest';
import { applyTheme } from './theme';
import type { WorldMeta } from './theme';

describe('applyTheme', () => {
  beforeEach(() => {
    document.documentElement.removeAttribute('style');
  });

  it('sets CSS custom properties from a full theme', () => {
    const meta: WorldMeta = {
      name: 'testworld',
      tagline: 'test',
      theme: {
        bg: '#000000',
        fg: '#ffffff',
        accent: '#ff0000',
        dim: '#333333',
        border: '#444444',
        error: '#ff5555',
        success: '#00ff00',
      },
    };
    applyTheme(meta);
    const s = document.documentElement.style;
    expect(s.getPropertyValue('--bg')).toBe('#000000');
    expect(s.getPropertyValue('--fg')).toBe('#ffffff');
    expect(s.getPropertyValue('--accent')).toBe('#ff0000');
    expect(s.getPropertyValue('--error')).toBe('#ff5555');
    expect(s.getPropertyValue('--success')).toBe('#00ff00');
  });

  it('skips empty theme values', () => {
    const meta: WorldMeta = {
      name: 'minimal',
      tagline: '',
      theme: { bg: '#111111', fg: '', accent: '', dim: '', border: '', error: '', success: '' },
    };
    applyTheme(meta);
    const s = document.documentElement.style;
    expect(s.getPropertyValue('--bg')).toBe('#111111');
    expect(s.getPropertyValue('--fg')).toBe('');
  });

  it('does not throw with empty theme object', () => {
    const meta: WorldMeta = {
      name: 'empty',
      tagline: '',
      theme: { bg: '', fg: '', accent: '', dim: '', border: '', error: '', success: '' },
    };
    expect(() => applyTheme(meta)).not.toThrow();
  });
});
