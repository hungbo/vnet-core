<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { newIdempotencyKey } from '@/utils/idempotency';
import { formatAmount, moneyInput } from '@/utils/money';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');
const filterStatus = ref('');

// Đúng năm trạng thái backend ghi. "confirmed" từng có ở đây nhưng backend
// không bao giờ đặt nó — lọc theo nó luôn ra rỗng (giờ backend trả 400) —
// còn "completed" thì có thật mà thiếu nhãn.
const statusOptions = computed(() => [
  { value: 'pending', label: $t('vnetPages.bookings.statusLabels.pending') },
  {
    value: 'checked_in',
    label: $t('vnetPages.bookings.statusLabels.checkedIn')
  },
  {
    value: 'completed',
    label: $t('vnetPages.bookings.statusLabels.completed')
  },
  {
    value: 'cancelled',
    label: $t('vnetPages.bookings.statusLabels.cancelled')
  },
  { value: 'no_show', label: $t('vnetPages.bookings.statusLabels.noShow') }
]);

function statusType(status: string): any {
  const map: Record<string, string> = {
    pending: 'warning',
    checked_in: 'success',
    completed: 'primary',
    cancelled: 'info',
    no_show: 'danger'
  };
  return map[status] || 'info';
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    pending: $t('vnetPages.bookings.statusLabels.pending'),
    checked_in: $t('vnetPages.bookings.statusLabels.checkedIn'),
    completed: $t('vnetPages.bookings.statusLabels.completed'),
    cancelled: $t('vnetPages.bookings.statusLabels.cancelled'),
    no_show: $t('vnetPages.bookings.statusLabels.noShow')
  };
  return map[status] || status;
}

function canCheckIn(row: any) {
  return row.status === 'pending';
}

function canCancel(row: any) {
  return row.status === 'pending';
}

// Khớp luật xoá của backend: lịch đang chờ mà còn giữ cọc phải Huỷ (để hoàn
// cọc), lịch đã nhận máy đợi tới khi hoàn tất. Còn lại xoá được.
function canDelete(row: any) {
  if (row.status === 'checked_in') return false;
  if (row.status === 'pending' && row.deposit_transaction_id) return false;
  return true;
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
    {
      prop: 'customer_name',
      label: $t('vnetPages.bookings.customer'),
      minWidth: 160
    },
    {
      prop: 'machine_code',
      label: $t('vnetPages.bookings.machineCode'),
      width: 110
    },
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
      formatter: (row: any) => formatAmount(row.deposit_amount)
    }
  ]
});

function fetchData() {
  getData();
}

// Hộp xác nhận và lời gọi API phải nằm ở hai khối try khác nhau. Gói chung vào
// một `catch {}` rỗng thì khi backend từ chối — quá hạn huỷ, đặt chỗ đã
// check-in — nhân viên bấm nút, xác nhận, rồi không thấy gì xảy ra và không
// biết vì sao.
async function confirmed(message: string, type: 'info' | 'warning') {
  try {
    await ElMessageBox.confirm(message, $t('vnetPages.common.confirm'), { type });
    return true;
  } catch {
    return false;
  }
}

