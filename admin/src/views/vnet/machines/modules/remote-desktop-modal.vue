<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
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
    rfb.addEventListener('clipboard', (e: any) => copyToLocal(e.detail.text));
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

// Dán: noVNC chặn mặc định mọi phím nên Ctrl+V không thành sự kiện paste, và
// trang chạy http nên không đọc được navigator.clipboard. Giữ Ctrl+V (Cmd+V
// trên Mac) lại không cho noVNC thấy, để trình duyệt phát paste; chữ dán được
// đưa vào clipboard máy trạm rồi bấm Ctrl+V bên đó.
const XK_CONTROL_L = 0xffe3;
const XK_ALT_L = 0xffe9;
const XK_V = 0x0076;
let pasteWithCmd = false;

function onKeydown(e: KeyboardEvent) {
  if (!rfb || viewOnly.value || e.code !== 'KeyV' || !(e.ctrlKey || e.metaKey)) return;
  pasteWithCmd = e.metaKey && !e.ctrlKey;
  e.stopPropagation();
}

function onPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text/plain');
  if (!rfb || viewOnly.value || !text) return;
  e.preventDefault();
  rfb.clipboardPasteFrom(text);
  rfb.sendKey(XK_CONTROL_L, 'ControlLeft', true);
  // noVNC gửi Cmd thành Alt; nhả Alt sau Ctrl để máy trạm nhận Ctrl+V chứ không
  // phải Ctrl+Alt+V, và Windows không bật thanh menu.
  if (pasteWithCmd) rfb.sendKey(XK_ALT_L, 'AltLeft', false);
  rfb.sendKey(XK_V, 'KeyV');
  rfb.sendKey(XK_CONTROL_L, 'ControlLeft', false);
}

// Copy: máy trạm báo clipboard đổi thì chép sang máy admin. execCommand('copy')
// chạy được trên http, miễn người dùng vừa bấm phím/chuột (Ctrl+C, chuột phải →
// Copy). noVNC không báo clipboard khi đang Chỉ xem.
function copyToLocal(text: string) {
  const onCopy = (e: ClipboardEvent) => {
    e.clipboardData?.setData('text/plain', text);
    e.preventDefault();
  };
  document.addEventListener('copy', onCopy);
  const ok = document.execCommand('copy');
  document.removeEventListener('copy', onCopy);
  if (ok) ElMessage.success($t('vnetPages.machines.remote.rdCopied'));
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
    <div ref="wrapEl" class="rd-wrap" @keydown.capture="onKeydown" @paste="onPaste">
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
