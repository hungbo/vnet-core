<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const filterAction = ref(null);
const filterEntity = ref(null);
const dateRange = ref(null);

const actionOptions = ['create', 'update', 'delete', 'login', 'logout', 'pay', 'cancel', 'export'];
const entityOptions = [
  'member',
  'machine',
  'order',
  'product',
  'category',
  'session',
  'shift',
  'promotion',
  'combo',
  'booking',
  'user'
];

function formatJson(data: any) {
  if (!data) return '{}';
  try {
    return JSON.stringify(typeof data === 'string' ? JSON.parse(data) : data, null, 2);
  } catch {
    return String(data);
  }
}

function actionLabel(action: string) {
  return $t(`vnetPages.audit.actionLabels.${action}` as any) || action;
}

function entityLabel(entity: string) {
  return $t(`vnetPages.audit.entityLabels.${entity}` as any) || entity;
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/audit-logs', {
      params: {
        page,
        page_size: pageSize,
        action: filterAction.value || undefined,
        entity_type: filterEntity.value || undefined,
        date_from: dateRange.value?.[0] || undefined,
        date_to: dateRange.value?.[1] || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'action', label: $t('vnetPages.audit.action'), width: 100 },
    { prop: 'entity_type', label: $t('vnetPages.audit.target'), width: 100 },
    {
      prop: 'description',
      label: $t('vnetPages.audit.description'),
      minWidth: 300,
      formatter: (row: any) => row.description || `${actionLabel(row.action)} ${entityLabel(row.entity_type)}`
    },
    {
      prop: 'user_name',
      label: $t('vnetPages.audit.user'),
      width: 140,
      formatter: (row: any) => row.user_name || row.user_id || '-'
    },
    {
      prop: 'created_at',
      label: $t('vnetPages.audit.time'),
      width: 160,
      formatter: (row: any) => (row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm:ss') : '')
    }
  ]
});

function fetchData() {
  getData();
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.audit.title') }}</span>
          <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @refresh="getData">
            <template #prefix>
              <ElSelect
                v-model="filterAction"
                :placeholder="$t('vnetPages.audit.action')"
                clearable
                style="width: 140px"
                @change="fetchData"
              >
                <ElOption v-for="a in actionOptions" :key="a" :label="a" :value="a" />
              </ElSelect>
              <ElSelect
                v-model="filterEntity"
                :placeholder="$t('vnetPages.audit.target')"
                clearable
                style="width: 140px"
                @change="fetchData"
              >
                <ElOption v-for="e in entityOptions" :key="e" :label="e" :value="e" />
              </ElSelect>
              <ElDatePicker
                v-model="dateRange"
                type="daterange"
                range-separator="->"
                :start-placeholder="$t('vnetPages.audit.from')"
                :end-placeholder="$t('vnetPages.audit.to')"
                value-format="YYYY-MM-DD"
                @change="fetchData"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%">
        <ElTableColumn type="expand">
          <template #default="{ row }">
            <pre
              style="
                background: #f5f7fa;
                padding: 12px;
                border-radius: 4px;
                font-size: 12px;
                max-height: 300px;
                overflow: auto;
              "
              >{{ formatJson(row.metadata || row.details) }}</pre
            >
          </template>
        </ElTableColumn>
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
      </ElTable>
      <div class="mt-16px flex justify-end">
        <ElPagination
          v-if="mobilePagination.total"
          layout="total, sizes, prev, pager, next"
          v-bind="mobilePagination"
          @current-change="mobilePagination['current-change']"
          @size-change="mobilePagination['size-change']"
        />
      </div>
    </ElCard>
  </div>
</template>
