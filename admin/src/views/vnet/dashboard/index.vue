<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useAuth } from '@/hooks/business/auth';
import { coTheHuy, coTheThanhToan, coTheXacNhan, tongTien, useOrderBatch } from '@/hooks/business/order-batch';
import { usePaymentMethods } from '@/hooks/business/payment-methods';
import { formatPlayed, formatRemainingHMS } from '@/utils/remaining';
import { formatPrice } from '@/utils/money';
import MachineRemoteActions from '../machines/modules/machine-remote-actions.vue';
import MachineNoteCell from '../machines/modules/machine-note-cell.vue';
import MachineOpenOrdersTag from '../machines/modules/machine-open-orders-tag.vue';
import { OPEN_ORDERS_ROW_CLASS, fetchOpenOrders } from '../machines/modules/machine-open-orders';
import MemberTopupDialog from '../members/modules/member-topup-dialog.vue';

const { t: $t } = useI18n();
// Nút thiếu quyền của API tương ứng thì ẩn: nạp tiền (members.topup), sửa ghi
// chú máy (machines.update), xem đơn (orders.view), xác nhận/huỷ đơn
// (orders.status), thu tiền đơn (orders.pay).
const { hasAuth } = useAuth();

const stats = ref([
  {
    label: 'vnetPages.dashboard.stats.members',
    value: 0,
    icon: 'User',
    color: '#409eff'
  },
  {
    label: 'vnetPages.dashboard.stats.onlineMachines',
    value: 0,
    icon: 'Monitor',
    color: '#67c23a'
  },
  {
    label: 'vnetPages.dashboard.stats.playing',
    value: 0,
    icon: 'Timer',
    color: '#e6a23c'
  },
  {
    label: 'vnetPages.dashboard.stats.revenueToday',
    value: '0₫',
    icon: 'Money',
    color: '#f56c6c'
  }
]);

const activeSessions = ref<any[]>([]);
// Bảng "Máy đang hoạt động": máy đang chơi (lấy từ phiên) và máy sẵn sàng (đã
// bật, chưa ai ngồi). Máy sẵn sàng không có phiên nên các cột phiên để trống.
// Máy đã tắt mà còn đơn chưa thu tiền cũng ở lại bảng: đó đúng là máy khách vừa
// bỏ về, mất khỏi bảng thì nhân viên không còn thấy nó nữa.
const activeMachines = ref<any[]>([]);

const statusTags: Record<string, { type: 'warning' | 'success' | 'danger'; label: string }> = {
  in_use: { type: 'warning', label: 'vnetPages.machines.statusLabels.inUse' },
  available: { type: 'success', label: 'vnetPages.machines.statusLabels.available' },
  offline: { type: 'danger', label: 'vnetPages.machines.statusLabels.offline' }
};
const router = useRouter();

// --- Thao tác trên từng máy đang chơi ----------------------------------------
// Khách đang ngồi máy hỏi nạp tiền hay hỏi món tới chưa: nhân viên làm ngay tại
// dòng đó, không phải đi tìm hội viên hay đơn ở trang khác.

const topupVisible = ref(false);
const topupRow = ref<any>(null);

function openTopup(row: any) {
  topupRow.value = row;
  topupVisible.value = true;
}

const ordersVisible = ref(false);
const ordersLoading = ref(false);
const ordersRow = ref<any>(null);
const orders = ref<any[]>([]);

// Chọn nhiều đơn để xác nhận hay huỷ một lần; tổng tiền ở chân hộp.
const ordersTableRef = ref<any>(null);
const ordersSelected = ref<any[]>([]);
const ordersBusy = ref(false);
const ordersBatch = useOrderBatch();
const selectedConfirmable = computed(() => ordersSelected.value.filter(coTheXacNhan));
const selectedCancellable = computed(() => ordersSelected.value.filter(coTheHuy));
// Máy khách đã bỏ về thì dòng không còn hội viên: lấy tên từ chính các đơn.
const ordersMember = computed(
  () =>
    ordersRow.value?.member_name ||
    [...new Set(orders.value.map(o => o.member_name || o.member_username).filter(Boolean))].join(', ') ||
    '—'
);

