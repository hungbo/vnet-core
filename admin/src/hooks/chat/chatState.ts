import { ref } from 'vue';

export const isChatOpen = ref(false);
export const chatUnreadCount = ref(0);
export const roomUnreadCounts = ref<Record<string, number>>({});

export function toggleChat() {
  isChatOpen.value = !isChatOpen.value;
}

