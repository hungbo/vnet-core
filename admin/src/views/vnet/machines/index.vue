<script setup lang="ts">
import { h, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();
const router = useRouter();
const wsStore = useWebSocketStore();

const search = ref('');



// --- Điều khiển từ xa -------------------------------------------------------
// Danh sách này phải khớp remoteActions bên backend
// (internal/service/machine.go). Backend trả 409 khi máy chưa kết nối, nên
// "đã gửi lệnh" ở đây nghĩa là máy thật sự đã nhận.

// --- Nhật ký phần cứng --------------------------------------------------------
// GET /machines/:id/hardware trả về chuỗi số đo mà máy trạm gửi kèm mỗi
// heartbeat. Không có màn hình nào đọc nó, nên nhiệt độ và mức tải máy — thứ
// quán cần để biết máy nào sắp hỏng — không xem được ở đâu cả.

const hwVisible = ref(false);
const hwLoading = ref(false);
const hwMachine = ref<any>(null);
const hwRows = ref<any[]>([]);

async function openHardware(row: any) {
  hwMachine.value = row;
  hwRows.value = [];
  hwVisible.value = true;
  hwLoading.value = true;
  try {
    const res: any = await client.get(`/machines/${row.id}/hardware`, {
      params: { page: 1, page_size: 50 }
    });
    hwRows.value = Array.isArray(res) ? res : res?.items || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machines.messages.loadError'));
  } finally {
    hwLoading.value = false;
  }
}

function pct(v: number | null | undefined) {
  return v == null ? '-' : `${Number(v).toFixed(0)}%`;
}

// Máy trạm gửi uptime theo giây. Hiện nguyên số giây thì không ai đọc được —
// điều người vận hành muốn biết là "máy này bật liên tục mấy ngày rồi".
function uptime(v: number | null | undefined) {
  if (!v) return '-';
  const d = Math.floor(v / 86400);
  const h = Math.floor((v % 86400) / 3600);
  const m = Math.floor((v % 3600) / 60);
  if (d > 0) return `${d}n ${h}g`;
  if (h > 0) return `${h}g ${m}p`;
  return `${m}p`;
}

function temp(v: number | null | undefined) {
  return v ? `${Number(v).toFixed(1)}°C` : '-';
}

const remoteBusy = ref('');

async function sendRemote(row: any, action: string, payload?: Record<string, unknown>) {
  remoteBusy.value = `${row.id}:${action}`;
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
  } finally {
    remoteBusy.value = '';
  }
}

async function handleLock(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      $t('vnetPages.machines.remote.lockReasonPrompt'),
      $t('vnetPages.machines.remote.lock'),
      {
        inputPlaceholder: $t('vnetPages.machines.remote.lockReasonPlaceholder'),
        inputValue: ''
      }
    );
    await sendRemote(row, 'lock', { reason: value || '' });
  } catch {
    // người dùng bấm huỷ
  }
}

// App.BlockApp/UnblockApp đã có sẵn trong máy khách nhưng trước nay không lối
// vào ở cả hai phía — viết rồi mà chưa từng gọi được. Chặn theo TÊN tiến trình,
// không phải câu lệnh tuỳ ý.
function handleRemoteCommand(row: any, cmd: string) {
  if (cmd === 'screenshot') return handleScreenshot(row);
  if (cmd === 'processes') return handleProcesses(row);
  if (cmd === 'message') return handleMessage(row);
  if (cmd === 'block-app' || cmd === 'unblock-app') return handleBlockApp(row, cmd);
  return handlePower(row, cmd as 'shutdown' | 'restart');
}

async function handleBlockApp(row: any, action: 'block-app' | 'unblock-app') {
  let value: string;
  try {
    const res = await ElMessageBox.prompt(
      $t('vnetPages.machines.remote.appPrompt'),
      $t(`vnetPages.machines.remote.${action === 'block-app' ? 'blockApp' : 'unblockApp'}`),
      { inputPlaceholder: $t('vnetPages.machines.remote.appPlaceholder') }
    );
    value = res.value;
  } catch {
    return;
  }
  if (!value?.trim()) {
    ElMessage.warning($t('vnetPages.machines.remote.appRequired'));
    return;
  }
  await sendRemote(row, action, { process: value.trim() });
}