// Thu tiền ngay trong hộp: khách ra quầy trả máy thì thanh toán các đơn đang tích,
// không tích đơn nào là tất cả — không phải sang trang Đơn hàng dò từng đơn.
const { enabledMethods } = usePaymentMethods();
const ordersPayable = computed(() =>
  (ordersSelected.value.length ? ordersSelected.value : orders.value).filter(coTheThanhToan)
);
const payVisible = ref(false);
// Chốt danh sách lúc mở hộp: chưa tích gì thì bảng vẫn tải lại theo thời gian thực,
// đơn mới tới giữa chừng không được lẳng lặng gộp vào số tiền đang thu.
const payList = ref<any[]>([]);
const payForm = ref({ payment_method: 'cash', reference_code: '' });

function openPay() {
  payList.value = [...ordersPayable.value];
  payForm.value = { payment_method: 'cash', reference_code: '' };
  payVisible.value = true;
}

async function submitPay() {
  ordersBusy.value = true;
  try {
    await ordersBatch.thanhToan(payList.value, payForm.value);
    payVisible.value = false;
    await afterOrdersBatch();
  } finally {
    ordersBusy.value = false;
  }
}

async function afterOrdersBatch() {
  ordersTableRef.value?.clearSelection();
  await loadOrders();
}

async function batchConfirmOrders() {
  ordersBusy.value = true;
  try {
    await ordersBatch.xacNhan(selectedConfirmable.value);
    await afterOrdersBatch();
  } finally {
    ordersBusy.value = false;
  }
}

async function batchCancelOrders() {
  const list = selectedCancellable.value;
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.orders.batchCancelConfirm', { count: list.length, total: formatPrice(tongTien(list)) }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  ordersBusy.value = true;
  try {
    await ordersBatch.huy(list);
    await afterOrdersBatch();
  } finally {
    ordersBusy.value = false;
  }
}

async function openOrders(row: any) {
  ordersRow.value = row;
  orders.value = [];
  ordersSelected.value = [];
  ordersVisible.value = true;
  await loadOrders();
}

async function loadOrders() {
  const row = ordersRow.value;
  if (!row) return;
  ordersLoading.value = true;
  try {
    // Theo MÁY chứ không theo hội viên: máy khách vừa bỏ về hay đã tắt không còn
    // phiên, không còn hội viên, mà đơn chưa thu tiền vẫn nằm đó. API chỉ trả đơn
    // chờ duyệt hoặc đã xác nhận mà chưa thanh toán.
    const res: any = await client.get(`/machines/${row.machine_id}/open-orders`, { params: { page_size: 100 } });
    orders.value = Array.isArray(res) ? res : res?.items || [];
  } catch {
    orders.value = [];
  } finally {
    ordersLoading.value = false;
  }
}

function orderStatusType(status: string) {
  return status === 'confirmed' ? 'primary' : 'info';
}

function goOrders() {
  ordersVisible.value = false;
  router.push({ name: 'vnet_orders' });
}

// Hai ô này trước đây hiện chuỗi thô "2026-08-22T19:49:52.482957+07:00" và
// "p" rỗng: mốc thời gian không được định dạng, còn cột thời lượng đọc
// row.duration trong khi API trả duration_minutes.
function formatTime(v: string | null | undefined) {
  return v ? dayjs(v).format('DD/MM HH:mm') : '-';
}

function formatMoney(v: number) {
  return `${new Intl.NumberFormat('vi-VN').format(v || 0)}₫`;
}

// Toàn bộ máy: API trả tối đa 100 máy một trang, quán lớn hơn thì tải nốt các
// trang còn lại, nếu không thẻ online/tổng và bảng máy sẵn sàng thiếu máy.
async function fetchAllMachines(): Promise<{ items: any[]; total: number } | null> {
  const pageSize = 100;
  const first: any = await (
    client.get('/machines', { params: { page: 1, page_size: pageSize } }) as Promise<any>
  ).catch(() => null);
  if (!first) return null;
  const items: any[] = Array.isArray(first) ? first : first.items || [];
  const total: number = first.total ?? items.length;
  const pages = Math.ceil(total / pageSize);
  if (pages > 1) {
    const rest = await Promise.all(
      Array.from({ length: pages - 1 }, (_, i) =>
        (client.get('/machines', { params: { page: i + 2, page_size: pageSize } }) as Promise<any>).catch(() => null)
      )
    );
    for (const r of rest) items.push(...(r?.items || []));
  }
  return { items, total };
}

