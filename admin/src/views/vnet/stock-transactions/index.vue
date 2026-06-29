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

function formatPrice(price: number) {
  return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(price || 0);
}

const saving = ref(false);

const products = ref<any[]>([]);

const dialog = ref(false);
const formRef = ref<any>(null);
const form = ref<any>({ product_id: null, transaction_type: 'inbound', quantity: 1, unit_price: 0 });
const rules = {
  product_id: [{ required: true, message: $t('vnetPages.stockTransactions.form.productRequired'), trigger: 'change' }],
  quantity: [{ required: true, message: $t('vnetPages.stockTransactions.form.quantityRequired'), trigger: 'blur' }]
};

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) => client.get('/stock-transactions', { params: { page, page_size: pageSize } }),
  transform: vnetTransform,
  columns: () => [
    {
      prop: 'product_name',
      label: $t('vnetPages.stockTransactions.product'),
      width: 200,
      formatter: (row: any) => row.product_name || '-'
    },
    {
      prop: 'transaction_type',
      label: $t('vnetPages.stockTransactions.transactionType'),
      width: 100,
      formatter: (row: any) =>
        h(ElTag, { type: row.transaction_type === 'inbound' ? 'success' : 'danger' }, () =>
          row.transaction_type === 'inbound'
            ? $t('vnetPages.stockTransactions.importLabel')
            : $t('vnetPages.stockTransactions.exportLabel')
        )
    },
    { prop: 'quantity', label: $t('vnetPages.stockTransactions.quantity'), width: 90 },
    { prop: 'stock_before', label: $t('vnetPages.stockTransactions.before'), width: 80 },
    { prop: 'stock_after', label: $t('vnetPages.stockTransactions.after'), width: 80 },
    {
      prop: 'total_price',
      label: $t('vnetPages.stockTransactions.totalPrice'),
      width: 130,
      formatter: (row: any) => formatPrice(row.total_price)
    },
    {
      prop: 'created_by_name',
      label: $t('vnetPages.transactions.createdBy'),
      width: 140,
      formatter: (row: any) => row.created_by_name || '-'
    },
    {
      prop: 'created_at',
      label: $t('vnetPages.stockTransactions.createdAt'),
      width: 160,
      formatter: (row: any) => (row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm') : '')
    }
  ]
});

async function fetchProducts() {
  try {
    const res: any = await client.get('/products', { params: { page_size: 1000 } });
    products.value = Array.isArray(res) ? res : res?.items || [];
  } catch (_) {}
}

function handleCreate() {
  form.value = { product_id: null, transaction_type: 'inbound', quantity: 1, unit_price: 0 };
  dialog.value = true;
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    await client.post('/stock-transactions', form.value);
    ElMessage.success($t('vnetPages.stockTransactions.messages.createTransactionSuccess'));
    dialog.value = false;
    await getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.stockTransactions.messages.saveError'));
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.stockTransactions.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            @add="handleCreate"
            @refresh="getData"
          />
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%">
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

    <ElDialog v-model="dialog" :title="$t('vnetPages.stockTransactions.createTransaction')" width="500px">
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.stockTransactions.product')" prop="product_id">
          <ElSelect
            v-model="form.product_id"
            :placeholder="$t('vnetPages.stockTransactions.form.productRequired')"
            style="width: 100%"
          >
            <ElOption v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.stockTransactions.transactionType')" prop="transaction_type">
          <ElRadioGroup v-model="form.transaction_type">
            <ElRadio value="inbound">{{ $t('vnetPages.stockTransactions.importLabel') }}</ElRadio>
            <ElRadio value="outbound">{{ $t('vnetPages.stockTransactions.exportLabel') }}</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.stockTransactions.quantity')" prop="quantity">
          <ElInputNumber v-model="form.quantity" :min="1" style="width: 100%" />
        </ElFormItem>
        <ElFormItem
          v-if="form.transaction_type === 'inbound'"
          :label="$t('vnetPages.stockTransactions.unitPrice')"
          prop="unit_price"
        >
          <ElInputNumber v-model="form.unit_price" :min="0" :precision="0" style="width: 100%" />
        </ElFormItem>
        <ElFormItem v-if="form.transaction_type === 'inbound'" :label="$t('vnetPages.stockTransactions.totalPrice')">
          <span style="font-weight: 600; color: var(--el-color-primary)">
            {{ formatPrice(form.unit_price * form.quantity) }}
          </span>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialog = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
