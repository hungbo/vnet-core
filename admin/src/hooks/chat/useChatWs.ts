import type { Ref } from 'vue';
import { ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import client from '@/api/client';
import { useAuthStore } from '@/store/modules/auth';
import { chatUnreadCount, isChatOpen } from '@/hooks/chat/chatState';
import { $t } from '@/locales';

/** Tên hiển thị của người gửi. Dùng chung cho khung chat và thông báo nổi. */
function senderLabel(msg: any) {
  if (msg.sender_username) return `${msg.sender_type} - ${msg.sender_username}`;
  return msg.sender_type === 'admin' ? 'Admin' : 'Hội viên';
}

export function useChatWs(
  currentRoomId: Ref<string>,
  messages: Ref<any[]>,
  rooms: Ref<any[]>,
  fetchRooms: () => Promise<void>
) {
  // Lấy thẳng từ store thay vì nhận thêm một tham số: hàm này đã chạm trần
  // max-params, và id người đang đăng nhập thì chỗ nào cũng lấy được như nhau.
  const authStore = useAuthStore();
  /** Tính lại con số trên chuông từ danh sách phòng — nguồn duy nhất là máy chủ. */
  function demLaiChuaDoc() {
    chatUnreadCount.value = rooms.value.reduce((sum: number, r: any) => sum + (r.unreadCount || 0), 0);
  }
  function mapMessage(msg: any) {
    return {
      _id: msg.id,
      content: msg.message,
      senderId: msg.sender_id,
      username: senderLabel(msg),
      date: dayjs(msg.created_at).format('DD/MM/YYYY'),
      timestamp: dayjs(msg.created_at).format('HH:mm:ss'),
      createdAt: msg.created_at,
      saved: true,
      distributed: msg.status === 'delivered' || msg.status === 'read',
      seen: msg.status === 'read',
      disableActions: true,
      messageType: msg.message_type,
      senderType: msg.sender_type
    };
  }

  function sortMessages(msgs: any[]) {
    return msgs.sort((a: any, b: any) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());
  }

  function playNotificationSound() {
    try {
      const audio = new Audio('/audio/mesage.mp3');
      audio.volume = 0.5;
      audio.play().catch(() => {});
    } catch {
      // ignore
    }
  }

  const wsChatHandler = async (msg: any) => {
    // Tin do CHÍNH tài khoản này gửi, vọng về từ một thiết bị khác của mình.
    //
    // Máy chủ không còn bỏ qua người gửi khi phát tin (bỏ qua thì lọc theo tài
    // khoản, tức là bịt luôn mọi thiết bị khác của người đó). Nên ở đây phải tự
    // phân biệt: chỉ chèn vào khung chat, không kêu chuông và không cộng số
    // chưa-đọc cho câu do chính mình vừa gõ ở máy bên cạnh.
    const laTinCuaMinh = msg.sender_type === 'admin' && msg.sender_id === authStore.userInfo?.id;

    if (msg.room_id === currentRoomId.value && isChatOpen.value) {
      if (!messages.value.some((m: any) => m._id === msg.id)) {
        messages.value = sortMessages([...messages.value, mapMessage(msg)]);
      }
      if (laTinCuaMinh) return;
      try {
        await client.put(`/chat/rooms/${currentRoomId.value}/read`);
      } catch {}
      return;
    }

    // Phòng không đang mở. Tin của chính mình thì không có gì để báo.
    if (laTinCuaMinh) return;

    {
      playNotificationSound();
      const room = rooms.value.find((r: any) => r.roomId === msg.room_id);
      if (room) {
        room.unreadCount = (room.unreadCount || 0) + 1;
        demLaiChuaDoc();
      } else {
        // Phòng chưa có trong danh sách (vừa được tạo): lấy lại từ máy chủ thay
        // vì tự đoán một con số.
        fetchRooms();
      }

      // Tiếng chuông và con số trên icon không nói được AI nhắn và nhắn GÌ, nên
      // nhân viên phải mở khung chat mới biết có đáng bỏ việc đang làm không.
      const body = String(msg.message ?? '');
      const notification = ElNotification({
        title: $t('vnetPages.realtime.chatTitle', { sender: senderLabel(msg) }),
        message: body.length > 60 ? `${body.slice(0, 60)}…` : body,
        type: 'info',
        duration: 6000,
        onClick: () => {
          // ElNotification không tự đóng khi bấm.
          notification.close();
          // Gán phòng TRƯỚC khi mở: watch(isChatOpen) trong ChatWidget chỉ nhảy
          // về rooms[0] khi currentRoomId còn rỗng.
          currentRoomId.value = msg.room_id;
          isChatOpen.value = true;
        }
      });
    }
  };

  const wsStatusHandler = (data: any) => {
    messages.value = messages.value.map((m: any) => {
      if (m._id === data.id) {
        return {
          ...m,
          distributed: data.status === 'delivered' || data.status === 'read',
          seen: data.status === 'read'
        };
      }
      return m;
    });
  };

  const wsRoomDeleted = (data: any) => {
    if (data.room_id === currentRoomId.value) {
      currentRoomId.value = '';
      messages.value = [];
    }
    fetchRooms();
  };

  const wsRoomsCleared = () => {
    currentRoomId.value = '';
    messages.value = [];
    fetchRooms();
  };

  const wsRoomNew = () => {
    fetchRooms();
  };

  const wsRoomRead = (data: any) => {
    // Nhân viên nào đó đã đọc phòng này — có thể là chính người đang ngồi đây,
    // trên điện thoại. Trạng thái đã-đọc phía nhân viên là chung cho cả phòng,
    // nên mọi thiết bị nhân viên phải tắt con số cùng lúc.
    //
    // Khách đọc thì KHÔNG đụng tới: nó chỉ nói tin của nhân viên đã tới mắt
    // khách, không nói nhân viên đã xem tin của khách.
    if (data.reader_type === 'admin') {
      const room = rooms.value.find((r: any) => r.roomId === data.room_id);
      if (room) {
        room.unreadCount = 0;
        demLaiChuaDoc();
      }
    }

    if (data.room_id !== currentRoomId.value) return;
    messages.value = messages.value.map((m: any) => ({
      ...m,
      distributed: true,
      seen: true
    }));
  };

  return {
    wsChatHandler,
    wsStatusHandler,
    wsRoomDeleted,
    wsRoomsCleared,
    wsRoomNew,
    wsRoomRead,
    sortMessages,
    mapMessage
  };
}
