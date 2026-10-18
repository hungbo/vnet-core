<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { buildWsUrl } from '@/service/ws/config';
import client from '@/api/client';
import { localStg } from '@/utils/storage';

// Remote desktop: noVNC nối WebSocket tới backend, backend chuyển byte tới
// TightVNC (cổng 5900) trên máy trạm. Mật khẩu VNC lấy qua API, không hiện ra.

const props = defineProps<{ machine: { id: string; machine_code: string } | null }>();
const visible = defineModel<boolean>('visible', { default: false });

const { t: $t } = useI18n();

type Status = 'connecting' | 'connected' | 'disconnected';
const status = ref<Status>('connecting');
const errorText = ref('');
// Mặc định chỉ xem: mở ra để quan sát thì không lỡ tay bấm vào máy khách.
const viewOnly = ref(true);
const screenEl = ref<HTMLDivElement>();
const wrapEl = ref<HTMLDivElement>();
let rfb: any = null;

async function connect() {
  if (!props.machine) return;
  status.value = 'connecting';
  errorText.value = '';
  try {
    // Gọi trước để nhận lỗi đọc được (máy tắt, chưa cài TightVNC, tường lửa):
    // lỗi trong lúc bắt tay WebSocket thì trình duyệt không cho đọc nội dung.
    const res: any = await client.get(`/machines/${props.machine.id}/remote-desktop`);
    // noVNC dùng top-level await nên nạp động, ngoài bundle chính.
    const { default: RFB } = await import('@novnc/novnc');
    await nextTick();
    if (!visible.value || !screenEl.value) return;
    const url = buildWsUrl(`/api/machines/${props.machine.id}/remote-desktop/ws`, localStg.get('token') || '');
    rfb = new RFB(screenEl.value, url, { credentials: { password: res.password } });
    rfb.scaleViewport = true;
    rfb.viewOnly = viewOnly.value;
    rfb.addEventListener('connect', () => {
      status.value = 'connected';
      rfb?.focus();
    });
    rfb.addEventListener('disconnect', (e: any) => {
      status.value = 'disconnected';
      if (!e.detail?.clean) errorText.value = $t('vnetPages.machines.remote.rdLost');
      rfb = null;
    });
    rfb.addEventListener('securityfailure', (e: any) => {
      errorText.value = e.detail?.reason || $t('vnetPages.machines.remote.rdAuthFailed');
    });
  } catch (e: any) {
    status.value = 'disconnected';
    errorText.value = e?.message || $t('vnetPages.common.error');
  }
}

function disconnect() {
  rfb?.disconnect();
  rfb = null;
}

function toggleViewOnly() {
  viewOnly.value = !viewOnly.value;
  if (rfb) rfb.viewOnly = viewOnly.value;
}

function fullscreen() {
  wrapEl.value?.requestFullscreen?.();
}

watch(visible, v => (v ? connect() : disconnect()));
onBeforeUnmount(disconnect);
</script>

<template>
  <ElDialog
    v-model="visible"
    :title="$t('vnetPages.machines.remote.rdTitle', { code: machine?.machine_code })"
    width="90vw"
    top="3vh"
    destroy-on-close
  >
    <div class="mb-2 flex items-center gap-2">
      <ElTag :type="status === 'connected' ? 'success' : status === 'connecting' ? 'warning' : 'info'">
        {{ $t(`vnetPages.machines.remote.rd_${status}`) }}
      </ElTag>
      <ElButton size="small" :disabled="status !== 'connected'" @click="rfb?.sendCtrlAltDel()">Ctrl+Alt+Del</ElButton>
      <ElButton size="small" :type="viewOnly ? 'primary' : 'default'" @click="toggleViewOnly">
        {{ $t('vnetPages.machines.remote.rdViewOnly') }}
      </ElButton>
      <ElButton size="small" :disabled="status !== 'connected'" @click="fullscreen">
        {{ $t('vnetPages.machines.remote.rdFullscreen') }}
      </ElButton>
      <ElButton v-if="status === 'disconnected'" size="small" type="primary" @click="connect">
        {{ $t('vnetPages.machines.remote.rdReconnect') }}
      </ElButton>
      <span v-if="errorText" class="text-sm text-red-500">{{ errorText }}</span>
    </div>
    <div ref="wrapEl" class="rd-wrap">
      <div ref="screenEl" class="rd-screen" />
    </div>
  </ElDialog>
</template>

<style scoped>
.rd-wrap {
  background: #111;
  height: 80vh;
}
.rd-screen {
  width: 100%;
  height: 100%;
}
</style>
