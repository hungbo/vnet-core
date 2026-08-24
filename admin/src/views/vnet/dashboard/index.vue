<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { formatRemaining } from '@/utils/remaining';
import RevenueChart from './modules/revenue-chart.vue';

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
const revenueChartRef = ref<InstanceType<typeof RevenueChart> | null>(null);

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
    <ElRow :gutter="20" class="stat-cards">
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
    <ElRow :gutter="20">
      <ElCol :span="12">
        <ElCard>
          <template #header>
            <span>{{ $t('vnetPages.dashboard.revenue7d') }}</span>
          </template>
          <RevenueChart ref="revenueChartRef" />
        </ElCard>
      </ElCol>
      <ElCol :span="12">
        <ElCard>
          <template #header>
            <span>{{ $t('vnetPages.dashboard.activeMachines') }}</span>
          </template>
          <ElTable :data="activeSessions" style="width: 100%" size="small">
            <ElTableColumn prop="machine_code" :label="$t('vnetPages.dashboard.machine')" width="80" />
            <ElTableColumn prop="member_name" :label="$t('vnetPages.dashboard.member')" />
            <ElTableColumn :label="$t('vnetPages.dashboard.startTime')" width="150">
              <template #default="{ row }">{{ formatTime(row.started_at) }}</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.dashboard.duration')" width="100">
              <template #default="{ row }">{{ row.duration_minutes ?? 0 }}p</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.dashboard.remaining')" width="110">
              <template #default="{ row }">{{ formatRemaining(row.affordable_until, now) }}</template>
            </ElTableColumn>
          </ElTable>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>
