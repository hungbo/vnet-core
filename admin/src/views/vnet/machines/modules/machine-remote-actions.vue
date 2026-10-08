<script setup lang="ts">
// Nút điều khiển một máy trạm: menu Khác (bật máy qua Wake-on-LAN, điều khiển
// từ xa, chụp màn hình, tiến trình, nhắn tin, khởi động lại, tắt máy). Chặn ứng dụng
// không nằm ở đây: nó là cài đặt chung cho mọi máy (Cài đặt > Máy trạm),
// kèm các hộp thoại của chúng. Dùng chung cho trang Máy và bảng "Máy đang hoạt
// động" ở Bảng điều khiển — hai bản chép là hai chỗ để lệch nhau.
//
// Danh sách lệnh phải khớp remoteActions bên backend
// (internal/service/machine.go). Backend trả 409 khi máy chưa kết nối, nên
// "đã gửi lệnh" ở đây nghĩa là máy thật sự đã nhận.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import RemoteDesktopModal from './remote-desktop-modal.vue';

defineOptions({ name: 'MachineRemoteActions' });

const props = defineProps<{ machine: { id: string; machine_code: string } }>();

const { t: $t } = useI18n();
const wsStore = useWebSocketStore();

async function sendRemote(action: string, payload?: Record<string, unknown>) {
  const row = props.machine;
  try {
    await client.post(`/machines/${row.id}/remote/${action}`, payload ?? {});
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.machines.remote.sent', { code: row.machine_code })
    });
  } catch (e: any) {
    // 409 = máy chưa kết nối. Phân biệt rõ với lỗi thật để nhân viên không đi
    // tìm sự cố trong khi máy chỉ đang tắt.
    const offline = e?.status === 409;
    ElMessage({
      type: offline ? 'warning' : 'error',
      message: offline
        ? $t('vnetPages.machines.remote.offline', { code: row.machine_code })
        : e?.message || $t('vnetPages.common.error')
    });
  }
}

const rdVisible = ref(false);

// Bật máy không đi qua WebSocket như các lệnh khác — máy đang tắt thì không có
// kết nối nào. Máy chủ phát magic packet ra mạng LAN (POST /machines/:id/wake).
async function handleWake() {
  const code = props.machine.machine_code;
  try {
    await ElMessageBox.confirm($t('vnetPages.machines.remote.wakeConfirm', { code }), $t('vnetPages.common.confirm'), {
      type: 'info'
    });
  } catch {
    return;
  }
  try {
    await client.post(`/machines/${props.machine.id}/wake`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.machines.remote.wakeSent', { code })
    });
  } catch (e: any) {
    ElMessage({ type: e?.status === 409 ? 'warning' : 'error', message: e?.message || $t('vnetPages.common.error') });
  }
}

function handleRemoteCommand(cmd: string) {
  if (cmd === 'wake') return handleWake();
  if (cmd === 'remote-desktop') {
    rdVisible.value = true;
    return undefined;
  }
  if (cmd === 'screenshot') return handleScreenshot();
  if (cmd === 'processes') return handleProcesses();
  if (cmd === 'message') return handleMessage();
  return handlePower(cmd as 'shutdown' | 'restart');
}

