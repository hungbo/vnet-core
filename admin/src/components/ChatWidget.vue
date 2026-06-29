<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ElButton, ElDialog, ElForm, ElFormItem, ElInputNumber, ElMessage, ElOption, ElSelect } from 'element-plus';
import { register } from 'vue-advanced-chat';
register();
import { useAuthStore } from '@/store/modules/auth';
import { useWebSocketStore } from '@/store/modules/ws';
import client from '@/api/client';
import { useChatRooms } from '@/hooks/chat/useChatRooms';
import { useChatTopup } from '@/hooks/chat/useChatTopup';
import { useChatWs } from '@/hooks/chat/useChatWs';
import { isChatOpen, chatUnreadCount } from '@/hooks/chat/chatState';

const authStore = useAuthStore();
const wsStore = useWebSocketStore();
const currentUserId = computed(() => authStore.userInfo?.id || '');

// ── UI state ──
const posX = ref((window.innerWidth - window.innerWidth * 0.6) / 2);
const posY = ref(100);
const isDragging = ref(false);
const dragOffset = { x: 0, y: 0 };

const messages = ref<any[]>([]);
const currentRoomId = ref('');
const messagesLoaded = ref(true);
const chatRef = ref<any>(null);

// ── Composables ──
const convs = useChatRooms(messages, currentRoomId, messagesLoaded);
const { rooms, showNewRoomDialog, roomForm, users, saving,
        fetchRooms, onFetchMessages, fetchMembers, handleCreateRoom,
        deleteRoom, deleteAllRooms } = convs;
const { wsChatHandler, wsStatusHandler, wsRoomDeleted, wsRoomsCleared, wsRoomNew, sortMessages, mapMessage } = useChatWs(
  currentRoomId, messages, fetchRooms,
);
const { showTopupDialog, topupAmount, topupMethod, topupMemberId, topupRoomId,
        openTopupDialog, openTopupFromMessage, confirmTopup } = useChatTopup(
  rooms, currentUserId, currentRoomId,
);

// ── vue-advanced-chat config ──
const menuActions = [
  { name: 'newRoom', title: 'Tạo phòng' },
  { name: 'deleteRoom', title: 'Xoá' },
  { name: 'topup', title: 'Nạp tiền' },
];
const messageActions = [
  { name: 'approveTopup', title: 'Duyệt nạp' },
  { name: 'cancelTopup', title: 'Từ chối' },
];
const autoScroll = {
  send: { new: true, newAfterScrollUp: false },
  receive: { new: false, newAfterScrollUp: true },
};
const chatStyles = {
  general: { color: '#333', borderStyle: '1px solid #e4e7ed' },
  footer: { background: '#fff' },
};

// ── Send message ──
async function onSendMessage($event: any) {
  const raw = $event?.detail ?? $event;
  const payload = Array.isArray(raw) ? raw[0] : raw;
  const { roomId, content } = payload;
  if (!content || !roomId) return;
  try {
    const postRes: any = await client.post('/chat/messages', {
      room_id: roomId, message: content,
      sender_type: 'admin', sender_id: currentUserId.value,
    });
    if (postRes?.id) {
      messages.value = sortMessages([...messages.value, {
        _id: postRes.id, content, senderId: currentUserId.value,
        username: 'Admin', date: new Date().toLocaleDateString('vi-VN'),
        timestamp: new Date().toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }),
        createdAt: new Date().toISOString(), saved: true, distributed: false, seen: false,
        disableActions: true, messageType: 'text', senderType: 'admin',
      }]);
    }
  } catch {
    ElMessage.error('Gửi tin nhắn thất bại');
  }
}

// ── Menu action handler ──
async function onMenuAction($event: any) {
  const raw = $event?.detail ?? $event;
  const payload = Array.isArray(raw) ? raw[0] : raw;
  const { roomId, action } = payload;
  if (action.name === 'newRoom') {
    showNewRoomDialog.value = true;
    fetchMembers();
  } else if (action.name === 'deleteRoom') {
    deleteRoom(roomId);
  } else if (action.name === 'topup') {
    openTopupDialog(roomId);
  }
}

// ── Message action handler ──
async function onMessageAction($event: any) {
  const raw = $event?.detail ?? $event;
  const payload = Array.isArray(raw) ? raw[0] : raw;
  const { roomId, action, message } = payload;
  if (action.name === 'approveTopup') {
    openTopupFromMessage(message, roomId);
  } else if (action.name === 'cancelTopup') {
    try {
      await client.post('/chat/messages', {
        room_id: roomId,
        message: '❌ Yêu cầu nạp không được duyệt',
        sender_type: 'admin', sender_id: currentUserId.value, message_type: 'text',
      });
      currentRoomId.value = '';
      nextTick(() => { currentRoomId.value = roomId; });
    } catch {
      ElMessage.error('Từ chối thất bại');
    }
  }
}

// ── Drag ──
function startDrag(e: MouseEvent) {
  isDragging.value = true;
  dragOffset.x = e.clientX - posX.value;
  dragOffset.y = e.clientY - posY.value;
  document.addEventListener('mousemove', onDrag);
  document.addEventListener('mouseup', stopDrag);
}
function onDrag(e: MouseEvent) {
  if (!isDragging.value) return;
  posX.value = e.clientX - dragOffset.x;
  posY.value = Math.max(0, e.clientY - dragOffset.y);
}
function stopDrag() {
  isDragging.value = false;
  document.removeEventListener('mousemove', onDrag);
  document.removeEventListener('mouseup', stopDrag);
}

