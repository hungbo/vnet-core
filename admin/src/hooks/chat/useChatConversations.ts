import type { Ref } from 'vue';
import { reactive, ref } from 'vue';
import dayjs from 'dayjs';
import { ElMessage, ElMessageBox } from 'element-plus';
import client from '@/api/client';
import { chatUnreadCount } from '@/hooks/chat/chatState';

export function useChatConversations(messages: Ref<any[]>, currentRoomId: Ref<string>) {
  const rooms = ref<any[]>([]);
  const unreadCount = ref(0);
  const showNewConversationDialog = ref(false);
  const convForm = reactive({ participant_ids: [] as string[] });
  const users = ref<any[]>([]);
  const saving = ref(false);

  function mapRoom(conv: any) {
    return {
      roomId: conv.id,
      roomName: conv.participants?.map((p: any) => p.name || p.username).filter(Boolean).join(', ') || 'Hỗ trợ',
      unreadCount: conv.unread_count || 0,
      lastMessage: conv.last_message
        ? {
            content: conv.last_message.message,
            senderId: conv.last_message.sender_id,
            timestamp: dayjs(conv.last_message.created_at).format('HH:mm'),
            saved: true,
            distributed: conv.last_message.status === 'delivered' || conv.last_message.status === 'read',
            seen: conv.last_message.status === 'read',
          }
        : undefined,
      users: conv.participants?.map((p: any) => ({
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
      const res: any = await client.get('/chat/conversations');
      const items = Array.isArray(res) ? res : res?.items || [];
      rooms.value = items.map(mapRoom);
      updateUnreadCount();
    } catch {
      // ignore
    }
  }

  function onRoomSelected(e: Event) {
    const detail = (e as CustomEvent).detail;
    const payload = Array.isArray(detail) ? detail[0] : detail;
    const roomId = payload?.room?.roomId;
    if (roomId) {
      currentRoomId.value = roomId;
    }
  }

  async function fetchMembers() {
    try {
      const res: any = await client.get('/members');
      users.value = Array.isArray(res) ? res : res?.items || [];
    } catch {
      // ignore
    }
  }

  async function handleCreateConversation() {
    if (!convForm.participant_ids.length) return;
    saving.value = true;
    try {
      await client.post('/chat/conversations', {
        participant_id: convForm.participant_ids[0],
        participant_type: 'member',
        title: 'Hỗ trợ',
      });
      showNewConversationDialog.value = false;
      convForm.participant_ids = [];
      fetchRooms();
    } catch (e: any) {
      ElMessage.error(e.message || 'Tạo hội thoại thất bại');
    } finally {
      saving.value = false;
    }
  }

  async function deleteSingleConversation(roomId: string) {
    try {
      await ElMessageBox.confirm('Xoá cuộc hội thoại này?', 'Xác nhận', { type: 'warning' });
      await client.delete(`/chat/conversations/${roomId}`);
      if (currentRoomId.value === roomId) {
        currentRoomId.value = '';
        messages.value = [];
      }
      await fetchRooms();
    } catch (e: any) {
      if (e !== 'cancel') ElMessage.error(e.message || 'Lỗi');
    }
  }

  async function deleteAllConversations() {
    try {
      await ElMessageBox.confirm('Xoá tất cả cuộc hội thoại? Hành động này không thể hoàn tác!', {
        type: 'warning',
        confirmButtonText: 'Xoá tất cả',
      });
      await client.delete('/chat/conversations');
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
    showNewConversationDialog,
    convForm,
    users,
    saving,
    fetchRooms,
    mapRoom,
    onRoomSelected,
    fetchMembers,
    handleCreateConversation,
    deleteSingleConversation,
    deleteAllConversations,
    updateUnreadCount,
  };
}
