<script setup lang="ts">
// Tiền vào: mọi thông báo biến động số dư mà điện thoại báo tiền ở quầy gửi về
// (máy chủ: qr_pay.go). Khoản khớp mã nạp đã cộng tự động; còn lại nhân viên
// duyệt, cộng cho đúng hội viên, hoặc bỏ qua. Giữ nguyên văn để đối chiếu sao kê.
// Bảng thứ hai "Mã QR đã tạo": nhật ký mọi mã đưa cho khách, tạo lúc nào và bao
// lâu sau thì tiền vào.
import { computed, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAuth } from '@/hooks/business/auth';
import { useQRStatus } from '@/hooks/business/qr-status';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useTodayRange } from '@/hooks/common/today-filter';
import { useFitPageSize } from '@/hooks/common/table';
import { formatPrice } from '@/utils/money';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();
const { status: qrStatus } = useQRStatus();

const STATUS_TAG: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'primary'> = {
  matched: 'success',
  assigned: 'success',
  suggested: 'warning',
  amount_mismatch: 'danger',
  unmatched: 'danger',
  ignored: 'info',
  unparsed: 'info'
};
const STATUSES = Object.keys(STATUS_TAG);

const QR_STATUS_TAG: Record<string, 'success' | 'warning' | 'info'> = {
  pending: 'warning',
  paid: 'success',
  expired: 'info',
  cancelled: 'info'
};

const view = ref<'in' | 'qr'>('in');
const statusOptions = computed(() =>
  view.value === 'qr'
    ? Object.keys(QR_STATUS_TAG).map(s => ({ value: s, label: $t(`vnetPages.qrPay.qrStatuses.${s}`) }))
    : STATUSES.map(s => ({ value: s, label: $t(`vnetPages.qrPay.statuses.${s}`) }))
);

const list = ref<any[]>([]);
const loading = ref(false);
const page = ref(1);
const pageSize = ref(20);
const fitter = useFitPageSize({
  getPageSize: () => pageSize.value,
  setPageSize: size => {
    pageSize.value = size;
    page.value = 1;
    fetchData();
  }
});
const total = ref(0);
const search = ref('');
const statusFilter = ref('');
// Mặc định hôm nay; xoá ô ngày để xem mọi ngày.
const { range: dateRange, follow: followToday } = useTodayRange();
const sort = ref<string>();
const order = ref<'asc' | 'desc'>();

function onSortChange(e: { prop: string | null; order: 'ascending' | 'descending' | null }) {
  if (e.order && e.prop) {
    sort.value = e.prop;
    order.value = e.order === 'ascending' ? 'asc' : 'desc';
  } else {
    sort.value = undefined;
    order.value = undefined;
  }
  page.value = 1;
  fetchData();
}

async function fetchData() {
  followToday();
  loading.value = true;
  try {
    const params: any = { page: page.value, page_size: pageSize.value, sort: sort.value, order: order.value };
    if (search.value) params.search = search.value;
    if (statusFilter.value) params.status = statusFilter.value;
    if (dateRange.value) {
      params.date_from = dateRange.value[0];
      params.date_to = dateRange.value[1];
    }
    const res: any = await client.get(view.value === 'qr' ? '/qr/requests' : '/bank-notifications', { params });
    list.value = res.items || [];
    total.value = res.total || 0;
    await fitter.afterLoad();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  if (!fitter.fit()) fetchData();
});

// Thông báo mới tới (bank-notifications), đơn nạp được duyệt/huỷ ở nơi khác,
// mã QR được tạo/huỷ/hết hạn (qr).
useRealtimeRefresh(['bank-notifications', 'orders', 'members', 'qr'], fetchData);

function switchView() {
  list.value = [];
  statusFilter.value = '';
  sort.value = undefined;
  order.value = undefined;
  page.value = 1;
  fetchData();
}

function when(v?: string | null) {
  return v ? dayjs(v).format('DD/MM HH:mm:ss') : '-';
}

