import { ref } from 'vue';
import { defineStore } from 'pinia';
import { SetupStoreId } from '@/enum';
import { getWsUrl } from '@/service/ws/config';
import { localStg } from '@/utils/storage';

type WsHandler = (data: any) => void;

export const useWebSocketStore = defineStore(SetupStoreId.Ws, () => {
  const connected = ref(false);
  const connecting = ref(false);
  const reconnectAttempt = ref(0);
  const lastEvent = ref('');

  let ws: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  const handlers = new Map<string, WsHandler[]>();

  function connect() {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const token = localStg.get('token');
    if (!token) return;

    connecting.value = true;

    try {
      ws = new WebSocket(getWsUrl(token));
    } catch (e) {
      connecting.value = false;
      scheduleReconnect();
      return;
    }

    ws.onopen = () => {
      connected.value = true;
      connecting.value = false;
      reconnectAttempt.value = 0;
    };

    ws.onmessage = (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data);
        lastEvent.value = msg.type || '';
        emit(msg.type, msg.data || msg.payload);
      } catch { /* ignore */ }
    };

    ws.onclose = () => {
      connected.value = false;
      connecting.value = false;
      ws = null;
      scheduleReconnect();
    };

    ws.onerror = () => {
      connecting.value = false;
    };
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    reconnectAttempt.value = 0;
    handlers.clear();
    if (ws) {
      ws.onclose = null;
      ws.close();
      ws = null;
    }
    connected.value = false;
    connecting.value = false;
  }

  function scheduleReconnect() {
    if (reconnectTimer) return;
    const maxDelay = 60000;
    const delay = Math.min(1000 * Math.pow(2, reconnectAttempt.value), maxDelay);
    reconnectAttempt.value++;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delay);
  }

  function on(event: string, handler: WsHandler) {
    if (!handlers.has(event)) {
      handlers.set(event, []);
    }
    handlers.get(event)!.push(handler);
  }

  function off(event: string, handler?: WsHandler) {
    if (!handler) {
      handlers.delete(event);
      return;
    }
    const list = handlers.get(event);
    if (list) {
      const idx = list.indexOf(handler);
      if (idx >= 0) list.splice(idx, 1);
    }
  }

  function emit(event: string, data: any) {
    const list = handlers.get(event);
    if (list) {
      list.forEach(h => h(data));
    }
  }

  return {
    connected,
    connecting,
    reconnectAttempt,
    lastEvent,
    connect,
    disconnect,
    on,
    off
  };
});
