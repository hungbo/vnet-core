import { onBeforeUnmount, onMounted, ref } from 'vue';
import client from '@/api/client';
import { useRealtimeRefresh } from '@/hooks/common/realtime';

/** `GET /qr/status`: nạp QR có dùng được không và điện thoại báo tiền ra sao. */
export interface QRStatus {
  enabled: boolean;
  configured: boolean;
  paired: boolean;
  phone_online: boolean;
  ready: boolean;
  auto_confirm: boolean;
  bank: string;
  bank_name: string;
  account_name: string;
  last_seen: string | null;
  last_notification_at: string | null;
  app_version: string;
  queue: number;
  battery: number;
  charging: boolean;
  listener_ok: boolean;
  expire_minutes: number;
  /** Điện thoại đang giữ WebSocket (app 0.4.0+); không thì chỉ có nhịp HTTP. */
  ws_connected: boolean;
}

/**
 * Trạng thái nạp QR, tự tải lại khi máy chủ báo "qr:changed" (ghép/gỡ, điện
 * thoại vừa mất hay có lại kết nối) và mỗi 30 giây để "lần cuối thấy" không đứng
 * yên.
 */
export function useQRStatus() {
  const status = ref<QRStatus | null>(null);

  async function load() {
    try {
      status.value = (await client.get('/qr/status')) as any;
    } catch {
      status.value = null;
    }
  }

  useRealtimeRefresh(['qr', 'bank-notifications', 'settings'], load);

  let timer: ReturnType<typeof setInterval> | null = null;
  onMounted(() => {
    load();
    timer = setInterval(load, 30_000);
  });
  onBeforeUnmount(() => {
    if (timer) clearInterval(timer);
  });

  return { status, reload: load };
}
