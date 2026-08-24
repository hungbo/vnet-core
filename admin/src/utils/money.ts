/**
 * Định dạng tiền tệ dùng chung cho toàn bộ giao diện VNET.
 *
 * Trước đây sáu trang tự khai lại một bản `formatPrice` giống hệt nhau, còn
 * mười bốn chỗ khác gọi thẳng `Number.toLocaleString()` — hàm đó lấy ngôn ngữ
 * của TRÌNH DUYỆT, nên cùng một số tiền hiện `500.000` trên máy đặt tiếng Việt
 * và `500,000` trên máy đặt tiếng Anh. Quán không có cách nào biết máy nào đang
 * hiện kiểu nào.
 */

const vnd = new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' });
const plain = new Intl.NumberFormat('vi-VN');

/** Tiền có ký hiệu: `15.000 ₫`. */
export function formatPrice(value?: number | null) {
  return vnd.format(value ?? 0);
}

/** Số có phân nhóm hàng nghìn, không ký hiệu: `2.000.000`. */
export function formatAmount(value?: number | null) {
  return plain.format(value ?? 0);
}
