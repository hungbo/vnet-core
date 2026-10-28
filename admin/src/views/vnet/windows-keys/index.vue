<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
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

const canManage = computed(() => hasAuth('windows_keys.manage'));
const canReveal = computed(() => hasAuth('windows_keys.reveal'));

const search = ref('');
const filter = ref('');

function formatTime(v?: string | null) {
  return v ? dayjs(v).format('DD/MM/YYYY HH:mm') : '-';
}

function statusType(status: string): any {
  return ({ offline: 'danger', available: 'success', in_use: 'warning' } as Record<string, string>)[status] || 'info';
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    offline: $t('vnetPages.machines.statusLabels.offline'),
    available: $t('vnetPages.machines.statusLabels.available'),
    in_use: $t('vnetPages.machines.statusLabels.inUse')
  };
  return map[status] || status;
}

// ---- Số liệu đầu trang ---------------------------------------------------

const summary = ref({ total: 0, with_key: 0, no_key: 0, mismatch: 0, unlicensed: 0 });

async function getSummary() {
  try {
    summary.value = (await client.get('/windows-keys/summary')) as any;
  } catch {
    // Chỉ là số liệu phụ: bảng vẫn dùng được khi nó lỗi.
  }
}

// ---- Bảng ----------------------------------------------------------------

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  // Hook gọi api({ page, pageSize }) nhưng khai kiểu là () => …; tham số tuỳ chọn khớp được cả hai.
  api: (p?: { page: number; pageSize: number }) =>
    client.get('/windows-keys', {
      params: {
        page: p?.page,
        page_size: p?.pageSize,
        search: search.value.trim() || undefined,
        filter: filter.value || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'machine_code', label: $t('vnetPages.windowsKeys.machine'), width: 150 },
    { prop: 'machine_status', label: $t('vnetPages.windowsKeys.machineStatus'), width: 110 },
    { prop: 'key_masked', label: $t('vnetPages.windowsKeys.key'), minWidth: 260 },
    { prop: 'edition', label: $t('vnetPages.windowsKeys.edition'), width: 150, showOverflowTooltip: true },
    { prop: 'note', label: $t('vnetPages.windowsKeys.note'), minWidth: 160, showOverflowTooltip: true },
    { prop: 'win_license', label: $t('vnetPages.windowsKeys.license'), width: 150 },
    { prop: 'match', label: $t('vnetPages.windowsKeys.match'), width: 130 },
    {
      prop: 'updated_at',
      label: $t('vnetPages.windowsKeys.updatedAt'),
      width: 150,
      formatter: (row: any) => formatTime(row.updated_at)
    },
    { prop: 'actions', label: '', width: 220, fixed: 'right' }
  ]
});

function reload() {
  getData();
  getSummary();
}

function onFilter() {
  reload();
}

// Key đổi qua trang này (PUT/DELETE/import → DataChanged "windows-keys"), máy báo bản quyền
// qua nhịp tim (service tự phát cùng chủ đề), máy mới hoặc đổi trạng thái ("machines").
useRealtimeRefresh(['windows-keys', 'machines'], reload);

onMounted(getSummary);

// ---- Đặt / sửa key của một máy --------------------------------------------

const editVisible = ref(false);
const saving = ref(false);
const editForm = reactive({ machine_id: '', machine_code: '', key: '', edition: '', note: '', hadKey: false });

function openEdit(row: any) {
  Object.assign(editForm, {
    machine_id: row.machine_id,
    machine_code: row.machine_code,
    key: '',
    edition: row.edition || '',
    note: row.note || '',
    hadKey: row.has_key
  });
  editVisible.value = true;
}

async function saveKey() {
  saving.value = true;
  try {
    await client.put(`/windows-keys/${editForm.machine_id}`, {
      key: editForm.key,
      edition: editForm.edition,
      note: editForm.note
    });
    ElMessage.success($t('vnetPages.windowsKeys.saved'));
    editVisible.value = false;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.windowsKeys.saveFailed'));
  } finally {
    saving.value = false;
  }
}