/** 75 → "1:15", 3725 → "1:02:05". */
function duration(sec?: number | null) {
  if (sec === null || sec === undefined) return '-';
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = String(sec % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
}

/** Nhân viên duyệt tay thì tiền đã báo về trước lúc hoàn thành: hiện riêng lúc tiền về. */
function bankArrival(row: any) {
  if (!row.bank_received_at || !row.paid_at) return '';
  if (dayjs(row.paid_at).diff(row.bank_received_at, 'second') < 5) return '';
  return duration(dayjs(row.bank_received_at).diff(row.created_at, 'second'));
}

async function act(fn: () => Promise<unknown>) {
  try {
    await fn();
    ElMessage.success($t('vnetPages.qrPay.done'));
    fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

function confirmRow(row: any) {
  act(() => client.post(`/bank-notifications/${row.id}/confirm`));
}

async function ignoreRow(row: any) {
  try {
    await ElMessageBox.confirm($t('vnetPages.qrPay.ignoreConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return;
  }
  act(() => client.post(`/bank-notifications/${row.id}/ignore`, {}));
}

// --- Cộng cho hội viên ---------------------------------------------------------
const assignVisible = ref(false);
const assignRow = ref<any>(null);
const assignMember = ref('');
const members = ref<any[]>([]);
const membersLoading = ref(false);

async function searchMembers(query: string) {
  if (!query) {
    members.value = [];
    return;
  }
  membersLoading.value = true;
  try {
    const res: any = await client.get('/members', { params: { page: 1, page_size: 20, search: query } });
    members.value = res.items || [];
  } catch {
    members.value = [];
  } finally {
    membersLoading.value = false;
  }
}

function openAssign(row: any) {
  assignRow.value = row;
  // Mã nạp hết hạn hay lệch số tiền vẫn biết của ai: chọn sẵn hội viên đó.
  assignMember.value = row.member_id || '';
  members.value = row.member_id ? [{ id: row.member_id, username: row.member_name }] : [];
  assignVisible.value = true;
}

async function submitAssign() {
  if (!assignRow.value || !assignMember.value) return;
  await act(() => client.post(`/bank-notifications/${assignRow.value.id}/assign`, { member_id: assignMember.value }));
  assignVisible.value = false;
}

const canAct = (row: any, ...states: string[]) => hasAuth('members.topup') && states.includes(row.status);
</script>

<template>
  <div class="page-fill">
    <ElCard class="fill-card">
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-8px">
          <div class="flex flex-wrap items-center gap-8px">
            <ElRadioGroup v-model="view" @change="switchView">
              <ElRadioButton value="in">{{ $t('vnetPages.qrPay.title') }}</ElRadioButton>
              <ElRadioButton value="qr">{{ $t('vnetPages.qrPay.qrLog') }}</ElRadioButton>
            </ElRadioGroup>
            <ElTag v-if="qrStatus && !qrStatus.paired" type="info">{{ $t('vnetPages.qrPay.notPaired') }}</ElTag>
            <ElTag v-else-if="qrStatus" :type="qrStatus.phone_online ? 'success' : 'danger'">
              {{ $t('vnetPages.qrPay.phone') }}:
              {{ qrStatus.phone_online ? $t('vnetPages.qrPay.online') : $t('vnetPages.qrPay.offline') }}
              <template v-if="qrStatus.last_seen">
                · {{ $t('vnetPages.qrPay.battery') }} {{ qrStatus.battery }}%
              </template>
            </ElTag>
          </div>
          <div class="flex flex-wrap items-center gap-8px">
            <ElDatePicker
              v-model="dateRange"
              type="daterange"
              range-separator="→"
              value-format="YYYY-MM-DD"
              clearable
              style="width: 260px"
              @change="fetchData"
            />
            <ElSelect
              v-model="statusFilter"
              :placeholder="$t('vnetPages.qrPay.allStatuses')"
              clearable
              style="width: 160px"
              @change="fetchData"
            >
              <ElOption v-for="s in statusOptions" :key="s.value" :label="s.label" :value="s.value" />
            </ElSelect>
            <ElInput
              v-model="search"
              :placeholder="
                $t(view === 'qr' ? 'vnetPages.qrPay.qrSearchPlaceholder' : 'vnetPages.qrPay.searchPlaceholder')
              "
              clearable
              style="width: 220px"
              @keyup.enter="fetchData"
            />
            <ElButton type="primary" @click="fetchData">{{ $t('vnetPages.common.search') }}</ElButton>
          </div>
        </div>
      </template>
      <ElTable
        v-if="view === 'qr'"
        v-loading="loading"
        :data="list"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn prop="created_at" :label="$t('vnetPages.qrPay.createdAt')" width="140" sortable="custom">
          <template #default="{ row }">{{ when(row.created_at) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="code" :label="$t('vnetPages.qrPay.code')" width="110" sortable="custom">
          <template #default="{ row }">
            <span class="mono">{{ row.code }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="member_name" :label="$t('vnetPages.qrPay.member')" min-width="130" sortable="custom">
          <template #default="{ row }">
            {{ row.member_name || '-' }}
            <div v-if="row.machine_name" class="sub">{{ row.machine_name }}</div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.qrPay.order')" min-width="150">
          <template #default="{ row }">
            {{ row.order_code || '-' }}
            <div class="sub">
              {{ $t(row.order_type === 'topup' ? 'vnetPages.qrPay.forTopup' : 'vnetPages.qrPay.forOrder') }}
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="amount" :label="$t('vnetPages.qrPay.amount')" width="120" align="right" sortable="custom">
          <template #default="{ row }">{{ formatPrice(row.amount) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="status" :label="$t('vnetPages.qrPay.status')" width="120" sortable="custom">
          <template #default="{ row }">
            <ElTag :type="QR_STATUS_TAG[row.status] || 'info'" size="small">
              {{ $t(`vnetPages.qrPay.qrStatuses.${row.status}`) }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="paid_at" :label="$t('vnetPages.qrPay.paidAt')" width="140" sortable="custom">
          <template #default="{ row }">{{ when(row.paid_at) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="duration_seconds" :label="$t('vnetPages.qrPay.duration')" width="150" sortable="custom">
          <template #default="{ row }">
            <b v-if="row.duration_seconds !== null">{{ duration(row.duration_seconds) }}</b>
            <span v-else>-</span>
            <div v-if="bankArrival(row)" class="sub">
              {{ $t('vnetPages.qrPay.bankArrivedAfter', { time: bankArrival(row) }) }}
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.qrPay.createdBy')" width="120">
          <template #default="{ row }">
            {{
              row.created_by_name ||
              (row.created_by && row.created_by === row.member_id ? $t('vnetPages.qrPay.selfCreated') : '-')
            }}
          </template>
        </ElTableColumn>
      </ElTable>
      <ElTable
        v-else
        v-loading="loading"
        :data="list"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn type="expand" width="36">
          <template #default="{ row }">
            <div class="raw">
              <b>{{ $t('vnetPages.qrPay.rawText') }}:</b>
              {{ row.title }} — {{ row.text }}
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="received_at" :label="$t('vnetPages.qrPay.receivedAt')" width="140" sortable="custom">
          <template #default="{ row }">{{ when(row.received_at) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="amount" :label="$t('vnetPages.qrPay.amount')" width="120" align="right" sortable="custom">
          <template #default="{ row }">
            <span :class="row.incoming ? 'money-in' : 'money-out'">
              {{ row.incoming ? '+' : '-' }}{{ formatPrice(row.amount) }}
            </span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.qrPay.sender')" min-width="150">
          <template #default="{ row }">
            {{ row.sender_name || '-' }}
            <div v-if="row.sender_account" class="sub">{{ row.sender_account }}</div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="content" :label="$t('vnetPages.qrPay.contentCol')" min-width="220" show-overflow-tooltip />
        <ElTableColumn prop="code" :label="$t('vnetPages.qrPay.code')" width="110" sortable="custom">
          <template #default="{ row }">
            <span class="mono">{{ row.code || '-' }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="member_name" :label="$t('vnetPages.qrPay.member')" width="130" sortable="custom">
          <template #default="{ row }">{{ row.member_name || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="status" :label="$t('vnetPages.qrPay.status')" width="130" sortable="custom">
          <template #default="{ row }">
            <ElTag :type="STATUS_TAG[row.status] || 'info'" size="small">
              {{ $t(`vnetPages.qrPay.statuses.${row.status}`) }}
            </ElTag>
            <div v-if="row.note" class="sub">{{ row.note }}</div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.qrPay.handledBy')" width="110">
          <template #default="{ row }">{{ row.handled_by_name || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="230" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <ElButton v-if="canAct(row, 'suggested')" size="small" type="success" @click="confirmRow(row)">
                {{ $t('vnetPages.qrPay.confirm') }}
              </ElButton>
              <ElButton
                v-if="canAct(row, 'unmatched', 'amount_mismatch') && row.incoming"
                size="small"
                type="primary"
                @click="openAssign(row)"
              >
                {{ $t('vnetPages.qrPay.assign') }}
              </ElButton>
              <ElButton
                v-if="canAct(row, 'unmatched', 'amount_mismatch', 'unparsed', 'suggested')"
                size="small"
                @click="ignoreRow(row)"
              >
                {{ $t('vnetPages.qrPay.ignore') }}
              </ElButton>
            </div>
          </template>
        </ElTableColumn>
      </ElTable>
      <div class="mt-12px flex justify-center">
        <ElPagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="fetchData"
        />
      </div>
    </ElCard>

    <ElDialog
      v-model="assignVisible"
      :title="$t('vnetPages.qrPay.assignTitle', { amount: formatPrice(assignRow?.amount) })"
      width="420px"
    >
      <ElSelect
        v-model="assignMember"
        filterable
        remote
        clearable
        :remote-method="searchMembers"
        :loading="membersLoading"
        :placeholder="$t('vnetPages.qrPay.selectMember')"
        class="w-full"
      >
        <ElOption
          v-for="m in members"
          :key="m.id"
          :label="`${m.full_name || m.username}${m.phone ? ` — ${m.phone}` : ''}`"
          :value="m.id"
        />
      </ElSelect>
      <div v-if="assignRow" class="sub mt-8px">{{ assignRow.content }}</div>
      <template #footer>
        <ElButton @click="assignVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :disabled="!assignMember" @click="submitAssign">
          {{ $t('vnetPages.qrPay.assign') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
.money-in {
  color: var(--el-color-success);
  font-weight: 600;
}
.money-out {
  color: var(--el-text-color-secondary);
}
.sub {
  font-size: 12px;
  line-height: 1.4;
  color: var(--el-text-color-secondary);
}
.mono {
  font-family: monospace;
}
.raw {
  padding: 4px 16px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
}
.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.row-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}
</style>
