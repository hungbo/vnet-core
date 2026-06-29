import type { Ref } from 'vue';
import { reactive, ref } from 'vue';
import dayjs from 'dayjs';
import { ElMessage, ElMessageBox } from 'element-plus';
import client from '@/api/client';
import { chatUnreadCount, roomUnreadCounts } from '@/hooks/chat/chatState';

export function useChatRooms(messages: Ref<any[]>, currentRoomId: Ref<string>, messagesLoaded: Ref<boolean>) {
  const rooms = ref<any[]>([]);
  const unreadCount = ref(0);
  const showNewRoomDialog = ref(false);
  const roomForm = reactive({ participant_ids: [] as string[] });
  const users = ref<any[]>([]);
  const saving = ref(false);
  let currentPage = 1;
  let latestFetch = 0;

  function mapRoom(room: any) {
    const p = room.participants?.[0];
    let roomName = 'Hỗ trợ';
    if (p) {
      if (p.username && p.machineCode) {
        roomName = `${p.username} - ${p.machineCode}`;
      } else if (p.username) {
        roomName = p.username;
      }
    }
    return {
      roomId: room.id,
      roomName,
      unreadCount: room.unread_count || 0,
      lastMessage: room.last_message
        ? {
            content: room.last_message.message,
            senderId: room.last_message.sender_id,
            timestamp: dayjs(room.last_message.created_at).format('HH:mm'),
            saved: true,
            distributed: room.last_message.status === 'delivered' || room.last_message.status === 'read',
            seen: room.last_message.status === 'read',
          }
        : undefined,
      users: room.participants?.map((p: any) => ({
        _id: p.id,
        username: p.name || p.username || 'User',
      })) || [],
    };
  }

  function updateUnreadCount() {
    unreadCount.value = rooms.value.reduce((sum: number, r: any) => sum + (r.unreadCount || 0), 0);
    chatUnreadCount.value = unreadCount.value;
  }

  async function fetchRooms() {
    try {
      const res: any = await client.get('/chat/rooms');
      const items = Array.isArray(res) ? res : res?.items || [];
      rooms.value = items.map((room: any) => ({
        ...mapRoom(room),
        unreadCount: (room.unread_count || 0) + (roomUnreadCounts.value[room.id] || 0),
      }));
      updateUnreadCount();
    } catch {
      // ignore
    }
  }

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

  async function fetchPage(roomId: string, page: number) {
    const res: any = await client.get(`/chat/rooms/${roomId}/messages`, {
      params: { page, page_size: 20 },
    });
    return Array.isArray(res) ? res : res?.items || [];
  }

  async function onFetchMessages(e: Event) {
    const detail = (e as CustomEvent).detail;
    const payload = Array.isArray(detail) ? detail[0] : detail;
    const roomId = payload?.room?.roomId;
    const before = payload?.options?.before;
    if (!roomId) return;

    const isNewRoom = roomId !== currentRoomId.value;
    if (isNewRoom) {
      currentRoomId.value = roomId;
      currentPage = 1;
    }

    const fetchId = ++latestFetch;
    messagesLoaded.value = false;

    try {
      if (!before) {
        currentPage = 1;
        const items = await fetchPage(roomId, 1);
        if (fetchId !== latestFetch) return;
        messages.value = sortMessages(items.map(mapMessage));
      } else {
        const items = await fetchPage(roomId, currentPage + 1);
        if (fetchId !== latestFetch) return;
        if (items.length > 0) {
          currentPage++;
          const oldMsgs = items.map(mapMessage);
          messages.value = sortMessages([...oldMsgs, ...messages.value]);
        }
      }
    } catch {
      // ignore
    }
    messagesLoaded.value = true;
  }

  async function fetchMembers() {
    try {
      const res: any = await client.get('/members');
      users.value = Array.isArray(res) ? res : res?.items || [];
    } catch {
      // ignore
    }
  }

  async function handleCreateRoom() {
    if (!roomForm.participant_ids.length) return;
    saving.value = true;
    try {
      await client.post('/chat/rooms', {
        participant_id: roomForm.participant_ids[0],
        participant_type: 'member',
        title: 'Hỗ trợ',
      });
      showNewRoomDialog.value = false;
      roomForm.participant_ids = [];
      fetchRooms();
    } catch (e: any) {
      ElMessage.error(e.message || 'Tạo hội thoại thất bại');
    } finally {
      saving.value = false;
    }
  }

  async function deleteRoom(roomId: string) {
    try {
      await ElMessageBox.confirm('Xoá cuộc hội thoại này?', 'Xác nhận', { type: 'warning' });
      await client.delete(`/chat/rooms/${roomId}`);
      if (currentRoomId.value === roomId) {
        currentRoomId.value = '';
        messages.value = [];
      }
      await fetchRooms();
    } catch (e: any) {
      if (e !== 'cancel') ElMessage.error(e.message || 'Lỗi');
    }
  }

  async function deleteAllRooms() {
    try {
      await ElMessageBox.confirm('Xoá tất cả cuộc hội thoại? Hành động này không thể hoàn tác!', {
        type: 'warning',
        confirmButtonText: 'Xoá tất cả',
      });
      await client.delete('/chat/rooms');
      currentRoomId.value = '';
      messages.value = [];
      await fetchRooms();
    } catch (e: any) {
      if (e !== 'cancel') ElMessage.error(e.message || 'Lỗi');
    }
  }

  return {
    rooms,
    unreadCount,
    showNewRoomDialog,
    roomForm,
    users,
    saving,
    fetchRooms,
    mapRoom,
    onFetchMessages,
    fetchMembers,
    handleCreateRoom,
    deleteRoom,
    deleteAllRooms,
    updateUnreadCount,
  };
}
