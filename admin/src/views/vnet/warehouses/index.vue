<script setup lang="ts">
import { h, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const saving = ref(false);

const dialog = ref(false);
const isEdit = ref(false);
const formRef = ref<any>(null);
const form = reactive<any>({ name: '', address: '' });
const rules = { name: [{ required: true, message: '', trigger: 'blur' }] };

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: () => client.get('/warehouses', { params: { page_size: 1000 } }),
  transform: vnetSimpleTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.warehouses.name'), minWidth: 160 },
    { prop: 'address', label: $t('vnetPages.warehouses.address'), minWidth: 250 }
  ]
});

function handleCreate() {
  isEdit.value = false;
  form.name = '';
  form.address = '';
  dialog.value = true;
}

function handleEdit(row: any) {
  isEdit.value = true;
  form.id = row.id;
  form.name = row.name;
  form.address = row.address || '';
  dialog.value = true;
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    if (isEdit.value) {
      await client.put(`/warehouses/${form.id}`, form);
      ElMessage.success($t('vnetPages.warehouses.messages.editSuccess'));
    } else {
      await client.post('/warehouses', form);
      ElMessage.success($t('vnetPages.warehouses.messages.addSuccess'));
    }
    dialog.value = false;
    await getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.warehouses.messages.saveError'));
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
          <span>{{ $t('vnetPages.warehouses.title') }}</span>
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
        <ElTableColumn :label="$t('vnetPages.common.action')" width="150" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="handleEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog
      v-model="dialog"
      :title="isEdit ? $t('vnetPages.common.edit') : $t('vnetPages.warehouses.addWarehouse')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="100">
        <ElFormItem :label="$t('vnetPages.warehouses.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.warehouses.address')" prop="address">
          <ElInput v-model="form.address" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialog = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
