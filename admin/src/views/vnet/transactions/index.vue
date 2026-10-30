<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { usePaymentMethods } from '@/hooks/business/payment-methods';
import { useTransactionTypeLabel } from '@/hooks/business/transaction-types';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useTodayRange } from '@/hooks/common/today-filter';
import { useFitPageSize } from '@/hooks/common/table';
import { formatPrice } from '@/utils/money';

const { t: $t } = useI18n();
const { paymentLabel } = usePaymentMethods();

const list = ref<any[]>([]);
const loading = ref(false);
const page = ref(1);
const pageSize = ref(20);
// Số dòng/trang vừa khít chiều cao bảng (xem useFitPageSize).
const fitter = useFitPageSize({
  getPageSize: () => pageSize.value,
  setPageSize: size => {
    pageSize.value = size;
    page.value = 1;
    fetchData();
  }
});
const total = ref(0);
const search = ref('');
const typeFilter = ref('');
// Mặc định hôm nay; xoá ô ngày để xem mọi ngày.
const { range: dateRange, follow: followToday } = useTodayRange();
// Sắp xếp phía máy chủ trên toàn bộ sổ: prop của cột bấm và 'asc' | 'desc'.
const sort = ref<string>();
const order = ref<'asc' | 'desc'>();

const typeLabel = useTransactionTypeLabel();

function onSortChange(e: { prop: string | null; order: 'ascending' | 'descending' | null }) {
  if (e.order && e.prop) {
    sort.value = e.prop;
    order.value = e.order === 'ascending' ? 'asc' : 'desc';
  } else {
    sort.value = undefined;
    order.value = undefined;
  }
  page.value = 1;
  fetchData();
}

async function fetchData() {
  followToday();
  loading.value = true;
  try {
    const params: any = { page: page.value, page_size: pageSize.value, sort: sort.value, order: order.value };
    if (search.value) params.search = search.value;
    if (typeFilter.value) params.transaction_type = typeFilter.value;
    if (dateRange.value) {
      params.date_from = dateRange.value[0];
      params.date_to = dateRange.value[1];
    }
    const res: any = await client.get('/transactions', { params });
    list.value = res.items || [];
    total.value = res.total || 0;
    await fitter.afterLoad();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.transactions.messages.loadError'));
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  if (!fitter.fit()) fetchData();
});

// Sổ giao dịch không có route ghi nào của riêng nó: mỗi dòng do một nghiệp vụ
// khác ghi — nạp/hoàn/tặng (members), mua gói (combos), cọc đặt chỗ (bookings),
// vòng quay (lucky-spin), điểm danh (attendance), đóng phiên (sessions). Mọi
// đường đổi số dư, kể cả không qua HTTP (tác vụ nền đóng phiên, đăng nhập máy
// trạm tự nhận lịch đặt), đều phát member:updated — bắt nó là đủ cho phía đơn
// hàng, nên không nghe "orders" (đổi trạng thái bếp liên tục mà không ghi sổ).
// systemManage: tên nhân viên ở cột "Người tạo".
useRealtimeRefresh(
  ['members', 'combos', 'bookings', 'lucky-spin', 'attendance', 'sessions', 'systemManage'],
  fetchData,
  { events: ['member:updated'] }
);
</script>

