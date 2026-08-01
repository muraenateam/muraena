const BASE = '/api/v1';
let accessToken = null;
let refreshToken = null;

export function setTokens({ access, refresh }) {
  accessToken = access ?? accessToken;
  refreshToken = refresh ?? refreshToken;
}
export function getAccess() { return accessToken; }
export function clearTokens() { accessToken = null; refreshToken = null; }

export class ApiError extends Error {
  constructor(status, message) { super(message); this.status = status; }
}

async function doFetch(path, { method = 'GET', body } = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (accessToken) headers.Authorization = 'Bearer ' + accessToken;
  return fetch(BASE + path, {
    method, headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

async function refresh() {
  const res = await fetch(BASE + '/auth/refresh', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh: refreshToken }),
  });
  if (!res.ok) throw new ApiError(res.status, 'refresh failed');
  const t = await res.json();
  setTokens({ access: t.access, refresh: t.refresh });
}

export async function api(path, opts = {}) {
  let res = await doFetch(path, opts);
  if (res.status === 401 && refreshToken) {
    await refresh();          // refresh once
    res = await doFetch(path, opts); // retry
  }
  if (!res.ok) {
    let msg = res.statusText;
    try { msg = (await res.json())?.error?.message ?? msg; } catch (_) {}
    throw new ApiError(res.status, msg);
  }
  return res.json();
}
