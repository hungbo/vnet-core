<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useWebSocketStore } from '@/store/modules/ws';
import client from '@/api/client';

const { t: $t } = useI18n();
const wsStore = useWebSocketStore();

const stats = ref([
  { label: 'vnetPages.dashboard.stats.members', value: 0, icon: 'User', color: '#409eff' },
  { label: 'vnetPages.dashboard.stats.onlineMachines', value: 0, icon: 'Monitor', color: '#67c23a' },
  { label: 'vnetPages.dashboard.stats.playing', value: 0, icon: 'Timer', color: '#e6a23c' },
  { label: 'vnetPages.dashboard.stats.revenueToday', value: '0₫', icon: 'Money', color: '#f56c6c' }
]);

const activeSessions = ref<any[]>([]);

async function refreshStats() {
  try {
    const [machines, sessions] = await Promise.all([
      (client.get('/machines', { params: { page_size: 1 } }) as Promise<any>).catch(() => ({ items: [], total: 0 })),
      client.get('/sessions/active').catch(() => [])
    ]);
    stats.value[1].value = (machines as any).total || 0;
    activeSessions.value = Array.isArray(sessions) ? sessions : [];
    stats.value[2].value = activeSessions.value.length;
  } catch (_) {}
}

onMounted(async () => {
  await refreshStats();

  wsStore.on('session:started', () => { refreshStats(); });
  wsStore.on('session:ended', () => { refreshStats(); });
  wsStore.on('machine:status', () => { refreshStats(); });
});

onBeforeUnmount(() => {
  wsStore.off('session:started');
  wsStore.off('session:ended');
  wsStore.off('machine:status');
});
</script>

<template>
  <div>
    <ElRow :gutter="20" class="stat-cards">
      <ElCol v-for="stat in stats" :key="stat.label" :span="6">
        <ElCard shadow="hover">
          <div style="display: flex; justify-content: space-between; align-items: center">
            <div>
              <div style="font-size: 13px; color: #909399">{{ $t(stat.label) }}</div>
              <div style="font-size: 24px; font-weight: bold; margin-top: 8px">{{ stat.value }}</div>
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
            <span>{{ $t('vnetPages.dashboard.revenueToday') }}</span>
          </template>
          <div style="height: 300px; display: flex; align-items: center; justify-content: center; color: #909399">
            {{ $t('vnetPages.dashboard.revenueChart') }}
          </div>
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
            <ElTableColumn prop="started_at" :label="$t('vnetPages.dashboard.startTime')" width="150" />
            <ElTableColumn :label="$t('vnetPages.dashboard.duration')" width="100">
              <template #default="{ row }">{{ row.duration }}p</template>
            </ElTableColumn>
          </ElTable>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>
