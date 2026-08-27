<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

// Bốn giá trị này là enum của cột notifications.type ở backend. Bảng và ô chọn
// phải đọc chung một danh sách, nếu không giao diện lại hiện mã thô "promotion".
const TYPES = ['info', 'warning', 'promotion', 'system'] as const;
const typeOptions = computed(() => TYPES.map(v => ({ value: v, label: $t(`vnetPages.notifications.types.${v}`) })));

const saving = ref(false);
const dialogVisible = ref(false);
const isEdit = ref(false);
const formRef = ref<any>(null);

const form = ref<any>({ type: 'info', title: '', content: '' });

const rules = {
  type: [
    {
      required: true,
      message: $t('vnetPages.notifications.form.typeRequired'),
      trigger: 'blur'
    }
  ],
  title: [
    {
      required: true,
      message: $t('vnetPages.notifications.form.titleRequired'),
      trigger: 'blur'
    }
  ]
};

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }: { page: number; pageSize: number }) =>
    client.get('/admin/notifications', {
      params: { page, page_size: pageSize }
    }),
  transform: vnetTransform,
  columns: () => [
    {
      prop: 'type',
      label: $t('vnetPages.notifications.type'),
      width: 120,
      formatter: (row: any) =>
        (TYPES as readonly string[]).includes(row.type) ? $t(`vnetPages.notifications.types.${row.type}`) : row.type
    },
    {
      prop: 'title',
      label: $t('vnetPages.notifications.title'),
      minWidth: 200
    },
    {
      prop: 'content',
      label: $t('vnetPages.notifications.content'),
      minWidth: 300,
      showOverflowTooltip: true
    },
    {
      prop: 'created_at',
      label: $t('vnetPages.notifications.createdAt'),
      width: 180,
      formatter: (row: any) => (row.created_at ? dayjs(row.created_at).format('DD/MM/YYYY HH:mm') : '-')
    }
  ]
});

function resetForm() {
  form.value = { type: 'info', title: '', content: '' };
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
    type: row.type,
    title: row.title,
    content: row.content || ''
  };
  dialogVisible.value = true;
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    if (isEdit.value) {
      await client.put(`/admin/notifications/${form.value.id}`, form.value);
      ElMessage.success($t('vnetPages.notifications.messages.editSuccess'));
    } else {
      await client.post('/admin/notifications', form.value);
      ElMessage.success($t('vnetPages.notifications.messages.addSuccess'));
    }
    dialogVisible.value = false;
    await getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.notifications.messages.saveError'));
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm($t('vnetPages.notifications.messages.deleteConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
    await client.delete(`/admin/notifications/${row.id}`);
    ElMessage.success($t('vnetPages.notifications.messages.deleteSuccess'));
    await getData();
  } catch (e: any) {
    // Người dùng bấm Huỷ ở hộp xác nhận thì e === 'cancel'. Mọi thứ khác là
    // lỗi nghiệp vụ — điển hình là 409 "còn dữ liệu phụ thuộc" — và phải nói
    // ra; catch rỗng làm nút bấm vào không có phản hồi gì.
    if (e !== 'cancel') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleDispatch(row: any) {
  try {
    await ElMessageBox.confirm($t('vnetPages.notifications.messages.dispatchConfirm'), $t('vnetPages.common.confirm'), {
      type: 'info'
    });
    const res: any = await client.post(`/admin/notifications/${row.id}/dispatch`);
    ElMessage.success(
      $t('vnetPages.notifications.messages.dispatchSuccess', {
        count: res.dispatched ?? 0
      })
    );
  } catch (_) {}
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.notifications.pageTitle') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-delete="false"
            @add="handleCreate"
            @refresh="getData"
          ></TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="280" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" type="primary" @click="handleDispatch(row)">
              {{ $t('vnetPages.notifications.dispatch') }}
            </ElButton>
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
      :title="isEdit ? $t('vnetPages.notifications.edit') : $t('vnetPages.notifications.add')"
      width="600px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="80">
        <ElFormItem :label="$t('vnetPages.notifications.type')" prop="type">
          <ElSelect v-model="form.type" style="width: 100%">
            <ElOption v-for="opt in typeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.notifications.title')" prop="title">
          <ElInput v-model="form.title" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.notifications.content')" prop="content">
          <ElInput v-model="form.content" type="textarea" :rows="4" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
