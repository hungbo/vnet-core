<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const formRef = ref<FormInstance>();

const rewardsVisible = ref(false);
const rewardsLoading = ref(false);
const rewards = ref<any[]>([]);

const form = ref({
  name: '',
  type: '',
  priority: 0,
  is_active: true,
  valid_from: '',
  valid_to: '',
  conditions: '',
  rewards: ''
});

const rules: FormRules = {
  name: [{ required: true, message: $t('vnetPages.promotions.form.nameRequired'), trigger: 'blur' }],
  type: [{ required: true, message: $t('vnetPages.promotions.form.typeRequired'), trigger: 'blur' }]
};

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

async function fetchRewards() {
  rewardsVisible.value = true;
  rewardsLoading.value = true;
  try {
    const res: any = await client.get('/lucky-spin/rewards');
    rewards.value = Array.isArray(res) ? res : res?.items || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.promotions.messages.loadRewardsError'));
    rewards.value = [];
  } finally {
    rewardsLoading.value = false;
  }
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/promotions', { params: { page, page_size: pageSize, search: search.value || undefined } }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.promotions.name'), minWidth: 160 },
    { prop: 'type', label: $t('vnetPages.promotions.type'), width: 120 },
    { prop: 'priority', label: $t('vnetPages.promotions.priority'), width: 80 },
    {
      prop: 'is_active',
      label: $t('vnetPages.promotions.isActive'),
      width: 90,
      formatter: (row: any) =>
        h(ElTag, { type: row.is_active ? 'success' : 'danger', size: 'small' }, () =>
          row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no')
        )
    },
    {
      prop: 'valid_period',
      label: $t('vnetPages.promotions.validPeriod'),
      minWidth: 240,
      formatter: (row: any) => `${formatDate(row.valid_from)} → ${formatDate(row.valid_to)}`
    }
  ]
});

function searchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = {
    name: '',
    type: '',
    priority: 0,
    is_active: true,
    valid_from: '',
    valid_to: '',
    conditions: '',
    rewards: ''
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    type: row.type || '',
    priority: row.priority ?? 0,
    is_active: row.is_active ?? true,
    valid_from: row.valid_from || '',
    valid_to: row.valid_to || '',
    conditions: row.conditions ? JSON.stringify(row.conditions, null, 2) : '',
    rewards: row.rewards ? JSON.stringify(row.rewards, null, 2) : ''
  };
  dialogVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    const payload: any = {
      name: form.value.name,
      type: form.value.type,
      priority: form.value.priority,
      is_active: form.value.is_active,
      valid_from: form.value.valid_from || null,
      valid_to: form.value.valid_to || null
    };
    if (form.value.conditions) {
      try {
        payload.conditions = JSON.parse(form.value.conditions);
      } catch {
        payload.conditions = form.value.conditions;
      }
    }
    if (form.value.rewards) {
      try {
        payload.rewards = JSON.parse(form.value.rewards);
      } catch {
        payload.rewards = form.value.rewards;
      }
    }
    if (isEdit.value && editingId.value) {
      await client.put(`/promotions/${editingId.value}`, payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.promotions.messages.editSuccess')
      });
    } else {
      await client.post('/promotions', payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.promotions.messages.addSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.promotions.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.promotions.messages.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/promotions/${row.id}`);
    ElMessage.success($t('vnetPages.promotions.messages.deleteSuccess'));
    getData();
  } catch {}
}
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.promotions.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="searchData"
          />
          <ElButton type="primary" @click="searchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @add="openCreate" @refresh="getData">
          <template #prefix>
            <ElButton @click="fetchRewards">{{ $t('vnetPages.promotions.luckySpinRewards') }}</ElButton>
          </template>
        </TableHeaderOperation>
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="200" fixed="right">
          <template #default="{ row }">
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

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.promotions.edit') : $t('vnetPages.promotions.add')"
      width="650px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.promotions.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.type')" prop="type">
          <ElInput v-model="form.type" :placeholder="$t('vnetPages.promotions.form.typePlaceholder')" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.priority')" prop="priority">
          <ElInputNumber v-model="form.priority" :min="0" :max="999" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.isActive')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.conditions')">
          <ElInput
            v-model="form.conditions"
            type="textarea"
            :rows="3"
            placeholder='VD: {"min_spend": 100000, "member_tier": "gold"}'
          />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.rewards')">
          <ElInput
            v-model="form.rewards"
            type="textarea"
            :rows="3"
            placeholder='VD: {"discount_percent": 10, "bonus_minutes": 30}'
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="rewardsVisible" :title="$t('vnetPages.promotions.luckySpinRewards')" width="600px">
      <ElTable v-loading="rewardsLoading" :data="rewards" border stripe style="width: 100%">
        <ElTableColumn prop="name" :label="$t('vnetPages.promotions.name')" min-width="140" />
        <ElTableColumn prop="type" :label="$t('vnetPages.promotions.type')" width="100" />
        <ElTableColumn :label="$t('vnetPages.promotions.value')" width="100">
          <template #default="{ row }">{{ row.value?.toLocaleString() }}</template>
        </ElTableColumn>
        <ElTableColumn prop="probability" :label="$t('vnetPages.promotions.probability')" width="90">
          <template #default="{ row }">{{ row.probability }}%</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.promotions.isActive')" width="80">
          <template #default="{ row }">
            <ElTag :type="row.is_active ? 'success' : 'danger'" size="small">
              {{ row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no') }}
            </ElTag>
          </template>
        </ElTableColumn>
      </ElTable>
      <template #footer>
        <ElButton @click="rewardsVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
