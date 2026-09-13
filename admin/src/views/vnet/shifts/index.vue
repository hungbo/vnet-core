<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { formatPrice } from '@/utils/money';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const saving = ref(false);
const search = ref('');
const filterStatus = ref(null);

const openDialog = ref(false);
const openFormRef = ref<any>(null);
const openForm = ref({ opening_balance: 0 });
const openRules = {
  opening_balance: [
    {
      required: true,
      message: $t('vnetPages.shifts.form.amountRequired'),
      trigger: 'blur'
    }
  ]
};

const closeDialog = ref(false);
const closeFormRef = ref<any>(null);
const closeForm = ref({ id: null, closing_balance: 0 });
const closeRules = {
  closing_balance: [
    {
      required: true,
      message: $t('vnetPages.shifts.form.amountRequired'),
      trigger: 'blur'
    }
  ]
};

const handoverDialog = ref(false);
const handoverFormRef = ref<any>(null);
const handoverForm = ref({
  id: null,
  amount: 0,
  handover_type: 'cash_in',
  reason: ''
});
const handoverRules = {
  amount: [
    {
      required: true,
      message: $t('vnetPages.shifts.form.amountRequired'),
      trigger: 'blur'
    }
  ],
  handover_type: [
    {
      required: true,
      message: $t('vnetPages.shifts.form.handoverTypePlaceholder'),
      trigger: 'change'
    }
  ]
};

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/shifts', {
      params: {
        page,
        page_size: pageSize,
        search: search.value || undefined,
        status: filterStatus.value || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    // Trước đây đọc user_id nên bảng hiện UUID thô; backend nay trả kèm user_name.
    {
      prop: 'user_name',
      label: $t('vnetPages.shifts.user'),
      minWidth: 140,
      formatter: (row: any) => row.user_name || row.user_id || '-'
    },
    {
      prop: 'started_at',
      label: $t('vnetPages.shifts.startTime'),
      width: 160,
      formatter: (row: any) => (row.started_at ? dayjs(row.started_at).format('DD/MM/YYYY HH:mm') : '')
    },
    {
      prop: 'ended_at',
      label: $t('vnetPages.shifts.endTime'),
      width: 160,
      formatter: (row: any) => (row.ended_at ? dayjs(row.ended_at).format('DD/MM/YYYY HH:mm') : '-')
    },
    {
      prop: 'status',
      label: $t('vnetPages.common.status'),
      width: 100,
      formatter: (row: any) =>
        h(ElTag, { type: row.status === 'open' ? 'success' : 'info' }, () =>
          row.status === 'open' ? $t('vnetPages.shifts.open') : $t('vnetPages.shifts.closed')
        )
    },
    {
      prop: 'opening_balance',
      label: $t('vnetPages.shifts.openingCash'),
      width: 120,
      formatter: (row: any) => formatPrice(row.opening_balance)
    },
    {
      prop: 'closing_balance',
      label: $t('vnetPages.shifts.closingCash'),
      width: 120,
      formatter: (row: any) => (row.closing_balance != null ? formatPrice(row.closing_balance) : '-')
    },
    {
      prop: 'expected_total',
      label: $t('vnetPages.shifts.expectedTotal'),
      width: 130,
      // expected_total của backend là tiền THU TRONG CA, chưa gồm tiền đầu ca.
      // Cột này mang nhãn "Tiền dự kiến" — tức số đáng lẽ phải có trong két —
      // nên phải cộng tiền đầu ca vào, đúng công thức mà backend dùng để tính
      // Thừa/Thiếu: discrepancy = tiền cuối − (tiền đầu + tiền thu trong ca).
      // Hiện thẳng expected_total làm hai cột cạnh nhau tự mâu thuẫn: ca đầu
      // 500.000, thu 40.000, đếm được 540.000 thì bảng ghi "dự kiến 40.000"
      // ngay cạnh "Thừa/Thiếu 0".
      formatter: (row: any) =>
        row.expected_total != null ? formatPrice((row.opening_balance || 0) + row.expected_total) : '-'
    },
    {
      // The two numbers a shift close exists to produce. They were computed but
      // never shown, so nobody could see whether the till balanced.
      prop: 'discrepancy',
      label: $t('vnetPages.shifts.discrepancy'),
      width: 130,
      formatter: (row: any) => {
        if (row.discrepancy == null) return '-';
        const type = row.discrepancy === 0 ? 'success' : 'danger';
        const sign = row.discrepancy > 0 ? '+' : '';
        return h(ElTag, { type, size: 'small' }, () => `${sign}${formatPrice(row.discrepancy)}`);
      }
    }
  ]
});

