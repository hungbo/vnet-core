<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const creating = ref(false);

function formatFileSize(bytes: number) {
  if (!bytes) return '-';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let size = bytes;
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024;
    i++;
  }
  return `${size.toFixed(2)} ${units[i]}`;
}

function statusType(status: string): any {
  const map: Record<string, string> = {
    running: 'warning',
    completed: 'success',
    failed: 'danger'
  };
  return map[status] || 'info';
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    running: $t('vnetPages.backups.running'),
    completed: $t('vnetPages.backups.completed'),
    failed: $t('vnetPages.backups.failed')
  };
  return map[status] || status;
}

const { columns, columnChecks, data, getData, getDataByPage, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) => client.get('/backups', { params: { page, page_size: pageSize } }),
  transform: vnetTransform,
  columns: () => [
    {
      prop: 'file_name',
      label: $t('vnetPages.backups.fileName'),
      minWidth: 200
    },
    {
      prop: 'file_size',
      label: $t('vnetPages.backups.size'),
      width: 120,
      formatter: (row: any) => formatFileSize(row.file_size)
    },
    {
      prop: 'status',
      label: $t('vnetPages.backups.status'),
      width: 120,
      // title mang lý do hỏng: "Thất bại" trơ trọi không cho người trực biết
      // phải sửa gì. Chuỗi lý do do backend ghi vào notes.
      formatter: (row: any) =>
        h(ElTag, { type: statusType(row.status), title: row.notes || undefined }, () => statusLabel(row.status))
    },
    {
      // model.BackupLog KHÔNG có created_at, chỉ có started_at — cột cũ đọc một
      // khoá không tồn tại nên luôn trống.
      prop: 'started_at',
      label: $t('vnetPages.backups.createdAt'),
      width: 160,
      formatter: (row: any) => (row.started_at ? dayjs(row.started_at).format('DD/MM/YYYY HH:mm') : '-')
    },
    {
      prop: 'completed_at',
      label: $t('vnetPages.backups.completed'),
      width: 160,
      formatter: (row: any) => (row.completed_at ? dayjs(row.completed_at).format('DD/MM/YYYY HH:mm') : '-')
    }
  ]
});

async function handleCreateBackup() {
  try {
    await ElMessageBox.confirm($t('vnetPages.backups.messages.createConfirm'), $t('vnetPages.common.confirm'), {
      type: 'info'
    });
    creating.value = true;
    await client.post('/backups');
    ElMessage.success($t('vnetPages.backups.messages.creating'));
    await getData();
  } catch (e: any) {
    // Giống handleDelete bên dưới: 'cancel' là người dùng bấm Huỷ, còn lại là
    // lỗi thật và phải nói ra. Nuốt hết thì bấm "Tạo sao lưu" xong màn hình
    // không nhúc nhích, người trực tưởng bản sao lưu đang chạy.
    if (e !== 'cancel') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    creating.value = false;
  }
}

async function handleRestore(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.backups.messages.restoreConfirm', { file: row.file_name }),
      $t('common.warning'),
      {
        type: 'warning',
        confirmButtonText: $t('vnetPages.backups.restore'),
        cancelButtonText: $t('vnetPages.common.cancel')
      }
    );
    await client.post(`/backups/${row.id}/restore`);
    ElMessage.success($t('vnetPages.backups.messages.restoring'));
    await getData();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.backups.messages.deleteConfirm', { file: row.file_name }),
      $t('common.warning'),
      {
        type: 'warning',
        confirmButtonText: $t('vnetPages.common.delete'),
        cancelButtonText: $t('vnetPages.common.cancel')
      }
    );
    await client.delete(`/backups/${row.id}`);
    ElMessage.success($t('vnetPages.backups.messages.deleteSuccess'));
    await getData();
  } catch (e: any) {
    // Người dùng bấm Huỷ ở hộp xác nhận thì e === 'cancel'. Mọi thứ khác là
    // lỗi nghiệp vụ — điển hình là 409 "còn dữ liệu phụ thuộc" — và phải nói
    // ra; catch rỗng làm nút bấm vào không có phản hồi gì.
    if (e !== 'cancel') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.backups.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-add="false"
            :show-delete="false"
            @refresh="getData"
          >
            <ElButton type="primary" :loading="creating" @click="handleCreateBackup">
              {{ $t('vnetPages.backups.createBackup') }}
            </ElButton>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="160" fixed="right">
          <template #default="{ row }">
            <ElButton v-if="row.status === 'completed'" size="small" type="warning" @click="handleRestore(row)">
              {{ $t('vnetPages.backups.restore') }}
            </ElButton>
            <ElButton v-if="row.status !== 'running'" size="small" type="danger" @click="handleDelete(row)">
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
  </div>
</template>
