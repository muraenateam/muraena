import { writable } from 'svelte/store';
import { api, setTokens, clearTokens } from './api.js';

export const authed = writable(false);

export async function login(username, password) {
  const t = await api('/auth/login', { method: 'POST', body: { username, password } });
  setTokens({ access: t.access, refresh: t.refresh });
  authed.set(true);
  return t;
}

export function logout() {
  clearTokens();
  authed.set(false);
}

export async function me() { return api('/me'); }
