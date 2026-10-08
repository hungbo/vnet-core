<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const loading = ref(false);
const rows = ref<any[]>([]);
const info = ref<{ role: string; root: string; upstream_url: string; update_window: string }>({
  role: '',
  root: '',
  upstream_url: '',
  update_window: ''
});
// Ô riêng chứ không nằm trong info: vòng làm mới 3 giây ghi đè info, đang gõ dở sẽ mất chữ.
const clientRoot = ref('');
const updateWindow = ref('');

const isMaster = computed(() => info.value.role === 'master');
const isCafe = computed(() => info.value.role === 'cafe');

function formatSize(v: number) {
  if (!v) return '-';
  if (v >= 1024 ** 3) return `${(v / 1024 ** 3).toFixed(1)} GB`;
  return `${(v / 1024 ** 2).toFixed(1)} MB`;
}

const statusType: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'primary'> = {
  ready: 'success',
  downloading: 'primary',
  publishing: 'primary',
  queued: 'warning',
  error: 'danger',
  not_installed: 'info'
};

// Ở quán, "cần cập nhật" là khi master đã có bản mới hơn bản trên ổ.
function outdated(row: any) {
  return isCafe.value && row.remote_infohash && row.remote_infohash !== row.infohash;
}

let timer: ReturnType<typeof setTimeout> | undefined;

async function load(silent = false) {
  if (!silent) loading.value = true;
  try {
    const [i, list]: any = await Promise.all([client.get('/games/info'), client.get('/games')]);
    info.value = i;
    if (!silent) {
      clientRoot.value = i.client_root || '';
      updateWindow.value = i.update_window || '';
    }
    rows.value = list || [];
  } catch (e: any) {
    if (!silent) ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    loading.value = false;
  }
  // Đang tải thì cập nhật % mỗi 3 giây; xong hết thì thôi.
  clearTimeout(timer);
  if (rows.value.some(r => ['downloading', 'queued', 'publishing'].includes(r.status))) {
    timer = setTimeout(() => load(true), 3000);
  }
}