async function refreshStats() {
  // Cả bốn thẻ đều lấy số thật. Trước đây thẻ hội viên và doanh thu là số 0
  // cứng, còn thẻ "máy online" đếm toàn bộ máy kể cả máy đang tắt.
  // Ngày theo giờ máy (giờ Việt Nam). toISOString() là ngày UTC: từ 0h tới 7h
  // sáng nó còn là hôm qua, nên ca đêm thấy "Tiền thu hôm nay" của ngày trước.
  const today = dayjs().format('YYYY-MM-DD');

  const [members, machines, sessions, revenue, donMo] = await Promise.all([
    (client.get('/members', { params: { page_size: 1 } }) as Promise<any>).catch(() => null),
    fetchAllMachines(),
    (client.get('/sessions/active') as Promise<any>).catch(() => null),
    (
      client.get('/reports/daily-revenue', {
        params: { date_from: today, date_to: today }
      }) as Promise<any>
    ).catch(() => null),
    fetchOpenOrders()
  ]);

  stats.value[0].value = members?.total ?? 0;

  const machineList: any[] = machines?.items || [];
  const online = machineList.filter(m => m.status && m.status !== 'offline').length;
  stats.value[1].value = `${online}/${machines?.total ?? machineList.length}`;

  activeSessions.value = Array.isArray(sessions) ? sessions : sessions?.items || [];
  stats.value[2].value = activeSessions.value.length;

  const playing = new Set(activeSessions.value.map(r => r.machine_id));
  // Ghi chú nằm ở máy chứ không ở phiên, nên tra từ danh sách máy vừa tải.
  const noteOf = new Map(machineList.map(m => [m.id, m.note || '']));
  const available = machineList
    .filter(m => (m.status === 'available' || donMo.has(m.id)) && !playing.has(m.id))
    .map(m => ({ machine_id: m.id, machine_code: m.machine_code, status: m.status, note: m.note || '' }))
    .sort((a, b) => String(a.machine_code).localeCompare(String(b.machine_code)));
  activeMachines.value = [
    ...activeSessions.value.map(r => ({ ...r, status: 'in_use', note: noteOf.get(r.machine_id) || '' })),
    ...available
  ].map(r => ({ ...r, open_orders: donMo.get(r.machine_id) }));

  const rows: any[] = Array.isArray(revenue) ? revenue : revenue?.items || [];
  stats.value[3].value = formatMoney(rows.reduce((sum, r) => sum + (r.revenue || 0), 0));
}

// Cột "Còn lại" phải nhúc nhích mỗi giây, nếu không nhân viên tưởng số bị treo.
const now = ref(Date.now());
let tick: ReturnType<typeof setInterval> | null = null;

// Bốn thẻ và bảng máy đổi theo: hội viên thêm/xoá, nạp/hoàn tiền ("members"),
// máy bật/tắt ("machines", machine:status), mở/trả máy và tiền giờ trừ mỗi phút
// ("sessions"), đơn thu tiền mặt ("orders") và gói cước bán tiền mặt ("combos")
// — doanh thu hôm nay cộng cả ba nguồn sau. Gộp 2 giây: trang này mở cả ngày.
useRealtimeRefresh(['members', 'machines', 'sessions', 'orders', 'combos'], refreshStats, {
  events: ['session:started', 'session:ended', 'machine:status'],
  delay: 2000
});

// Hộp "Đơn đang xử lý" của một khách: đơn mới, đổi trạng thái, thanh toán.
// Đang tích chọn thì hoãn: thay dữ liệu bảng là mất hết ô đã tích.
const { resume: resumeOrders } = useRealtimeRefresh(['orders'], () => ordersVisible.value && loadOrders(), {
  events: ['order:new', 'order:updated'],
  paused: () => ordersSelected.value.length > 0
});

function onOrdersSelection(rows: any[]) {
  ordersSelected.value = rows;
  if (!rows.length) resumeOrders();
}

onMounted(async () => {
  tick = setInterval(() => {
    now.value = Date.now();
  }, 1000);
  await refreshStats();
});

onBeforeUnmount(() => {
  if (tick) clearInterval(tick);
});
</script>

