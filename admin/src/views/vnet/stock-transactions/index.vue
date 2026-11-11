<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useAuth } from '@/hooks/business/auth';
import { formatPrice, moneyInput } from '@/utils/money';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();

const saving = ref(false);

const products = ref<any[]>([]);

// Thanh lọc: ô tìm kiếm (tên sản phẩm, ghi chú — phiếu theo đơn ghi mã đơn ở đây —
// và người thực hiện, không dấu), nguồn phiếu, khoảng ngày. Ngày để trống mặc định:
// trang này để tra lại lịch sử kho.
const search = ref('');
const filterKind = ref('');
// Nguồn phiếu (kind ở API), chia hai nhóm cho ô lọc.
const kindGroups = [
  { label: 'manual', kinds: ['manual_in', 'manual_out'] },
  { label: 'system', kinds: ['order_out', 'order_in', 'count'] }
];
const systemKinds = new Set(kindGroups[1].kinds);
const dateRange = ref<[string, string] | null>(null);

const dialog = ref(false);
const formRef = ref<any>(null);
const form = ref<any>({
  product_id: null,
  transaction_type: 'inbound',
  quantity: 1,
  unit_price: 0
});
const rules = {
  product_id: [
    {
      required: true,
      message: $t('vnetPages.stockTransactions.form.productRequired'),
      trigger: 'change'
    }
  ],
  quantity: [
    {
      required: true,
      message: $t('vnetPages.stockTransactions.form.quantityRequired'),
      trigger: 'blur'
    }
  ]
};

const stockTable = useUIPaginatedTable({
  api: ({ page, pageSize, sort, order }) =>
    client.get('/stock-transactions', {
      params: {
        page,
        page_size: pageSize,
        sort,
        order,
        search: search.value || undefined,
        kind: filterKind.value || undefined,
        date_from: dateRange.value?.[0] || undefined,
        date_to: dateRange.value?.[1] || undefined
      }
    }),
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
      width: 130,
      sortable: 'custom',
      // Hiện NGUỒN phiếu chứ không chỉ Nhập/Xuất: hàng nhân viên tự nhập lẫn với hàng
      // hệ thống trừ khi bán thì đọc sổ không ra. Phiếu hệ thống tự ghi viền rỗng
      // (plain) để tách khỏi phiếu tay. "count" là chốt kiểm kê (số lượng là phần
      // lệch, có thể dương) — gộp vào "Xuất" thì kiểm thấy dư 2 lại hiện "Xuất 2".
      formatter: (row: any) => {
        const tag = { manual_in: 'success', order_in: 'success', count: 'warning' }[row.kind as string] ?? 'danger';
        return h(ElTag, { type: tag as any, effect: systemKinds.has(row.kind) ? 'plain' : 'light' }, () =>
          $t(`vnetPages.stockTransactions.kinds.${row.kind}` as any)
        );
      }
    },
    {
      prop: 'quantity',
      label: $t('vnetPages.stockTransactions.quantity'),
      width: 90,
      sortable: 'custom'
    },
    {
      prop: 'stock_before',
      label: $t('vnetPages.stockTransactions.before'),
      width: 80,
      sortable: 'custom'
    },
    {
      prop: 'stock_after',
      label: $t('vnetPages.stockTransactions.after'),
      width: 80,
      sortable: 'custom'
    },
    {
      prop: 'total_price',
      label: $t('vnetPages.stockTransactions.totalPrice'),
      width: 130,
      sortable: 'custom',
      formatter: (row: any) => formatPrice(row.total_price)
    },
    {
      prop: 'description',
      label: $t('vnetPages.stockTransactions.description'),
      minWidth: 220,
      formatter: (row: any) => row.description || '-'
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
      sortable: 'custom',
      formatter: (row: any) => (row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm') : '')
    }
  ]
});
const { columns, columnChecks, data, getData, getDataByPage, loading, mobilePagination, onSortChange } = stockTable;

// Đổi bộ lọc thì về trang 1: đang ở trang 5 mà kết quả lọc chỉ còn một trang thì
// bảng trống.
function reload() {
  getDataByPage(1);
}

// Phiếu kho phần lớn sinh ra ở nơi khác: xác nhận, thanh toán, huỷ đơn; chốt
// kiểm kê; nhập/xuất ở trang Sản phẩm. Mọi đường đó đều phát stock:changed sau
// khi commit. Tên sản phẩm và người thực hiện tra từ bảng khác.
useRealtimeRefresh(['stock-transactions', 'products', 'systemManage'], getData, { events: ['stock:changed'] });

async function fetchProducts() {
  try {
    const res: any = await client.get('/products', {
      params: { page_size: 1000 }
    });
    products.value = Array.isArray(res) ? res : res?.items || [];
  } catch (_) {}
}

async function handleCreate() {
  form.value = {
    product_id: null,
    transaction_type: 'inbound',
    quantity: 1,
    unit_price: 0
  };
  // fetchProducts trước đây không ai gọi (tệp không có onMounted), nên ô chọn
  // sản phẩm luôn rỗng và hộp thoại này không tạo được phiếu nào.
  await fetchProducts();
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
  <div class="page-fill">
    <ElCard class="fill-card">
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.stockTransactions.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-add="hasAuth('stock.create')"
            :show-delete="false"
            @add="handleCreate"
            @refresh="getData"
          >
            <template #prefix>
              <ElSelect
                v-model="filterKind"
                :placeholder="$t('vnetPages.stockTransactions.transactionType')"
                clearable
                style="width: 150px"
                @change="reload"
              >
                <ElOptionGroup
                  v-for="g in kindGroups"
                  :key="g.label"
                  :label="$t(`vnetPages.stockTransactions.kindGroups.${g.label}` as any)"
                >
                  <ElOption
                    v-for="k in g.kinds"
                    :key="k"
                    :value="k"
                    :label="$t(`vnetPages.stockTransactions.kinds.${k}` as any)"
                  />
                </ElOptionGroup>
              </ElSelect>
              <ElDatePicker
                v-model="dateRange"
                type="daterange"
                range-separator="→"
                :start-placeholder="$t('vnetPages.reports.from')"
                :end-placeholder="$t('vnetPages.reports.to')"
                value-format="YYYY-MM-DD"
                @change="reload"
              />
              <ElInput
                v-model="search"
                :placeholder="$t('vnetPages.stockTransactions.searchPlaceholder')"
                clearable
                style="width: 300px"
                @input="reload"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable
        v-loading="loading"
        :data="data"
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
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
            <ElOption
              v-for="p in products"
              :key="p.id"
              :label="`${p.name} — ${$t('vnetPages.stockTransactions.before')}: ${p.current_stock ?? 0}`"
              :value="p.id"
            />
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
          <ElInputNumber v-bind="moneyInput" v-model="form.unit_price" :min="0" :precision="0" style="width: 100%" />
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