<template>
  <div class="page-fill">
    <ElCard class="fill-card">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>{{ $t('vnetPages.transactions.title') }}</span>
          <div style="display: flex; flex-wrap: wrap; gap: 8px; align-items: center">
            <ElDatePicker
              v-model="dateRange"
              type="daterange"
              range-separator="->"
              :start-placeholder="$t('vnetPages.transactions.from')"
              :end-placeholder="$t('vnetPages.transactions.to')"
              value-format="YYYY-MM-DD"
              clearable
              @change="fetchData"
            />
            <ElSelect
              v-model="typeFilter"
              :placeholder="$t('vnetPages.transactions.typePlaceholder')"
              clearable
              style="width: 140px"
              @change="fetchData"
            >
              <ElOption :label="$t('vnetPages.transactions.all')" value="" />
              <ElOption :label="$t('vnetPages.transactions.topup')" value="topup" />
              <ElOption :label="$t('vnetPages.transactions.sessionFee')" value="session_fee" />
              <ElOption :label="$t('vnetPages.transactions.refund')" value="refund" />
              <ElOption :label="$t('vnetPages.transactions.cancel')" value="cancel" />
              <ElOption :label="$t('vnetPages.transactions.comboPurchase')" value="combo_purchase" />
            </ElSelect>
            <ElInput
              v-model="search"
              :placeholder="$t('vnetPages.transactions.searchPlaceholder')"
              clearable
              style="width: 250px"
              @keyup.enter="fetchData"
            />
            <ElButton type="primary" @click="fetchData">{{ $t('vnetPages.common.search') }}</ElButton>
          </div>
        </div>
      </template>
      <ElTable
        v-loading="loading"
        :data="list"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn prop="created_at" :label="$t('vnetPages.transactions.date')" width="160" sortable="custom" />
        <ElTableColumn
          prop="member_username"
          :label="$t('vnetPages.transactions.member')"
          min-width="140"
          sortable="custom"
        >
          <template #default="{ row }">{{ row.member_username || row.member_name }}</template>
        </ElTableColumn>
        <ElTableColumn prop="transaction_type" :label="$t('vnetPages.transactions.type')" width="120" sortable="custom">
          <template #default="{ row }">{{ typeLabel(row.transaction_type) }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="amount"
          :label="$t('vnetPages.transactions.amount')"
          width="140"
          align="right"
          sortable="custom"
        >
          <template #default="{ row }">{{ formatPrice(row.amount) }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="balance_before"
          :label="$t('vnetPages.transactions.balanceBefore')"
          width="130"
          align="right"
          sortable="custom"
        >
          <template #default="{ row }">{{ formatPrice(row.balance_before) }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="balance_after"
          :label="$t('vnetPages.transactions.balanceAfter')"
          width="130"
          align="right"
          sortable="custom"
        >
          <template #default="{ row }">{{ formatPrice(row.balance_after) }}</template>
        </ElTableColumn>
        <!--
 Thưởng điểm danh, phí chơi trừ vào khuyến mãi, vòng quay… chỉ động
             tới số dư khuyến mãi. Thiếu hai cột này thì dòng "+25.000₫" nằm
             cạnh "số dư trước = số dư sau" và sổ sách không đối chiếu được —
             API đã trả sẵn bonus_before/bonus_after từ trước. 
-->
        <ElTableColumn
          prop="bonus_before"
          :label="$t('vnetPages.transactions.bonusBefore')"
          width="130"
          align="right"
          sortable="custom"
        >
          <template #default="{ row }">{{ formatPrice(row.bonus_before) }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="bonus_after"
          :label="$t('vnetPages.transactions.bonusAfter')"
          width="130"
          align="right"
          sortable="custom"
        >
          <template #default="{ row }">{{ formatPrice(row.bonus_after) }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="payment_method"
          :label="$t('vnetPages.transactions.paymentMethod')"
          width="110"
          sortable="custom"
        >
          <template #default="{ row }">{{ paymentLabel(row.payment_method) }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="description"
          :label="$t('vnetPages.transactions.description')"
          min-width="200"
          show-overflow-tooltip
          sortable="custom"
        />
        <ElTableColumn :label="$t('vnetPages.transactions.createdBy')" width="130">
          <template #default="{ row }">{{ row.created_by_name || row.created_by }}</template>
        </ElTableColumn>
      </ElTable>
      <div style="display: flex; justify-content: center; margin-top: 16px">
        <ElPagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next, total"
          @current-change="fetchData"
        />
      </div>
    </ElCard>
  </div>
</template>
