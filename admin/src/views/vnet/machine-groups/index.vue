<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
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
  color: '',
  price_per_hour: 0,
  sort_order: 0,
  description: ''
});

const rules: FormRules = {
  name: [{ required: true, message: $t('vnetPages.machineGroups.nameRequired'), trigger: 'blur' }]
};

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: () => client.get('/machine-groups', { params: { search: search.value || undefined } }),
  transform: vnetSimpleTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.machineGroups.name'), minWidth: 150 },
    {
      prop: 'color',
      label: $t('vnetPages.machineGroups.color'),
      width: 100,
      formatter: (row: any) =>
        row.color
          ? h(ElTag, { color: row.color, style: 'color:#fff;border:none' }, () => row.color)
          : (h('span', '-') as any)
    },
    {
      prop: 'price_per_hour',
      label: $t('vnetPages.machineGroups.pricePerHour'),
      width: 130,
      align: 'center',
      formatter: (row: any) => (row.price_per_hour != null ? `${row.price_per_hour.toLocaleString()}đ/h` : '-')
    },
    { prop: 'sort_order', label: $t('vnetPages.machineGroups.sortOrder'), width: 100, align: 'center' },
    { prop: 'description', label: $t('vnetPages.machineGroups.description'), minWidth: 200 }
  ]
});

function searchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = { name: '', color: '', price_per_hour: 0, sort_order: 0, description: '' };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    color: row.color || '',
    price_per_hour: row.price_per_hour ?? 0,
    sort_order: row.sort_order ?? 0,
    description: row.description || ''
  };
  dialogVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    if (isEdit.value && editingId.value) {
      await client.put(`/machine-groups/${editingId.value}`, form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machineGroups.editSuccess')
      });
    } else {
      await client.post('/machine-groups', form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machineGroups.createSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machineGroups.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.machineGroups.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/machine-groups/${row.id}`);
    ElMessage.success($t('vnetPages.machineGroups.deleteSuccess'));
    getData();
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e?.message || 'Delete failed');
    }
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
      :title="isEdit ? $t('vnetPages.machineGroups.edit') : $t('vnetPages.machineGroups.create')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.machineGroups.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.color')" prop="color">
          <ElInput v-model="form.color" :placeholder="$t('vnetPages.machineGroups.colorPlaceholder')">
            <template #prefix>
              <div
                v-if="form.color"
                :style="{
                  width: '14px',
                  height: '14px',
                  borderRadius: '2px',
                  backgroundColor: form.color,
                  marginTop: '9px'
                }"
              />
            </template>
          </ElInput>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.pricePerHour')" prop="price_per_hour">
          <ElInputNumber v-model="form.price_per_hour" :min="0" :max="999999" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.sortOrder')" prop="sort_order">
          <ElInputNumber v-model="form.sort_order" :min="0" :max="999" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.description')" prop="description">
          <ElInput v-model="form.description" type="textarea" :rows="3" />
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
