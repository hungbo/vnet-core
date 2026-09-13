<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { useTableOperate, useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { formatPrice } from '@/utils/money';
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
    {
      prop: 'order_code',
      label: $t('vnetPages.orders.orderCode'),
      width: 140
    },
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
      // Trước khi có bộ máy khuyến mãi, phải-trả luôn bằng tổng nên bảng chỉ
      // hiện một cột. Nay hai số có thể lệch nhau, và nhân viên thu tiền theo
      // cột phải-trả.
      prop: 'discount_amount',
      label: $t('vnetPages.orders.discount'),
      width: 150,
      formatter: (row: any) =>
        row.discount_amount > 0
          ? `-${formatPrice(row.discount_amount)}${row.promotion_name ? ` (${row.promotion_name})` : ''}`
          : '-'
    },
    {
      prop: 'final_amount',
      label: $t('vnetPages.orders.finalAmount'),
      width: 130,
      formatter: (row: any) => formatPrice(row.final_amount)
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

// --- In hoá đơn --------------------------------------------------------------
// Hai loại giấy khác nhau: hoá đơn khách có giá, phiếu chế biến chỉ có món và
// số lượng. Bảng phân luồng ở trang Máy in quyết định món nào ra máy nào.

const printBusy = ref('');
const previewVisible = ref(false);
const previewText = ref('');
const previewLoading = ref(false);

async function handlePrint(row: any) {
  printBusy.value = `${row.id}:receipt`;
  try {
    await client.post(`/orders/${row.id}/print`, {});
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.orders.printed')
    });
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.orders.printFailed'));
  } finally {
    printBusy.value = '';
  }
}

async function handlePrintStations(row: any) {
  printBusy.value = `${row.id}:stations`;
  try {
    const res: any = await client.post(`/orders/${row.id}/print-stations`, {});
    const jobs = res?.jobs || [];
    const unrouted: string[] = res?.unrouted || [];
    if (!jobs.length) {
      ElMessage.warning($t('vnetPages.orders.noStation'));
    } else {
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.orders.printedStations', { count: jobs.length })
      });
    }
    // Món không gán máy in nào thì không có phiếu — nhân viên phải biết ngay,
    // không thì bếp sẽ thiếu món mà không ai hay.
    if (unrouted.length) {
      ElMessage.warning($t('vnetPages.orders.unrouted', { items: unrouted.join(', ') }));
    }
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.orders.printFailed'));
  } finally {
    printBusy.value = '';
  }
}

