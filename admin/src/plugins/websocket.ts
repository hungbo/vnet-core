import { useWebSocketStore } from '@/store/modules/ws';

export function setupWebSocket() {
  const wsStore = useWebSocketStore();
  wsStore.connect();
}