// ── vue-advanced-chat lifecycle ──
function attachChatListener() {
  const el = document.querySelector('vue-advanced-chat');
  if (el) el.addEventListener('fetch-messages', onFetchMessages);
}
function detachChatListener() {
  const el = document.querySelector('vue-advanced-chat');
  if (el) el.removeEventListener('fetch-messages', onFetchMessages);
}

onMounted(() => {
  fetchRooms();
  wsStore.on('chat:message', wsChatHandler);
  wsStore.on('message:status:updated', wsStatusHandler);
  wsStore.on('room:deleted', wsRoomDeleted);
  wsStore.on('rooms:cleared', wsRoomsCleared);
  wsStore.on('room:new', wsRoomNew);
});

onBeforeUnmount(() => {
  wsStore.off('chat:message', wsChatHandler);
  wsStore.off('message:status:updated', wsStatusHandler);
  wsStore.off('room:deleted', wsRoomDeleted);
  wsStore.off('rooms:cleared', wsRoomsCleared);
  wsStore.off('room:new', wsRoomNew);
  detachChatListener();
});

watch(isChatOpen, async (open) => {
  if (open) {
    await fetchRooms();
    chatUnreadCount.value = rooms.value.reduce((sum: number, r: any) => sum + (r.unreadCount || 0), 0);
    messagesLoaded.value = false;
    await nextTick();
    messagesLoaded.value = true;
    requestAnimationFrame(() => attachChatListener());
  } else {
    currentRoomId.value = '';
    detachChatListener();
  }
});

watch(currentRoomId, async (newId) => {
  if (!newId) return;
  try {
    await client.put(`/chat/rooms/${newId}/read`);
    const readRoom = rooms.value.find((r: any) => r.roomId === newId);
    if (readRoom?.unreadCount) {
      chatUnreadCount.value = Math.max(0, chatUnreadCount.value - readRoom.unreadCount);
    }
  } catch {
    // ignore
  }
});
</script>

<template>
  <!-- Chat panel -->
  <div v-if="isChatOpen" class="chat-panel" :style="{ left: posX + 'px', top: posY + 'px' }">
    <div class="chat-panel-handle" @mousedown="startDrag">
      <span>Chat hỗ trợ</span>
      <div>
        <ElButton size="small" text @click.stop="deleteAllRooms">🗑</ElButton>
        <ElButton size="small" text @click.stop="isChatOpen = false">─</ElButton>
      </div>
    </div>
    <vue-advanced-chat
      ref="chatRef"
      :current-user-id="currentUserId"
      :rooms="JSON.stringify(rooms)"
      :messages="JSON.stringify(messages)"
      :messages-loaded="messagesLoaded"
      rooms-loaded
      :room-actions="JSON.stringify(menuActions)"
      :message-actions="JSON.stringify(messageActions)"
      :show-add-room="false"
      :show-search="false"
      :show-files="false"
      :show-audio="false"
      :show-emojis="false"
      :show-reaction-emojis="false"
      :show-new-messages-divider="false"
      :auto-scroll="JSON.stringify(autoScroll)"
      :styles="JSON.stringify(chatStyles)"
      height="100%"
      :room-id="currentRoomId"
      @send-message="onSendMessage"
      @menu-action-handler="onMenuAction"
      @message-action-handler="onMessageAction"
    />
  </div>

  <!-- Dialog tạo hội thoại -->
  <ElDialog v-model="showNewRoomDialog" title="Tạo phòng mới" width="350px" :close-on-click-modal="false">
    <ElForm label-width="100px">
      <ElFormItem label="Người nhận" prop="participant_ids">
        <ElSelect
          v-model="roomForm.participant_ids"
          multiple
          placeholder="Chọn người nhận"
          style="width: 100%"
        >
          <ElOption v-for="u in users" :key="u.id" :label="u.username || u.full_name" :value="u.id" />
        </ElSelect>
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="showNewRoomDialog = false">Huỷ</ElButton>
      <ElButton type="primary" :loading="saving" @click="handleCreateRoom">Tạo</ElButton>
    </template>
  </ElDialog>

  <!-- Dialog nạp tiền -->
  <ElDialog v-model="showTopupDialog" title="Nạp tiền" width="350px" :close-on-click-modal="false">
    <ElForm label-width="100px">
      <ElFormItem label="Số tiền">
        <ElInputNumber v-model="topupAmount" :min="1000" :step="10000" :max="100000000" style="width:100%" />
      </ElFormItem>
      <ElFormItem label="Hình thức">
        <ElSelect v-model="topupMethod" style="width:100%">
          <ElOption label="Tiền mặt" value="cash" />
          <ElOption label="Chuyển khoản" value="transfer" />
          <ElOption label="Bonus" value="bonus_balance" />
        </ElSelect>
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="showTopupDialog = false">Huỷ</ElButton>
      <ElButton type="primary" @click="confirmTopup">Xác nhận nạp</ElButton>
    </template>
  </ElDialog>
</template>

<style scoped>
.chat-panel {
  position: fixed;
  z-index: 2000;
  width: 60vw;
  height: 80vh;
  max-height: calc(100vh - 80px);
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.chat-panel-handle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #f5f7fa;
  cursor: grab;
  user-select: none;
  font-size: 13px;
  font-weight: 500;
  border-bottom: 1px solid #e4e7ed;
  flex-shrink: 0;
}
.chat-panel-handle:active {
  cursor: grabbing;
}
.chat-panel-handle :deep(.el-button) {
  cursor: pointer !important;
}
.chat-panel :deep(vue-advanced-chat) {
  flex: 1;
  min-height: 0;
}
</style>
