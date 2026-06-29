<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import { useWebSocketStore } from '@/store/modules/ws';
import client from '@/api/client';
import { useUIPaginatedTable, useTableOperate } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();
const wsStore = useWebSocketStore();

const search = ref('');
const filterStatus = ref(null);
const filterType = ref('');
const dateRange = ref(null);
const detailVisible = ref(false);
const detail = ref<any>(null);

const statusOptions = computed(() => [
  { value: 'pending', label: $t('vnetPages.orders.statusLabels.pending') },
  { value: 'confirmed', label: $t('vnetPages.orders.statusLabels.confirmed') },
  { value: 'completed', label: $t('vnetPages.orders.statusLabels.completed') },
  { value: 'cancelled', label: $t('vnetPages.orders.statusLabels.cancelled') }
]);

const statusMap = computed(() => Object.fromEntries(statusOptions.value.map(s => [s.value, s])));

function statusType(status: string): any {
  const map: Record<string, string> = {
    pending: 'info',
    confirmed: 'primary',
    completed: 'success',
    cancelled: 'danger'
  };
  return map[status] || 'info';
}

function statusLabel(status: string) {
  return statusMap.value[status]?.label || status;
}

function formatPrice(price: number) {
  return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(price || 0);
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/orders', {
      params: {
        page,
        page_size: pageSize,
        search: search.value || undefined,
        status: filterStatus.value || undefined,
        order_type: filterType.value || undefined,
        date_from: dateRange.value?.[0] || undefined,
        date_to: dateRange.value?.[1] || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'selection', type: 'selection', width: 48 },
    { prop: 'order_code', label: $t('vnetPages.orders.orderCode'), width: 140 },
    {
      prop: 'status',
      label: $t('vnetPages.common.status'),
      width: 120,
      formatter: (row: any) => h(ElTag, { type: statusType(row.status) }, () => statusLabel(row.status))
    },
    {
      prop: 'order_type',
      label: $t('vnetPages.orders.orderType'),
      width: 100,
      formatter: (row: any) => {
        if (row.order_type === 'topup') return $t('vnetPages.orders.topup');
        return $t('vnetPages.orders.product');
      }
    },
    {
      prop: 'member_name',
      label: $t('vnetPages.orders.member'),
      width: 120,
      formatter: (row: any) => row.member_username || row.member_name || '-'
    },
    {
      prop: 'total_amount',
      label: $t('vnetPages.orders.total'),
      width: 130,
      formatter: (row: any) => formatPrice(row.total_amount)
    },
    {
      prop: 'payment_method',
      label: $t('vnetPages.orders.paymentMethod'),
      width: 110,
      formatter: (row: any) => row.payment_method || '-'
    },
    {
      prop: 'machine_code',
      label: $t('vnetPages.orders.machineCode'),
      width: 100,
      formatter: (row: any) => row.machine_code || row.machine_id || '-'
    },
    {
      prop: 'updated_by_name',
      label: $t('vnetPages.orders.operator'),
      width: 120,
      formatter: (row: any) => row.updated_by_name || '-'
    },
    {
      prop: 'created_at',
      label: $t('vnetPages.orders.createdAt'),
      width: 160,
      formatter: (row: any) => (row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm') : '')
    },
    {
      prop: 'completed_at',
      label: $t('vnetPages.orders.completedAt'),
      width: 160,
      formatter: (row: any) => (row.completed_at ? dayjs(row.completed_at).format('DD/MM/YYYY HH:mm') : '-')
    }
  ]
});

function fetchData() {
  getData();
}

async function viewDetail(row: any) {
  try {
    const res: any = await client.get(`/orders/${row.id}`);
    detail.value = res;
    detailVisible.value = true;
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.orders.messages.loadDetailError'));
  }
}

async function updateStatus(row: any, status: string) {
  try {
    await client.post(`/orders/${row.id}/status`, { status });
    ElMessage.success($t('vnetPages.orders.messages.updateSuccess'));
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.orders.messages.updateError'));
  }
}

