import { describe, it, expect, beforeEach } from 'vitest';
import { wsURL } from './ws.js';
import { setTokens } from './api.js';

beforeEach(() => setTokens({ access: 'TKN', refresh: 'R' }));

describe('wsURL', () => {
  it('includes token and params, correct scheme', () => {
    // jsdom default location is http://localhost
    const u = wsURL('/ws/traffic', { host: 'x.example' });
    expect(u).toContain('/api/v1/ws/traffic');
    expect(u).toContain('token=TKN');
    expect(u).toContain('host=x.example');
    expect(u.startsWith('ws://')).toBe(true);
  });
});
