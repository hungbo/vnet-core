import type { ComputedRef, Ref } from 'vue';
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import client from '@/api/client';

export function useChatTopup(
  rooms: Ref<any[]>,
  currentUserId: ComputedRef<string>,
  currentRoomId: Ref<string>,
) {
  const showTopupDialog = ref(false);
  const topupAmount = ref(50000);
  const topupMethod = ref('cash');
  const topupMemberId = ref('');
  const topupRoomId = ref('');

  function extractAmount(text: string): number {
    const match = text.match(/([\d.]+)\s*(k|K|tr)/);
    if (!match) return 50000;
    const num = parseFloat(match[1].replace(/\./g, ''));
    if (match[2] === 'k' || match[2] === 'K') return num * 1000;
    if (match[2] === 'tr') return num * 1000000;
    return num;
  }

  function openTopupDialog(roomId: string) {
    const conv = rooms.value.find((r: any) => r.roomId === roomId);
    if (!conv) return;
    const userId = conv.users?.find((u: any) => u._id !== currentUserId.value)?._id;
    if (!userId) return;
    topupMemberId.value = userId;
    topupRoomId.value = roomId;
    topupAmount.value = 50000;
    topupMethod.value = 'cash';
    showTopupDialog.value = true;
  }

  function openTopupFromMessage(message: any, roomId: string) {
    const conv = rooms.value.find((r: any) => r.roomId === roomId);
    const memberId = conv?.users?.find((u: any) => u._id !== currentUserId.value)?._id;
    if (memberId) {
      topupMemberId.value = memberId;
      topupRoomId.value = roomId;
      topupAmount.value = extractAmount(message.content);
      topupMethod.value = 'cash';
      showTopupDialog.value = true;
    }
  }

  async function confirmTopup() {
    if (topupAmount.value <= 0 || !topupMemberId.value) return;
    try {
      await client.post(`/members/${topupMemberId.value}/topup`, {
        amount: topupAmount.value,
        payment_method: topupMethod.value,
        description: 'Nạp qua chat',
      });
      ElMessage.success(`Đã nạp ${topupAmount.value.toLocaleString()}đ thành công`);
      showTopupDialog.value = false;
      await client.post('/chat/messages', {
        room_id: topupRoomId.value,
        message: `✅ Đã nạp ${topupAmount.value.toLocaleString()}đ`,
        sender_type: 'admin',
        sender_id: currentUserId.value,
        message_type: 'text',
      });
      currentRoomId.value = topupRoomId.value;
    } catch (e: any) {
      ElMessage.error(e.message || 'Nạp tiền thất bại');
    }
  }

  return {
    showTopupDialog,
    topupAmount,
    topupMethod,
    topupMemberId,
    topupRoomId,
    extractAmount,
    openTopupDialog,
    openTopupFromMessage,
    confirmTopup,
  };
}
