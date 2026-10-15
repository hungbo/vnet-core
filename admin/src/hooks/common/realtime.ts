import { onActivated, onBeforeUnmount, onDeactivated, onMounted, watch } from 'vue';
import { useWebSocketStore } from '@/store/modules/ws';

interface RealtimeRefreshOptions {
  /** Sự kiện riêng nghe thêm, ngoài "<chủ đề>:changed" (vd. 'session:started', 'machine:status'). */
  events?: string[];
  /** Gộp các sự kiện tới trong quãng này (ms) thành một lần tải lại. */
  delay?: number;
  /** Trả về true thì hoãn tải lại (vd. đang sửa dở một biểu mẫu); tải bù khi lần sau được phép. */
  paused?: () => boolean;
}

/**
 * Tải lại dữ liệu của trang khi máy chủ báo có thay đổi.
 *
 * Backend phát "<chủ đề>:changed" sau MỌI yêu cầu ghi thành công (middleware
 * DataChanged), với chủ đề là đoạn đầu đường dẫn API: "members", "orders",
 * "systemManage", "admin-notifications"... Thay đổi không đi qua yêu cầu HTTP
 * (tác vụ hẹn giờ, nhịp tim máy trạm) thì service phát qua hub.Changed hoặc
 * một sự kiện riêng.
 *
 * Trang khai báo MỌI chủ đề có thể đổi dữ liệu nó đang hiện, kể cả gián tiếp:
 * nạp tiền ở /members đổi cả trang Giao dịch; mở phiên ở /sessions đổi cả trạng
 * thái trên trang Máy trạm.
 *
 * - Gộp: nhiều sự kiện trong `delay` ms thành một lần tải lại.
 * - Tab đang ẩn, hoặc trang đang nằm trong KeepAlive mà không hiện, thì không
 *   tải; tải một lần khi hiện lại.
 * - Mất kết nối WebSocket là có thể đã lỡ sự kiện: nối lại xong thì tải lại.
 * - Khôi phục bản sao lưu (POST /backups/:id/restore) thì trang nào cũng tải lại.
 */
export function useRealtimeRefresh(topics: string[], reload: () => unknown, options: RealtimeRefreshOptions = {}) {
  const wsStore = useWebSocketStore();
  const names = [...topics.map(t => `${t}:changed`), ...(options.events ?? [])];
  const delay = options.delay ?? 500;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let pending = false;
  let active = true;

  function run() {
    timer = null;
    if (!active || document.hidden || options.paused?.()) {
      pending = true;
      return;
    }
    pending = false;
    reload();
  }

  function schedule() {
    if (timer) return;
    timer = setTimeout(run, delay);
  }

  function onVisible() {
    if (!document.hidden && pending) schedule();
  }

  // Nối lại sau khi rớt mạng thì có thể đã lỡ sự kiện: tải lại. Trừ lần nối ĐẦU
  // ngay sau khi tải trang — trang vừa tự tải dữ liệu lúc mount, tải thêm là thừa.
  let mountedAt = 0;
  watch(
    () => wsStore.connected,
    (now, before) => {
      if (now && before === false && Date.now() - mountedAt > 3000) schedule();
    }
  );

  // Khôi phục bản sao lưu thay CẢ cơ sở dữ liệu: mọi trang đang mở đều phải tải lại.
  function onBackup(data: any) {
    if (typeof data?.route === 'string' && data.route.endsWith('/restore')) schedule();
  }

  onMounted(() => {
    mountedAt = Date.now();
    names.forEach(n => wsStore.on(n, schedule));
    if (!names.includes('backups:changed')) wsStore.on('backups:changed', onBackup);
    document.addEventListener('visibilitychange', onVisible);
  });

  onActivated(() => {
    active = true;
    if (pending) schedule();
  });

  onDeactivated(() => {
    active = false;
  });

  onBeforeUnmount(() => {
    names.forEach(n => wsStore.off(n, schedule));
    if (!names.includes('backups:changed')) wsStore.off('backups:changed', onBackup);
    document.removeEventListener('visibilitychange', onVisible);
    if (timer) clearTimeout(timer);
  });

  /** Gọi khi điều kiện `paused` vừa hết (vd. đóng biểu mẫu) để tải bù nếu đã lỡ. */
  function resume() {
    if (pending) schedule();
  }

  return { schedule, resume };
}
