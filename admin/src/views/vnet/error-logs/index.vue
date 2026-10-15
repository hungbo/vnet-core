<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

// Mỗi dòng là MỘT LOẠI lỗi (server gộp các lần giống nhau), không phải một lần
// xảy ra — cột "Số lần" cho biết nó lặp bao nhiêu.
const filterSource = ref<string | null>(null);
const filterStatus = ref('open');

const sourceTagType: Record<string, 'danger' | 'warning' | 'info'> = {
  server: 'danger',
  admin: 'warning',
  client: 'info'
};

function formatTime(v?: string) {
  return v ? dayjs(v).format('DD/MM/YYYY HH:mm:ss') : '';
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  // Hook gọi api({ page, pageSize }) (hooks/common/table.ts) nhưng khai kiểu là
  // () => …; tham số tuỳ chọn khớp được cả hai.
  api: (p?: { page: number; pageSize: number }) =>
    client.get('/error-reports', {
      params: {
        page: p?.page,
        page_size: p?.pageSize,
        source: filterSource.value || undefined,
        status: filterStatus.value
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'source', label: $t('vnetPages.errorLogs.source'), width: 110 },
    {
      prop: 'kind',
      label: $t('vnetPages.errorLogs.kind'),
      width: 90,
      formatter: (row: any) => $t(`vnetPages.errorLogs.kinds.${row.kind}` as any)
    },
    { prop: 'message', label: $t('vnetPages.errorLogs.message'), minWidth: 320, showOverflowTooltip: true },
    { prop: 'location', label: $t('vnetPages.errorLogs.location'), width: 180, showOverflowTooltip: true },
    { prop: 'count', label: $t('vnetPages.errorLogs.count'), width: 90, align: 'right' },
    { prop: 'reporter', label: $t('vnetPages.errorLogs.reporter'), width: 120 },
    {
      prop: 'last_seen_at',
      label: $t('vnetPages.errorLogs.lastSeen'),
      width: 160,
      formatter: (row: any) => formatTime(row.last_seen_at)
    },
    // Ghim bên phải: bảng rộng hơn màn hình thường, nút nằm sau thanh cuộn ngang
    // là nút không ai thấy.
    { prop: 'actions', label: '', width: 110, fixed: 'right' }
  ]
});

// Lỗi tới từ ba nơi: trang quản trị (POST /error-reports), máy trạm (POST
// /machines/by-code/:code/errors — middleware gắn chủ đề "error-reports") và
// chính máy chủ (lỗi 500, main.go tự phát). Bảng có row-key nên dòng đang mở
// xem chi tiết vẫn mở sau khi tải lại.
useRealtimeRefresh(['error-reports'], getData);

async function resolve(row: any) {
  try {
    await client.put(`/error-reports/${row.id}/resolve`);
    ElMessage.success($t('vnetPages.errorLogs.resolveSuccess'));
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.errorLogs.resolve'));
  }
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.errorLogs.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-add="false"
            :show-delete="false"
            @refresh="getData"
          >
            <template #prefix>
              <ElSelect
                v-model="filterSource"
                :placeholder="$t('vnetPages.errorLogs.source')"
                clearable
                style="width: 140px"
                @change="getData"
              >
                <ElOption
                  v-for="s in ['server', 'admin', 'client']"
                  :key="s"
                  :label="$t(`vnetPages.errorLogs.sources.${s}` as any)"
                  :value="s"
                />
              </ElSelect>
              <ElSelect v-model="filterStatus" style="width: 140px" @change="getData">
                <ElOption
                  v-for="s in ['open', 'resolved', 'all']"
                  :key="s"
                  :label="$t(`vnetPages.errorLogs.statuses.${s}` as any)"
                  :value="s"
                />
              </ElSelect>
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" row-key="id" style="width: 100%">
        <ElTableColumn type="expand">
          <template #default="{ row }">
            <div class="px-12px text-12px leading-20px">
              <div>{{ $t('vnetPages.errorLogs.firstSeen') }}: {{ formatTime(row.first_seen_at) }}</div>
              <div v-if="row.request_id">{{ $t('vnetPages.errorLogs.requestId') }}: {{ row.request_id }}</div>
              <div v-if="row.version">{{ $t('vnetPages.errorLogs.version') }}: {{ row.version }}</div>
              <div v-if="row.resolved_at">
                {{ $t('vnetPages.errorLogs.resolvedAt') }}: {{ formatTime(row.resolved_at) }}
              </div>
              <div class="mt-8px">{{ $t('vnetPages.errorLogs.message') }}:</div>
              <pre class="error-pre">{{ row.message }}</pre>
              <div class="mt-8px">{{ $t('vnetPages.errorLogs.detail') }}:</div>
              <pre class="error-pre">{{ row.detail || $t('vnetPages.errorLogs.noDetail') }}</pre>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col">
          <template v-if="col.prop === 'source'" #default="{ row }">
            <ElTag :type="sourceTagType[row.source] || 'info'" size="small">
              {{ $t(`vnetPages.errorLogs.sources.${row.source}` as any) }}
            </ElTag>
          </template>
          <template v-else-if="col.prop === 'actions'" #default="{ row }">
            <ElButton v-if="!row.resolved_at" size="small" type="success" plain @click="resolve(row)">
              {{ $t('vnetPages.errorLogs.resolve') }}
            </ElButton>
            <ElTag v-else type="success" size="small">{{ $t('vnetPages.errorLogs.statuses.resolved') }}</ElTag>
          </template>
        </ElTableColumn>
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

<style scoped>
.error-pre {
  background: var(--el-fill-color-light);
  padding: 12px;
  border-radius: 4px;
  font-size: 12px;
  max-height: 360px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