<template>
  <div class="page-fill">
    <ElRow :gutter="20" class="stat-cards mb-16px">
      <ElCol v-for="stat in stats" :key="stat.label" :span="6" :xs="12">
        <ElCard shadow="hover" class="h-full">
          <div style="display: flex; justify-content: space-between; align-items: center">
            <div>
              <div style="font-size: 13px; color: #909399">
                {{ $t(stat.label) }}
              </div>
              <div style="font-size: 24px; font-weight: bold; margin-top: 8px">
                {{ stat.value }}
              </div>
            </div>
            <ElIcon :size="40" :color="stat.color" class="lt-sm:hidden"><component :is="stat.icon" /></ElIcon>
          </div>
        </ElCard>
      </ElCol>
    </ElRow>
    <!-- Máy đang hoạt động chiếm trọn chiều ngang: đây là bảng nhân viên nhìn
         nhiều nhất, cột hẹp thì mã máy dài như DESKTOP-TA2SVLV bị cắt. -->
    <ElCard class="fill-card">
      <template #header>
        <span>{{ $t('vnetPages.dashboard.activeMachines') }}</span>
      </template>
      <!-- row-key: tải lại theo thời gian thực đổi thứ tự dòng; khoá theo vị trí thì
           hộp thoại điều khiển từ xa đang mở của một dòng nhảy sang máy khác. -->
      <ElTable
        :data="activeMachines"
        row-key="machine_id"
        :row-class-name="({ row }: any) => (row.open_orders ? OPEN_ORDERS_ROW_CLASS : '')"
        class="fill-table"
        height="100%"
        style="width: 100%"
      >
        <ElTableColumn prop="machine_code" :label="$t('vnetPages.dashboard.machine')" min-width="160" sortable />
        <ElTableColumn prop="status" :label="$t('vnetPages.common.status')" min-width="170" sortable>
          <template #default="{ row }">
            <div class="machine-status">
              <ElTag :type="statusTags[row.status]?.type ?? 'info'" size="small">
                {{ statusTags[row.status] ? $t(statusTags[row.status].label) : row.status }}
              </ElTag>
              <MachineOpenOrdersTag :orders="row.open_orders" />
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="member_name" :label="$t('vnetPages.dashboard.member')" min-width="180" sortable />
        <ElTableColumn prop="started_at" :label="$t('vnetPages.dashboard.startTime')" min-width="140" sortable>
          <template #default="{ row }">{{ formatTime(row.started_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.duration')" min-width="120">
          <template #default="{ row }">{{ row.started_at ? formatPlayed(row.started_at, null, now) : '-' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="affordable_until" :label="$t('vnetPages.dashboard.remaining')" min-width="120" sortable>
          <template #default="{ row }">
            {{ row.status === 'in_use' ? formatRemainingHMS(row.affordable_until, now) : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn prop="note" :label="$t('vnetPages.machines.noteCol')" min-width="220" sortable>
          <template #default="{ row }">
            <MachineNoteCell
              :machine-id="row.machine_id"
              :note="row.note"
              :readonly="!hasAuth('machines.update')"
              @saved="note => (row.note = note)"
            />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.actions')" width="280" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <MachineRemoteActions
                :machine="{ id: row.machine_id, machine_code: row.machine_code }"
                :member-id="row.member_id"
              />
              <!-- Phiên khách vãng lai không có hội viên: không có ví để nạp. -->
              <ElButton
                v-if="hasAuth('members.topup')"
                size="small"
                type="success"
                :disabled="!row.member_id"
                @click="openTopup(row)"
              >
                {{ $t('vnetPages.dashboard.topUp') }}
              </ElButton>
              <ElButton v-if="hasAuth('orders.view')" size="small" @click="openOrders(row)">
                {{ $t('vnetPages.dashboard.orders') }}
              </ElButton>
            </div>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <MemberTopupDialog
      v-model:visible="topupVisible"
      :member-id="topupRow?.member_id ?? null"
      :member-name="topupRow?.member_name"
      @success="refreshStats"
    />

    <ElDialog
      v-model="ordersVisible"
      :title="
        $t('vnetPages.dashboard.ordersTitle', {
          member: ordersMember,
          code: ordersRow?.machine_code || ''
        })
      "
      width="720px"
    >
      <ElTable
        ref="ordersTableRef"
        v-loading="ordersLoading"
        :data="orders"
        row-key="id"
        :empty-text="$t('vnetPages.dashboard.noOrders')"
        @selection-change="onOrdersSelection"
      >
        <ElTableColumn type="selection" width="40" />
        <ElTableColumn prop="order_code" :label="$t('vnetPages.orders.orderCode')" width="120" sortable />
        <ElTableColumn prop="created_at" :label="$t('vnetPages.dashboard.orderTime')" width="110" sortable>
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.orderItems')" min-width="180">
          <template #default="{ row }">
            <div v-for="it in row.items || []" :key="it.id">{{ it.quantity }}× {{ it.product_name }}</div>
          </template>
        </ElTableColumn>
        <ElTableColumn
          prop="final_amount"
          :label="$t('vnetPages.dashboard.orderTotal')"
          width="100"
          align="right"
          sortable
        >
          <template #default="{ row }">{{ formatMoney(row.final_amount) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="status" :label="$t('vnetPages.common.status')" width="120" align="center" sortable>
          <template #default="{ row }">
            <ElTag :type="orderStatusType(row.status)" size="small">
              {{ $t(`vnetPages.orders.statusLabels.${row.status}`) }}
            </ElTag>
          </template>
        </ElTableColumn>
      </ElTable>
      <template #footer>
        <div class="orders-footer">
          <span class="orders-total">
            {{
              ordersSelected.length
                ? $t('vnetPages.orders.selectedSummary', {
                    count: ordersSelected.length,
                    total: formatPrice(tongTien(ordersSelected))
                  })
                : $t('vnetPages.orders.listTotal', { count: orders.length, total: formatPrice(tongTien(orders)) })
            }}
          </span>
          <ElButton
            v-if="hasAuth('orders.pay')"
            type="success"
            :loading="ordersBusy"
            :disabled="!ordersPayable.length"
            @click="openPay"
          >
            {{ $t('vnetPages.orders.batchPay', { count: ordersPayable.length }) }}
          </ElButton>
          <ElButton
            v-if="ordersSelected.length && hasAuth('orders.status')"
            type="primary"
            :loading="ordersBusy"
            :disabled="!selectedConfirmable.length"
            @click="batchConfirmOrders"
          >
            {{ $t('vnetPages.orders.batchConfirm', { count: selectedConfirmable.length }) }}
          </ElButton>
          <ElButton
            v-if="ordersSelected.length && hasAuth('orders.status')"
            type="danger"
            plain
            :loading="ordersBusy"
            :disabled="!selectedCancellable.length"
            @click="batchCancelOrders"
          >
            {{ $t('vnetPages.orders.batchCancel', { count: selectedCancellable.length }) }}
          </ElButton>
          <ElButton @click="goOrders">{{ $t('vnetPages.dashboard.goOrders') }}</ElButton>
        </div>
      </template>
    </ElDialog>

    <ElDialog v-model="payVisible" :title="$t('vnetPages.orders.batchPayTitle')" width="420px" append-to-body>
      <ElForm label-width="130px">
        <ElFormItem :label="$t('vnetPages.orders.batchOrderCount')">{{ payList.length }}</ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.payAmount')">
          <strong>{{ formatPrice(tongTien(payList)) }}</strong>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.payMethod')">
          <ElSelect v-model="payForm.payment_method" class="w-full">
            <ElOption v-for="m in enabledMethods" :key="m.code" :label="m.name" :value="m.code" />
            <ElOption :label="$t('vnetPages.orders.payBalance')" value="balance" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem
          v-if="payForm.payment_method !== 'cash' && payForm.payment_method !== 'balance'"
          :label="$t('vnetPages.orders.payReference')"
        >
          <ElInput v-model="payForm.reference_code" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="payVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="ordersBusy" @click="submitPay">
          {{ $t('vnetPages.orders.pay') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
/* Điện thoại xếp 2×2 (:xs="12"): cần khoảng cách giữa hai hàng thẻ. */
.stat-cards {
  row-gap: 12px;
}

.machine-status {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

/* Máy còn đơn khách đang gọi hoặc chưa thanh toán: cả dòng một màu khác — giống
   trang Máy trạm. !important vì dòng đang rê chuột và cột cố định tự tô nền ô. */
:deep(.el-table__row.machine-row--open-orders > td.el-table__cell) {
  background-color: var(--el-color-danger-light-8) !important;
}

.orders-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.orders-footer :deep(.el-button + .el-button) {
  margin-left: 0;
}

.orders-total {
  margin-right: auto;
  font-weight: 600;
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.row-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}
</style>
