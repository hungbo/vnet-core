<script setup lang="ts">
import { computed, h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');
const filterStatus = ref('');

const statusOptions = computed(() => [
  { value: 'pending', label: $t('vnetPages.bookings.statusLabels.pending') },
  { value: 'confirmed', label: $t('vnetPages.bookings.statusLabels.confirmed') },
  { value: 'checked_in', label: $t('vnetPages.bookings.statusLabels.checkedIn') },
  { value: 'cancelled', label: $t('vnetPages.bookings.statusLabels.cancelled') },
  { value: 'no_show', label: $t('vnetPages.bookings.statusLabels.noShow') }
]);

function statusType(status: string): any {
  const map: Record<string, string> = {
    pending: 'warning',
    confirmed: 'primary',
    checked_in: 'success',
    cancelled: 'info',
    no_show: 'danger'
  };
  return map[status] || 'info';
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    pending: $t('vnetPages.bookings.statusLabels.pending'),
    confirmed: $t('vnetPages.bookings.statusLabels.confirmed'),
    checked_in: $t('vnetPages.bookings.statusLabels.checkedIn'),
    cancelled: $t('vnetPages.bookings.statusLabels.cancelled'),
    no_show: $t('vnetPages.bookings.statusLabels.noShow')
  };
  return map[status] || status;
}

function canCheckIn(row: any) {
  return row.status === 'confirmed' || row.status === 'pending';
}

function canCancel(row: any) {
  return row.status === 'pending' || row.status === 'confirmed';
}

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/bookings', {
      params: {
        page,
        page_size: pageSize,
        search: search.value || undefined,
        status: filterStatus.value || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'customer_name', label: $t('vnetPages.bookings.customer'), minWidth: 160 },
    { prop: 'machine_code', label: $t('vnetPages.bookings.machineCode'), width: 110 },
    {
      prop: 'booked_from',
      label: $t('vnetPages.bookings.from'),
      width: 150,
      formatter: (row: any) => formatDate(row.booked_from)
    },
    {
      prop: 'booked_to',
      label: $t('vnetPages.bookings.to'),
      width: 150,
      formatter: (row: any) => formatDate(row.booked_to)
    },
    {
      prop: 'status',
      label: $t('vnetPages.common.status'),
      width: 110,
      formatter: (row: any) => h(ElTag, { type: statusType(row.status), size: 'small' }, () => statusLabel(row.status))
    },
    {
      prop: 'deposit_amount',
      label: $t('vnetPages.bookings.deposit'),
      width: 110,
      formatter: (row: any) => row.deposit_amount?.toLocaleString() ?? ''
    }
  ]
});

function fetchData() {
  getData();
}

async function handleCheckIn(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.bookings.messages.checkInConfirm', { customer: row.customer_name, code: row.machine_code }),
      $t('vnetPages.common.confirm'),
      { type: 'info' }
    );
    await client.post(`/bookings/${row.id}/check-in`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.bookings.messages.checkInSuccess')
    });
    fetchData();
  } catch {}
}

async function handleCancel(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.bookings.messages.cancelConfirm', { customer: row.customer_name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.post(`/bookings/${row.id}/cancel`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.bookings.messages.cancelSuccess')
    });
    fetchData();
  } catch {}
}
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElSelect
            v-model="filterStatus"
            :placeholder="$t('vnetPages.common.status')"
            clearable
            style="width: 160px"
            @change="fetchData"
          >
            <ElOption v-for="s in statusOptions" :key="s.value" :label="s.label" :value="s.value" />
          </ElSelect>
          <ElInput
            v-model="search"
            placeholder="Tìm theo tên / mã máy"
            clearable
            style="width: 260px"
            @keyup.enter="fetchData"
          />
          <ElButton type="primary" @click="fetchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @refresh="getData" />
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="200" fixed="right">
          <template #default="{ row }">
            <ElButton v-if="canCheckIn(row)" size="small" type="success" @click="handleCheckIn(row)">
              {{ $t('vnetPages.bookings.checkIn') }}
            </ElButton>
            <ElButton v-if="canCancel(row)" size="small" type="danger" @click="handleCancel(row)">
              {{ $t('vnetPages.common.cancel') }}
            </ElButton>
            <span v-if="!canCheckIn(row) && !canCancel(row)">-</span>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="mt-16px flex justify-center">
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
