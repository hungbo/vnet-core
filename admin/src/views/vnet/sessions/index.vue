<script setup lang="ts">
import { h, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import { formatRemaining } from '@/utils/remaining';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

// Cột "Còn lại" phải nhúc nhích, nếu không nhân viên tưởng số bị treo.
const now = ref(Date.now());
let tick: ReturnType<typeof setInterval> | null = null;

const total = ref(0);

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

function formatMoney(v: number | null | undefined) {
  return `${new Intl.NumberFormat('vi-VN').format(v || 0)}₫`;
}

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: async () => {
    const res: any = await client.get('/sessions/active');
    const items = Array.isArray(res) ? res : res?.items || [];
    total.value = items.length;
    return { items };
  },
  transform: vnetSimpleTransform,
  columns: () => [
    {
      prop: 'machine_code',
      label: $t('vnetPages.sessions.machineCode'),
      width: 120
    },
    {
      prop: 'member_name',
      label: $t('vnetPages.sessions.member'),
      minWidth: 160
    },
    {
      prop: 'started_at',
      label: $t('vnetPages.sessions.startTime'),
      width: 160,
      formatter: (row: any) => formatDate(row.started_at)
    },
    {
      prop: 'duration_minutes',
      label: $t('vnetPages.sessions.duration'),
      width: 100,
      formatter: (row: any) => (row.duration_minutes != null ? `${row.duration_minutes}p` : '')
    },
    {
      // Nhân viên cần biết máy nào SẮP hết tiền, không chỉ máy nào đã ngồi lâu.
      prop: 'affordable_until',
      label: $t('vnetPages.sessions.remaining'),
      width: 110,
      formatter: (row: any) => (row.is_active ? formatRemaining(row.affordable_until, now.value) : '—')
    },
    {
      prop: 'is_active',
      label: $t('vnetPages.sessions.status'),
      width: 110,
      formatter: (row: any) =>
        h(ElTag, { type: row.is_active ? 'warning' : 'info', size: 'small' }, () =>
          row.is_active ? $t('vnetPages.sessions.running') : $t('vnetPages.sessions.ended')
        )
    }
  ]
});

// --- Mở máy ------------------------------------------------------------------
// Thao tác cốt lõi nhất của tiệm net, trước đây không làm được từ trang quản trị.

const startVisible = ref(false);
const starting = ref(false);
const machines = ref<any[]>([]);
const members = ref<any[]>([]);
const combos = ref<any[]>([]);
const startForm = ref({
  machine_id: '',
  member_id: '',
  combo_purchase_id: '',
  duration: 60
});
const estimate = ref<any>(null);

async function loadPickers() {
  const [m, mem] = await Promise.all([
    (client.get('/machines', { params: { page_size: 100 } }) as Promise<any>).catch(() => null),
    (client.get('/members', { params: { page_size: 100 } }) as Promise<any>).catch(() => null)
  ]);
  const machineList: any[] = Array.isArray(m) ? m : m?.items || [];
  // Chỉ loại máy đang có người dùng. Máy chưa gửi tín hiệu vẫn hiện ra vì
  // backend cho phép mở — lọc theo "available" sẽ chặn cả máy chưa cài agent.
  machines.value = machineList.filter(x => x.status !== 'in_use');
  members.value = Array.isArray(mem) ? mem : mem?.items || [];
}

async function loadCombos(memberId: string) {
  combos.value = [];
  if (!memberId) return;
  const res: any = await client.get(`/members/${memberId}/combos`).catch(() => null);
  const list: any[] = Array.isArray(res) ? res : res?.items || [];
  // Gói đã kích hoạt và còn phút mới dùng để mở máy được.
  combos.value = list.filter(c => c.activated && (c.remaining_minutes ?? 0) > 0);
}

async function openStart() {
  startForm.value = {
    machine_id: '',
    member_id: '',
    combo_purchase_id: '',
    duration: 60
  };
  estimate.value = null;
  combos.value = [];
  await loadPickers();
  startVisible.value = true;
}

function onMemberChange(id: string) {
  startForm.value.combo_purchase_id = '';
  loadCombos(id);
}

async function calcEstimate() {
  const { machine_id, member_id, duration } = startForm.value;
  if (!machine_id || !duration) return;
  estimate.value = await client
    .get('/sessions/calculate-cost', {
      params: { machine_id, member_id, duration_minutes: duration }
    })
    .catch(() => null);
}

async function submitStart() {
  if (!startForm.value.machine_id || !startForm.value.member_id) {
    ElMessage.warning($t('vnetPages.sessions.selectMachine'));
    return;
  }
  starting.value = true;
  try {
    await client.post('/sessions/start', {
      machine_id: startForm.value.machine_id,
      member_id: startForm.value.member_id,
      combo_purchase_id: startForm.value.combo_purchase_id || undefined
    });
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.sessions.startSuccess')
    });
    startVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.common.error'));
  } finally {
    starting.value = false;
  }
}

// --- Đổi máy -----------------------------------------------------------------

const switchVisible = ref(false);
const switching = ref(false);
const switchingSession = ref<any>(null);
const switchTarget = ref('');

async function openSwitch(row: any) {
  switchingSession.value = row;
  switchTarget.value = '';
  await loadPickers();
  switchVisible.value = true;
}

async function submitSwitch() {
  if (!switchTarget.value) {
    ElMessage.warning($t('vnetPages.sessions.newMachine'));
    return;
  }
  switching.value = true;
  try {
    await client.post(`/sessions/${switchingSession.value.id}/switch-machine`, {
      new_machine_id: switchTarget.value
    });
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.sessions.switchSuccess')
    });
    switchVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.common.error'));
  } finally {
    switching.value = false;
  }
}

