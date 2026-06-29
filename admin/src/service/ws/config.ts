export function getWsUrl(token: string): string {
  const baseURL = import.meta.env.VITE_SERVICE_BASE_URL || '/api';
  const isProxy = baseURL === '/api';

  if (isProxy) {
    const loc = window.location;
    const protocol = loc.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${loc.host}/api/ws/client?token=${encodeURIComponent(token)}`;
  }

  const wsBase = baseURL.replace(/^http:/, 'ws:').replace(/^https:/, 'wss:');
  return `${wsBase}/api/ws/client?token=${encodeURIComponent(token)}`;
}
