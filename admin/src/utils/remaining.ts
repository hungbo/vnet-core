/**
 * Thời gian số dư còn mua được, dạng "1g 20p".
 *
 * Máy chủ tính sẵn mốc hết tiền lên chính dòng phiên (`affordable_until`), nên ở
 * đây chỉ việc lấy hiệu — không trang nào phải nạp đơn giá và số dư của từng hội
 * viên rồi tự nhân chia.
 *
 * Trả về '—' khi không có giới hạn: máy chưa gán nhóm nên chưa có giá, hoặc
 * khách đang dùng gói khung giờ đã trả tiền trọn khung.
 */
export function formatRemaining(affordableUntil?: string | null, now: number = Date.now()) {
  if (!affordableUntil) return '—';
  const left = Math.floor((new Date(affordableUntil).getTime() - now) / 60000);
  if (left <= 0) return 'Hết';
  const h = Math.floor(left / 60);
  const m = left % 60;
  return h > 0 ? `${h}g ${m}p` : `${m}p`;
}
