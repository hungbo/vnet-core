<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { newIdempotencyKey } from '@/utils/idempotency';
import { formatAmount } from '@/utils/money';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const formRef = ref<FormInstance>();

const form = ref({
  name: '',
  type: 'fixed_slot',
  price: 0,
  description: '',
  validity_days: 30,
  apply_days: [] as number[],
  minutes: 60,
  slots: [{ start: null, end: null }],
  is_active: true
});

const purchaseDialogVisible = ref(false);
const purchaseSubmitting = ref(false);
const purchaseFormRef = ref<FormInstance>();
const purchaseForm = ref({
  member_id: '' as string,
  customer_name: '',
  customer_phone: '',
  payment_method: 'cash',
  idempotency_key: newIdempotencyKey()
});
const members = ref<any[]>([]);
const memberSearchLoading = ref(false);
const purchaseComboId = ref('');

async function searchMembers(query: string) {
  if (!query) {
    members.value = [];
    return;
  }
  memberSearchLoading.value = true;
  try {
    const res: any = await client.get('/members', {
      params: { page: 1, page_size: 20, search: query }
    });
    members.value = res.items || [];
  } catch {
    members.value = [];
  } finally {
    memberSearchLoading.value = false;
  }
}

const rules: FormRules = {
  name: [
    {
      required: true,
      message: $t('vnetPages.combos.form.nameRequired'),
      trigger: 'blur'
    }
  ],
  type: [
    {
      required: true,
      message: $t('vnetPages.combos.form.typeRequired'),
      trigger: 'change'
    }
  ],
  price: [
    {
      required: true,
      message: $t('vnetPages.combos.form.priceRequired'),
      trigger: 'blur'
    }
  ],
  minutes: [
    {
      validator: (_rule: any, value: number, callback: any) => {
        if (form.value.type === 'prepaid' && (!value || value <= 0)) {
          callback(new Error($t('vnetPages.combos.form.minutesRequired')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ]
};

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/combos', {
      params: {
        page,
        page_size: pageSize,
        search: search.value || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.combos.name'), minWidth: 160 },
    {
      prop: 'type',
      label: $t('vnetPages.combos.type'),
      width: 110,
      formatter: (row: any) =>
        h(
          ElTag,
          {
            type: row.type === 'fixed_slot' ? 'warning' : 'success',
            size: 'small'
          },
          () => (row.type === 'fixed_slot' ? $t('vnetPages.combos.fixedSlot') : $t('vnetPages.combos.prepaid'))
        )
    },
    {
      prop: 'price',
      label: $t('vnetPages.combos.price'),
      width: 110,
      formatter: (row: any) => formatAmount(row.price)
    },
    {
      prop: 'is_active',
      label: $t('vnetPages.combos.isActive'),
      width: 90,
      formatter: (row: any) =>
        h(ElTag, { type: row.is_active ? 'success' : 'danger', size: 'small' }, () =>
          row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no')
        )
    }
  ]
});

function fetchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = {
    name: '',
    type: 'fixed_slot',
    price: 0,
    description: '',
    validity_days: 30,
    apply_days: [],
    minutes: 60,
    slots: [{ start: null, end: null }],
    is_active: true
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    type: row.type || 'fixed_slot',
    price: row.price ?? 0,
    description: row.description || '',
    validity_days: row.validity_days ?? 30,
    apply_days: row.apply_days ?? [],
    minutes: row.minutes ?? 60,
    slots: row.slots?.length
      ? row.slots.map((s: any) => ({ start: s.start, end: s.end }))
      : [{ start: null, end: null }],
    is_active: row.is_active ?? true
  };
  dialogVisible.value = true;
}

function openPurchase(row: any) {
  purchaseComboId.value = row.id;
  purchaseForm.value = {
    member_id: '',
    customer_name: '',
    customer_phone: '',
    payment_method: 'cash',
    // Khoá mới cho mỗi lần mở hộp thoại; mọi lần bấm Xác nhận của lần mở này
    // mang cùng khoá nên máy chủ chỉ bán một gói.
    idempotency_key: newIdempotencyKey()
  };
  members.value = [];
  purchaseDialogVisible.value = true;
}

async function handlePurchase() {
  purchaseSubmitting.value = true;
  try {
    const payload: any = {
      payment_method: purchaseForm.value.payment_method,
      idempotency_key: purchaseForm.value.idempotency_key
    };
    if (purchaseForm.value.member_id) {
      payload.member_id = purchaseForm.value.member_id;
    } else {
      if (!purchaseForm.value.customer_name) throw new Error(`${$t('vnetPages.combos.customerName')} required`);
      payload.customer_name = purchaseForm.value.customer_name;
      payload.customer_phone = purchaseForm.value.customer_phone;
    }
    await client.post(`/combos/${purchaseComboId.value}/purchase`, payload);
    purchaseDialogVisible.value = false;
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.combos.messages.purchaseSuccess')
    });
    fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.combos.messages.saveError'));
  } finally {
    purchaseSubmitting.value = false;
  }
}

