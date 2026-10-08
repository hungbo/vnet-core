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
import { formatPlayed, formatRemainingHMS } from '@/utils/remaining';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';
import MachineRemoteActions from './modules/machine-remote-actions.vue';

const { t: $t } = useI18n();
const router = useRouter();
const wsStore = useWebSocketStore();

const search = ref('');
// Chỉ hiện máy đang có cảnh báo (tài khoản Windows quản trị, card mạng lạ).
const onlyWarnings = ref(false);

// Cột Phiên chạy từng giây theo đồng hồ của trang, không đợi tải lại danh sách.
const now = ref(Date.now());
let tick: ReturnType<typeof setInterval> | null = null;

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

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const formRef = ref<FormInstance>();

const groups = ref<any[]>([]);

const form = ref({
  machine_code: '',
  group_id: null as number | null,
  is_active: true
});
// Cấu hình máy do máy trạm tự gửi ở nhịp tim đầu tiên — chỉ để xem, không sửa
// tay: gõ vào cũng bị nhịp tim kế tiếp ghi đè.
const hw = ref<any>({});

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
  // Danh sách máy không kèm phiên, nên lấy song song các phiên đang chạy rồi
  // ghép theo machine_id: máy đang dùng thì nhân viên thấy ngay ai ngồi, đã
  // chơi bao lâu và còn bao lâu.
  api: async ({ page, pageSize }) => {
    const [res, active]: any[] = await Promise.all([
      client.get('/machines', {
        params: {
          page,
          page_size: pageSize,
          search: search.value || undefined,
          warning: onlyWarnings.value ? 1 : undefined
        }
      }),
      client.get('/sessions/active').catch(() => [])
    ]);
    const phien = new Map((Array.isArray(active) ? active : []).map((p: any) => [p.machine_id, p]));
    for (const m of res?.items || []) m.session = phien.get(m.id);
    return res;
  },
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
      width: 150,
      // API danh sách máy trả `group_id` chứ KHÔNG kèm object `group`, nên công
      // thức cũ (`row.group?.name`) rỗng với mọi máy — cột này chưa bao giờ hiện
      // tên nhóm. Tra tên từ danh sách nhóm mà trang đã nạp sẵn.
      //
      // Máy không có nhóm thì không tra ra giá nào: phiên mở trên nó tính 0₫/giờ,
      // nên đánh dấu hẳn bằng nhãn vàng thay vì một dấu gạch ngang.
      formatter: (row: any) => {
        const g = groups.value.find((x: any) => x.id === row.group_id);
        if (g) return g.name;
        return h(ElTag, { type: 'warning', size: 'small' }, () => $t('vnetPages.machines.noGroupTag'));
      }
    },
    {
      prop: 'ip_address',
      label: $t('vnetPages.machines.ip'),
      width: 130,
      formatter: (row: any) => row.ip_address || '—'
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
    {
      prop: 'warnings',
      label: $t('vnetPages.machines.warnings'),
      minWidth: 190,
      // Hai dấu hiệu máy trạm tự báo. Tài khoản quản trị là ĐỎ: người ngồi máy
      // gỡ được mọi lớp bảo vệ. Card mạng lạ là VÀNG: chỉ để chủ quán biết.
      formatter: (row: any) => {
        const tags = [];
        if (row.user_is_admin) {
          tags.push(
            h(ElTooltip, { content: $t('vnetPages.machines.warnAdminHint'), placement: 'top' }, () =>
              h(ElTag, { type: 'danger', size: 'small' }, () => $t('vnetPages.machines.warnAdmin'))
            )
          );
        }
        if (row.extra_network) {
          tags.push(
            h(ElTooltip, { content: row.extra_network, placement: 'top' }, () =>
              h(ElTag, { type: 'warning', size: 'small' }, () => $t('vnetPages.machines.warnNetwork'))
            )
          );
        }
        return tags.length ? h('div', { style: 'display:flex;gap:4px;flex-wrap:wrap' }, tags) : '—';
      }
    },
    {
      prop: 'session',
      label: $t('vnetPages.machines.sessionCol'),
      minWidth: 190,
      formatter: (row: any) => {
        const p = row.session;
        if (!p) return '—';
        return h('div', { style: 'line-height:1.4' }, [
          h('div', { style: 'font-weight:600' }, p.member_name || '—'),
          h(
            'div',
            { style: 'font-size:12px;color:var(--el-text-color-secondary)' },
            `${$t('vnetPages.machines.played')} ${formatPlayed(p.started_at, null, now.value)} · ${$t('vnetPages.machines.left')} ${formatRemainingHMS(p.affordable_until, now.value)}`
          )
        ]);
      }
    },
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

// --- Tạo máy hàng loạt ------------------------------------------------------
// Quán 50 máy mà tạo lẻ là 50 lần gõ tay, và mã máy phải khớp TỪNG KÝ TỰ với mã
// ghi trong cấu hình máy trạm — lệch một ký tự thì máy đó vĩnh viễn hiện ngoại
// tuyến, mà triệu chứng lại xuất hiện muộn.
//
// Máy chủ tạo cả lô trong một giao dịch: vướng một mã thì không tạo máy nào. Ở
// đây gọi trước bằng dry_run để báo trước tạo được hay không, thay vì để người
// dùng bấm rồi mới biết.

const batchVisible = ref(false);
const batchSubmitting = ref(false);
const batchChecking = ref(false);
const batchForm = ref({
  prefix: 'PC-',
  from: 1,
  to: 20,
  digits: 2,
  group_id: null as string | null
});
const batchResult = ref<any>(null);
let batchTimer: ReturnType<typeof setTimeout> | null = null;

function openBatch() {
  batchResult.value = null;
  batchVisible.value = true;
  checkBatchSoon();
}

/** Mã sẽ sinh ra, tính ở client để xem trước mà không cần hỏi máy chủ. */
function batchCodes(): string[] {
  const { prefix, from, to, digits } = batchForm.value;
  if (!prefix?.trim() || to < from) return [];
  const out: string[] = [];
  for (let n = from; n <= to && out.length <= 500; n += 1) {
    out.push(`${prefix.trim()}${String(n).padStart(digits || 0, '0')}`);
  }
  return out;
}

function checkBatchSoon() {
  batchResult.value = null;
  if (batchTimer) clearTimeout(batchTimer);
  batchTimer = setTimeout(checkBatch, 400);
}

async function checkBatch() {
  if (!batchCodes().length) return;
  batchChecking.value = true;
  try {
    batchResult.value = await client.post('/machines/batch', { ...batchForm.value, dry_run: true });
  } catch (e: any) {
    // Lỗi khoảng số hoặc mã quá dài: máy chủ nói rõ vì sao, hiện thẳng lên.
    batchResult.value = { error: e?.message || $t('vnetPages.machines.messages.loadError') };
  } finally {
    batchChecking.value = false;
  }
}

/** Lô này tạo được hay không — nguồn cho cả biểu ngữ lẫn trạng thái nút Tạo. */
function batchOK() {
  const r = batchResult.value;
  return Boolean(r) && !r.error && !r.conflicts?.length && !r.deleted?.length;
}

async function submitBatch() {
  batchSubmitting.value = true;
  try {
    const res: any = await client.post('/machines/batch', { ...batchForm.value, dry_run: false });
    ElMessage.success($t('vnetPages.machines.batch.created', { n: res?.created ?? 0 }));
    batchVisible.value = false;
    getData();
  } catch (e: any) {
    // Máy chủ trả 409 kèm danh sách mã vướng. Kiểm lại để biểu ngữ hiện đúng
    // hiện trạng — có thể người khác vừa tạo trùng trong lúc hộp thoại đang mở.
    ElMessage.error(e?.message || $t('vnetPages.machines.batch.failed'));
    checkBatch();
  } finally {
    batchSubmitting.value = false;
  }
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = {
    machine_code: '',
    group_id: null,
    is_active: true
  };
  hw.value = {};
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    machine_code: row.machine_code || '',
    group_id: row.group?.id ?? row.group_id ?? null,
    // Máy cũ tạo trước khi có cột này thì backend trả true; !== false để một
    // giá trị thiếu không vô tình hiện thành "đang tạm ngừng".
    is_active: row.is_active !== false
  };
  hw.value = row;
  dialogVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  // Chỉ gửi mã, nhóm, trạng thái: cấu hình máy do máy trạm báo, máy chủ giữ
  // nguyên trường nào không được gửi.
  const payload = { ...form.value };
  try {
    if (isEdit.value && editingId.value) {
      await client.put(`/machines/${editingId.value}`, payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machines.messages.editSuccess')
      });
    } else {
      const created: any = await client.post('/machines', payload);
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

// Máy trạm vừa báo một dấu hiệu mới (hiện chỉ có card mạng lạ). Hiện thông báo
// không tự tắt: chủ quán có thể không ngồi trước màn hình lúc nó xảy ra.
function onMachineAlert(alert: any) {
  ElNotification({
    title: $t('vnetPages.machines.alertTitle', { code: alert?.machine_code || '' }),
    message: `${$t('vnetPages.machines.warnNetwork')}: ${alert?.detail || ''}`,
    type: 'warning',
    duration: 0
  });
  getData();
}

onMounted(() => {
  // fetchGroups vốn được định nghĩa nhưng không ai gọi, nên ô chọn nhóm luôn
  // rỗng và KHÔNG tạo được máy nào từ giao diện — nhóm là trường bắt buộc.
  fetchGroups();
  tick = setInterval(() => {
    now.value = Date.now();
  }, 1000);
  wsStore.on('machine:status', onMachineStatus);
  // Mở/trả máy đổi cột Trạng thái y như machine:status. Thiếu hai dòng này thì
  // tắt máy từ xa xong bảng vẫn hiện "Đang sử dụng" và nhân viên tưởng lệnh hỏng.
  wsStore.on('session:started', onMachineStatus);
  wsStore.on('session:ended', onMachineStatus);
  wsStore.on('machine:alert', onMachineAlert);
});

onBeforeUnmount(() => {
  if (tick) clearInterval(tick);
  wsStore.off('machine:status', onMachineStatus);
  wsStore.off('session:started', onMachineStatus);
  wsStore.off('session:ended', onMachineStatus);
  wsStore.off('machine:alert', onMachineAlert);
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
          <ElCheckbox v-model="onlyWarnings" style="margin-left: 8px" @change="searchData">
            {{ $t('vnetPages.machines.onlyWarnings') }}
          </ElCheckbox>
        </div>
        <div style="display: flex; gap: 8px; align-items: center">
          <ElButton @click="openBatch">{{ $t('vnetPages.machines.batch.open') }}</ElButton>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-delete="false"
            @add="openCreate"
            @refresh="getData"
          />
        </div>
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.machines.remote.title')" width="110" fixed="right">
          <template #default="{ row }">
            <MachineRemoteActions :machine="row as any" />
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
        <!-- Mã máy là khoá máy trạm dùng khi gửi heartbeat: đổi ở đây thì máy trạm
             vẫn gửi mã cũ và server tự tạo một máy mới. Chỉ đặt lúc thêm máy. -->
        <ElFormItem :label="$t('vnetPages.machines.code')" prop="machine_code">
          <ElInput v-model="form.machine_code" :disabled="isEdit" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.group')" prop="group_id">
          <ElSelect v-model="form.group_id" style="width: 100%" filterable>
            <ElOption v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="isEdit" :label="$t('vnetPages.machines.active')">
          <div>
            <ElSwitch v-model="form.is_active" />
            <div class="text-12px" style="color: var(--el-text-color-secondary); line-height: 1.4">
              {{ $t('vnetPages.machines.activeHint') }}
            </div>
          </div>
        </ElFormItem>
        <ElAlert v-if="!isEdit" type="info" :closable="false" show-icon :title="$t('vnetPages.machines.specsAuto')" />
        <!-- Máy trạm tự báo ở nhịp tim đầu tiên; chỉ xem, không sửa tay. -->
        <template v-if="isEdit">
          <ElFormItem :label="$t('vnetPages.machines.cpu')">{{ hw.cpu_name || '—' }}</ElFormItem>
          <ElFormItem :label="$t('vnetPages.machines.gpu')">{{ hw.gpu_name || '—' }}</ElFormItem>
          <ElFormItem :label="$t('vnetPages.machines.ram')">{{ hw.ram_gb || '—' }}</ElFormItem>
          <ElFormItem :label="$t('vnetPages.machines.disk')">{{ hw.storage_gb || '—' }}</ElFormItem>
          <ElFormItem :label="$t('vnetPages.machines.os')">{{ hw.os_info || '—' }}</ElFormItem>
          <ElFormItem :label="$t('vnetPages.machines.peripherals')">
            <div v-if="hw.peripherals?.length" style="display: flex; flex-wrap: wrap; gap: 4px">
              <ElTag v-for="p in hw.peripherals" :key="p" size="small" type="info">{{ p }}</ElTag>
            </div>
            <span v-else style="color: #909399">{{ $t('vnetPages.machines.noPeripherals') }}</span>
          </ElFormItem>
        </template>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="batchVisible" :title="$t('vnetPages.machines.batch.title')" width="620px" top="6vh">
      <ElForm :model="batchForm" :label-width="130">
        <ElFormItem :label="$t('vnetPages.machines.batch.prefix')">
          <ElInput v-model="batchForm.prefix" placeholder="PC-" style="width: 180px" @input="checkBatchSoon" />
          <span class="text-12px" style="margin-left: 12px; color: var(--el-text-color-secondary)">
            {{ $t('vnetPages.machines.batch.prefixHint') }}
          </span>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.batch.range')">
          <ElInputNumber v-model="batchForm.from" :min="0" :max="99999" @change="checkBatchSoon" />
          <span style="margin: 0 8px">→</span>
          <ElInputNumber v-model="batchForm.to" :min="0" :max="99999" @change="checkBatchSoon" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.batch.digits')">
          <ElInputNumber v-model="batchForm.digits" :min="0" :max="6" @change="checkBatchSoon" />
          <span class="text-12px" style="margin-left: 12px; color: var(--el-text-color-secondary)">
            {{ $t('vnetPages.machines.batch.digitsHint') }}
          </span>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.group')">
          <ElSelect v-model="batchForm.group_id" clearable filterable style="width: 100%">
            <ElOption v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <ElAlert type="info" :closable="false" show-icon :title="$t('vnetPages.machines.specsAuto')" style="margin-bottom: 12px" />

      <ElAlert v-if="batchChecking" type="info" :closable="false" :title="$t('vnetPages.machines.batch.checking')" />
      <ElAlert v-else-if="batchResult?.error" type="error" :closable="false" show-icon :title="batchResult.error" />
      <ElAlert
        v-else-if="batchResult && batchOK()"
        type="success"
        :closable="false"
        show-icon
        :title="$t('vnetPages.machines.batch.canCreate', { n: batchResult.codes?.length ?? 0 })"
        :description="`${batchCodes()[0]} … ${batchCodes()[batchCodes().length - 1]}`"
      />
      <ElAlert
        v-else-if="batchResult"
        type="error"
        :closable="false"
        show-icon
        :title="$t('vnetPages.machines.batch.cannotCreate')"
      >
        <div v-if="batchResult.conflicts?.length">
          {{ $t('vnetPages.machines.batch.conflicts', { n: batchResult.conflicts.length }) }}:
          {{ batchResult.conflicts.slice(0, 12).join(', ') }}
        </div>
        <div v-if="batchResult.deleted?.length" style="margin-top: 4px">
          {{ $t('vnetPages.machines.batch.deleted', { n: batchResult.deleted.length }) }}:
          {{ batchResult.deleted.slice(0, 12).join(', ') }}
        </div>
      </ElAlert>

      <template #footer>
        <ElButton @click="batchVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="batchSubmitting" :disabled="!batchOK()" @click="submitBatch">
          {{ $t('vnetPages.machines.batch.submit') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
