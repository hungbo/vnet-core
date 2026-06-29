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
const form = reactive<any>({ name: '', phone: '', email: '' });
const rules = { name: [{ required: true, message: '', trigger: 'blur' }] };

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: () => client.get('/suppliers', { params: { page_size: 1000 } }),
  transform: vnetSimpleTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.suppliers.name'), minWidth: 160 },
    { prop: 'phone', label: $t('vnetPages.suppliers.phone'), width: 130 },
    { prop: 'email', label: $t('vnetPages.suppliers.email'), minWidth: 180 }
  ]
});

function handleCreate() {
  isEdit.value = false;
  form.name = '';
  form.phone = '';
  form.email = '';
  dialog.value = true;
}

function handleEdit(row: any) {
  isEdit.value = true;
  form.id = row.id;
  form.name = row.name;
  form.phone = row.phone || '';
  form.email = row.email || '';
  dialog.value = true;
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    if (isEdit.value) {
      await client.put(`/suppliers/${form.id}`, form);
      ElMessage.success($t('vnetPages.suppliers.messages.editSuccess'));
    } else {
      await client.post('/suppliers', form);
      ElMessage.success($t('vnetPages.suppliers.messages.addSuccess'));
    }
    dialog.value = false;
    await getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.suppliers.messages.saveError'));
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
          <span>{{ $t('vnetPages.suppliers.title') }}</span>
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
      :title="isEdit ? $t('vnetPages.common.edit') : $t('vnetPages.suppliers.addSupplier')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="100">
        <ElFormItem :label="$t('vnetPages.suppliers.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.suppliers.phone')" prop="phone">
          <ElInput v-model="form.phone" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.suppliers.email')" prop="email">
          <ElInput v-model="form.email" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialog = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
