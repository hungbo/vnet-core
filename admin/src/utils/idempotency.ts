/**
 * Sinh khoá chống-nạp-trùng cho một thao tác chuyển tiền.
 *
 * Sinh MỘT lần lúc mở hộp thoại và gửi kèm mọi lần bấm Xác nhận của lần mở đó:
 * bấm đúp, hay bấm lại vì tưởng mạng treo, đều mang cùng một khoá nên máy chủ
 * chỉ thực hiện một lần. Mở lại hộp thoại là một ý định mới, phải là khoá mới —
 * nạp hai lần cùng số tiền cho cùng khách là chuyện hợp lệ.
 */
export function newIdempotencyKey(): string {
  // crypto.randomUUID chỉ tồn tại trong secure context. Trang quản trị chạy
  // http trên IP nội bộ của quán thì không có nó, nên phải có đường lui.
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `k-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}
