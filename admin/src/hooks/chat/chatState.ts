import { ref } from 'vue';
import client from '@/api/client';

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

/** Phòng mà khung chat phải chọn khi mở (ChatWidget đọc rồi xoá). */
export const chatRequestedRoomId = ref('');

/**
 * Mở khung chat ở đúng cuộc trò chuyện với một hội viên — nút "Nhắn tin" ở trang
 * Máy và Bảng điều khiển. Hội viên chưa có phòng nào thì tạo phòng mới, giống nút
 * "＋" trong khung chat. Khác "Thông báo" (lệnh remote "message"): thông báo chỉ
 * hiện một hộp trên màn hình máy, khách không trả lời được.
 */
export async function openChatWithMember(memberId: string) {
  const res: any = await client.get('/chat/rooms', {
    params: { participant_id: memberId, participant_type: 'member', page_size: 1 }
  });
  const found = Array.isArray(res) ? res : res?.items || [];
  let roomId: string = found[0]?.id || '';
  if (!roomId) {
    const created: any = await client.post('/chat/rooms', {
      participant_id: memberId,
      participant_type: 'member',
      title: 'Hỗ trợ'
    });
    roomId = created?.id || '';
  }
  if (!roomId) throw new Error('Không mở được cuộc trò chuyện');
  chatRequestedRoomId.value = roomId;
  isChatOpen.value = true;
}
