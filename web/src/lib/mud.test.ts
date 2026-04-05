import { describe, it, expect } from 'vitest';
import { buildWsUrl, getWorldActions } from './mud';

describe('buildWsUrl', () => {
  it('constructs ws:// URL with world param', () => {
    const url = buildWsUrl('ws:', 'localhost:8080', 'cyberspace');
    expect(url).toBe('ws://localhost:8080/ws?world=cyberspace');
  });

  it('constructs wss:// URL for https', () => {
    const url = buildWsUrl('wss:', 'example.com', 'blockhaven');
    expect(url).toBe('wss://example.com/ws?world=blockhaven');
  });

  it('percent-encodes world names with spaces', () => {
    const url = buildWsUrl('ws:', 'localhost', 'my world');
    expect(url).toBe('ws://localhost/ws?world=my%20world');
  });
});

describe('getWorldActions', () => {
  it('returns mudout-specific actions including Scavenge and Mine', () => {
    const actions = getWorldActions('mudout');
    const labels = actions.map(a => a.label);
    expect(labels).toContain('Look');
    expect(labels).toContain('Attack');
    expect(labels).toContain('Scavenge');
    expect(labels).toContain('Mine');
    expect(labels).toContain('Craft');
  });

  it('falls back to cyberspace actions for unknown world', () => {
    const actions = getWorldActions('unknown-world');
    const labels = actions.map(a => a.label);
    expect(labels).toContain('Look');
    expect(labels).toContain('Hack');
  });

  it('returns blockhaven actions for blockhaven world', () => {
    const actions = getWorldActions('blockhaven');
    const labels = actions.map(a => a.label);
    expect(labels).toContain('Forage');
  });
});