// --- Gói đã bán & kích hoạt ---------------------------------------------------
// POST /combos/:id/activate nhận ID của LƯỢT MUA chứ không phải ID gói, nên
// thao tác này phải bắt đầu từ hội viên: chọn người → xem gói họ đã mua → kích
// hoạt lên một máy. Không có màn hình này thì gói bán ra không bao giờ dùng được.

const purchasesVisible = ref(false);
const purchasesLoading = ref(false);
const purchasesMemberId = ref('');
const purchases = ref<any[]>([]);
const activatingId = ref('');
const activateMachineId = ref('');
const activateMachines = ref<any[]>([]);

async function openPurchases() {
  purchasesMemberId.value = '';
  purchases.value = [];
  members.value = [];
  activatingId.value = '';
  activateMachineId.value = '';
  const res: any = await client.get('/machines', { params: { page_size: 200 } }).catch(() => null);
  const list: any[] = Array.isArray(res) ? res : res?.items || [];
  // Máy đang có người ngồi thì backend từ chối kích hoạt, nên không đưa vào ô chọn.
  activateMachines.value = list.filter(m => m.status !== 'in_use');
  purchasesVisible.value = true;
}

async function loadPurchases() {
  if (!purchasesMemberId.value) {
    purchases.value = [];
    return;
  }
  purchasesLoading.value = true;
  try {
    const res: any = await client.get(`/members/${purchasesMemberId.value}/combos`);
    purchases.value = Array.isArray(res) ? res : res?.items || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.combos.messages.loadError'));
    purchases.value = [];
  } finally {
    purchasesLoading.value = false;
  }
}

