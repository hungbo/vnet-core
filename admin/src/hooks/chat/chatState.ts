import { ref } from 'vue';

export const isChatOpen = ref(false);
export const chatUnreadCount = ref(0);

export function toggleChat() {
  isChatOpen.value = !isChatOpen.value;
}

export function incrementUnread() {
  chatUnreadCount.value++;
}

export function resetUnread() {
  chatUnreadCount.value = 0;
}
