<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<string | null>(null);

const formRef = ref<FormInstance>();

const form = ref({
  name: '',
  min_spent: 0,
  discount_percent: 0,
  is_default: false
});

const rules: FormRules = {
  name: [{ required: true, message: $t('vnetPages.memberGroups.nameRequired'), trigger: 'blur' }]
};

function formatDate(dateStr: string | undefined) {
  if (!dateStr) return '-';
  return dayjs(dateStr).format('DD/MM/YYYY HH:mm');
}

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: () => client.get('/member-groups', { params: { search: search.value || undefined } }),
  transform: vnetSimpleTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.memberGroups.name'), minWidth: 150 },
    {
      prop: 'min_spent',
      label: $t('vnetPages.memberGroups.minSpent'),
      width: 130,
      align: 'right',
      formatter: (row: any) => row.min_spent?.toLocaleString() ?? ''
    },
    {
      prop: 'discount_percent',
      label: $t('vnetPages.memberGroups.discountPercent'),
      width: 130,
      align: 'center',
      formatter: (row: any) => (row.discount_percent != null ? `${row.discount_percent}%` : '')
    },
    {
      prop: 'is_default',
      label: $t('vnetPages.memberGroups.isDefault'),
      width: 100,
      align: 'center',
      formatter: (row: any) =>
        row.is_default
          ? h(ElTag, { type: 'success', size: 'small' }, () => $t('vnetPages.common.yes'))
          : (h('span', '-') as any)
    },
    {
      prop: 'created_at',
      label: $t('vnetPages.memberGroups.createdAt'),
      width: 180,
      formatter: (row: any) => formatDate(row.created_at)
    }
  ]
});

function searchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = { name: '', min_spent: 0, discount_percent: 0, is_default: false };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    min_spent: row.min_spent || 0,
    discount_percent: row.discount_percent || 0,
    is_default: row.is_default || false
  };
  dialogVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    if (isEdit.value && editingId.value) {
      await client.put(`/member-groups/${editingId.value}`, form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.memberGroups.editSuccess')
      });
    } else {
      await client.post('/member-groups', form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.memberGroups.createSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.memberGroups.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.memberGroups.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/member-groups/${row.id}`);
    ElMessage.success($t('vnetPages.memberGroups.deleteSuccess'));
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
            :placeholder="$t('vnetPages.common.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="searchData"
          />
          <ElButton type="primary" @click="searchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @add="openCreate" @refresh="getData" />
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="180" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.memberGroups.edit') : $t('vnetPages.memberGroups.create')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="150">
        <ElFormItem :label="$t('vnetPages.memberGroups.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.memberGroups.minSpent')" prop="min_spent">
          <ElInputNumber v-model="form.min_spent" :min="0" :step="100000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.memberGroups.discountPercent')" prop="discount_percent">
          <ElInputNumber v-model="form.discount_percent" :min="0" :max="100" :step="1" style="width: 100%">
            <template #suffix>%</template>
          </ElInputNumber>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.memberGroups.isDefault')" prop="is_default">
          <ElSwitch v-model="form.is_default" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
