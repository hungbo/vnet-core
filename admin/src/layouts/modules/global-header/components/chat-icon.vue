<script setup lang="ts">
import { chatUnreadCount, toggleChat } from '@/hooks/chat/chatState';

defineOptions({ name: 'ChatIcon' });
</script>

<template>
  <ElBadge :value="chatUnreadCount" :hidden="chatUnreadCount === 0" :max="99" class="flex-center">
    <ButtonIcon icon="ri:chat-1-line" tooltip-content="Chat" @click="toggleChat" />
  </ElBadge>
</template>

<style scoped></style>
