<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'

// Lớp phủ khoá máy.
//
// Đây KHÔNG phải LockScreen.vue — cái đó là màn hình đăng nhập. Lớp phủ này
// nằm trên mọi thứ, kể cả màn hình đăng nhập, vì lệnh khoá từ quầy phải có
// hiệu lực bất kể khách đang ở bước nào.
//
// Chặn phím do phía Go lo (locker.go, hook cấp thấp). Lớp phủ chỉ là phần nhìn
// thấy được: nó không tự nó ngăn được ai làm gì.

const locked = ref(false)
const reason = ref('')
const now = ref(new Date())

let timer: number | undefined

function tick() {
  now.value = new Date()
}

onMounted(() => {
  EventsOn('vnet:machine:locked', (r: string) => {
    reason.value = r || ''
    locked.value = true
  })
  EventsOn('vnet:machine:unlocked', () => {
    locked.value = false
    reason.value = ''
  })
  timer = window.setInterval(tick, 1000)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})

function pad(n: number) {
  return n < 10 ? `0${n}` : `${n}`
}
</script>

<template>
  <div v-if="locked" class="machine-lock">
    <div class="machine-lock__inner">
      <div class="machine-lock__icon">🔒</div>
      <h1>Máy đang tạm khoá</h1>
      <p class="machine-lock__reason">
        {{ reason || 'Vui lòng liên hệ quầy để được mở khoá.' }}
      </p>
      <div class="machine-lock__clock">
        {{ pad(now.getHours()) }}:{{ pad(now.getMinutes()) }}:{{ pad(now.getSeconds()) }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.machine-lock {
  position: fixed;
  inset: 0;
  /* Trên mọi thứ, kể cả hộp thoại của Element Plus (z-index ~2000-3000). */
  z-index: 999999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0f1021;
  color: #e8e9f3;
  user-select: none;
  cursor: not-allowed;
}

.machine-lock__inner {
  text-align: center;
  padding: 48px;
  max-width: 640px;
}

.machine-lock__icon {
  font-size: 72px;
  line-height: 1;
  margin-bottom: 24px;
}

.machine-lock h1 {
  font-size: 34px;
  font-weight: 600;
  margin: 0 0 16px;
  letter-spacing: 0.5px;
}

.machine-lock__reason {
  font-size: 18px;
  line-height: 1.6;
  color: #a5a8c4;
  margin: 0 0 40px;
}

.machine-lock__clock {
  font-size: 56px;
  font-weight: 300;
  font-variant-numeric: tabular-nums;
  color: #6f74a8;
}
</style>