// --- Trả máy -----------------------------------------------------------------

async function handleEnd(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.sessions.messages.endConfirm', {
        member: row.member_name,
        code: row.machine_code
      }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }

  try {
    const res: any = await client.post(`/sessions/${row.id}/end`);
    // Trả máy luôn thành công kể cả khi hội viên không đủ tiền; phần thiếu
    // được ghi nợ nên nhân viên cần thấy con số đó ngay.
    const unpaid = res?.amount_unpaid || 0;
    ElNotification({
      type: unpaid > 0 ? 'warning' : 'success',
      title: $t('vnetPages.common.success'),
      message:
        unpaid > 0
          ? `${$t('vnetPages.sessions.messages.endSuccess')} — ${$t('vnetPages.members.debt')}: ${formatMoney(unpaid)}`
          : $t('vnetPages.sessions.messages.endSuccess')
    });
    getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.common.error'));
  }
}
const wsStore = useWebSocketStore();

// Nhịp setInterval bên dưới chỉ đếm lại số phút đã chơi trên các dòng SẴN CÓ —
// nó không gọi lại danh sách. Máy mở hay trả ở quầy bên cạnh, hoặc ở thiết bị
// khác của chính người đang ngồi đây, thì bảng này đứng yên cho tới khi tải lại.
function onSessionChanged() {
  getData();
}

onMounted(() => {
  tick = setInterval(() => {
    now.value = Date.now();
  }, 1000);
  wsStore.on('session:started', onSessionChanged);
  wsStore.on('session:ended', onSessionChanged);
});

onBeforeUnmount(() => {
  if (tick) clearInterval(tick);
  wsStore.off('session:started', onSessionChanged);
  wsStore.off('session:ended', onSessionChanged);
});
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.sessions.activeSessions') }}</span>
          <div style="display: flex; gap: 8px; align-items: center">
            <ElButton type="primary" @click="openStart">{{ $t('vnetPages.sessions.start') }}</ElButton>
            <TableHeaderOperation
              v-model:columns="columnChecks"
              :loading="loading"
              :show-add="false"
              :show-delete="false"
              @refresh="getData"
            />
          </div>
        </div>
      </template>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="200" fixed="right">
          <template #default="{ row }">
            <template v-if="row.is_active">
              <ElButton size="small" @click="openSwitch(row)">{{ $t('vnetPages.sessions.switchMachine') }}</ElButton>
              <ElButton size="small" type="danger" @click="handleEnd(row)">
                {{ $t('vnetPages.sessions.end') }}
              </ElButton>
            </template>
            <span v-else>-</span>
          </template>
        </ElTableColumn>
      </ElTable>

      <div v-if="total > 0" style="margin-top: 16px; text-align: center; color: #909399; font-size: 13px">
        {{ $t('vnetPages.sessions.totalActive', { count: total }) }}
      </div>
    </ElCard>

    <ElDialog v-model="startVisible" :title="$t('vnetPages.sessions.start')" width="480px">
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.sessions.selectMachine')">
          <ElSelect v-model="startForm.machine_id" filterable style="width: 100%" @change="calcEstimate">
            <ElOption
              v-for="m in machines"
              :key="m.id"
              :label="m.status === 'offline' ? `${m.machine_code} (offline)` : m.machine_code"
              :value="m.id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.sessions.selectMember')">
          <ElSelect v-model="startForm.member_id" filterable style="width: 100%" @change="onMemberChange">
            <ElOption
              v-for="m in members"
              :key="m.id"
              :label="`${m.full_name || m.username} — ${formatMoney(m.balance)}`"
              :value="m.id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.sessions.selectCombo')">
          <ElSelect
            v-model="startForm.combo_purchase_id"
            clearable
            :placeholder="$t('vnetPages.sessions.noCombo')"
            style="width: 100%"
          >
            <ElOption
              v-for="c in combos"
              :key="c.id"
              :label="`${c.combo_name} — ${c.remaining_minutes}p`"
              :value="c.id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.sessions.durationMinutes')">
          <div style="display: flex; gap: 8px; width: 100%">
            <ElInputNumber v-model="startForm.duration" :min="1" :max="1440" style="flex: 1" />
            <ElButton @click="calcEstimate">{{ $t('vnetPages.sessions.estimate') }}</ElButton>
          </div>
        </ElFormItem>
        <ElFormItem v-if="estimate" :label="$t('vnetPages.sessions.costPreview')">
          <div>
            <div>
              {{ formatMoney(estimate.price_per_hour) }}/h ·
              {{ estimate.machine_group_name }}
            </div>
            <div v-if="estimate.discount_amount > 0" style="color: #67c23a">
              -{{ estimate.discount_percent }}% = -{{ formatMoney(estimate.discount_amount) }}
            </div>
            <strong>{{ formatMoney(estimate.final_cost) }}</strong>
          </div>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="startVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="starting" @click="submitStart">
          {{ $t('vnetPages.sessions.start') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="switchVisible" :title="$t('vnetPages.sessions.switchMachine')" width="420px">
      <ElForm label-width="130px">
        <ElFormItem :label="$t('vnetPages.sessions.machineCode')">
          <strong>{{ switchingSession?.machine_code }}</strong>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.sessions.newMachine')">
          <ElSelect v-model="switchTarget" filterable style="width: 100%">
            <ElOption v-for="m in machines" :key="m.id" :label="m.machine_code" :value="m.id" />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="switchVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="switching" @click="submitSwitch">
          {{ $t('vnetPages.sessions.switchMachine') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