async function handlePower(action: 'shutdown' | 'restart') {
  try {
    // confirmAction đã có sẵn trong locale từ trước: 'Thực hiện "{action}" trên máy {code}?'
    await ElMessageBox.confirm(
      $t('vnetPages.machines.remote.confirmAction', {
        action: $t(`vnetPages.machines.remote.${action}`),
        code: props.machine.machine_code
      }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  await sendRemote(action);
}

async function handleMessage() {
  try {
    const { value } = await ElMessageBox.prompt(
      $t('vnetPages.machines.remote.messagePrompt'),
      $t('vnetPages.machines.remote.message'),
      {
        inputPattern: /\S/,
        inputErrorMessage: $t('vnetPages.machines.remote.messageRequired')
      }
    );
    await sendRemote('message', { title: 'VNET', message: value });
  } catch {
    // người dùng bấm huỷ
  }
}

// --- Giám sát: chụp màn hình + tiến trình -------------------------------------
// Lệnh đi xuống máy trạm, dữ liệu bay NGƯỢC về qua WebSocket. Khớp bằng
// request_id để mở nhiều máy cùng lúc không bị lẫn ảnh của nhau.

const shotVisible = ref(false);
const shotImage = ref('');
const shotReqId = ref('');

const procVisible = ref(false);
const procReqId = ref('');
const procList = ref<any[]>([]);
const procLoading = ref(false);
const procSearch = ref('');
// Lọc tại chỗ theo tên: máy trạm gửi cả trăm tiến trình, cuộn tìm từng dòng rất mất công.
const procFiltered = computed(() => {
  const q = procSearch.value.trim().toLowerCase();
  return q ? procList.value.filter(p => String(p.name).toLowerCase().includes(q)) : procList.value;
});

// sendRemoteForResult như sendRemote nhưng TRẢ VỀ data để lấy request_id. Tách
// riêng vì sendRemote nuốt kết quả và chỉ hiện toast.
async function sendRemoteForResult(action: string): Promise<any | null> {
  const row = props.machine;
  try {
    return await client.post(`/machines/${row.id}/remote/${action}`, {});
  } catch (e: any) {
    const offline = e?.status === 409;
    ElMessage({
      type: offline ? 'warning' : 'error',
      message: offline
        ? $t('vnetPages.machines.remote.offline', { code: row.machine_code })
        : e?.message || $t('vnetPages.common.error')
    });
    return null;
  }
}

async function handleScreenshot() {
  const res = await sendRemoteForResult('screenshot');
  if (!res) return;
  shotImage.value = '';
  shotReqId.value = res.request_id || '';
  shotVisible.value = true;
}

async function handleProcesses() {
  const res = await sendRemoteForResult('process-list');
  if (!res) return;
  procList.value = [];
  procSearch.value = '';
  procReqId.value = res.request_id || '';
  procLoading.value = true;
  procVisible.value = true;
}

async function handleKill(name: string) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.machines.remote.killConfirm', { name }),
      $t('vnetPages.machines.remote.processKill'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  procLoading.value = true;
  try {
    await client.post(`/machines/${props.machine.id}/remote/process-kill`, { process: name });
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    procLoading.value = false;
  }
}

// Chỉ nhận ảnh JPEG/PNG dạng data URI. Ảnh tới từ đường không xác thực của máy
// trạm rồi được dùng làm src và href của nút "Tải ảnh": một chuỗi javascript:…
// lọt qua là chạy mã trong phiên của nhân viên. Máy chủ đã chặn; đây là lớp hai.
const ANH_CHUP_HOP_LE = /^data:image\/(jpeg|png);base64,[A-Za-z0-9+/]+={0,2}$/;

function downloadShot() {
  if (!ANH_CHUP_HOP_LE.test(shotImage.value)) return;
  const a = document.createElement('a');
  a.href = shotImage.value;
  a.download = `${props.machine.machine_code}-${dayjs().format('YYYYMMDD-HHmmss')}.jpg`;
  a.click();
}

function onScreenshot(payload: any) {
  if (!shotReqId.value || payload?.request_id !== shotReqId.value) return;
  const img = typeof payload.image === 'string' ? payload.image : '';
  shotImage.value = ANH_CHUP_HOP_LE.test(img) ? img : '';
}

function onProcesses(payload: any) {
  if (!procReqId.value || payload?.request_id !== procReqId.value) return;
  procList.value = payload.processes || [];
  procLoading.value = false;
  if (typeof payload.killed === 'number' && payload.killed === -1) {
    // -1 nghĩa là tiến trình nằm trong danh sách cấm tắt (không phải "không thấy").
  } else if (typeof payload.killed === 'number' && payload.killed >= 0) {
    ElMessage.success($t('vnetPages.machines.remote.killed', { n: payload.killed }));
  }
}

onMounted(() => {
  wsStore.on('machine:screenshot', onScreenshot);
  wsStore.on('machine:processes', onProcesses);
});

onBeforeUnmount(() => {
  wsStore.off('machine:screenshot', onScreenshot);
  wsStore.off('machine:processes', onProcesses);
});
</script>

<template>
  <span class="remote-actions">
    <ElDropdown @command="handleRemoteCommand">
      <ElButton size="small">{{ $t('vnetPages.machines.remote.more') }}</ElButton>
      <template #dropdown>
        <ElDropdownMenu>
          <ElDropdownItem command="wake">{{ $t('vnetPages.machines.remote.wake') }}</ElDropdownItem>
          <ElDropdownItem command="remote-desktop" divided>
            {{ $t('vnetPages.machines.remote.remoteDesktop') }}
          </ElDropdownItem>
          <ElDropdownItem command="screenshot">{{ $t('vnetPages.machines.remote.screenshot') }}</ElDropdownItem>
          <ElDropdownItem command="processes">{{ $t('vnetPages.machines.remote.processes') }}</ElDropdownItem>
          <ElDropdownItem command="message" divided>
            {{ $t('vnetPages.machines.remote.message') }}
          </ElDropdownItem>
          <ElDropdownItem command="restart" divided>
            {{ $t('vnetPages.machines.remote.restart') }}
          </ElDropdownItem>
          <ElDropdownItem command="shutdown">
            {{ $t('vnetPages.machines.remote.shutdown') }}
          </ElDropdownItem>
        </ElDropdownMenu>
      </template>
    </ElDropdown>

    <ElDialog
      v-model="shotVisible"
      :title="$t('vnetPages.machines.remote.screenshotTitle', { code: machine.machine_code })"
      width="80%"
      top="4vh"
      append-to-body
    >
      <div v-if="!shotImage" class="shot-waiting">
        {{ $t('vnetPages.machines.remote.waiting') }}
      </div>
      <template v-else>
        <!--
          Bấm vào ảnh mở trình xem toàn màn hình: lăn chuột để phóng to/thu nhỏ,
          kéo để di chuyển, nút 1:1 để xem đúng điểm ảnh — đọc được chữ trên
          màn hình khách.
        -->
        <ElImage
          :src="shotImage"
          :preview-src-list="[shotImage]"
          :zoom-rate="1.25"
          :max-scale="8"
          fit="contain"
          preview-teleported
          hide-on-click-modal
          class="shot-image"
        />
        <div class="shot-hint">{{ $t('vnetPages.machines.remote.screenshotHint') }}</div>
      </template>
      <template #footer>
        <ElButton :disabled="!shotImage" @click="downloadShot">
          {{ $t('vnetPages.machines.remote.screenshotDownload') }}
        </ElButton>
        <ElButton type="primary" @click="handleScreenshot">
          {{ $t('vnetPages.machines.remote.screenshotRetake') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="procVisible"
      :title="$t('vnetPages.machines.remote.processesTitle', { code: machine.machine_code })"
      width="600px"
      top="6vh"
      append-to-body
    >
      <ElInput v-model="procSearch" :placeholder="$t('vnetPages.machines.remote.procSearch')" clearable class="mb-3" />
      <ElTable v-loading="procLoading" :data="procFiltered" border stripe max-height="60vh">
        <ElTableColumn :label="$t('vnetPages.machines.remote.procName')" prop="name" />
        <ElTableColumn :label="$t('vnetPages.machines.remote.procCount')" prop="count" width="90" align="center" />
        <ElTableColumn :label="$t('vnetPages.machines.remote.procRam')" width="110" align="right">
          <template #default="{ row }">{{ row.ram_mb }} MB</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="90" align="center">
          <template #default="{ row }">
            <ElButton size="small" type="danger" @click="handleKill(row.name)">
              {{ $t('vnetPages.machines.remote.kill') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElDialog>

    <RemoteDesktopModal v-model:visible="rdVisible" :machine="machine" append-to-body />
  </span>
</template>

<style scoped>
.remote-actions {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
}

.shot-waiting {
  text-align: center;
  padding: 60px;
  color: var(--el-text-color-secondary);
}

.shot-image {
  display: block;
  width: 100%;
  max-height: 72vh;
  border-radius: 4px;
  cursor: zoom-in;
}

.shot-hint {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  text-align: center;
}
</style>
