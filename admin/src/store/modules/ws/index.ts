import { ref } from 'vue';
import { defineStore } from 'pinia';
import { getWsUrl } from '@/service/ws/config';
import { localStg } from '@/utils/storage';
import { SetupStoreId } from '@/enum';

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
      } catch {
        /* ignore */
      }
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
    const delay = Math.min(1000 * 2 ** reconnectAttempt.value, maxDelay);
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

  /**
   * Gỡ đúng MỘT handler.
   *
   * `handler` là bắt buộc, không phải tuỳ chọn. Bản cũ cho phép gọi
   * `off('order:new')` trần và khi đó xoá SẠCH danh sách của sự kiện đó — ba
   * trang đang gọi kiểu này, nên chỉ cần vào rồi rời trang Đơn hàng một lần là
   * mọi handler khác của `order:new` chết theo, kể cả handler toàn cục. Bắt buộc
   * tham số để TypeScript chỉ ra đúng mọi chỗ sai thay vì hỏng lặng lẽ lúc chạy.
   */
  function off(event: string, handler: WsHandler) {
    const list = handlers.get(event);
    if (!list) return;
    const idx = list.indexOf(handler);
    if (idx >= 0) list.splice(idx, 1);
    if (list.length === 0) handlers.delete(event);
  }

  /**
   * Phát sự kiện cho mọi handler đã đăng ký.
   *
   * Mỗi handler chạy trong try/catch riêng. `forEach` trần thì một handler ném
   * lỗi là dừng luôn cả vòng lặp: những trang đăng ký sau nó im lặng không nhận
   * được gì nữa, và triệu chứng — màn hình này cập nhật, màn hình kia không —
   * trông y hệt mất kết nối chứ không giống lỗi lập trình.
   */
  function emit(event: string, data: any) {
    const list = handlers.get(event);
    if (!list) return;
    // Sao chép trước: handler có thể gọi off() ngay trong lúc chạy.
    [...list].forEach(h => {
      try {
        h(data);
      } catch (e) {
        console.error(`[ws] handler lỗi khi xử lý "${event}"`, e);
      }
    });
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