async function handlePreview(row: any) {
  previewVisible.value = true;
  previewLoading.value = true;
  previewText.value = '';
  try {
    const res: any = await client.get(`/orders/${row.id}/receipt-preview`);
    previewText.value = res?.text || '';
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    previewVisible.value = false;
  } finally {
    previewLoading.value = false;
  }
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

// Completing used to go through the status endpoint, which always recorded the
// payment as cash and never touched a member's balance. Payment now goes
// through /pay, where the method is explicit and a balance payment is real.
const payDialogVisible = ref(false);
const payingOrder = ref<any>(null);
const paying = ref(false);
// Id đơn đang gửi lệnh duyệt. Nút không khoá thì cú bấm thứ hai bắn thêm một
// POST nữa trong lúc cái đầu còn đang bay.
const approvingId = ref('');
const payForm = ref({ payment_method: 'cash', reference_code: '' });

function handleComplete(row: any) {
  payingOrder.value = row;
  payForm.value = { payment_method: 'cash', reference_code: '' };
  payDialogVisible.value = true;
}

async function submitPayment() {
  if (!payingOrder.value) return;
  paying.value = true;
  try {
    // The backend rejects any amount that does not match the order, so the
    // order's own total is what gets sent.
    await client.post(`/orders/${payingOrder.value.id}/pay`, {
      payment_method: payForm.value.payment_method,
      amount: payingOrder.value.final_amount,
      reference_code: payForm.value.reference_code
    });
    ElMessage.success($t('vnetPages.orders.messages.paymentSuccess'));
    payDialogVisible.value = false;
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.orders.messages.updateError'));
  } finally {
    paying.value = false;
  }
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
  } catch (_) {
    // Người dùng bấm Huỷ trong hộp xác nhận.
    return;
  }

  approvingId.value = row.id;
  try {
    await client.post(`/orders/${row.id}/status`, { status: 'completed' });
    ElMessage.success(`Đã nạp ${formatPrice(row.final_amount)} thành công`);
    await fetchData();
  } catch (e: any) {
    // Máy chủ từ chối đơn đã xử lý ("đơn đã được xử lý"). Nuốt lỗi ở đây là để
    // lại toast xanh của lần duyệt trước trên màn hình, và nhân viên tưởng lần
    // bấm này cũng thành công — tức là tưởng đã nạp hai lần.
    ElMessage.error(e?.message || 'Duyệt đơn nạp tiền thất bại');
    await fetchData();
  } finally {
    approvingId.value = '';
  }
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
  return {
    product_id: '',
    quantity: 1,
    selectedOptions: {} as Record<string, number>
  };
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
    const res: any = await client.get('/products', {
      params: { page_size: 1000, is_retail: true }
    });
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

// --- Luồng chế biến -----------------------------------------------------------
// OrderItem.Status khai từ đầu với mặc định "pending" nhưng KHÔNG nơi nào đổi:
// bếp in được phiếu chế biến mà không có cách nào báo lại món đã xong, nên quầy
// phải chạy xuống hỏi.

const ITEM_STATUSES = ['pending', 'preparing', 'ready', 'served'];
const itemStatusBusy = ref('');

async function setItemStatus(item: any, status: string) {
  itemStatusBusy.value = item.id;
  try {
    await client.post(`/orders/${detail.value.id}/items/${item.id}/status`, { status });
    item.status = status;
    ElMessage.success($t('vnetPages.orders.itemStatusChanged'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    itemStatusBusy.value = '';
  }
}

// --- Sửa đơn -----------------------------------------------------------------
// PUT /orders/:id chỉ đổi được số bàn, ghi chú, hội viên và máy — KHÔNG đổi món.
// Form vì vậy chỉ mở đúng bốn trường đó, thay vì hứa sửa cả đơn rồi lặng lẽ bỏ
// qua thay đổi về món.

const editVisible = ref(false);
const editSaving = ref(false);
const editingOrder = ref<any>(null);
const editForm = ref<any>({ table_number: '', note: '', member_id: '', machine_id: '' });
const editMembers = ref<any[]>([]);
const editMachines = ref<any[]>([]);

async function openEdit(row: any) {
  editingOrder.value = row;
  editForm.value = {
    table_number: row.table_number || '',
    note: row.note || '',
    member_id: row.member_id || '',
    machine_id: row.machine_id || ''
  };
  const [mem, mac] = await Promise.all([
    (client.get('/members', { params: { page_size: 200 } }) as Promise<any>).catch(() => null),
    (client.get('/machines', { params: { page_size: 200 } }) as Promise<any>).catch(() => null)
  ]);
  editMembers.value = Array.isArray(mem) ? mem : mem?.items || [];
  editMachines.value = Array.isArray(mac) ? mac : mac?.items || [];
  editVisible.value = true;
}

async function submitEdit() {
  editSaving.value = true;
  try {
    await client.put(`/orders/${editingOrder.value.id}`, {
      table_number: editForm.value.table_number,
      note: editForm.value.note,
      member_id: editForm.value.member_id || undefined,
      machine_id: editForm.value.machine_id || undefined
    });
    ElMessage.success($t('vnetPages.orders.messages.editSuccess'));
    editVisible.value = false;
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.orders.messages.updateError'));
  } finally {
    editSaving.value = false;
  }
}

// --- Tách đơn ----------------------------------------------------------------
// Khách chung bàn trả riêng. Backend nhận danh sách {order_item_id, new_quantity}
// và bắt buộc new_quantity nhỏ hơn số lượng đang có, nên món tách trọn vẹn phải
// làm bằng cách khác — ô nhập vì thế chặn ở max = quantity - 1.

const splitVisible = ref(false);
const splitSaving = ref(false);
const splitOrder = ref<any>(null);
const splitItems = ref<any[]>([]);

// Món chỉ có 1 đơn vị thì không tách được: backend bắt new_quantity nhỏ hơn số
// đang có. Nói thẳng ra thay vì để ô nhập kẹt ở 0 mà không rõ vì sao.
const splittable = computed(() => splitItems.value.some(it => (it.quantity || 0) > 1));

async function openSplit(row: any) {
  splitOrder.value = row;
  splitItems.value = [];
  try {
    const res: any = await client.get(`/orders/${row.id}`);
    splitItems.value = (res?.items || []).map((it: any) => ({ ...it, split_qty: 0 }));
    splitVisible.value = true;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.orders.messages.loadDetailError'));
  }
}

async function submitSplit() {
  const items = splitItems.value
    .filter(it => (it.split_qty || 0) > 0)
    .map(it => ({ order_item_id: it.id, new_quantity: it.split_qty }));
  if (!items.length) {
    ElMessage.warning($t('vnetPages.orders.messages.splitNothing'));
    return;
  }
  splitSaving.value = true;
  try {
    const res: any = await client.post(`/orders/${splitOrder.value.id}/split`, { items });
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.orders.messages.splitSuccess', { code: res?.order_code || '' })
    });
    splitVisible.value = false;
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    splitSaving.value = false;
  }
}

const { checkedRowKeys, onBatchDeleted } = useTableOperate(data, 'id', getData);

async function handleBatchDelete() {
  try {
    await client.delete('/orders/batch-delete', {
      data: { ids: checkedRowKeys.value }
    });
  } catch (e: any) {
    // Không bắt thì 409 ("đơn đã thanh toán, không xoá được") thành unhandled
    // rejection và onBatchDeleted vẫn chạy — bảng làm mới như thể đã xoá xong.
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    return;
  }
  onBatchDeleted();
}

// Toast và âm thanh cho đơn mới nay do useWsNotify() ở base-layout lo, để nhân
// viên nghe được dù đang đứng ở trang nào. Ở đây chỉ còn việc làm mới bảng —
// giữ cả hai chỗ cùng báo sẽ ra hai thông báo chồng nhau.
async function onOrderNew() {
  await getData();
}

// Đơn đổi trạng thái ở một thiết bị khác — kể cả thiết bị khác của chính người
// đang ngồi đây. Không có handler này thì nút "Duyệt" vẫn nằm trên một đơn đã
// nạp xong cho tới khi có người tự tải lại trang.
async function onOrderUpdated() {
  await getData();
}

onMounted(() => {
  wsStore.on('order:new', onOrderNew);
  wsStore.on('order:updated', onOrderUpdated);
});

onBeforeUnmount(() => {
  wsStore.off('order:new', onOrderNew);
  wsStore.off('order:updated', onOrderUpdated);
});
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.orders.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :disabled-delete="checkedRowKeys.length === 0"
            @add="openCreateDialog"
            @delete="handleBatchDelete"
            @refresh="getData"
          >
            <template #prefix>
              <ElRadioGroup v-model="filterType" style="margin-right: 12px" @change="fetchData">
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
      <ElTable
        v-loading="loading"
        :data="data"
        style="width: 100%"
        @selection-change="checkedRowKeys = $event.map((r: any) => r.id)"
      >
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="470" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="viewDetail(row)">{{ $t('vnetPages.common.detail') }}</ElButton>
            <ElDropdown
              v-if="row.order_type !== 'topup'"
              style="margin-left: 8px; margin-right: 4px"
              @command="
                (cmd: string) =>
                  cmd === 'receipt'
                    ? handlePrint(row)
                    : cmd === 'stations'
                      ? handlePrintStations(row)
                      : handlePreview(row)
              "
            >
              <ElButton size="small" :loading="printBusy.startsWith(row.id)">
                {{ $t('vnetPages.orders.print') }}
              </ElButton>
              <template #dropdown>
                <ElDropdownMenu>
                  <ElDropdownItem command="preview">{{ $t('vnetPages.orders.preview') }}</ElDropdownItem>
                  <ElDropdownItem command="receipt" divided>{{ $t('vnetPages.orders.print') }}</ElDropdownItem>
                  <ElDropdownItem command="stations">{{ $t('vnetPages.orders.printStations') }}</ElDropdownItem>
                </ElDropdownMenu>
              </template>
            </ElDropdown>
            <template v-if="row.order_type === 'topup'">
              <ElButton
                v-if="row.status === 'pending'"
                size="small"
                type="success"
                :loading="approvingId === row.id"
                :disabled="approvingId !== ''"
                @click="handleApproveTopup(row)"
              >
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
                @click="openEdit(row)"
              >
                {{ $t('vnetPages.orders.editOrder') }}
              </ElButton>
              <ElButton
                v-if="row.status !== 'completed' && row.status !== 'cancelled'"
                size="small"
                @click="openSplit(row)"
              >
                {{ $t('vnetPages.orders.splitOrder') }}
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

    <ElDialog v-model="editVisible" :title="$t('vnetPages.orders.editOrder')" width="480px">
      <ElForm label-width="120px">
        <ElFormItem :label="$t('vnetPages.orders.orderCode')">
          <strong>{{ editingOrder?.order_code }}</strong>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.tableNumber')">
          <ElInput v-model="editForm.table_number" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.member')">
          <ElSelect v-model="editForm.member_id" filterable clearable style="width: 100%">
            <ElOption v-for="m in editMembers" :key="m.id" :label="m.full_name || m.username" :value="m.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.machineCode')">
          <ElSelect v-model="editForm.machine_id" filterable clearable style="width: 100%">
            <ElOption v-for="m in editMachines" :key="m.id" :label="m.machine_code" :value="m.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.note')">
          <ElInput v-model="editForm.note" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="editVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="editSaving" @click="submitEdit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="splitVisible" :title="$t('vnetPages.orders.splitOrder')" width="620px">
      <ElAlert :type="splittable ? 'info' : 'warning'" :closable="false" show-icon style="margin-bottom: 12px">
        {{ splittable ? $t('vnetPages.orders.splitHint') : $t('vnetPages.orders.splitImpossible') }}
      </ElAlert>
      <ElTable :data="splitItems" border stripe size="small" style="width: 100%">
        <ElTableColumn prop="product_name" :label="$t('vnetPages.orders.product')" min-width="160" />
        <ElTableColumn prop="quantity" :label="$t('vnetPages.orders.quantity')" width="100" align="center" />
        <ElTableColumn :label="$t('vnetPages.orders.unitPrice')" width="120" align="right">
          <template #default="{ row }">{{ formatPrice(row.unit_price) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.orders.splitQuantity')" width="160">
          <template #default="{ row }">
            <ElInputNumber
              v-model="row.split_qty"
              :min="0"
              :max="Math.max(0, row.quantity - 1)"
              :disabled="row.quantity <= 1"
              size="small"
            />
          </template>
        </ElTableColumn>
      </ElTable>
      <template #footer>
        <ElButton @click="splitVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="splitSaving" :disabled="!splittable" @click="submitSplit">
          {{ $t('vnetPages.orders.splitOrder') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="previewVisible" :title="$t('vnetPages.orders.preview')" width="460px">
      <div v-loading="previewLoading">
        <!--
 Giấy nhiệt là font đơn cách; xem trước phải cùng kiểu chữ thì mới
             thấy đúng cột tiền có thẳng hàng hay không. 
-->
        <pre
          style="
            font-family: ui-monospace, Menlo, Consolas, monospace;
            font-size: 13px;
            line-height: 1.5;
            background: var(--el-fill-color-light);
            padding: 16px;
            border-radius: 4px;
            overflow-x: auto;
            margin: 0;
          "
          >{{ previewText }}</pre>
      </div>
      <template #footer>
        <ElButton @click="previewVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="detailVisible" :title="$t('vnetPages.orders.detail')" width="840px">
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
          <ElDescriptionsItem v-if="detail.discount_amount > 0" :label="$t('vnetPages.orders.discount')">
            <span style="color: #67c23a">-{{ formatPrice(detail.discount_amount) }}</span>
            <span v-if="detail.promotion_name">({{ detail.promotion_name }})</span>
          </ElDescriptionsItem>
          <ElDescriptionsItem :label="$t('vnetPages.orders.finalAmount')">
            <strong>{{ formatPrice(detail.final_amount) }}</strong>
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
        <h4 style="margin: 20px 0 10px">
          {{ $t('vnetPages.orders.products') }}
        </h4>
        <ElTable :data="detail.items || []" border size="small">
          <ElTableColumn prop="product_name" :label="$t('vnetPages.orders.product')" min-width="120" />
          <ElTableColumn prop="quantity" :label="$t('vnetPages.orders.quantity')" width="60" />
          <ElTableColumn :label="$t('vnetPages.orders.options')" min-width="180">
            <template #default="{ row }">
              <div v-for="opt in row.option_list || []" :key="opt.option_id" style="font-size: 12px; line-height: 1.6">
                {{ opt.name }} (x{{ opt.quantity }}):
                {{ formatPrice(opt.price) }}
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
          <ElTableColumn :label="$t('vnetPages.orders.itemStatus')" width="150">
            <template #default="{ row }">
              <ElSelect
                :model-value="row.status || 'pending'"
                size="small"
                :loading="itemStatusBusy === row.id"
                :disabled="detail.status === 'cancelled'"
                style="width: 130px"
                @change="(v: string) => setItemStatus(row, v)"
              >
                <ElOption
                  v-for="st in ITEM_STATUSES"
                  :key="st"
                  :label="$t(`vnetPages.orders.itemStatusLabels.${st}`)"
                  :value="st"
                />
              </ElSelect>
            </template>
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
    <ElDialog v-model="payDialogVisible" :title="$t('vnetPages.orders.payTitle')" width="420px">
      <ElForm label-width="130px">
        <ElFormItem :label="$t('vnetPages.orders.payAmount')">
          <strong>{{ formatPrice(payingOrder?.final_amount ?? 0) }}</strong>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.orders.payMethod')">
          <ElSelect v-model="payForm.payment_method" style="width: 100%">
            <ElOption :label="$t('vnetPages.orders.payCash')" value="cash" />
            <ElOption :label="$t('vnetPages.orders.payBalance')" value="balance" />
            <ElOption :label="$t('vnetPages.orders.payTransfer')" value="transfer" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="payForm.payment_method === 'transfer'" :label="$t('vnetPages.orders.payReference')">
          <ElInput v-model="payForm.reference_code" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="payDialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="paying" @click="submitPayment">
          {{ $t('vnetPages.orders.pay') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
