import { describe, it, expect } from 'vitest';
import { buildWsUrl } from './mud';

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
