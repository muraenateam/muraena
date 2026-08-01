import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api, setTokens, getAccess } from './api.js';

beforeEach(() => setTokens({ access: 'A', refresh: 'R' }));

describe('api client', () => {
  it('attaches bearer token', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true, status: 200, json: async () => ({ ok: true }),
    });
    await api('/me');
    const [, opts] = globalThis.fetch.mock.calls[0];
    expect(opts.headers.Authorization).toBe('Bearer A');
  });

  it('refreshes once on 401 then retries', async () => {
    const calls = [];
    globalThis.fetch = vi.fn().mockImplementation((url, opts) => {
      calls.push(url);
      if (url === '/api/v1/me' && opts.headers.Authorization === 'Bearer A') {
        return Promise.resolve({ ok: false, status: 401, json: async () => ({}) });
      }
      if (url === '/api/v1/auth/refresh') {
        return Promise.resolve({ ok: true, status: 200, json: async () => ({ access: 'A2', refresh: 'R2' }) });
      }
      // retry with new token
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ ok: true }) });
    });

    const res = await api('/me');
    expect(res.ok).toBe(true);
    expect(getAccess()).toBe('A2');
    expect(calls).toContain('/api/v1/auth/refresh');
  });
});
