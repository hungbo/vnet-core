<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { formatPrice } from '@/utils/money';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const activeTab = ref('daily-revenue');
const dateRange = ref(null);

const { columns, columnChecks, data, getData, loading, reloadColumns } = useUITable({
  api: () => {
    const params: any = {};
    const dr: any = dateRange.value;
    // Mọi tab, kể cả "Doanh thu tháng", gửi đúng khoảng ngày đã chọn. Bản cũ
    // đổi khoảng ngày thành năm/tháng của NGÀY ĐẦU, nên chọn 01/09 → 17/10 chỉ
    // ra mỗi tháng 9.
    if (dr) {
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
            sortable: true,
            formatter: (row: any) => dayjs(row.date).format('DD/MM/YYYY')
          },
          {
            prop: 'revenue',
            label: $t('vnetPages.reports.totalRevenue'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.revenue)
          },
          {
            prop: 'total_orders',
            label: $t('vnetPages.reports.orderCount'),
            width: 120,
            sortable: true
          }
        ];
      case 'monthly-revenue':
        return [
          {
            prop: 'month',
            label: $t('vnetPages.reports.month'),
            width: 120,
            sortable: true,
            formatter: (row: any) => dayjs(`${row.month}-01`).format('MM/YYYY')
          },
          {
            prop: 'revenue',
            label: $t('vnetPages.reports.totalRevenue'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.revenue)
          },
          {
            // service.MonthlyRevenueRow trả total_orders. Tên cũ order_count
            // không tồn tại trong phản hồi nên cột này luôn trống.
            prop: 'total_orders',
            label: $t('vnetPages.reports.orderCount'),
            width: 120,
            sortable: true
          }
        ];
      case 'by-member':
        return [
          {
            prop: 'member_name',
            label: $t('vnetPages.reports.member'),
            minWidth: 150,
            sortable: true
          },
          {
            // Tiền giờ + đơn hàng + gói cước, trả bằng gì cũng tính. Tiền nạp là
            // cột riêng bên cạnh, không còn cộng vào đây.
            prop: 'total_spent',
            label: $t('vnetPages.reports.totalSpent'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.total_spent)
          },
          {
            prop: 'total_topup',
            label: $t('vnetPages.reports.totalTopup'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.total_topup)
          },
          {
            prop: 'visit_count',
            label: $t('vnetPages.reports.visitCount'),
            width: 110,
            sortable: true
          }
        ];
      case 'by-machine':
        // Bốn trong năm cột trước đây đọc khoá API không tồn tại
        // (machine_code/revenue/total_hours/session_count trong khi API trả
        // machine_name/total_sales/usage_hours và không có số phiên), nên cả tab
        // này chỉ hiện đúng một chuỗi "0 ₫". Máy không có tên riêng, chỉ có mã,
        // nên cột "Tên máy" cũng đã bỏ.
        return [
          {
            prop: 'machine_code',
            label: $t('vnetPages.reports.machine'),
            minWidth: 140,
            sortable: true
          },
          {
            prop: 'total_sales',
            label: $t('vnetPages.reports.revenue'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.total_sales)
          },
          {
            prop: 'usage_hours',
            label: $t('vnetPages.reports.hours'),
            width: 100,
            sortable: true,
            formatter: (row: any) => `${row.usage_hours ?? 0}g`
          },
          {
            prop: 'session_count',
            label: $t('vnetPages.reports.sessionCount'),
            width: 100,
            sortable: true
          }
        ];
      case 'by-employee':
        return [
          {
            prop: 'employee_name',
            label: $t('vnetPages.reports.employee'),
            minWidth: 180,
            sortable: true
          },
          {
            prop: 'orders_taken',
            label: $t('vnetPages.reports.ordersTaken'),
            width: 130,
            sortable: true
          },
          {
            prop: 'total_sales',
            label: $t('vnetPages.reports.totalSales'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.total_sales)
          }
        ];
      case 'top-products':
        return [
          {
            prop: 'product_name',
            label: $t('vnetPages.reports.product'),
            minWidth: 200,
            sortable: true
          },
          {
            prop: 'quantity',
            label: $t('vnetPages.reports.quantitySold'),
            width: 130,
            sortable: true
          },
          {
            prop: 'total_sales',
            label: $t('vnetPages.reports.totalSales'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.total_sales)
          }
        ];
      case 'promotion-usage':
        return [
          {
            prop: 'promotion_name',
            label: $t('vnetPages.reports.promotion'),
            minWidth: 200,
            sortable: true,
            // Đơn được giảm mà khuyến mãi đã bị xoá vẫn phải hiện ra, nếu không
            // tổng tiền giảm trên báo cáo sẽ không khớp với sổ sách.
            formatter: (row: any) => row.promotion_name || $t('vnetPages.reports.promotionDeleted')
          },
          {
            prop: 'usage_count',
            label: $t('vnetPages.reports.usageCount'),
            width: 130,
            sortable: true
          },
          {
            prop: 'discount_given',
            label: $t('vnetPages.reports.discountGiven'),
            minWidth: 150,
            sortable: true,
            formatter: (row: any) => formatPrice(row.discount_given)
          }
        ];
      default:
        return [];
    }
  }
});

// Báo cáo đọc đơn hàng (orders), sổ ví hội viên (nạp/hoàn ở members, mua gói ở
// combos), phiên đã kết thúc (sessions, hay tắt máy từ xa ở machines) và tên ở
// members/machines/promotions/systemManage. Phiên do tác vụ nền đóng (hết giờ,
// hết tiền, máy mất tín hiệu) không qua HTTP nhưng vẫn phát session:ended. Gộp
// 2 giây để một loạt thao tác chỉ chạy truy vấn tổng hợp một lần.
useRealtimeRefresh(['orders', 'members', 'combos', 'sessions', 'machines', 'promotions', 'systemManage'], getData, {
  events: ['session:ended'],
  delay: 2000
});

function onTabChange() {
  // Mỗi tab có bộ cột riêng, nhưng columnChecks chỉ được dựng MỘT LẦN lúc khởi
  // tạo — từ cột của tab đầu tiên. Không dựng lại thì cột của tab mới bị lọc
  // theo danh sách cũ, không khớp gì, và bảng trống trơn.
  //
  // Đây là lý do trước đây chỉ tab "Doanh thu ngày" hiện được dữ liệu.
  reloadColumns();
  getData();
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.reports.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-add="false"
            :show-delete="false"
            @refresh="getData"
          >
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
        <ElTabPane :label="$t('vnetPages.reports.byEmployee')" name="by-employee">
          <ElTable v-loading="loading" :data="data" style="width: 100%">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
          </ElTable>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.reports.topProducts')" name="top-products">
          <ElTable v-loading="loading" :data="data" style="width: 100%">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
          </ElTable>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.reports.promotionUsage')" name="promotion-usage">
          <ElTable v-loading="loading" :data="data" style="width: 100%">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
          </ElTable>
        </ElTabPane>
      </ElTabs>
    </ElCard>
  </div>
</template>
