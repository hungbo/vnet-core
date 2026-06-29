<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const saving = ref(false);
const search = ref('');
const treeOptions = ref<any[]>([]);
const dialogVisible = ref(false);
const isEdit = ref(false);
const formRef = ref<any>(null);

const form = ref<any>({ name: '', icon: '', parent_id: null, sort_order: 0 });

const rules = {
  name: [{ required: true, message: $t('vnetPages.categories.form.nameRequired'), trigger: 'blur' }]
};

async function fetchTree() {
  try {
    const res: any = await client.get('/categories', { params: { page_size: 1000 } });
    const list = Array.isArray(res) ? res : res?.items || [];
    treeOptions.value = buildTree(list);
  } catch (_) {}
}

function buildTree(items: any[]): any[] {
  const map = new Map<number, any>();
  const roots: any[] = [];
  items.forEach((item: any) => map.set(item.id, { ...item, children: [] }));
  items.forEach((item: any) => {
    const node = map.get(item.id);
    if (item.parent_id && map.has(item.parent_id)) {
      map.get(item.parent_id).children.push(node);
    } else if (!item.parent_id) {
      roots.push(node);
    }
  });
  return roots;
}

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: () => client.get('/categories', { params: { search: search.value || undefined } }),
  transform: vnetSimpleTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.categories.name'), minWidth: 200 },
    { prop: 'icon', label: $t('vnetPages.categories.icon'), width: 100 },
    { prop: 'sort_order', label: $t('vnetPages.categories.order'), width: 80 }
  ]
});

function resetForm() {
  form.value = { name: '', icon: '', parent_id: null, sort_order: 0 };
}

function handleCreate() {
  isEdit.value = false;
  resetForm();
  dialogVisible.value = true;
}

function handleEdit(row: any) {
  isEdit.value = true;
  form.value = {
    id: row.id,
    name: row.name,
    icon: row.icon || '',
    parent_id: row.parent_id || null,
    sort_order: row.sort_order ?? 0
  };
  dialogVisible.value = true;
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    if (isEdit.value) {
      await client.put(`/categories/${form.value.id}`, form.value);
      ElMessage.success($t('vnetPages.categories.messages.editSuccess'));
    } else {
      await client.post('/categories', form.value);
      ElMessage.success($t('vnetPages.categories.messages.addSuccess'));
    }
    dialogVisible.value = false;
    await getData();
    await fetchTree();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.categories.messages.saveError'));
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm($t('vnetPages.categories.messages.deleteConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
    await client.delete(`/categories/${row.id}`);
    ElMessage.success($t('vnetPages.categories.messages.deleteSuccess'));
    await getData();
    await fetchTree();
  } catch (_) {}
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.categories.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            @add="handleCreate"
            @refresh="getData"
          >
            <template #prefix>
              <ElInput
                v-model="search"
                :placeholder="$t('vnetPages.categories.searchPlaceholder')"
                clearable
                style="width: 200px"
                @input="getData"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable
        v-loading="loading"
        :data="data"
        row-key="id"
        :tree-props="{ children: 'children', hasChildren: 'has_children' }"
        default-expand-all
        style="width: 100%"
      >
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
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.categories.edit') : $t('vnetPages.categories.add')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="100">
        <ElFormItem :label="$t('vnetPages.categories.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.categories.icon')" prop="icon">
          <ElInput v-model="form.icon" :placeholder="$t('vnetPages.categories.iconPlaceholder')" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.categories.parent')" prop="parent_id">
          <ElTreeSelect
            v-model="form.parent_id"
            :data="treeOptions"
            :props="{ label: 'name', value: 'id', children: 'children' } as any"
            :placeholder="$t('vnetPages.categories.parentPlaceholder')"
            clearable
            check-strictly
            style="width: 100%"
          />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.categories.order')" prop="sort_order">
          <ElInputNumber v-model="form.sort_order" :min="0" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
