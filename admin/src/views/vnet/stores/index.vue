<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const saving = ref(false);
const search = ref('');

const dialogVisible = ref(false);
const isEdit = ref(false);
const formRef = ref<any>(null);

const form = ref<any>({
  name: '',
  code: '',
  phone: '',
  address: '',
  is_active: true
});

const rules = {
  name: [{ required: true, message: $t('vnetPages.stores.form.nameRequired'), trigger: 'blur' }],
  code: [{ required: true, message: $t('vnetPages.stores.form.codeRequired'), trigger: 'blur' }]
};

const { columns, columnChecks, data, getData, getDataByPage, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/stores', { params: { page, page_size: pageSize, search: search.value || undefined } }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.stores.name'), minWidth: 160 },
    { prop: 'code', label: $t('vnetPages.stores.code'), width: 100 },
    { prop: 'phone', label: $t('vnetPages.stores.phone'), width: 130 },
    { prop: 'address', label: $t('vnetPages.stores.address'), minWidth: 250 },
    {
      prop: 'is_active',
      label: $t('vnetPages.stores.isActive'),
      width: 90,
      formatter: (row: any) =>
        h(ElTag, { type: row.is_active ? 'success' : 'info' }, () =>
          row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no')
        )
    }
  ]
});

function searchData() {
  getDataByPage(1);
}

function resetForm() {
  form.value = { name: '', code: '', phone: '', address: '', is_active: true };
}

function handleCreate() {
  isEdit.value = false;
  resetForm();
  dialogVisible.value = true;
}

function handleEdit(row: any) {
  isEdit.value = true;
  form.value = { id: row.id, ...row };
  dialogVisible.value = true;
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    if (isEdit.value) {
      await client.put(`/stores/${form.value.id}`, form.value);
      ElMessage.success($t('vnetPages.stores.messages.editSuccess'));
    } else {
      await client.post('/stores', form.value);
      ElMessage.success($t('vnetPages.stores.messages.addSuccess'));
    }
    dialogVisible.value = false;
    await getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.stores.messages.saveError'));
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm($t('vnetPages.stores.messages.deleteConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
    await client.delete(`/stores/${row.id}`);
    ElMessage.success($t('vnetPages.stores.messages.deleteSuccess'));
    await getData();
  } catch (_) {}
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.stores.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            @add="handleCreate"
            @refresh="getData"
          >
            <template #prefix>
              <ElInput
                v-model="search"
                :placeholder="$t('vnetPages.stores.searchPlaceholder')"
                clearable
                style="width: 200px"
                @input="searchData"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="200" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="handleEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
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

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.stores.edit') : $t('vnetPages.stores.add')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="100">
        <ElFormItem :label="$t('vnetPages.stores.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.stores.code')" prop="code">
          <ElInput v-model="form.code" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.stores.phone')" prop="phone">
          <ElInput v-model="form.phone" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.stores.address')" prop="address">
          <ElInput v-model="form.address" type="textarea" :rows="2" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.stores.isActive')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