async function handleActivate(row: any) {
  if (!activateMachineId.value) {
    ElMessage.warning($t('vnetPages.combos.selectMachine'));
    return;
  }
  activatingId.value = row.id;
  try {
    await client.post(`/combos/${row.id}/activate`, { machine_id: activateMachineId.value });
    const machine = activateMachines.value.find(m => m.id === activateMachineId.value);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.combos.activateSuccess', { code: machine?.machine_code || '' })
    });
    await loadPurchases();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    activatingId.value = '';
  }
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    if (form.value.type === 'fixed_slot' && (!form.value.slots?.[0]?.start || !form.value.slots?.[0]?.end)) {
      ElMessage.warning('Slot start and end are required for fixed slot');
      submitting.value = false;
      return;
    }
    const payload: any = {
      name: form.value.name,
      type: form.value.type,
      price: form.value.price,
      description: form.value.description || '',
      validity_days: form.value.validity_days,
      apply_days: form.value.apply_days,
      is_active: form.value.is_active
    };
    if (form.value.type === 'fixed_slot') {
      payload.slot_start = form.value.slots?.[0]?.start ? dayjs(form.value.slots[0].start).format('HH:mm') : '';
      payload.slot_end = form.value.slots?.[0]?.end ? dayjs(form.value.slots[0].end).format('HH:mm') : '';
    } else {
      payload.total_minutes = form.value.minutes;
    }
    if (isEdit.value && editingId.value) {
      await client.put(`/combos/${editingId.value}`, payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.combos.messages.editSuccess')
      });
    } else {
      await client.post('/combos', payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.combos.messages.addSuccess')
      });
    }
    dialogVisible.value = false;
    fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.combos.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.combos.messages.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/combos/${row.id}`);
    ElMessage.success($t('vnetPages.combos.messages.deleteSuccess'));
    fetchData();
  } catch (e: any) {
    // ElMessageBox từ chối bằng chuỗi 'cancel'/'close' khi người dùng bấm Huỷ —
    // đó không phải lỗi. Còn lại là backend từ chối (ràng buộc dữ liệu, quy tắc
    // nghiệp vụ) và phải nói ra; `catch {}` rỗng làm nút bấm vào im lặng.
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.combos.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="fetchData"
          />
          <ElButton type="primary" @click="fetchData">{{ $t('vnetPages.common.search') }}</ElButton>
          <ElButton @click="openPurchases">{{ $t('vnetPages.combos.comboPurchases') }}</ElButton>
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
        <ElTableColumn :label="$t('vnetPages.common.action')" width="270" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openPurchase(row)">{{ $t('vnetPages.combos.purchase') }}</ElButton>
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
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

    <ElDialog v-model="purchasesVisible" :title="$t('vnetPages.combos.comboPurchases')" width="820px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        {{ $t('vnetPages.combos.purchasesHint') }}
      </ElAlert>

      <div style="display: flex; gap: 8px; margin-bottom: 12px; align-items: center">
        <ElSelect
          v-model="purchasesMemberId"
          filterable
          remote
          clearable
          :remote-method="searchMembers"
          :loading="memberSearchLoading"
          :placeholder="$t('vnetPages.combos.selectMember')"
          style="width: 260px"
          @change="loadPurchases"
        >
          <ElOption
            v-for="m in members"
            :key="m.id"
            :label="`${m.full_name || m.username} — ${m.phone || ''}`"
            :value="m.id"
          />
        </ElSelect>
        <ElSelect
          v-model="activateMachineId"
          filterable
          clearable
          :placeholder="$t('vnetPages.combos.selectMachine')"
          style="width: 200px"
        >
          <ElOption v-for="m in activateMachines" :key="m.id" :label="m.machine_code" :value="m.id" />
        </ElSelect>
      </div>

      <ElTable v-loading="purchasesLoading" :data="purchases" border stripe size="small" style="width: 100%">
        <ElTableColumn prop="combo_name" :label="$t('vnetPages.combos.name')" min-width="160" />
        <ElTableColumn :label="$t('vnetPages.combos.purchasedAt')" width="150">
          <template #default="{ row }">
            {{ row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm') : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.combos.remainingMinutes')" width="100" align="center">
          <template #default="{ row }">{{ row.remaining_minutes ?? 0 }}p</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.combos.status')" width="130">
          <template #default="{ row }">
            <ElTag :type="row.activated ? 'success' : 'warning'" size="small">
              {{ row.activated ? $t('vnetPages.combos.activated') : $t('vnetPages.combos.pending') }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="120">
          <template #default="{ row }">
            <ElButton
              v-if="!row.activated"
              size="small"
              type="primary"
              :loading="activatingId === row.id"
              @click="handleActivate(row)"
            >
              {{ $t('vnetPages.combos.activate') }}
            </ElButton>
            <span v-else style="color: #909399">-</span>
          </template>
        </ElTableColumn>
        <template #empty>
          <span style="color: #909399">{{ $t('vnetPages.combos.noPurchases') }}</span>
        </template>
      </ElTable>

      <template #footer>
        <ElButton @click="purchasesVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.combos.edit') : $t('vnetPages.combos.add')"
      width="550px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.combos.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.type')" prop="type">
          <ElSelect v-model="form.type" style="width: 100%">
            <ElOption :label="$t('vnetPages.combos.fixedSlot')" value="fixed_slot" />
            <ElOption :label="$t('vnetPages.combos.prepaid')" value="prepaid" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.price')" prop="price">
          <ElInputNumber v-model="form.price" :min="0" :step="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.description')" prop="description">
          <ElInput v-model="form.description" type="textarea" :rows="2" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.validityDays')" prop="validity_days">
          <ElInputNumber v-model="form.validity_days" :min="0" :step="1" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.applyDays')" prop="apply_days">
          <ElCheckboxGroup v-model="form.apply_days">
            <ElCheckbox v-for="d in 7" :key="d - 1" :label="d - 1" :value="d - 1">
              {{ ['CN', 'T2', 'T3', 'T4', 'T5', 'T6', 'T7'][d - 1] }}
            </ElCheckbox>
          </ElCheckboxGroup>
        </ElFormItem>
        <template v-if="form.type === 'fixed_slot'">
          <ElFormItem v-for="(slot, idx) in form.slots" :key="idx" :label="`Slot ${idx + 1}`">
            <div style="display: flex; gap: 8px; width: 100%">
              <ElTimePicker
                v-model="slot.start"
                format="HH:mm"
                :placeholder="$t('vnetPages.combos.from')"
                style="flex: 1"
              />
              <ElTimePicker
                v-model="slot.end"
                format="HH:mm"
                :placeholder="$t('vnetPages.combos.to')"
                style="flex: 1"
              />
              <ElButton icon="Delete" @click="form.slots.splice(idx, 1)" />
            </div>
          </ElFormItem>
          <ElFormItem label=" ">
            <ElButton type="primary" link @click="form.slots.push({ start: null, end: null })">
              {{ $t('vnetPages.combos.addSlot') }}
            </ElButton>
          </ElFormItem>
        </template>
        <template v-if="form.type === 'prepaid'">
          <ElFormItem :label="$t('vnetPages.combos.minutes')" prop="minutes">
            <ElInputNumber v-model="form.minutes" :min="1" :step="30" style="width: 100%" />
          </ElFormItem>
        </template>
        <ElFormItem :label="$t('vnetPages.combos.isActive')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="purchaseDialogVisible" :title="$t('vnetPages.combos.purchaseDialog')" width="450px">
      <ElForm ref="purchaseFormRef" :model="purchaseForm" :label-width="120">
        <ElFormItem :label="$t('vnetPages.combos.selectMember')">
          <ElSelect
            v-model="purchaseForm.member_id"
            filterable
            remote
            :remote-method="searchMembers"
            :loading="memberSearchLoading"
            clearable
            style="width: 100%"
            :placeholder="$t('vnetPages.combos.selectMember')"
          >
            <ElOption
              v-for="m in members"
              :key="m.id"
              :label="`${m.full_name || m.username} (${m.phone || '-'})`"
              :value="m.id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.customerName')" prop="customer_name">
          <ElInput v-model="purchaseForm.customer_name" :disabled="!!purchaseForm.member_id" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.customerPhone')" prop="customer_phone">
          <ElInput v-model="purchaseForm.customer_phone" :disabled="!!purchaseForm.member_id" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.paymentMethod')" prop="payment_method">
          <ElSelect v-model="purchaseForm.payment_method" style="width: 100%">
            <ElOption label="Cash" value="cash" />
            <ElOption label="Balance" value="balance" />
            <ElOption label="Transfer" value="transfer" />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="purchaseDialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="purchaseSubmitting" @click="handlePurchase">
          {{ $t('vnetPages.combos.purchase') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