async function patch(row: any, body: Record<string, any>) {
  try {
    await client.put(`/games/${row.id}`, body);
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function publish(row: any) {
  row.publishing = true;
  try {
    await client.post(`/games/${row.id}/publish`, {});
    ElMessage.success($t('vnetPages.games.publishing', { name: row.display_name }));
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    row.publishing = false;
  }
}

async function syncNow(row: any) {
  try {
    await client.post(`/games/${row.id}/sync`, {});
    ElMessage.success($t('vnetPages.games.queued', { name: row.display_name }));
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.games.deleteConfirm', { name: row.display_name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/games/${row.id}`);
    ElMessage.success($t('vnetPages.common.deleted'));
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

const windowSaving = ref(false);
async function saveWindow() {
  windowSaving.value = true;
  try {
    await client.put('/settings/games', { update_window: updateWindow.value.trim() });
    ElMessage.success($t('vnetPages.common.success'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    windowSaving.value = false;
  }
}

const clientRootSaving = ref(false);
async function saveClientRoot() {
  clientRootSaving.value = true;
  try {
    await client.put('/settings/games', { client_root: clientRoot.value.trim() });
    ElMessage.success($t('vnetPages.common.success'));
    // Tải lại để ô hiện giá trị máy chủ thật sự dùng (đã chuẩn hoá, sai thì về mặc định).
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    clientRootSaving.value = false;
  }
}

const dialogVisible = ref(false);
const saving = ref(false);
const emptyForm = () => ({ name: '', display_name: '', category: '', launcher: '', launch_args: '', priority: 0 });
const form = ref(emptyForm());

function openCreate() {
  form.value = emptyForm();
  dialogVisible.value = true;
}

async function submit() {
  saving.value = true;
  try {
    await client.post('/games', form.value);
    ElMessage.success($t('vnetPages.common.success'));
    dialogVisible.value = false;
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    saving.value = false;
  }
}

onMounted(() => load());
onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap">
          <div style="display: flex; align-items: center; gap: 8px">
            <ElTag :type="info.role ? 'success' : 'info'">
              {{ $t(`vnetPages.games.role_${info.role || 'off'}`) }}
            </ElTag>
            <span v-if="info.root" style="color: var(--el-text-color-secondary); font-size: 13px">
              {{ $t('vnetPages.games.root') }}: {{ info.root }}
            </span>
            <span v-if="isCafe" style="color: var(--el-text-color-secondary); font-size: 13px">
              · {{ $t('vnetPages.games.upstream') }}: {{ info.upstream_url }}
            </span>
          </div>
          <div style="display: flex; gap: 8px">
            <ElButton @click="load()">{{ $t('common.refresh') }}</ElButton>
            <ElButton v-if="isMaster" type="primary" @click="openCreate">{{ $t('vnetPages.games.add') }}</ElButton>
          </div>
        </div>
      </template>

      <ElAlert v-if="!info.role" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.offHint') }}
      </ElAlert>
      <ElAlert v-else type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ isMaster ? $t('vnetPages.games.masterHint') : $t('vnetPages.games.cafeHint') }}
      </ElAlert>

      <div v-if="info.role" class="mb-16px flex flex-wrap items-center gap-8px">
        <span>{{ $t('vnetPages.games.clientRoot') }}</span>
        <ElInput v-model="clientRoot" placeholder="D:\Games" class="w-240px" clearable />
        <ElButton :loading="clientRootSaving" @click="saveClientRoot">{{ $t('vnetPages.common.save') }}</ElButton>
        <span class="text-12px text-gray-500">{{ $t('vnetPages.games.clientRootHint') }}</span>
      </div>

      <div v-if="isCafe" style="display: flex; align-items: center; gap: 8px; margin-bottom: 16px">
        <span>{{ $t('vnetPages.games.updateWindow') }}</span>
        <ElInput v-model="updateWindow" placeholder="02:00-08:00" style="width: 160px" clearable />
        <ElButton :loading="windowSaving" @click="saveWindow">{{ $t('vnetPages.common.save') }}</ElButton>
        <span style="color: var(--el-text-color-secondary); font-size: 12px">
          {{ $t('vnetPages.games.updateWindowHint') }}
        </span>
      </div>

      <ElTable v-loading="loading" :data="rows" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.games.name')" min-width="200">
          <template #default="{ row }">
            <div style="font-weight: 600">{{ row.display_name }}</div>
            <div style="color: var(--el-text-color-secondary); font-size: 12px">{{ row.name }}</div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.category')" width="120">
          <template #default="{ row }">{{ row.category || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.version')" width="120">
          <template #default="{ row }">
            <span>{{ row.version || '-' }}</span>
            <span v-if="outdated(row)" style="color: var(--el-color-warning)">→ {{ row.remote_version }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.size')" width="100">
          <template #default="{ row }">{{ formatSize(row.size_bytes) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.status')" min-width="190">
          <template #default="{ row }">
            <ElTag :type="statusType[row.status] || 'info'" size="small">
              {{ $t(`vnetPages.games.status_${row.status || 'not_installed'}`) }}
            </ElTag>
            <ElProgress v-if="row.status === 'downloading'" :percentage="row.progress" style="margin-top: 6px" />
            <div
              v-if="row.status === 'error' && row.error"
              style="color: var(--el-color-danger); font-size: 12px; margin-top: 4px"
            >
              {{ row.error }}
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.autoUpdate')" width="110">
          <template #default="{ row }">
            <ElSwitch :model-value="row.auto_update" @change="(v: any) => patch(row, { auto_update: v })" />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.enabled')" width="90">
          <template #default="{ row }">
            <ElSwitch :model-value="row.enabled" @change="(v: any) => patch(row, { enabled: v })" />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.priority')" width="120">
          <template #default="{ row }">
            <ElInputNumber
              :model-value="row.priority"
              size="small"
              controls-position="right"
              style="width: 90px"
              @change="(v: any) => patch(row, { priority: Number(v) || 0 })"
            />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.games.lastUpdated')" width="150">
          <template #default="{ row }">
            {{ row.last_updated_at ? dayjs(row.last_updated_at).format('DD/MM/YYYY HH:mm') : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="190" fixed="right">
          <template #default="{ row }">
            <template v-if="isMaster">
              <ElButton size="small" type="primary" plain :loading="row.publishing" :disabled="row.status === 'publishing'" @click="publish(row)">
                {{ $t('vnetPages.games.publishNow') }}
              </ElButton>
              <ElButton size="small" type="danger" plain @click="handleDelete(row)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
            </template>
            <ElButton
              v-else-if="isCafe"
              size="small"
              type="primary"
              plain
              :disabled="row.status === 'downloading' || !row.remote_infohash"
              @click="syncNow(row)"
            >
              {{ $t('vnetPages.games.syncNow') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog v-model="dialogVisible" :title="$t('vnetPages.games.add')" width="560px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.addHint', { root: info.root }) }}
      </ElAlert>
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.games.folder')">
          <ElInput v-model="form.name" placeholder="League of Legends" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.displayName')">
          <ElInput v-model="form.display_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.category')">
          <ElInput v-model="form.category" placeholder="Online" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.launcher')">
          <ElInput v-model="form.launcher" placeholder="Riot Client/RiotClientServices.exe" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.launchArgs')">
          <ElInput v-model="form.launch_args" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.priority')">
          <ElInputNumber v-model="form.priority" style="width: 100%" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="submit">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
