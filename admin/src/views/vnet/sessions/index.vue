<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const total = ref(0);

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
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
    { prop: 'machine_code', label: $t('vnetPages.sessions.machineCode'), width: 120 },
    { prop: 'member_name', label: $t('vnetPages.sessions.member'), minWidth: 160 },
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

async function handleEnd(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.sessions.messages.endConfirm', { member: row.member_name, code: row.machine_code }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.post(`/sessions/${row.id}/end`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.sessions.messages.endSuccess')
    });
    getData();
  } catch {}
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.sessions.activeSessions') }}</span>
          <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @refresh="getData" />
        </div>
      </template>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="120" fixed="right">
          <template #default="{ row }">
            <ElButton v-if="row.is_active" size="small" type="danger" @click="handleEnd(row)">
              {{ $t('vnetPages.sessions.end') }}
            </ElButton>
            <span v-else>-</span>
          </template>
        </ElTableColumn>
      </ElTable>

      <div v-if="total > 0" style="margin-top: 16px; text-align: center; color: #909399; font-size: 13px">
        {{ $t('vnetPages.sessions.totalActive', { count: total }) }}
      </div>
    </ElCard>
  </div>
</template>
