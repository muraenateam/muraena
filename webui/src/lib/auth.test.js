import { describe, it, expect, vi } from 'vitest';
import { get } from 'svelte/store';
import { authed, login, logout } from './auth.js';

describe('auth store', () => {
  it('logs in and sets authed', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true, status: 200, json: async () => ({ access: 'A', refresh: 'R' }),
    });
    await login('admin', 'pw');
    expect(get(authed)).toBe(true);
    logout();
    expect(get(authed)).toBe(false);
  });
});
