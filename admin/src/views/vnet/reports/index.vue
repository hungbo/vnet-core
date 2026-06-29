<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const activeTab = ref('daily-revenue');
const dateRange = ref(null);

function formatPrice(price: number) {
  return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(price || 0);
}

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: () => {
    const params: any = {};
    const dr: any = dateRange.value;
    if (activeTab.value === 'monthly-revenue') {
      if (dr) {
        params.year = dr[0].split('-')[0];
        params.month = dr[0].split('-')[1];
      }
    } else if (dr) {
      params.date_from = dr[0];
      params.date_to = dr[1];
    }
    return client.get(`/reports/${activeTab.value}`, { params });
  },
  transform: vnetSimpleTransform,
  columns: () => {
    switch (activeTab.value) {
      case 'daily-revenue':
        return [
          {
            prop: 'date',
            label: $t('vnetPages.reports.date'),
            width: 120,
            formatter: (row: any) => dayjs(row.date || row.day).format('DD/MM/YYYY')
          },
          {
            prop: 'total_revenue',
            label: $t('vnetPages.reports.totalRevenue'),
            minWidth: 150,
            formatter: (row: any) => formatPrice(row.total_revenue || row.revenue)
          },
          { prop: 'total_orders', label: $t('vnetPages.reports.orderCount'), width: 120 }
        ];
      case 'monthly-revenue':
        return [
          {
            prop: 'month',
            label: $t('vnetPages.reports.month'),
            width: 120,
            formatter: (row: any) => row.month || row.year_month
          },
          {
            prop: 'total_revenue',
            label: $t('vnetPages.reports.totalRevenue'),
            minWidth: 150,
            formatter: (row: any) => formatPrice(row.total_revenue || row.revenue)
          },
          { prop: 'order_count', label: $t('vnetPages.reports.orderCount'), width: 120 }
        ];
      case 'by-member':
        return [
          { prop: 'member_name', label: $t('vnetPages.reports.member'), minWidth: 150 },
          {
            prop: 'total_spent',
            label: $t('vnetPages.reports.totalSpent'),
            minWidth: 150,
            formatter: (row: any) => formatPrice(row.total_spent || row.revenue)
          },
          { prop: 'visit_count', label: $t('vnetPages.reports.visitCount'), width: 80 }
        ];
      case 'by-machine':
        return [
          { prop: 'machine_code', label: $t('vnetPages.reports.machine'), width: 100 },
          { prop: 'machine_name', label: $t('vnetPages.reports.machineName'), minWidth: 150 },
          {
            prop: 'revenue',
            label: $t('vnetPages.reports.revenue'),
            minWidth: 150,
            formatter: (row: any) => formatPrice(row.total_revenue || row.revenue)
          },
          { prop: 'total_hours', label: $t('vnetPages.reports.hours'), width: 80 },
          { prop: 'session_count', label: $t('vnetPages.reports.sessionCount'), width: 80 }
        ];
      default:
        return [];
    }
  }
});

function onTabChange() {
  getData();
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.reports.title') }}</span>
          <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @refresh="getData">
            <template #prefix>
              <ElDatePicker
                v-model="dateRange"
                type="daterange"
                range-separator="->"
                :start-placeholder="$t('vnetPages.reports.from')"
                :end-placeholder="$t('vnetPages.reports.to')"
                value-format="YYYY-MM-DD"
                @change="getData"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTabs v-model="activeTab" @tab-change="onTabChange">
        <ElTabPane :label="$t('vnetPages.reports.dailyRevenue')" name="daily-revenue">
          <ElTable v-loading="loading" :data="data" style="width: 100%">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
          </ElTable>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.reports.monthlyRevenue')" name="monthly-revenue">
          <ElTable v-loading="loading" :data="data" style="width: 100%">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
          </ElTable>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.reports.byMember')" name="by-member">
          <ElTable v-loading="loading" :data="data" style="width: 100%">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
          </ElTable>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.reports.byMachine')" name="by-machine">
          <ElTable v-loading="loading" :data="data" style="width: 100%">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
          </ElTable>
        </ElTabPane>
      </ElTabs>
    </ElCard>
  </div>
</template>