async function handlePower(row: any, action: 'shutdown' | 'restart') {
  try {
    // confirmAction đã có sẵn trong locale từ trước: 'Thực hiện "{action}" trên máy {code}?'
    await ElMessageBox.confirm(
      $t('vnetPages.machines.remote.confirmAction', {
        action: $t(`vnetPages.machines.remote.${action}`),
        code: row.machine_code
      }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  await sendRemote(row, action);
}

async function handleMessage(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      $t('vnetPages.machines.remote.messagePrompt'),
      $t('vnetPages.machines.remote.message'),
      {
        inputPattern: /\S/,
        inputErrorMessage: $t('vnetPages.machines.remote.messageRequired')
      }
    );
    await sendRemote(row, 'message', { title: 'VNET', message: value });
  } catch {
    // người dùng bấm huỷ
  }
}

// --- Giám sát: chụp màn hình + tiến trình -------------------------------------
// Lệnh đi xuống máy trạm, dữ liệu bay NGƯỢC về qua WebSocket. Khớp bằng
// request_id để mở nhiều máy cùng lúc không bị lẫn ảnh của nhau.

const shotVisible = ref(false);
const shotImage = ref('');
const shotMachine = ref('');
const shotReqId = ref('');

const procVisible = ref(false);
const procMachine = ref('');
const procReqId = ref('');
const procList = ref<any[]>([]);
const procLoading = ref(false);

// sendRemoteForResult như sendRemote nhưng TRẢ VỀ data để lấy request_id. Tách
// riêng vì sendRemote nuốt kết quả và chỉ hiện toast.
async function sendRemoteForResult(row: any, action: string): Promise<any | null> {
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

async function handleScreenshot(row: any) {
  const res = await sendRemoteForResult(row, 'screenshot');
  if (!res) return;
  shotImage.value = '';
  shotMachine.value = row.machine_code;
  shotReqId.value = res.request_id || '';
  shotVisible.value = true;
}

async function handleProcesses(row: any) {
  const res = await sendRemoteForResult(row, 'process-list');
  if (!res) return;
  procList.value = [];
  procMachine.value = row.machine_code;
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
  // Không có row ở đây — dùng lại machine_code đang mở. Gửi bằng client.post
  // trực tiếp để kèm payload {process}.
  const machine = data.value.find((m: any) => m.machine_code === procMachine.value);
  if (!machine) return;
  procLoading.value = true;
  try {
    await client.post(`/machines/${machine.id}/remote/process-kill`, { process: name });
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    procLoading.value = false;
  }
}

function onScreenshot(payload: any) {
  if (payload?.request_id !== shotReqId.value) return;
  shotImage.value = payload.image || '';
}

function onProcesses(payload: any) {
  if (payload?.request_id !== procReqId.value) return;
  procList.value = payload.processes || [];
  procLoading.value = false;
  if (typeof payload.killed === 'number' && payload.killed === -1) {
    // -1 nghĩa là tiến trình nằm trong danh sách cấm tắt (không phải "không thấy").
  } else if (typeof payload.killed === 'number' && payload.killed >= 0) {
    ElMessage.success($t('vnetPages.machines.remote.killed', { n: payload.killed }));
  }
}

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const formRef = ref<FormInstance>();

const groups = ref<any[]>([]);

const form = ref({
  machine_code: '',
  group_id: null as number | null,
  cpu_name: '',
  gpu_name: '',
  ram_gb: 8,
  storage_gb: 256,
  os_info: '',
  is_active: true
});

const rules: FormRules = {
  machine_code: [
    {
      required: true,
      message: $t('vnetPages.machines.form.codeRequired'),
      trigger: 'blur'
    }
  ],
  group_id: [
    {
      required: true,
      message: $t('vnetPages.machines.form.groupRequired'),
      trigger: 'change'
    }
  ]
};

function statusType(status: string): any {
  const map: Record<string, string> = {
    offline: 'danger',
    available: 'success',
    in_use: 'warning'
  };
  return map[status] || 'info';
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    offline: $t('vnetPages.machines.statusLabels.offline'),
    available: $t('vnetPages.machines.statusLabels.available'),
    in_use: $t('vnetPages.machines.statusLabels.inUse')
  };
  return map[status] || status;
}

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

async function fetchGroups() {
  try {
    const res: any = await client.get('/machine-groups', {
      params: { page_size: 200 }
    });
    groups.value = Array.isArray(res) ? res : res?.items || [];
  } catch {
    groups.value = [];
  }
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/machines', {
      params: {
        page,
        page_size: pageSize,
        search: search.value || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    {
      prop: 'machine_code',
      label: $t('vnetPages.machines.code'),
      width: 130
    },
    {
      prop: 'group',
      label: $t('vnetPages.machines.group'),
      width: 120,
      formatter: (row: any) => row.group?.name || '-'
    },
    {
      prop: 'status',
      label: $t('vnetPages.common.status'),
      width: 110,
      // Máy tạm ngừng vẫn nằm trong danh sách (List không lọc is_active) nên
      // phải nhìn ra được, nếu không nhân viên tưởng máy hỏng.
      formatter: (row: any) =>
        row.is_active === false
          ? h(ElTag, { type: 'info', size: 'small' }, () => $t('vnetPages.machines.suspended'))
          : h(ElTag, { type: statusType(row.status), size: 'small' }, () => statusLabel(row.status))
    },
    { prop: 'cpu_name', label: $t('vnetPages.machines.cpu'), minWidth: 160 },
    { prop: 'gpu_name', label: $t('vnetPages.machines.gpu'), minWidth: 160 },
    {
      prop: 'last_heartbeat',
      label: $t('vnetPages.machines.lastHeartbeat'),
      width: 160,
      formatter: (row: any) => formatDate(row.last_heartbeat)
    }
  ]
});

function searchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = {
    machine_code: '',
    group_id: null,
    cpu_name: '',
    gpu_name: '',
    ram_gb: 8,
    storage_gb: 256,
    os_info: '',
    is_active: true
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    machine_code: row.machine_code || '',
    group_id: row.group?.id ?? row.group_id ?? null,
    cpu_name: row.cpu_name || '',
    gpu_name: row.gpu_name || '',
    ram_gb: row.ram_gb || 8,
    storage_gb: row.storage_gb || 256,
    os_info: row.os_info || '',
    // Máy cũ tạo trước khi có cột này thì backend trả true; !== false để một
    // giá trị thiếu không vô tình hiện thành "đang tạm ngừng".
    is_active: row.is_active !== false
  };
  dialogVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    if (isEdit.value && editingId.value) {
      await client.put(`/machines/${editingId.value}`, form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machines.messages.editSuccess')
      });
    } else {
      const created: any = await client.post('/machines', form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machines.messages.addSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machines.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.machines.messages.deleteConfirm', {
        code: row.machine_code
      }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/machines/${row.id}`);
    ElMessage.success($t('vnetPages.machines.messages.deleteSuccess'));
    getData();
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e?.message || $t('vnetPages.machines.messages.deleteError'));
    }
  }
}

function onMachineStatus() {
  getData();
}

onMounted(() => {
  // fetchGroups vốn được định nghĩa nhưng không ai gọi, nên ô chọn nhóm luôn
  // rỗng và KHÔNG tạo được máy nào từ giao diện — nhóm là trường bắt buộc.
  fetchGroups();
  wsStore.on('machine:status', onMachineStatus);
  // Mở/trả máy đổi cột Trạng thái y như machine:status. Thiếu hai dòng này thì
  // tắt máy từ xa xong bảng vẫn hiện "Đang sử dụng" và nhân viên tưởng lệnh hỏng.
  wsStore.on('session:started', onMachineStatus);
  wsStore.on('session:ended', onMachineStatus);
  wsStore.on('machine:screenshot', onScreenshot);
  wsStore.on('machine:processes', onProcesses);
});

onBeforeUnmount(() => {
  wsStore.off('machine:status', onMachineStatus);
  wsStore.off('session:started', onMachineStatus);
  wsStore.off('session:ended', onMachineStatus);
  wsStore.off('machine:screenshot', onScreenshot);
  wsStore.off('machine:processes', onProcesses);
});
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.machines.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="searchData"
          />
          <ElButton type="primary" @click="searchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :loading="loading"
          :show-delete="false"
          @add="openCreate"
          @refresh="getData"
        />
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.machines.remote.title')" width="230" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" :loading="remoteBusy === `${row.id}:lock`" @click="handleLock(row)">
              {{ $t('vnetPages.machines.remote.lock') }}
            </ElButton>
            <ElButton size="small" :loading="remoteBusy === `${row.id}:unlock`" @click="sendRemote(row, 'unlock')">
              {{ $t('vnetPages.machines.remote.unlock') }}
            </ElButton>
            <ElDropdown style="margin-left: 8px" @command="(cmd: string) => handleRemoteCommand(row, cmd)">
              <ElButton size="small">{{ $t('vnetPages.machines.remote.more') }}</ElButton>
              <template #dropdown>
                <ElDropdownMenu>
                  <ElDropdownItem command="screenshot">{{ $t('vnetPages.machines.remote.screenshot') }}</ElDropdownItem>
                  <ElDropdownItem command="processes">{{ $t('vnetPages.machines.remote.processes') }}</ElDropdownItem>
                  <ElDropdownItem command="message" divided>{{ $t('vnetPages.machines.remote.message') }}</ElDropdownItem>
                  <ElDropdownItem command="block-app" divided>
                    {{ $t('vnetPages.machines.remote.blockApp') }}
                  </ElDropdownItem>
                  <ElDropdownItem command="unblock-app">
                    {{ $t('vnetPages.machines.remote.unblockApp') }}
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
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="340" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" @click="openHardware(row)">
              {{ $t('vnetPages.machines.hardware') }}
            </ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="mt-16px flex justify-center">
        <ElPagination
          v-if="mobilePagination.total"
          layout="total, sizes, prev, pager, next"
          v-bind="mobilePagination"
          @current-change="mobilePagination['current-change']"
          @size-change="mobilePagination['size-change']"
        />
      </div>
    </ElCard>

    <!--
      880px chứ không phải 760: bảy cột cố định cộng lại rộng 790px, nên ở 760
      cột cuối bị cắt mất khỏi mép phải mà không có gì báo.
    -->
    <ElDialog
      v-model="hwVisible"
      :title="$t('vnetPages.machines.hardwareTitle', { code: hwMachine?.machine_code })"
      width="880px"
    >
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        {{ $t('vnetPages.machines.hardwareHint') }}
      </ElAlert>
      <ElTable v-loading="hwLoading" :data="hwRows" border stripe size="small" style="width: 100%" max-height="420">
        <ElTableColumn :label="$t('vnetPages.machines.recordedAt')" width="160">
          <template #default="{ row }">
            {{ row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm') : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.machines.cpuTemp')" width="110" align="right">
          <template #default="{ row }">{{ temp(row.cpu_temp) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.machines.gpuTemp')" width="110" align="right">
          <template #default="{ row }">{{ temp(row.gpu_temp) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.machines.cpuUsage')" width="100" align="right">
          <template #default="{ row }">{{ pct(row.cpu_usage) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.machines.ramUsage')" width="100" align="right">
          <template #default="{ row }">{{ pct(row.ram_usage) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.machines.diskUsage')" width="100" align="right">
          <template #default="{ row }">{{ pct(row.disk_usage) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.machines.uptime')" width="110" align="right">
          <template #default="{ row }">{{ uptime(row.uptime) }}</template>
        </ElTableColumn>
        <template #empty>
          <span style="color: #909399">{{ $t('vnetPages.machines.noHardware') }}</span>
        </template>
      </ElTable>
      <template #footer>
        <ElButton @click="hwVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.machines.edit') : $t('vnetPages.machines.add')"
      width="600px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.machines.code')" prop="machine_code">
          <ElInput v-model="form.machine_code" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.group')" prop="group_id">
          <ElSelect v-model="form.group_id" style="width: 100%" filterable>
            <ElOption v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.cpu')" prop="cpu_name">
          <ElInput v-model="form.cpu_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.gpu')" prop="gpu_name">
          <ElInput v-model="form.gpu_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.ram')" prop="ram_gb">
          <ElInputNumber v-model="form.ram_gb" :min="1" :max="1024" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.disk')" prop="storage_gb">
          <ElInputNumber v-model="form.storage_gb" :min="0" :max="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem v-if="isEdit" :label="$t('vnetPages.machines.active')">
          <div>
            <ElSwitch v-model="form.is_active" />
            <div class="text-12px" style="color: var(--el-text-color-secondary); line-height: 1.4">
              {{ $t('vnetPages.machines.activeHint') }}
            </div>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.os')" prop="os_info">
          <ElInput v-model="form.os_info" :placeholder="$t('vnetPages.machines.osPlaceholder')" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="shotVisible"
      :title="$t('vnetPages.machines.remote.screenshotTitle', { code: shotMachine })"
      width="80%"
      top="4vh"
    >
      <div v-if="!shotImage" style="text-align: center; padding: 60px; color: #909399">
        {{ $t('vnetPages.machines.remote.waiting') }}
      </div>
      <img v-else :src="shotImage" style="width: 100%; display: block; border-radius: 4px" />
    </ElDialog>

    <ElDialog
      v-model="procVisible"
      :title="$t('vnetPages.machines.remote.processesTitle', { code: procMachine })"
      width="600px"
      top="6vh"
    >
      <ElTable v-loading="procLoading" :data="procList" border stripe max-height="60vh">
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

  </div>
</template>
