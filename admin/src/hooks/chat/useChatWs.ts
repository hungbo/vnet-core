import dayjs from 'dayjs';
import type { Ref } from 'vue';
import client from '@/api/client';
import { isChatOpen, chatUnreadCount, roomUnreadCounts } from '@/hooks/chat/chatState';

export function useChatWs(
  currentRoomId: Ref<string>,
  messages: Ref<any[]>,
  rooms: Ref<any[]>,
  fetchRooms: () => Promise<void>,
) {
  function mapMessage(msg: any) {
    return {
      _id: msg.id,
      content: msg.message,
      senderId: msg.sender_id,
      username: msg.sender_username
        ? `${msg.sender_type} - ${msg.sender_username}`
        : (msg.sender_type === 'admin' ? 'Admin' : 'Hội viên'),
      date: dayjs(msg.created_at).format('DD/MM/YYYY'),
      timestamp: dayjs(msg.created_at).format('HH:mm:ss'),
      createdAt: msg.created_at,
      saved: true,
      distributed: msg.status === 'delivered' || msg.status === 'read',
      seen: msg.status === 'read',
      disableActions: true,
      messageType: msg.message_type,
      senderType: msg.sender_type,
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
    if (msg.room_id === currentRoomId.value && isChatOpen.value) {
      if (!messages.value.some((m: any) => m._id === msg.id)) {
        messages.value = sortMessages([...messages.value, mapMessage(msg)]);
      }
      try { await client.put(`/chat/rooms/${currentRoomId.value}/read`); } catch {}
    } else {
      playNotificationSound();
      roomUnreadCounts.value[msg.room_id] = (roomUnreadCounts.value[msg.room_id] || 0) + 1;
      chatUnreadCount.value += 1;
      const room = rooms.value.find((r: any) => r.roomId === msg.room_id);
      if (room) room.unreadCount = (room.unreadCount || 0) + 1;
    }
  };

  const wsStatusHandler = (data: any) => {
    messages.value = messages.value.map((m: any) => {
      if (m._id === data.id) {
        return {
          ...m,
          distributed: data.status === 'delivered' || data.status === 'read',
          seen: data.status === 'read',
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
    if (data.room_id !== currentRoomId.value) return;
    messages.value = messages.value.map((m: any) => ({
      ...m,
      distributed: true,
      seen: true,
    }));
  };

  return { wsChatHandler, wsStatusHandler, wsRoomDeleted, wsRoomsCleared, wsRoomNew, wsRoomRead, sortMessages, mapMessage };
}
