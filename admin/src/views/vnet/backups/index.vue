<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAuth } from '@/hooks/business/auth';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();

const creating = ref(false);

// --- Lịch sao lưu tự động --------------------------------------------------
// Lưu ở nhóm cài đặt "backup". Chưa lưu gì thì máy chủ dùng mặc định: mỗi 1 giờ,
// giữ 48 bản tự động — form hiện đúng các giá trị đó.
const autoForm = ref({ auto_enabled: true, auto_every: 1, auto_unit: 'hour', auto_keep: 48 });
const savingAuto = ref(false);

async function loadAutoBackup() {
  try {
    const rows: any = await client.get('/settings/backup');
    const m = Object.fromEntries((Array.isArray(rows) ? rows : []).map((r: any) => [r.key, r.value]));
    if (m.auto_enabled !== undefined) autoForm.value.auto_enabled = String(m.auto_enabled) !== 'false';
    if (Number(m.auto_every) >= 1) autoForm.value.auto_every = Number(m.auto_every);
    if (['minute', 'hour', 'day'].includes(m.auto_unit)) autoForm.value.auto_unit = m.auto_unit;
    if (m.auto_keep !== undefined && Number(m.auto_keep) >= 0) autoForm.value.auto_keep = Number(m.auto_keep);
  } catch {
    // Đọc hỏng thì giữ mặc định trên form; lưu lại sẽ ghi đè.
  }
}

async function saveAutoBackup() {
  savingAuto.value = true;
  try {
    await client.put('/settings/backup', { ...autoForm.value });
    ElMessage.success($t('vnetPages.backups.messages.autoSaved'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    savingAuto.value = false;
  }
}

loadAutoBackup();

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

const { columns, columnChecks, data, getData, getDataByPage, loading, mobilePagination, onSortChange } =
  useUIPaginatedTable({
    api: ({ page, pageSize, sort, order }) =>
      client.get('/backups', { params: { page, page_size: pageSize, sort, order } }),
    transform: vnetTransform,
    columns: () => [
      {
        prop: 'file_name',
        label: $t('vnetPages.backups.fileName'),
        minWidth: 200,
        sortable: 'custom'
      },
      {
        // Chỉ bản tự động bị dọn khi vượt số bản giữ lại, nên phải phân biệt được.
        prop: 'auto',
        label: $t('vnetPages.backups.type'),
        width: 110,
        formatter: (row: any) =>
          h(ElTag, { type: row.auto ? 'info' : 'primary' }, () =>
            row.auto ? $t('vnetPages.backups.typeAuto') : $t('vnetPages.backups.typeManual')
          )
      },
      {
        prop: 'file_size',
        label: $t('vnetPages.backups.size'),
        width: 120,
        sortable: 'custom',
        formatter: (row: any) => formatFileSize(row.file_size)
      },
      {
        prop: 'status',
        label: $t('vnetPages.backups.status'),
        width: 120,
        sortable: 'custom',
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
        sortable: 'custom',
        formatter: (row: any) => (row.started_at ? dayjs(row.started_at).format('DD/MM/YYYY HH:mm') : '-')
      },
      {
        prop: 'completed_at',
        label: $t('vnetPages.backups.completed'),
        width: 160,
        sortable: 'custom',
        formatter: (row: any) => (row.completed_at ? dayjs(row.completed_at).format('DD/MM/YYYY HH:mm') : '-')
      }
    ]
  });

// Tạo/phục hồi/xoá đi qua /backups; bản sao lưu chạy nền xong ("running" →
// "completed"/"failed") thì BackupService tự phát "backups:changed".
useRealtimeRefresh(['backups'], getData);

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
  <div class="page-fill">
    <ElCard class="fill-card">
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
            <ElButton v-if="hasAuth('backups.create')" type="primary" :loading="creating" @click="handleCreateBackup">
              {{ $t('vnetPages.backups.createBackup') }}
            </ElButton>
          </TableHeaderOperation>
        </div>
        <div class="mt-12px flex flex-wrap items-center gap-8px">
          <span class="font-medium">{{ $t('vnetPages.backups.autoBackup') }}</span>
          <ElSwitch v-model="autoForm.auto_enabled" :disabled="!hasAuth('settings.edit')" />
          <span>{{ $t('vnetPages.backups.every') }}</span>
          <ElInputNumber
            v-model="autoForm.auto_every"
            class="w-110px!"
            size="small"
            :min="1"
            :precision="0"
            :disabled="!autoForm.auto_enabled || !hasAuth('settings.edit')"
          />
          <ElSelect
            v-model="autoForm.auto_unit"
            class="w-90px!"
            size="small"
            :disabled="!autoForm.auto_enabled || !hasAuth('settings.edit')"
          >
            <ElOption value="minute" :label="$t('vnetPages.backups.unitMinute')" />
            <ElOption value="hour" :label="$t('vnetPages.backups.unitHour')" />
            <ElOption value="day" :label="$t('vnetPages.backups.unitDay')" />
          </ElSelect>
          <span>{{ $t('vnetPages.backups.keep') }}</span>
          <ElInputNumber
            v-model="autoForm.auto_keep"
            class="w-110px!"
            size="small"
            :min="0"
            :precision="0"
            :disabled="!autoForm.auto_enabled || !hasAuth('settings.edit')"
          />
          <span>{{ $t('vnetPages.backups.keepUnit') }}</span>
          <ElButton
            v-if="hasAuth('settings.edit')"
            size="small"
            type="primary"
            :loading="savingAuto"
            @click="saveAutoBackup"
          >
            {{ $t('vnetPages.common.save') }}
          </ElButton>
        </div>
        <div class="mt-4px text-12px text-gray-400">{{ $t('vnetPages.backups.autoHint') }}</div>
      </template>
      <ElTable
        v-loading="loading"
        :data="data"
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="160" fixed="right">
          <template #default="{ row }">
            <ElButton
              v-if="row.status === 'completed' && hasAuth('backups.restore')"
              size="small"
              type="warning"
              @click="handleRestore(row)"
            >
              {{ $t('vnetPages.backups.restore') }}
            </ElButton>
            <ElButton
              v-if="row.status !== 'running' && hasAuth('backups.delete')"
              size="small"
              type="danger"
              @click="handleDelete(row)"
            >
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
