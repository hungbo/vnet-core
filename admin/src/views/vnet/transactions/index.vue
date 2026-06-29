<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const list = ref<any[]>([]);
const loading = ref(false);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const search = ref('');
const typeFilter = ref('');
const dateRange = ref<any>(null);

function formatPrice(price: number) {
  return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(price || 0);
}

function typeLabel(type: string) {
  const map: Record<string, string> = {
    topup: $t('vnetPages.transactions.topup'),
    session_fee: $t('vnetPages.transactions.sessionFee'),
    refund: $t('vnetPages.transactions.refund'),
    cancel: $t('vnetPages.transactions.cancel'),
    combo_purchase: $t('vnetPages.transactions.comboPurchase')
  };
  return map[type] || type;
}

async function fetchData() {
  loading.value = true;
  try {
    const params: any = { page: page.value, page_size: pageSize.value };
    if (search.value) params.search = search.value;
    if (typeFilter.value) params.transaction_type = typeFilter.value;
    if (dateRange.value) {
      params.date_from = dateRange.value[0];
      params.date_to = dateRange.value[1];
    }
    const res: any = await client.get('/transactions', { params });
    list.value = res.items || [];
    total.value = res.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.transactions.messages.loadError'));
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  fetchData();
});
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>{{ $t('vnetPages.transactions.title') }}</span>
          <div style="display: flex; gap: 8px; align-items: center">
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
      <ElTable v-loading="loading" :data="list" border stripe style="width: 100%">
        <ElTableColumn prop="created_at" :label="$t('vnetPages.transactions.date')" width="160" />
        <ElTableColumn :label="$t('vnetPages.transactions.member')" min-width="140">
          <template #default="{ row }">{{ row.member_username || row.member_name }}</template>
        </ElTableColumn>
        <ElTableColumn prop="transaction_type" :label="$t('vnetPages.transactions.type')" width="120">
          <template #default="{ row }">{{ typeLabel(row.transaction_type) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.transactions.amount')" width="140" align="right">
          <template #default="{ row }">{{ formatPrice(row.amount) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.transactions.balanceBefore')" width="130" align="right">
          <template #default="{ row }">{{ formatPrice(row.balance_before) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.transactions.balanceAfter')" width="130" align="right">
          <template #default="{ row }">{{ formatPrice(row.balance_after) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="payment_method" :label="$t('vnetPages.transactions.paymentMethod')" width="110" />
        <ElTableColumn
          prop="description"
          :label="$t('vnetPages.transactions.description')"
          min-width="200"
          show-overflow-tooltip
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
