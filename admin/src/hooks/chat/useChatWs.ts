import dayjs from 'dayjs';
import type { Ref } from 'vue';
import { isChatOpen } from '@/hooks/chat/chatState';

export function useChatWs(
  currentRoomId: Ref<string>,
  messages: Ref<any[]>,
  fetchRooms: () => Promise<void>,
) {
  function mapMessage(msg: any) {
    return {
      _id: msg.id,
      content: msg.message,
      senderId: msg.sender_id,
      username: msg.sender_type === 'admin' ? 'Admin' : 'Hội viên',
      date: dayjs(msg.created_at).format('DD/MM/YYYY'),
      timestamp: dayjs(msg.created_at).format('HH:mm:ss'),
      createdAt: msg.created_at,
      saved: true,
      distributed: msg.status === 'delivered' || msg.status === 'read',
      seen: msg.status === 'read',
      disableActions: false,
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

  const wsChatHandler = (msg: any) => {
    if (msg.conversation_id === currentRoomId.value && isChatOpen.value) {
      if (!messages.value.some((m: any) => m._id === msg.id)) {
        messages.value = sortMessages([...messages.value, mapMessage(msg)]);
      }
    } else {
      playNotificationSound();
    }
    fetchRooms();
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

  const wsConversationDeleted = (data: any) => {
    if (data.conversation_id === currentRoomId.value) {
      currentRoomId.value = '';
      messages.value = [];
    }
    fetchRooms();
  };

  const wsConversationsCleared = () => {
    currentRoomId.value = '';
    messages.value = [];
    fetchRooms();
  };

  const wsConversationNew = () => {
    fetchRooms();
  };

  return { wsChatHandler, wsStatusHandler, wsConversationDeleted, wsConversationsCleared, wsConversationNew, sortMessages, mapMessage };
}
