/** Khoảng thời gian dạng "11:00:00" (giờ có thể quá 24). */
export function formatHMS(ms: number) {
  const total = Math.max(0, Math.floor(ms / 1000));
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(Math.floor(total / 3600))}:${p(Math.floor((total % 3600) / 60))}:${p(total % 60)}`;
}

/** Thời gian đã chơi: tới lúc kết thúc nếu phiên đã đóng, không thì tới bây giờ. */
export function formatPlayed(startedAt: string, endedAt?: string | null, now: number = Date.now()) {
  const end = endedAt ? new Date(endedAt).getTime() : now;
  return formatHMS(end - new Date(startedAt).getTime());
}

/**
 * Thời gian số dư còn mua được, dạng "11:00:00".
 *
 * Máy chủ tính sẵn mốc hết tiền lên chính dòng phiên (`affordable_until`), nên ở
 * đây chỉ việc lấy hiệu — không trang nào phải nạp đơn giá và số dư của từng hội
 * viên rồi tự nhân chia.
 *
 * Trả về '—' khi không có giới hạn: máy chưa gán nhóm nên chưa có giá, hoặc
 * khách đang dùng gói khung giờ đã trả tiền trọn khung.
 */
export function formatRemainingHMS(affordableUntil?: string | null, now: number = Date.now()) {
  if (!affordableUntil) return '—';
  const left = new Date(affordableUntil).getTime() - now;
  return left <= 0 ? 'Hết' : formatHMS(left);
}
