export function getWsUrl(token: string): string {
  return buildWsUrl('/api/ws/client', token);
}

/** Dựng URL WebSocket cho một đường dẫn API, kèm token qua ?token=. */
export function buildWsUrl(path: string, token: string): string {
  const baseURL = import.meta.env.VITE_SERVICE_BASE_URL || '/api';
  const isProxy = baseURL === '/api';

  if (isProxy) {
    const loc = window.location;
    const protocol = loc.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${loc.host}${path}?token=${encodeURIComponent(token)}`;
  }

  const wsBase = baseURL.replace(/^http:/, 'ws:').replace(/^https:/, 'wss:');
  return `${wsBase}${path}?token=${encodeURIComponent(token)}`;
}
