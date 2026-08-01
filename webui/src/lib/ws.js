import { getAccess } from './api.js';

export function wsURL(path, params = {}) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const qs = new URLSearchParams({ token: getAccess() ?? '', ...params });
  return `${proto}://${location.host}/api/v1${path}?${qs.toString()}`;
}

export function connect(path, params, onMessage) {
  const sock = new WebSocket(wsURL(path, params));
  sock.onmessage = (ev) => {
    try { onMessage(JSON.parse(ev.data)); } catch (_) {}
  };
  return sock;
}