async function removeKey(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.windowsKeys.removeConfirm', { code: row.machine_code }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/windows-keys/${row.machine_id}`);
    ElMessage.success($t('vnetPages.windowsKeys.removed'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.windowsKeys.saveFailed'));
  }
}

// Key đầy đủ chỉ lấy khi bấm, qua quyền riêng; máy chủ ghi nhật ký mỗi lần xem.
async function revealKey(row: any) {
  try {
    const res: any = await client.get(`/windows-keys/${row.machine_id}/reveal`);
    try {
      await ElMessageBox.confirm(res.key, `${row.machine_code} — ${$t('vnetPages.windowsKeys.key')}`, {
        confirmButtonText: $t('vnetPages.windowsKeys.copy'),
        cancelButtonText: $t('vnetPages.windowsKeys.close'),
        customClass: 'windows-key-reveal'
      });
    } catch {
      return;
    }
    await navigator.clipboard.writeText(res.key);
    ElMessage.success($t('vnetPages.windowsKeys.copied'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.windowsKeys.saveFailed'));
  }
}

// ---- Nhập hàng loạt ------------------------------------------------------

interface ImportRow {
  line: number;
  machine_code: string;
  key_masked: string;
  status: string;
}

const importVisible = ref(false);
const importText = ref('');
const importOverwrite = ref(false);
const importBusy = ref(false);
const preview = ref<{ rows: ImportRow[] } | null>(null);

const willWrite = computed(
  () => preview.value?.rows.filter(r => r.status === 'ok' || r.status === 'replaced').length || 0
);

function openImport() {
  importText.value = '';
  importOverwrite.value = false;
  preview.value = null;
  importVisible.value = true;
}

// Đổi nội dung hoặc tuỳ chọn thì bản xem trước cũ không còn đúng: bắt xem lại trước khi nhập.
function invalidatePreview() {
  preview.value = null;
}

async function runImport(dryRun: boolean) {
  importBusy.value = true;
  try {
    const res: any = await client.post('/windows-keys/import', {
      text: importText.value,
      overwrite: importOverwrite.value,
      dry_run: dryRun
    });
    if (dryRun) {
      preview.value = res;
    } else {
      ElMessage.success($t('vnetPages.windowsKeys.imported', { n: res.applied }));
      importVisible.value = false;
    }
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.windowsKeys.saveFailed'));
  } finally {
    importBusy.value = false;
  }
}

function importTagType(status: string): any {
  if (status === 'ok') return 'success';
  if (status === 'replaced') return 'warning';
  if (status === 'exists' || status === 'duplicate') return 'info';
  return 'danger';
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-8px">
          <span>{{ $t('vnetPages.windowsKeys.title') }}</span>
          <span class="text-13px">
            <ElTag size="small">{{ $t('vnetPages.windowsKeys.total') }} {{ summary.total }}</ElTag>
            <ElTag type="success" size="small" class="ml-4px">
              {{ $t('vnetPages.windowsKeys.withKey') }} {{ summary.with_key }}
            </ElTag>
            <ElTag type="warning" size="small" class="ml-4px">
              {{ $t('vnetPages.windowsKeys.noKey') }} {{ summary.no_key }}
            </ElTag>
            <ElTag type="danger" size="small" class="ml-4px">
              {{ $t('vnetPages.windowsKeys.mismatch') }} {{ summary.mismatch }}
            </ElTag>
            <ElTag type="info" size="small" class="ml-4px">
              {{ $t('vnetPages.windowsKeys.unlicensed') }} {{ summary.unlicensed }}
            </ElTag>
          </span>
        </div>
      </template>

      <ElAlert type="info" :closable="false" class="mb-12px" :title="$t('vnetPages.windowsKeys.hint')" />

      <div class="mb-12px flex flex-wrap items-center justify-between gap-8px">
        <div class="flex flex-wrap items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.windowsKeys.searchPlaceholder')"
            clearable
            style="width: 220px"
            @keyup.enter="onFilter"
            @clear="onFilter"
          />
          <ElSelect v-model="filter" style="width: 200px" @change="onFilter">
            <ElOption :label="$t('vnetPages.windowsKeys.filters.all')" value="" />
            <ElOption :label="$t('vnetPages.windowsKeys.filters.no_key')" value="no_key" />
            <ElOption :label="$t('vnetPages.windowsKeys.filters.mismatch')" value="mismatch" />
            <ElOption :label="$t('vnetPages.windowsKeys.filters.unlicensed')" value="unlicensed" />
          </ElSelect>
          <ElButton type="primary" @click="onFilter">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :loading="loading"
          :show-add="false"
          :show-delete="false"
          @refresh="reload"
        >
          <template #prefix>
            <ElButton v-if="canManage" type="primary" plain @click="openImport">
              {{ $t('vnetPages.windowsKeys.bulkImport') }}
            </ElButton>
          </template>
        </TableHeaderOperation>
      </div>

      <ElTable v-loading="loading" :data="data" row-key="machine_id" style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col">
          <template v-if="col.prop === 'machine_status'" #default="{ row }">
            <ElTag :type="statusType(row.machine_status)" size="small">{{ statusLabel(row.machine_status) }}</ElTag>
          </template>
          <template v-else-if="col.prop === 'key_masked'" #default="{ row }">
            <span v-if="row.has_key" class="key-mono">{{ row.key_masked }}</span>
            <ElTag v-else type="warning" size="small">{{ $t('vnetPages.windowsKeys.noKey') }}</ElTag>
          </template>
          <template v-else-if="col.prop === 'win_license'" #default="{ row }">
            <ElTag
              v-if="row.win_license"
              :type="row.win_license === 'licensed' ? 'success' : 'warning'"
              size="small"
            >
              {{ $t(`vnetPages.windowsKeys.licenses.${row.win_license}` as any) }}
            </ElTag>
            <span v-else>-</span>
          </template>
          <template v-else-if="col.prop === 'match'" #default="{ row }">
            <ElTag
              v-if="row.match"
              :type="row.match === 'match' ? 'success' : row.match === 'mismatch' ? 'danger' : 'info'"
              size="small"
            >
              {{ $t(`vnetPages.windowsKeys.matches.${row.match}` as any) }}
            </ElTag>
            <span v-else>-</span>
          </template>
          <template v-else-if="col.prop === 'actions'" #default="{ row }">
            <ElButton v-if="canManage" size="small" @click="openEdit(row)">
              {{ row.has_key ? $t('vnetPages.windowsKeys.replace') : $t('vnetPages.windowsKeys.set') }}
            </ElButton>
            <ElButton v-if="canReveal && row.has_key" size="small" @click="revealKey(row)">
              {{ $t('vnetPages.windowsKeys.reveal') }}
            </ElButton>
            <ElButton v-if="canManage && row.has_key" size="small" type="danger" plain @click="removeKey(row)">
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

    <ElDialog v-model="editVisible" :title="`${editForm.machine_code} — ${$t('vnetPages.windowsKeys.key')}`" width="520px">
      <ElForm label-width="110px" @submit.prevent="saveKey">
        <ElFormItem :label="$t('vnetPages.windowsKeys.key')">
          <ElInput
            v-model="editForm.key"
            placeholder="XXXXX-XXXXX-XXXXX-XXXXX-XXXXX"
            autocomplete="off"
            class="key-mono"
          />
        </ElFormItem>
        <ElAlert
          v-if="editForm.hadKey"
          type="warning"
          :closable="false"
          class="mb-12px"
          :title="$t('vnetPages.windowsKeys.replaceWarning')"
        />
        <ElFormItem :label="$t('vnetPages.windowsKeys.edition')">
          <ElInput v-model="editForm.edition" maxlength="40" placeholder="Windows 11 Pro" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.windowsKeys.note')">
          <ElInput v-model="editForm.note" maxlength="255" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="editVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" :disabled="!editForm.key.trim()" @click="saveKey">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="importVisible" :title="$t('vnetPages.windowsKeys.bulkImport')" width="720px">
      <p class="mb-8px text-13px">{{ $t('vnetPages.windowsKeys.importHelp') }}</p>
      <ElInput
        v-model="importText"
        type="textarea"
        :rows="8"
        placeholder="MAY01 XXXXX-XXXXX-XXXXX-XXXXX-XXXXX&#10;MAY02 XXXXX-XXXXX-XXXXX-XXXXX-XXXXX"
        class="key-mono"
        @input="invalidatePreview"
      />
      <ElCheckbox v-model="importOverwrite" class="mt-8px" @change="invalidatePreview">
        {{ $t('vnetPages.windowsKeys.overwrite') }}
      </ElCheckbox>
      <div v-if="preview" class="mt-12px">
        <div class="mb-8px text-13px">{{ $t('vnetPages.windowsKeys.previewSummary', { n: willWrite }) }}</div>
        <ElTable :data="preview.rows" size="small" max-height="260">
          <ElTableColumn prop="line" :label="$t('vnetPages.windowsKeys.line')" width="70" />
          <ElTableColumn prop="machine_code" :label="$t('vnetPages.windowsKeys.machine')" width="140" />
          <ElTableColumn prop="key_masked" :label="$t('vnetPages.windowsKeys.key')" min-width="220" />
          <ElTableColumn :label="$t('vnetPages.windowsKeys.result')" width="170">
            <template #default="{ row }">
              <ElTag :type="importTagType(row.status)" size="small">
                {{ $t(`vnetPages.windowsKeys.importStatus.${row.status}` as any) }}
              </ElTag>
            </template>
          </ElTableColumn>
        </ElTable>
      </div>
      <template #footer>
        <ElButton @click="importVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton :loading="importBusy" :disabled="!importText.trim()" @click="runImport(true)">
          {{ $t('vnetPages.windowsKeys.preview') }}
        </ElButton>
        <ElButton type="primary" :loading="importBusy" :disabled="!willWrite" @click="runImport(false)">
          {{ $t('vnetPages.windowsKeys.import', { n: willWrite }) }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
.key-mono,
.key-mono :deep(input),
.key-mono :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
</style>

<style>
.windows-key-reveal .el-message-box__message {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 18px;
  letter-spacing: 1px;
}
</style>