async function handleConfirm(row: any) {
  await updateStatus(row, 'confirmed');
}

async function handleComplete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.orders.messages.paymentConfirm', { code: row.order_code }),
      $t('vnetPages.common.confirm'),
      { type: 'info' }
    );
    await client.post(`/orders/${row.id}/status`, { status: 'completed' });
    ElMessage.success($t('vnetPages.orders.messages.paymentSuccess'));
    await fetchData();
  } catch (_) {}
}

async function handleCancel(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.orders.messages.cancelConfirm', { code: row.order_code }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await updateStatus(row, 'cancelled');
  } catch (_) {}
}

async function handleApproveTopup(row: any) {
  try {
    await ElMessageBox.confirm(
      `Xác nhận nạp ${formatPrice(row.final_amount)} cho ${row.member_username || row.member_name || row.member_id}?`,
      $t('vnetPages.common.confirm'),
      { type: 'info' }
    );
    await client.post(`/orders/${row.id}/status`, { status: 'completed' });
    ElMessage.success(`Đã nạp ${formatPrice(row.final_amount)} thành công`);
    await fetchData();
  } catch (_) {}
}

async function handleRejectTopup(row: any) {
  try {
    await ElMessageBox.confirm(
      `Từ chối nạp ${formatPrice(row.final_amount)} cho ${row.member_username || row.member_name || row.member_id}?`,
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await updateStatus(row, 'cancelled');
  } catch (_) {}
}

// Create order
const createVisible = ref(false);
const createSaving = ref(false);
const createFormRef = ref<any>(null);
const allProducts = ref<any[]>([]);

const createForm = ref<any>({ note: '', items: [] });
const createRules = {};

function newOrderItem() {
  return { product_id: '', quantity: 1, selectedOptions: {} as Record<string, number> };
}

function addItem() {
  createForm.value.items.push(newOrderItem());
}

function removeItem(idx: number) {
  createForm.value.items.splice(idx, 1);
}

function getProductOptions(pid: string) {
  const p = allProducts.value.find((p: any) => p.id === pid);
  return p?.options || [];
}

function onProductChange(idx: number, _val: string) {
  createForm.value.items[idx].selectedOptions = {};
}

const computeTotal = computed(() => {
  let total = 0;
  for (const item of createForm.value.items) {
    if (!item.product_id) continue;
    const p = allProducts.value.find((p: any) => p.id === item.product_id);
    if (!p) continue;
    let itemTotal = p.price * item.quantity;
    for (const [optId, qty] of Object.entries(item.selectedOptions || {})) {
      if (qty > 0) {
        const opt = getProductOptions(item.product_id).find((o: any) => o.id === optId);
        if (opt) itemTotal += opt.current_price * (qty as number);
      }
    }
    total += itemTotal;
  }
  return total;
});

async function openCreateDialog() {
  createForm.value = { note: '', items: [newOrderItem()] };
  try {
    const res: any = await client.get('/products', { params: { page_size: 1000, is_retail: true } });
    allProducts.value = Array.isArray(res) ? res : res?.items || [];
  } catch (_) {}
  createVisible.value = true;
}

async function handleCreate() {
  const valid = await createFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  createSaving.value = true;
  try {
    const items = createForm.value.items.map((item: any) => {
      const opts = Object.entries(item.selectedOptions || {})
        .filter(([_, qty]) => (qty as number) > 0)
        .map(([optId, qty]) => ({ option_id: optId, quantity: qty as number }));
      return {
        product_id: item.product_id,
        quantity: item.quantity,
        options: opts.length > 0 ? JSON.stringify(opts) : '',
        note: ''
      };
    });
    await client.post('/orders', { note: createForm.value.note, items });
    ElMessage.success($t('vnetPages.orders.messages.createSuccess'));
    createVisible.value = false;
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.orders.messages.createError'));
  } finally {
    createSaving.value = false;
  }
}

function playOrderSound(orderType?: string) {
  const src = orderType === 'topup' ? '/audio/deposit.mp3' : '/audio/order.mp3';
  const audio = new Audio(src);
  audio.volume = 0.5;
  audio.play().catch(() => {});
}

const { checkedRowKeys, onBatchDeleted } = useTableOperate(data, 'id', getData);

async function handleBatchDelete() {
  await client.delete('/orders/batch-delete', { data: { ids: checkedRowKeys.value } });
  onBatchDeleted();
}

onMounted(() => {
  wsStore.on('order:new', async (data?: any) => {
    await getData();
    playOrderSound(data?.order_type);
    ElNotification({
      title: $t('vnetPages.orders.newOrder'),
      message: $t('vnetPages.orders.newOrderMessage'),
      type: 'info',
      duration: 5000
    });
  });
});

onBeforeUnmount(() => {
  wsStore.off('order:new');
});
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.orders.title') }}</span>
          <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" :disabled-delete="checkedRowKeys.length === 0" @add="openCreateDialog" @delete="handleBatchDelete" @refresh="getData">
            <template #prefix>
              <ElRadioGroup v-model="filterType" @change="fetchData" style="margin-right: 12px">
                <ElRadioButton value="">{{ $t('vnetPages.common.all') }}</ElRadioButton>
                <ElRadioButton value="product">{{ $t('vnetPages.orders.product') }}</ElRadioButton>
                <ElRadioButton value="topup">{{ $t('vnetPages.orders.topup') }}</ElRadioButton>
              </ElRadioGroup>
              <ElSelect
                v-model="filterStatus"
                :placeholder="$t('vnetPages.common.status')"
                clearable
                style="width: 140px"
                @change="fetchData"
              >
                <ElOption v-for="s in statusOptions" :key="s.value" :label="s.label" :value="s.value" />
              </ElSelect>
              <ElDatePicker
                v-model="dateRange"
                type="daterange"
                range-separator="->"
                :start-placeholder="$t('vnetPages.reports.from')"
                :end-placeholder="$t('vnetPages.reports.to')"
                value-format="YYYY-MM-DD"
                @change="fetchData"
              />
              <ElInput
                v-model="search"
                :placeholder="$t('vnetPages.orders.search') + '...'"
                clearable
                style="width: 160px"
                @input="fetchData"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%" @selection-change="checkedRowKeys = $event.map((r: any) => r.id)">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="320" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="viewDetail(row)">{{ $t('vnetPages.common.detail') }}</ElButton>
            <template v-if="row.order_type === 'topup'">
              <ElButton v-if="row.status === 'pending'" size="small" type="success" @click="handleApproveTopup(row)">
                {{ $t('vnetPages.orders.approve') }}
              </ElButton>
              <ElButton
                v-if="row.status === 'pending'"
                size="small"
                type="danger"
                plain
                @click="handleRejectTopup(row)"
              >
                {{ $t('vnetPages.orders.reject') }}
              </ElButton>
            </template>
            <template v-else>
              <ElButton v-if="row.status === 'pending'" size="small" type="primary" @click="handleConfirm(row)">
                {{ $t('vnetPages.orders.confirm') }}
              </ElButton>
              <ElButton v-if="row.status === 'confirmed'" size="small" type="success" @click="handleComplete(row)">
                {{ $t('vnetPages.orders.pay') }}
              </ElButton>
              <ElButton
                v-if="row.status !== 'completed' && row.status !== 'cancelled'"
                size="small"
                type="danger"
                plain
                @click="handleCancel(row)"
              >
                {{ $t('vnetPages.orders.cancel') }}
              </ElButton>
            </template>
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

    <ElDialog v-model="detailVisible" :title="$t('vnetPages.orders.detail')" width="700px">
      <template v-if="detail">
        <ElDescriptions :column="2" border>
          <ElDescriptionsItem :label="$t('vnetPages.orders.orderCode')">{{ detail.order_code }}</ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.common.status')">
            <ElTag :type="statusType(detail.status)">{{ statusLabel(detail.status) }}</ElTag>
          </ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.orders.member')">
            {{ detail.member_name || detail.member_id || '-' }}
          </ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.orders.machineCode')">
            {{ detail.machine_code || detail.machine_id || '-' }}
          </ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.orders.total')">
            {{ formatPrice(detail.total_amount) }}
          </ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.orders.createdAt')">
            {{ dayjs(detail.created_at).format('DD/MM/YYYY HH:mm') }}
          </ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.orders.operator')">
            {{ detail.updated_by_name || '-' }}
          </ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.orders.note')" :span="2">
            {{ detail.note || '-' }}
          </ElDescriptionsItem>
        </ElDescriptions>
        <h4 style="margin: 20px 0 10px">{{ $t('vnetPages.orders.products') }}</h4>
        <ElTable :data="detail.items || []" border size="small">
          <ElTableColumn prop="product_name" :label="$t('vnetPages.orders.product')" min-width="120" />
          <ElTableColumn prop="quantity" :label="$t('vnetPages.orders.quantity')" width="60" />
          <ElTableColumn :label="$t('vnetPages.orders.options')" min-width="180">
            <template #default="{ row }">
              <div v-for="opt in row.option_list || []" :key="opt.option_id" style="font-size: 12px; line-height: 1.6">
                {{ opt.name }} (x{{ opt.quantity }}): {{ formatPrice(opt.price) }}
              </div>
              <span v-if="!row.option_list?.length" style="color: #999">—</span>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="$t('vnetPages.orders.unitPrice')" width="120">
            <template #default="{ row }">{{ formatPrice(row.unit_price) }}</template>
          </ElTableColumn>
          <ElTableColumn :label="$t('vnetPages.orders.subtotal')" width="120">
            <template #default="{ row }">{{ formatPrice(row.subtotal) }}</template>
          </ElTableColumn>
        </ElTable>
      </template>
      <template #footer>
        <ElButton @click="detailVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="createVisible" :title="$t('vnetPages.orders.createOrder')" width="700px">
      <ElForm ref="createFormRef" :model="createForm" :rules="createRules" :label-width="100">
        <ElFormItem :label="$t('vnetPages.orders.note')" prop="note">
          <ElInput v-model="createForm.note" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.selectProduct')" prop="items">
          <div style="width: 100%">
            <div
              v-for="(item, idx) in createForm.items"
              :key="idx"
              style="border: 1px solid #eee; border-radius: 6px; padding: 12px; margin-bottom: 12px"
            >
              <div style="display: flex; gap: 8px; align-items: center; margin-bottom: 8px">
                <ElSelect
                  v-model="item.product_id"
                  filterable
                  :placeholder="$t('vnetPages.orders.selectProduct')"
                  style="flex: 1"
                  @change="val => onProductChange(idx, val)"
                >
                  <ElOption v-for="p in allProducts" :key="p.id" :label="p.name" :value="p.id" />
                </ElSelect>
                <ElInputNumber v-model="item.quantity" :min="1" style="width: 100px" />
                <ElButton type="danger" size="small" @click="removeItem(idx)">
                  {{ $t('vnetPages.common.delete') }}
                </ElButton>
              </div>
              <div v-if="item.product_id && getProductOptions(item.product_id).length > 0" style="padding-left: 8px">
                <div
                  v-for="opt in getProductOptions(item.product_id)"
                  :key="opt.id"
                  style="display: flex; align-items: center; gap: 8px; margin: 4px 0"
                >
                  <span style="min-width: 120px">{{ opt.name }} (+{{ formatPrice(opt.current_price) }})</span>
                  <ElInputNumber
                    v-model="item.selectedOptions[opt.id]"
                    :min="0"
                    :max="99"
                    size="small"
                    style="width: 100px"
                  />
                </div>
              </div>
            </div>
            <ElButton size="small" @click="addItem">{{ $t('vnetPages.common.add') }}</ElButton>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.previewTotal')">
          <span style="font-size: 18px; font-weight: bold; color: #f56c6c">{{ formatPrice(computeTotal) }}</span>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="createVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="createSaving" @click="handleCreate">
          {{ $t('vnetPages.common.confirm') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
