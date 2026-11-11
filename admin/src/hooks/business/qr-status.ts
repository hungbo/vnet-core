import { onBeforeUnmount, onMounted, ref } from 'vue';
import client from '@/api/client';
import { useRealtimeRefresh } from '@/hooks/common/realtime';

/** `GET /qr/status`: nạp QR có dùng được không và có đang đọc được giao dịch từ Hub không. */
export interface QRStatus {
  enabled: boolean;
  configured: boolean;
  source_online: boolean;
  ready: boolean;
  auto_confirm: boolean;
  bank: string;
  bank_name: string;
  account_name: string;
  last_seen: string | null;
  last_notification_at: string | null;
  expire_minutes: number;
}

/**
 * Trạng thái nạp QR, tự tải lại khi máy chủ báo "qr:changed" (vừa mất hay có lại
 * kết nối tới Hub) và mỗi 30 giây.
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
