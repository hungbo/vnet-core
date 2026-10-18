import client from '@/api/client';
import { localStg } from '@/utils/storage';

// Gửi lỗi JS của trang quản trị về server (POST /error-reports) để dev thấy nó
// trong trang "Nhật ký lỗi". Trước đây lỗi chỉ nằm trong console trình duyệt
// của nhân viên — nơi không ai mở.

/** Một trang lỗi lặp trong vòng lặp không được dội hàng nghìn yêu cầu lên server. */
const MAX_PER_PAGE_LOAD = 20;
/** Cùng một lỗi ở cùng một trang: gửi một lần mỗi phút là đủ, server tự đếm. */
const DEDUPE_MS = 60_000;

let sentCount = 0;
const lastSent = new Map<string, number>();

export function reportAppError(err: unknown, context = '') {
  const message = err instanceof Error ? err.message : String(err ?? '');
  // Chưa đăng nhập thì API từ chối; lỗi ở trang đăng nhập không gửi được.
  if (!message || !localStg.get('token')) return;
  // Element Plus đo lại bảng liên tục; trình duyệt báo cảnh báo vô hại này dưới
  // dạng lỗi và nó sẽ lấp kín nhật ký.
  if (message.includes('ResizeObserver loop')) return;

  const page = window.location.pathname;
  const key = `${message}\n${page}`;
  const now = Date.now();
  if (sentCount >= MAX_PER_PAGE_LOAD || now - (lastSent.get(key) ?? 0) < DEDUPE_MS) return;
  sentCount += 1;
  lastSent.set(key, now);

  const stack = err instanceof Error && err.stack ? err.stack : '';
  client
    .post('/error-reports', {
      kind: 'error',
      message: message.slice(0, 2000),
      detail: [context, stack].filter(Boolean).join('\n').slice(0, 16000),
      location: page,
      version: BUILD_TIME
    })
    // Gửi lỗi mà lại lỗi thì bỏ qua: không được sinh thêm lỗi để báo tiếp.
    .catch(() => {});
}