function fetchData() {
  getData();
}

function handleOpenShift() {
  openForm.value = { opening_balance: 0 };
  openDialog.value = true;
}

async function handleSaveOpen() {
  const valid = await openFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    await client.post('/shifts/open', openForm.value);
    ElMessage.success($t('vnetPages.shifts.messages.openSuccess'));
    openDialog.value = false;
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.shifts.messages.openError'));
  } finally {
    saving.value = false;
  }
}

function handleCloseShift(row: any) {
  closeForm.value = { id: row.id, closing_balance: 0 };
  closeDialog.value = true;
}

async function handleSaveClose() {
  const valid = await closeFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    await client.post(`/shifts/${closeForm.value.id}/close`, {
      closing_balance: closeForm.value.closing_balance
    });
    ElMessage.success($t('vnetPages.shifts.messages.closeSuccess'));
    closeDialog.value = false;
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.shifts.messages.closeError'));
  } finally {
    saving.value = false;
  }
}

function handleHandover(row: any) {
  handoverForm.value = {
    id: row.id,
    amount: 0,
    handover_type: 'cash_in',
    reason: ''
  };
  handoverDialog.value = true;
}

async function handleSaveHandover() {
  const valid = await handoverFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    await client.post(`/shifts/${handoverForm.value.id}/handover`, {
      amount: handoverForm.value.amount,
      handover_type: handoverForm.value.handover_type,
      reason: handoverForm.value.reason
    });
    ElMessage.success($t('vnetPages.shifts.messages.handoverSuccess'));
    handoverDialog.value = false;
    await fetchData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.shifts.messages.handoverError'));
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
          <span>{{ $t('vnetPages.shifts.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-add="false"
            :show-delete="false"
            @refresh="getData"
          >
            <template #prefix>
              <ElSelect
                v-model="filterStatus"
                :placeholder="$t('vnetPages.common.status')"
                clearable
                style="width: 130px"
                @change="fetchData"
              >
                <ElOption :label="$t('vnetPages.shifts.open')" value="open" />
                <ElOption :label="$t('vnetPages.shifts.closed')" value="closed" />
              </ElSelect>
              <ElInput
                v-model="search"
                :placeholder="$t('vnetPages.shifts.user') + '...'"
                clearable
                style="width: 160px"
                @input="fetchData"
              />
              <ElButton type="primary" @click="handleOpenShift">{{ $t('vnetPages.shifts.openShift') }}</ElButton>
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="180" fixed="right">
          <template #default="{ row }">
            <ElButton v-if="row.status === 'open'" size="small" type="warning" @click="handleCloseShift(row)">
              {{ $t('vnetPages.shifts.closeShift') }}
            </ElButton>
            <ElButton v-if="row.status === 'open'" size="small" @click="handleHandover(row)">
              {{ $t('vnetPages.shifts.handover') }}
            </ElButton>
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

    <ElDialog v-model="openDialog" :title="$t('vnetPages.shifts.openShift')" width="400px">
      <ElForm ref="openFormRef" :model="openForm" :rules="openRules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.shifts.form.openingCash')" prop="opening_balance">
          <ElInputNumber v-model="openForm.opening_balance" :min="0" :precision="0" style="width: 100%" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="openDialog = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSaveOpen">
          {{ $t('vnetPages.common.confirm') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="closeDialog" :title="$t('vnetPages.shifts.closeShift')" width="400px">
      <ElForm ref="closeFormRef" :model="closeForm" :rules="closeRules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.shifts.form.closingCash')" prop="closing_balance">
          <ElInputNumber v-model="closeForm.closing_balance" :min="0" :precision="0" style="width: 100%" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="closeDialog = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSaveClose">
          {{ $t('vnetPages.common.confirm') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="handoverDialog" :title="$t('vnetPages.shifts.handover')" width="420px">
      <ElForm ref="handoverFormRef" :model="handoverForm" :rules="handoverRules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.shifts.amount')" prop="amount">
          <ElInputNumber v-model="handoverForm.amount" :min="0" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.shifts.type')" prop="handover_type">
          <ElSelect v-model="handoverForm.handover_type" style="width: 100%">
            <ElOption :label="$t('vnetPages.shifts.cashIn')" value="cash_in" />
            <ElOption :label="$t('vnetPages.shifts.cashOut')" value="cash_out" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.shifts.reason')" prop="reason">
          <ElInput v-model="handoverForm.reason" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="handoverDialog = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSaveHandover">
          {{ $t('vnetPages.common.confirm') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
