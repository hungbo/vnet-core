import { ref } from 'vue';

export const isChatOpen = ref(false);

/**
 * Tổng số tin chưa đọc hiện trên chuông.
 *
 * Con số này luôn được TÍNH LẠI từ `unreadCount` của từng phòng, mà `unreadCount`
 * lấy từ máy chủ. Bản cũ còn giữ thêm một bảng đếm cục bộ `roomUnreadCounts` rồi
 * CỘNG vào số của máy chủ, nên mỗi tin nhắn tới được đếm hai lần; con số đó cũng
 * mất sạch khi tải lại trang và không có cách nào khớp với thiết bị thứ hai của
 * cùng một người.
 */
export const chatUnreadCount = ref(0);

export function toggleChat() {
  isChatOpen.value = !isChatOpen.value;
}
