<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { formatPlayed, formatRemainingHMS } from '@/utils/remaining';
import MachineRemoteActions from '../machines/modules/machine-remote-actions.vue';
import MemberTopupDialog from '../members/modules/member-topup-dialog.vue';

const { t: $t } = useI18n();
const wsStore = useWebSocketStore();

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
const activeMachines = ref<any[]>([]);
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

// Chỉ đơn còn đang xử lý: chờ duyệt hoặc đã xác nhận nhưng chưa xong.
const DON_DANG_XU_LY = new Set(['pending', 'confirmed']);
const ordersVisible = ref(false);
const ordersLoading = ref(false);
const ordersRow = ref<any>(null);
const orders = ref<any[]>([]);

async function openOrders(row: any) {
  ordersRow.value = row;
  orders.value = [];
  ordersVisible.value = true;
  ordersLoading.value = true;
  try {
    const res: any = await client.get(`/members/${row.member_id}/orders`, { params: { page_size: 50 } });
    const list: any[] = Array.isArray(res) ? res : res?.items || [];
    orders.value = list.filter(o => DON_DANG_XU_LY.has(o.status));
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

async function refreshStats() {
  // Cả bốn thẻ đều lấy số thật. Trước đây thẻ hội viên và doanh thu là số 0
  // cứng, còn thẻ "máy online" đếm toàn bộ máy kể cả máy đang tắt.
  const today = new Date().toISOString().slice(0, 10);

  const [members, machines, sessions, revenue] = await Promise.all([
    (client.get('/members', { params: { page_size: 1 } }) as Promise<any>).catch(() => null),
    (client.get('/machines', { params: { page_size: 100 } }) as Promise<any>).catch(() => null),
    (client.get('/sessions/active') as Promise<any>).catch(() => null),
    (
      client.get('/reports/daily-revenue', {
        params: { date_from: today, date_to: today }
      }) as Promise<any>
    ).catch(() => null)
  ]);

  stats.value[0].value = members?.total ?? 0;

  const machineList: any[] = Array.isArray(machines) ? machines : machines?.items || [];
  stats.value[1].value = machineList.filter(m => m.status && m.status !== 'offline').length;

  activeSessions.value = Array.isArray(sessions) ? sessions : sessions?.items || [];
  stats.value[2].value = activeSessions.value.length;

  const playing = new Set(activeSessions.value.map(r => r.machine_id));
  const available = machineList
    .filter(m => m.status === 'available' && !playing.has(m.id))
    .map(m => ({ machine_id: m.id, machine_code: m.machine_code, status: 'available' }))
    .sort((a, b) => String(a.machine_code).localeCompare(String(b.machine_code)));
  activeMachines.value = [...activeSessions.value.map(r => ({ ...r, status: 'in_use' })), ...available];

  const rows: any[] = Array.isArray(revenue) ? revenue : revenue?.items || [];
  stats.value[3].value = formatMoney(rows.reduce((sum, r) => sum + (r.revenue || 0), 0));
}

// Phải là một hằng có tên, không phải hàm inline: gỡ handler cần đúng tham chiếu
// đã đăng ký.
// Cột "Còn lại" phải nhúc nhích mỗi giây, nếu không nhân viên tưởng số bị treo.
const now = ref(Date.now());
let tick: ReturnType<typeof setInterval> | null = null;

function onStatsChanged() {
  refreshStats();
}

onMounted(async () => {
  tick = setInterval(() => {
    now.value = Date.now();
  }, 1000);
  await refreshStats();

  wsStore.on('session:started', onStatsChanged);
  wsStore.on('session:ended', onStatsChanged);
  wsStore.on('machine:status', onStatsChanged);
});

onBeforeUnmount(() => {
  if (tick) clearInterval(tick);
  wsStore.off('session:started', onStatsChanged);
  wsStore.off('session:ended', onStatsChanged);
  wsStore.off('machine:status', onStatsChanged);
});
</script>

<template>
  <div>
    <ElRow :gutter="20" class="stat-cards mb-16px">
      <ElCol v-for="stat in stats" :key="stat.label" :span="6">
        <ElCard shadow="hover">
          <div style="display: flex; justify-content: space-between; align-items: center">
            <div>
              <div style="font-size: 13px; color: #909399">
                {{ $t(stat.label) }}
              </div>
              <div style="font-size: 24px; font-weight: bold; margin-top: 8px">
                {{ stat.value }}
              </div>
            </div>
            <ElIcon :size="40" :color="stat.color"><component :is="stat.icon" /></ElIcon>
          </div>
        </ElCard>
      </ElCol>
    </ElRow>
    <!-- Máy đang hoạt động chiếm trọn chiều ngang: đây là bảng nhân viên nhìn
         nhiều nhất, cột hẹp thì mã máy dài như DESKTOP-TA2SVLV bị cắt. -->
    <ElCard>
      <template #header>
        <span>{{ $t('vnetPages.dashboard.activeMachines') }}</span>
      </template>
      <ElTable :data="activeMachines" style="width: 100%">
        <ElTableColumn prop="machine_code" :label="$t('vnetPages.dashboard.machine')" min-width="160" />
        <ElTableColumn :label="$t('vnetPages.common.status')" min-width="120">
          <template #default="{ row }">
            <ElTag :type="row.status === 'in_use' ? 'warning' : 'success'" size="small">
              {{
                row.status === 'in_use'
                  ? $t('vnetPages.machines.statusLabels.inUse')
                  : $t('vnetPages.machines.statusLabels.available')
              }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="member_name" :label="$t('vnetPages.dashboard.member')" min-width="180" />
        <ElTableColumn :label="$t('vnetPages.dashboard.startTime')" min-width="140">
          <template #default="{ row }">{{ formatTime(row.started_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.duration')" min-width="120">
          <template #default="{ row }">{{ row.started_at ? formatPlayed(row.started_at, null, now) : '-' }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.remaining')" min-width="120">
          <template #default="{ row }">
            {{ row.status === 'in_use' ? formatRemainingHMS(row.affordable_until, now) : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.actions')" width="280" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <MachineRemoteActions :machine="{ id: row.machine_id, machine_code: row.machine_code }" />
              <!-- Phiên khách vãng lai không có hội viên: không có ví để nạp, không có đơn theo tên. -->
              <ElButton size="small" type="success" :disabled="!row.member_id" @click="openTopup(row)">
                {{ $t('vnetPages.dashboard.topUp') }}
              </ElButton>
              <ElButton size="small" :disabled="!row.member_id" @click="openOrders(row)">
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
          member: ordersRow?.member_name || '—',
          code: ordersRow?.machine_code || ''
        })
      "
      width="640px"
    >
      <ElTable v-loading="ordersLoading" :data="orders" :empty-text="$t('vnetPages.dashboard.noOrders')">
        <ElTableColumn prop="order_code" :label="$t('vnetPages.orders.orderCode')" width="120" />
        <ElTableColumn :label="$t('vnetPages.dashboard.orderTime')" width="110">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.orderItems')" min-width="180">
          <template #default="{ row }">
            <div v-for="it in row.items || []" :key="it.id">{{ it.quantity }}× {{ it.product_name }}</div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.dashboard.orderTotal')" width="100" align="right">
          <template #default="{ row }">{{ formatMoney(row.final_amount) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.status')" width="120" align="center">
          <template #default="{ row }">
            <ElTag :type="orderStatusType(row.status)" size="small">
              {{ $t(`vnetPages.orders.statusLabels.${row.status}`) }}
            </ElTag>
          </template>
        </ElTableColumn>
      </ElTable>
      <template #footer>
        <ElButton @click="goOrders">{{ $t('vnetPages.dashboard.goOrders') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
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