async function handleCheckIn(row: any) {
  const ok = await confirmed(
    $t('vnetPages.bookings.messages.checkInConfirm', {
      customer: row.customer_name,
      code: row.machine_code
    }),
    'info'
  );
  if (!ok) return;
  try {
    await client.post(`/bookings/${row.id}/check-in`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.bookings.messages.checkInSuccess')
    });
    fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleCancel(row: any) {
  const ok = await confirmed(
    $t('vnetPages.bookings.messages.cancelConfirm', { customer: row.customer_name }),
    'warning'
  );
  if (!ok) return;
  try {
    await client.post(`/bookings/${row.id}/cancel`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.bookings.messages.cancelSuccess')
    });
    fetchData();
  } catch (e: any) {
    // Cửa sổ huỷ (cancel_before_minutes) trả về lý do cụ thể — hiện nguyên văn.
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

// --- Tạo / sửa / xoá / đánh vắng ---------------------------------------------
// Backend đã có đủ POST/PUT/DELETE//no-show từ trước; trang này chỉ có check-in
// và huỷ, nên đặt chỗ phải chèn tay vào database mới có.

const dialogVisible = ref(false);
const isEdit = ref(false);
const saving = ref(false);
const editingId = ref('');
const machines = ref<any[]>([]);
const members = ref<any[]>([]);

const form = ref<any>({
  machine_id: '',
  machine_code: '',
  member_id: '',
  customer_name: '',
  customer_phone: '',
  range: [] as string[],
  deposit_amount: 0,
  notes: ''
});

async function loadPickers() {
  const [m, mem] = await Promise.all([
    (client.get('/machines', { params: { page_size: 200 } }) as Promise<any>).catch(() => null),
    (client.get('/members', { params: { page_size: 200 } }) as Promise<any>).catch(() => null)
  ]);
  machines.value = Array.isArray(m) ? m : m?.items || [];
  members.value = Array.isArray(mem) ? mem : mem?.items || [];
}

async function openCreate() {
  isEdit.value = false;
  editingId.value = '';
  form.value = {
    machine_id: '',
    machine_code: '',
    member_id: '',
    customer_name: '',
    customer_phone: '',
    range: [],
    deposit_amount: 0,
    notes: '',
    idempotency_key: newIdempotencyKey()
  };
  await loadPickers();
  dialogVisible.value = true;
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    machine_id: row.machine_id || '',
    machine_code: row.machine_code || '',
    member_id: row.member_id || '',
    customer_name: row.customer_name || '',
    customer_phone: row.customer_phone || '',
    range: [row.booked_from, row.booked_to],
    deposit_amount: row.deposit_amount ?? 0,
    notes: row.notes || ''
  };
  await loadPickers();
  dialogVisible.value = true;
}

// Cọc chỉ trừ từ ví hội viên — khách vãng lai không có ví để giữ cọc, nên bỏ
// chọn hội viên thì cọc về 0 (backend cũng từ chối cọc cho khách vãng lai).
function onMemberChange() {
  if (!form.value.member_id) form.value.deposit_amount = 0;
}

async function handleSave() {
  const f = form.value;
  if (!f.customer_name || !f.customer_phone) {
    ElMessage.warning($t('vnetPages.bookings.messages.nameRequired'));
    return;
  }
  if (!isEdit.value && !f.machine_id) {
    ElMessage.warning($t('vnetPages.bookings.messages.machineRequired'));
    return;
  }
  if (!f.range?.length || !f.range[0] || !f.range[1]) {
    ElMessage.warning($t('vnetPages.bookings.messages.rangeRequired'));
    return;
  }

  saving.value = true;
  // Backend đọc thời gian theo RFC3339; ô chọn giờ trả về Date nên phải quy đổi,
  // gửi chuỗi hiển thị "DD/MM/YYYY HH:mm" sẽ bị từ chối.
  const body: Record<string, unknown> = {
    customer_name: f.customer_name,
    customer_phone: f.customer_phone,
    booked_from: dayjs(f.range[0]).toISOString(),
    booked_to: dayjs(f.range[1]).toISOString(),
    notes: f.notes || ''
  };
  try {
    if (isEdit.value) {
      // PUT không nhận machine_id/member_id — đổi máy thì huỷ rồi đặt lại.
      // Cũng không gửi cọc: cọc đã trừ khỏi ví lúc tạo, backend từ chối đổi.
      await client.put(`/bookings/${editingId.value}`, body);
    } else {
      await client.post('/bookings', {
        ...body,
        deposit_amount: f.member_id ? f.deposit_amount || 0 : 0,
        machine_id: f.machine_id,
        member_id: f.member_id || undefined,
        // Đặt chỗ có trừ cọc, nên bấm đúp là trừ cọc hai lần.
        idempotency_key: f.idempotency_key
      });
    }
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.bookings.messages.saveSuccess')
    });
    dialogVisible.value = false;
    fetchData();
  } catch (e: any) {
    // Máy đã có người giữ trong khung giờ đó thì backend từ chối kèm lý do.
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.bookings.messages.deleteConfirm', {
        customer: row.customer_name
      }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/bookings/${row.id}`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.bookings.messages.deleteSuccess')
    });
    fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleNoShow(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.bookings.messages.noShowConfirm', {
        customer: row.customer_name
      }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.post(`/bookings/${row.id}/no-show`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.bookings.messages.noShowSuccess')
    });
    fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

const wsStore = useWebSocketStore();

// Nhận máy, huỷ, hay đánh vắng ở một thiết bị khác — kể cả thiết bị khác của
// cùng nhân viên. Không có handler này thì danh sách bên kia giữ trạng thái cũ
// tới khi có người tự tải lại trang.
function onBookingUpdated() {
  getData();
}

onMounted(() => {
  wsStore.on('booking:updated', onBookingUpdated);
});

onBeforeUnmount(() => {
  wsStore.off('booking:updated', onBookingUpdated);
});
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
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :loading="loading"
          :show-delete="false"
          @add="openCreate"
          @refresh="getData"
        />
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="360" fixed="right">
          <template #default="{ row }">
            <ElButton v-if="canCheckIn(row)" size="small" type="success" @click="handleCheckIn(row)">
              {{ $t('vnetPages.bookings.checkIn') }}
            </ElButton>
            <ElButton v-if="canCheckIn(row)" size="small" type="warning" @click="handleNoShow(row)">
              {{ $t('vnetPages.bookings.noShow') }}
            </ElButton>
            <ElButton v-if="canCancel(row)" size="small" @click="openEdit(row)">
              {{ $t('vnetPages.common.edit') }}
            </ElButton>
            <ElButton v-if="canCancel(row)" size="small" type="danger" plain @click="handleCancel(row)">
              {{ $t('vnetPages.common.cancel') }}
            </ElButton>
            <ElButton v-if="canDelete(row)" size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
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

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.bookings.edit') : $t('vnetPages.bookings.add')"
      width="520px"
    >
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.bookings.machine')">
          <!--
            PUT /bookings/:id không nhận machine_id: đổi máy phải huỷ rồi đặt lại.
            Lúc sửa hiện thẳng mã máy thay vì ô chọn bị khoá — ô khoá trông như
            trống rỗng và người dùng không biết đang giữ máy nào.
          -->
          <strong v-if="isEdit">{{ form.machine_code || '-' }}</strong>
          <ElSelect v-else v-model="form.machine_id" filterable style="width: 100%">
            <ElOption v-for="m in machines" :key="m.id" :label="m.machine_code" :value="m.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="!isEdit" :label="$t('vnetPages.bookings.member')">
          <ElSelect
            v-model="form.member_id"
            filterable
            clearable
            :placeholder="$t('vnetPages.bookings.noMember')"
            style="width: 100%"
            @change="onMemberChange"
          >
            <ElOption
              v-for="m in members"
              :key="m.id"
              :label="`${m.full_name || m.username} — ${m.phone || ''}`"
              :value="m.id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.bookings.customerName')">
          <ElInput v-model="form.customer_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.bookings.customerPhone')">
          <ElInput v-model="form.customer_phone" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.bookings.timeRange')">
          <ElDatePicker
            v-model="form.range"
            type="datetimerange"
            format="DD/MM/YYYY HH:mm"
            range-separator="→"
            style="width: 100%"
          />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.bookings.deposit')">
          <!-- Sửa lịch: cọc chỉ xem, không sửa. Tạo lịch: chỉ nhập được khi đã chọn hội viên. -->
          <ElInputNumber
            v-bind="moneyInput"
            v-model="form.deposit_amount"
            :min="0"
            :step="10000"
            :disabled="isEdit || !form.member_id"
            style="width: 100%"
          />
          <div class="text-12px text-gray-500">
            {{
              isEdit
                ? $t('vnetPages.bookings.depositLockedHint')
                : !form.member_id
                  ? $t('vnetPages.bookings.depositWalkInHint')
                  : $t('vnetPages.bookings.depositRuleHint')
            }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.bookings.notes')">
          <ElInput v-model="form.notes" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
