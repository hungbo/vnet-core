<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useTodayRange } from '@/hooks/common/today-filter';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const filterAction = ref(null);
const filterEntity = ref(null);
// Mặc định hôm nay; xoá ô ngày để xem mọi ngày.
const { range: dateRange, follow: followToday } = useTodayRange();

// Chỉ liệt kê giá trị mà backend thật sự ghi vào audit_logs. 'login', 'logout'
// và 'export' không có một dòng nào trong mã nguồn — chọn chúng thì bảng luôn
// rỗng, người trực tưởng hệ thống mất dữ liệu.
// 'request': mọi thao tác thay đổi dữ liệu của nhân viên, ghi ở tầng HTTP kèm
// người làm và IP (middleware.StaffActionLog); 'login' / 'login_failed': đăng
// nhập trang quản trị.
const actionOptions = [
  'request',
  'login',
  'login_failed',
  'create',
  'update',
  'update_status',
  'delete',
  'pay',
  'cancel',
  'topup',
  'refund'
];
const entityOptions = [
  'member',
  'machine',
  'order',
  'product',
  'category',
  // Bảng ghi là 'machine_session'; 'session' không tồn tại nên đây từng là một
  // lựa chọn chết.
  'machine_session',
  'shift',
  'promotion',
  'combo',
  // Tương tự: bảng ghi 'machine_booking'.
  'machine_booking',
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

const { columns, columnChecks, data, getData, loading, mobilePagination, onSortChange } = useUIPaginatedTable({
  api: ({ page, pageSize, sort, order }) => {
    followToday();
    return client.get('/audit-logs', {
      params: {
        page,
        page_size: pageSize,
        action: filterAction.value || undefined,
        entity_type: filterEntity.value || undefined,
        date_from: dateRange.value?.[0] || undefined,
        date_to: dateRange.value?.[1] || undefined,
        sort,
        order
      }
    });
  },
  transform: vnetTransform,
  columns: () => [
    { prop: 'action', label: $t('vnetPages.audit.action'), width: 100, sortable: 'custom' },
    { prop: 'entity_type', label: $t('vnetPages.audit.target'), width: 100, sortable: 'custom' },
    {
      prop: 'description',
      label: $t('vnetPages.audit.description'),
      minWidth: 300,
      sortable: 'custom',
      formatter: (row: any) => row.description || `${actionLabel(row.action)} ${entityLabel(row.entity_type)}`
    },
    {
      prop: 'user_name',
      label: $t('vnetPages.audit.user'),
      width: 140,
      sortable: 'custom',
      formatter: (row: any) => row.user_name || row.user_id || '-'
    },
    {
      prop: 'created_at',
      label: $t('vnetPages.audit.time'),
      width: 160,
      sortable: 'custom',
      formatter: (row: any) => (row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm:ss') : '')
    }
  ]
});

// Mọi thao tác ghi của nhân viên đều để lại một dòng nhật ký (StaffActionLog),
// nên trang nghe MỌI chủ đề ghi của API — một cụm thao tác gộp lại thành một
// lần tải sau 3 giây. "audit-logs": dòng ghi ngoài các yêu cầu ghi thành công
// (đăng nhập trang quản trị); các sự kiện riêng: việc tác vụ nền làm và có ghi
// nhật ký. Nhịp tim máy trạm cũng ghi nhật ký mỗi 15 giây mỗi máy nhưng cố ý
// không làm trang tải lại.
const AUDITED_TOPICS = [
  'members',
  'member-groups',
  'machines',
  'machine-groups',
  'machine-assets',
  'sessions',
  'combos',
  'bookings',
  'promotions',
  'lucky-spin',
  'curfew',
  'categories',
  'products',
  'orders',
  'games',
  'app-updates',
  'website-rules',
  'machine-prices',
  'time-pricing',
  'feedback',
  'attendance',
  'inventory-counts',
  'printers',
  'stock-transactions',
  'suppliers',
  'shifts',
  'settings',
  'error-reports',
  'backups',
  'notifications',
  'admin-notifications',
  'systemManage',
  'audit-logs'
];

useRealtimeRefresh(AUDITED_TOPICS, getData, {
  events: ['session:ended', 'curfew:enforced', 'machine:alert'],
  delay: 3000
});

function fetchData() {
  getData();
}
</script>

<template>
  <div class="page-fill">
    <ElCard class="fill-card">
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.audit.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-add="false"
            :show-delete="false"
            @refresh="getData"
          >
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
                range-separator="→"
                :start-placeholder="$t('vnetPages.audit.from')"
                :end-placeholder="$t('vnetPages.audit.to')"
                value-format="YYYY-MM-DD"
                @change="fetchData"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <!-- row-key: dòng đang mở xem chi tiết vẫn mở sau mỗi lần tự tải lại. -->
      <ElTable
        v-loading="loading"
        :data="data"
        row-key="id"
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn type="expand">
          <template #default="{ row }">
            <pre
              style="
                background: var(--el-fill-color-light);
                padding: 12px;
                border-radius: 4px;
                font-size: 12px;
                max-height: 300px;
                overflow: auto;
              "
              >{{ formatJson(row.metadata || row.details) }}</pre>
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
