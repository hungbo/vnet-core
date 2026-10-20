<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useRealtimeRefresh } from '@/hooks/common/realtime';

const { t: $t } = useI18n();

const loading = ref(false);
const rows = ref<any[]>([]);
const info = ref<{
  role: string;
  root: string;
  upstream_url: string;
  update_window: string;
  source: string;
  hub_url: string;
  has_key: boolean;
  key_hint: string;
  game_root: string;
  last_sync_at?: string;
  sync_error: string;
  key_status: string;
  key_expires_at?: string;
  key_grace_until?: string;
  events_connected: boolean;
  local_clash?: string[];
}>({
  role: '',
  root: '',
  upstream_url: '',
  update_window: '',
  source: '',
  hub_url: '',
  has_key: false,
  key_hint: '',
  game_root: '',
  sync_error: '',
  key_status: '',
  events_connected: false
});
// Ô riêng chứ không nằm trong info: vòng làm mới 3 giây ghi đè info, đang gõ dở sẽ mất chữ.
const clientRoot = ref('');
const updateWindow = ref('');
const hubUrl = ref('');
const hubKey = ref(''); // không bao giờ nạp từ máy chủ: máy chủ không trả key ra
const gameRoot = ref('');

// Tình trạng key do Hub báo (qua report). Sắp hết hạn: còn không quá 14 ngày.
const keyDaysLeft = computed(() => (info.value.key_expires_at ? dayjs(info.value.key_expires_at).diff(dayjs(), 'day') : 0));
const keyTagType = computed<'success' | 'warning' | 'danger' | 'info'>(() => {
  if (info.value.key_status === 'expired') return 'danger';
  if (info.value.key_status === 'grace') return 'warning';
  if (info.value.key_status === 'active') return keyDaysLeft.value <= 14 ? 'warning' : 'success';
  return 'info';
});
const keyLine = computed(() => {
  const exp = info.value.key_expires_at ? dayjs(info.value.key_expires_at).format('DD/MM/YYYY') : '';
  const grace = info.value.key_grace_until ? dayjs(info.value.key_grace_until).format('DD/MM/YYYY') : '';
  switch (info.value.key_status) {
    case 'active':
      return keyDaysLeft.value <= 14
        ? $t('vnetPages.games.keyLineExpiring', { date: exp, days: Math.max(keyDaysLeft.value, 0) })
        : $t('vnetPages.games.keyLineActive', { date: exp });
    case 'grace':
      return $t('vnetPages.games.keyLineGrace', { date: exp, grace });
    case 'expired':
      return $t('vnetPages.games.keyLineExpired', { date: exp });
    default:
      return '';
  }
});
const isMaster = computed(() => info.value.role === 'master');
const isCafe = computed(() => info.value.role === 'cafe');
// Game cục bộ cần biết GAME_ROOT để chọn thư mục và kiểm tệp chạy; master thì phát hành game, không có game cục bộ.
const canAddLocal = computed(() => isCafe.value && Boolean(info.value.root));
const isLocal = (row: any) => row.origin === 'local';

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

// Tải lặng chỉ đổi ô nào CHƯA bị sửa (còn đúng giá trị máy chủ lần trước):
// người khác vừa đổi cài đặt thì ô theo kịp, còn ô đang gõ dở giữ nguyên chữ.
function syncFields(i: any, silent: boolean) {
  const cu: any = info.value;
  const fields: [typeof clientRoot, string, string][] = [
    [clientRoot, cu.client_root, i.client_root],
    [updateWindow, cu.update_window, i.update_window],
    [hubUrl, cu.hub_url, i.hub_url],
    [gameRoot, cu.game_root, i.game_root]
  ];
  for (const [field, before, now] of fields) {
    if (!silent || field.value === (before || '')) field.value = now || '';
  }
}

async function load(silent = false) {
  if (!silent) loading.value = true;
  try {
    const [i, list]: any = await Promise.all([client.get('/games/info'), client.get('/games')]);
    syncFields(i, silent);
    info.value = i;
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

const hubSaving = ref(false);
async function saveHub() {
  hubSaving.value = true;
  try {
    // Máy chủ đọc thử catalog bằng key này trước khi lưu: key sai hay địa chỉ sai báo lỗi ngay ở đây.
    await client.put('/games/hub', {
      hub_url: hubUrl.value.trim(),
      hub_key: hubKey.value.trim(),
      game_root: gameRoot.value.trim()
    });
    hubKey.value = '';
    ElMessage.success($t('vnetPages.games.hubConnected'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    hubSaving.value = false;
  }
}

const hubClearing = ref(false);
async function clearHub() {
  try {
    await ElMessageBox.confirm($t('vnetPages.games.hubDisconnectConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return;
  }
  hubClearing.value = true;
  try {
    await client.delete('/games/hub');
    hubKey.value = '';
    ElMessage.success($t('vnetPages.games.hubCleared'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    hubClearing.value = false;
  }
}

const dialogVisible = ref(false);
const saving = ref(false);
// Một mục riêng trên menu máy trạm, cho thư mục có nhiều chương trình (LibreOffice → Word, Excel).
interface MenuEntry {
  display_name: string;
  launcher: string;
  launch_args: string;
  icon: string;
}
const emptyEntry = (): MenuEntry => ({ display_name: '', launcher: '', launch_args: '', icon: '' });
// Một bước chạy trước khi mở game (RunFirst).
interface RunStep {
  launcher: string;
  launch_args: string;
}
const emptyStep = (): RunStep => ({ launcher: '', launch_args: '' });
const cleanSteps = (list: RunStep[]) => list.filter(s => s.launcher.trim() !== '');
const emptyForm = () => ({
  name: '',
  display_name: '',
  category: '',
  launcher: '',
  launch_args: '',
  icon: '',
  menu_entries: [] as MenuEntry[],
  run_first: [] as RunStep[],
  client_root: '',
  priority: 0
});
const form = ref(emptyForm());
// Hộp thêm/sửa game cục bộ: thư mục và tệp chạy chọn từ gợi ý của máy chủ (vẫn gõ tay được cho tệp chạy).
const localMode = ref(false);
const localFolders = ref<string[]>([]);
const localLaunchers = ref<string[]>([]);
// Có id là đang sửa game đã có, rỗng là thêm mới. Thư mục game (name) là khoá nối
// master với các quán nên không đổi được.
const editingId = ref('');

function openCreate() {
  editingId.value = '';
  localMode.value = false;
  form.value = emptyForm();
  dialogVisible.value = true;
}

async function loadLocalLaunchers(folder: string) {
  localLaunchers.value = [];
  if (!folder) return;
  try {
    localLaunchers.value = ((await client.get('/games/local/launchers', { params: { folder } })) as string[]) || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function openCreateLocal() {
  editingId.value = '';
  localMode.value = true;
  form.value = emptyForm();
  localFolders.value = [];
  localLaunchers.value = [];
  dialogVisible.value = true;
  try {
    localFolders.value = ((await client.get('/games/local/folders')) as string[]) || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

function onLocalFolderPicked(folder: string) {
  if (!form.value.display_name) form.value.display_name = folder;
  form.value.launcher = '';
  loadLocalLaunchers(folder);
}

function openEdit(row: any) {
  editingId.value = row.id;
  localMode.value = isLocal(row);
  if (localMode.value) loadLocalLaunchers(row.name);
  form.value = {
    name: row.name,
    display_name: row.display_name,
    category: row.category || '',
    launcher: row.launcher || '',
    launch_args: row.launch_args || '',
    icon: row.icon || '',
    menu_entries: (row.menu_entries || []).map((e: Partial<MenuEntry>) => ({ ...emptyEntry(), ...e })),
    run_first: (row.run_first || []).map((s: Partial<RunStep>) => ({ ...emptyStep(), ...s })),
    client_root: row.custom?.client_root || '',
    priority: row.priority || 0
  };
  dialogVisible.value = true;
}

// Hộp "Tuỳ chỉnh" của quán trên một game: tên, icon, gốc game và lệnh chạy trước riêng. Catalog không ghi đè.
const customVisible = ref(false);
const customSaving = ref(false);
const customRow = ref<any>(null);
const customForm = ref({
  display_name: '',
  icon: '',
  client_root: '',
  follow_run_first: true,
  run_first: [] as RunStep[]
});
const hasCustom = (row: any) => Boolean(row.custom && Object.keys(row.custom).length);

function openCustom(row: any) {
  customRow.value = row;
  const c = row.custom || {};
  customForm.value = {
    display_name: c.display_name || '',
    icon: c.icon || '',
    client_root: c.client_root || '',
    follow_run_first: !c.run_first,
    run_first: (c.run_first || []).map((s: Partial<RunStep>) => ({ ...emptyStep(), ...s }))
  };
  customVisible.value = true;
}

async function saveCustom() {
  const f = customForm.value;
  customSaving.value = true;
  try {
    await client.put(`/games/${customRow.value.id}/custom`, {
      display_name: f.display_name,
      icon: f.icon,
      client_root: f.client_root,
      ...(f.follow_run_first ? { reset_run_first: true } : { run_first: cleanSteps(f.run_first) })
    });
    ElMessage.success($t('vnetPages.common.success'));
    customVisible.value = false;
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    customSaving.value = false;
  }
}

async function submit() {
  saving.value = true;
  try {
    if (editingId.value) {
      const { display_name, category, launcher, launch_args, icon, menu_entries, run_first, client_root, priority } =
        form.value;
      await client.put(`/games/${editingId.value}`, {
        display_name,
        category,
        launcher,
        launch_args,
        icon,
        menu_entries,
        run_first: cleanSteps(run_first),
        // Gốc game riêng chỉ đi cùng game cục bộ; game của Hub đặt ở hộp Tuỳ chỉnh.
        ...(localMode.value ? { client_root } : {}),
        priority
      });
    } else {
      await client.post(localMode.value ? '/games/local' : '/games', {
        ...form.value,
        run_first: cleanSteps(form.value.run_first)
      });
    }
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

// Trạng thái game đổi chủ yếu ở vòng nền (publish xong, tải xong hay lỗi, catalog
// của master có bản mới) — máy chủ báo "games". Khung giờ và gốc game nằm trong
// cài đặt ("settings"). Tải lặng (silent) để không ghi đè hai ô đang gõ dở; phần
// trăm đang tải vẫn hỏi mỗi 3 giây vì nó lấy thẳng từ engine, không ghi DB.
useRealtimeRefresh(['games', 'settings'], () => load(true));
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
            <ElButton v-if="canAddLocal" type="primary" @click="openCreateLocal">
              {{ $t('vnetPages.games.addLocal') }}
            </ElButton>
          </div>
        </div>
      </template>

      <ElAlert
        v-if="info.local_clash && info.local_clash.length"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      >
        {{ $t('vnetPages.games.localClash', { names: info.local_clash.join(', ') }) }}
      </ElAlert>

      <ElAlert v-if="!info.role" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.offHint') }}
      </ElAlert>
      <ElAlert v-else type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ isMaster ? $t('vnetPages.games.masterHint') : $t('vnetPages.games.cafeHint') }}
      </ElAlert>

      <div class="mb-16px">
        <div class="mb-8px font-600">{{ $t('vnetPages.games.hubTitle') }}</div>
        <ElAlert v-if="info.source === 'env'" type="info" :closable="false" show-icon>
          {{ $t('vnetPages.games.hubEnvHint') }}
        </ElAlert>
        <template v-else>
          <div class="flex flex-wrap items-end gap-8px">
            <div>
              <div class="mb-2px text-12px text-gray-500">{{ $t('vnetPages.games.hubUrl') }}</div>
              <ElInput
                v-model="hubUrl"
                :placeholder="$t('vnetPages.games.hubUrlPlaceholder')"
                :aria-label="$t('vnetPages.games.hubUrl')"
                class="w-260px"
                clearable
              />
            </div>
            <div>
              <div class="mb-2px text-12px text-gray-500">{{ $t('vnetPages.games.hubKey') }}</div>
              <ElInput
                v-model="hubKey"
                type="password"
                show-password
                autocomplete="new-password"
                :placeholder="
                  info.has_key
                    ? $t('vnetPages.games.hubKeySaved', { hint: info.key_hint })
                    : 'VGK-XXXXX-XXXXX-XXXXX-XXXXX'
                "
                :aria-label="$t('vnetPages.games.hubKey')"
                class="w-300px"
              />
            </div>
            <div>
              <div class="mb-2px text-12px text-gray-500">{{ $t('vnetPages.games.gameRoot') }}</div>
              <ElInput
                v-model="gameRoot"
                placeholder="D:\Games"
                :aria-label="$t('vnetPages.games.gameRoot')"
                class="w-220px"
                clearable
              />
            </div>
            <ElButton type="primary" :loading="hubSaving" @click="saveHub">
              {{ $t('vnetPages.games.hubConnect') }}
            </ElButton>
            <ElButton v-if="info.hub_url || info.has_key" :loading="hubClearing" @click="clearHub">
              {{ $t('vnetPages.games.hubDisconnect') }}
            </ElButton>
          </div>
          <div class="mt-4px text-12px text-gray-500">
            {{ $t('vnetPages.games.gameRootHint') }} {{ $t('vnetPages.games.hubKeyKeepHint') }}
          </div>
          <div v-if="info.source === 'settings'" class="mt-4px text-12px">
            <span v-if="info.sync_error" style="color: var(--el-color-danger)">
              {{ $t('vnetPages.games.hubSyncError', { error: info.sync_error }) }}
            </span>
            <span v-else-if="info.last_sync_at" style="color: var(--el-color-success)">
              {{
                $t('vnetPages.games.hubSyncOk', { time: dayjs(info.last_sync_at).format('DD/MM/YYYY HH:mm:ss') })
              }}
            </span>
            <span v-else class="text-gray-500">{{ $t('vnetPages.games.hubConnecting') }}</span>
            <span class="ml-8px text-gray-500">
              · {{ info.events_connected ? $t('vnetPages.games.eventsOn') : $t('vnetPages.games.eventsOff') }}
            </span>
          </div>
          <div v-if="info.source === 'settings' && info.key_status" class="mt-4px flex items-center gap-8px text-12px">
            <ElTag size="small" :type="keyTagType">{{ $t(`vnetPages.games.keyStatus_${info.key_status}`) }}</ElTag>
            <span>{{ keyLine }}</span>
          </div>
        </template>
      </div>

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
            <div style="font-weight: 600">
              {{ row.custom?.display_name || row.display_name }}
              <ElTag v-if="isLocal(row)" size="small" type="warning" style="margin-left: 6px">
                {{ $t('vnetPages.games.localTag') }}
              </ElTag>
              <ElTag v-if="hasCustom(row)" size="small" type="info" style="margin-left: 6px">
                {{ $t('vnetPages.games.customTag') }}
              </ElTag>
            </div>
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
        <!-- Số lượt khách bấm "Chơi" trên menu máy trạm (từ client 1.0.19). -->
        <ElTableColumn :label="$t('vnetPages.games.launchCount')" width="100" align="right">
          <template #default="{ row }">{{ (row.launch_count || 0).toLocaleString('vi-VN') }}</template>
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
            <span v-if="isLocal(row)" style="color: var(--el-text-color-secondary)">-</span>
            <ElSwitch v-else :model-value="row.auto_update" @change="(v: any) => patch(row, { auto_update: v })" />
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
        <ElTableColumn :label="$t('vnetPages.common.action')" width="250" fixed="right">
          <template #default="{ row }">
            <template v-if="isMaster">
              <ElButton size="small" @click="openEdit(row)">
                {{ $t('vnetPages.common.edit') }}
              </ElButton>
              <ElButton size="small" type="primary" plain :loading="row.publishing" :disabled="row.status === 'publishing'" @click="publish(row)">
                {{ $t('vnetPages.games.publishNow') }}
              </ElButton>
              <ElButton size="small" type="danger" plain @click="handleDelete(row)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
            </template>
            <template v-else-if="isCafe && isLocal(row)">
              <ElButton size="small" @click="openEdit(row)">
                {{ $t('vnetPages.common.edit') }}
              </ElButton>
              <ElButton size="small" type="danger" plain @click="handleDelete(row)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
            </template>
            <template v-else-if="isCafe">
              <ElButton size="small" @click="openCustom(row)">{{ $t('vnetPages.games.customize') }}</ElButton>
              <ElButton
                size="small"
                type="primary"
                plain
                :disabled="row.status === 'downloading' || !row.remote_infohash"
                @click="syncNow(row)"
              >
                {{ $t('vnetPages.games.syncNow') }}
              </ElButton>
            </template>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? $t('vnetPages.common.edit') : localMode ? $t('vnetPages.games.addLocal') : $t('vnetPages.games.add')"
      width="720px"
    >
      <ElAlert v-if="localMode" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.localHint', { root: info.root }) }}
      </ElAlert>
      <ElAlert v-else-if="!editingId" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.addHint', { root: info.root }) }}
      </ElAlert>
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.games.folder')">
          <ElSelect
            v-if="localMode && !editingId"
            v-model="form.name"
            filterable
            class="w-full"
            :placeholder="$t('vnetPages.games.localFolderPick')"
            :no-data-text="$t('vnetPages.games.localNoFolders', { root: info.root })"
            @change="onLocalFolderPicked"
          >
            <ElOption v-for="f in localFolders" :key="f" :label="f" :value="f" />
          </ElSelect>
          <ElInput v-else v-model="form.name" placeholder="League of Legends" :disabled="!!editingId" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.displayName')">
          <ElInput v-model="form.display_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.category')">
          <ElInput v-model="form.category" placeholder="Online" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.launcher')">
          <ElSelect
            v-if="localMode"
            v-model="form.launcher"
            filterable
            allow-create
            default-first-option
            class="w-full"
            :placeholder="$t('vnetPages.games.localLauncherPick')"
          >
            <ElOption v-for="f in localLaunchers" :key="f" :label="f" :value="f" />
          </ElSelect>
          <ElInput v-else v-model="form.launcher" placeholder="Riot Client/RiotClientServices.exe" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.launchArgs')">
          <ElInput v-model="form.launch_args" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.icon')">
          <ElInput v-model="form.icon" placeholder="League of Legends/LeagueClient.exe" />
          <div class="text-12px text-gray-500">{{ $t('vnetPages.games.iconHint') }}</div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.menuEntries')">
          <div class="w-full">
            <div class="text-12px text-gray-500">{{ $t('vnetPages.games.menuEntriesHint') }}</div>
            <div
              v-for="(e, i) in form.menu_entries"
              :key="i"
              class="grid grid-cols-[1fr_1fr_auto] mt-8px items-center gap-6px"
            >
              <ElInput v-model="e.display_name" :placeholder="$t('vnetPages.games.entryName')" />
              <ElInput v-model="e.launcher" placeholder="program/swriter.exe" />
              <ElButton type="danger" link @click="form.menu_entries.splice(i, 1)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
              <ElInput v-model="e.launch_args" :placeholder="$t('vnetPages.games.launchArgs')" />
              <ElInput v-model="e.icon" :placeholder="$t('vnetPages.games.icon')" />
            </div>
            <ElButton class="mt-8px" size="small" @click="form.menu_entries.push(emptyEntry())">
              {{ $t('vnetPages.games.addEntry') }}
            </ElButton>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.runFirst')">
          <div class="w-full">
            <div class="text-12px text-gray-500">{{ $t('vnetPages.games.runFirstHint') }}</div>
            <div v-for="(s, i) in form.run_first" :key="i" class="grid grid-cols-[1fr_1fr_auto] mt-8px items-center gap-6px">
              <ElInput v-model="s.launcher" :placeholder="$t('vnetPages.games.stepLauncher')" />
              <ElInput v-model="s.launch_args" :placeholder="$t('vnetPages.games.stepArgs')" />
              <ElButton type="danger" link @click="form.run_first.splice(i, 1)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
            </div>
            <ElButton v-if="form.run_first.length < 5" class="mt-8px" size="small" @click="form.run_first.push(emptyStep())">
              {{ $t('vnetPages.games.addStep') }}
            </ElButton>
          </div>
        </ElFormItem>
        <ElFormItem v-if="localMode && editingId" :label="$t('vnetPages.games.clientRootLocal')">
          <ElInput v-model="form.client_root" placeholder="E:\Games2" />
          <div class="text-12px text-gray-500">{{ $t('vnetPages.games.clientRootLocalHint') }}</div>
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

    <ElDialog v-model="customVisible" :title="$t('vnetPages.games.customTitle')" width="680px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.customHint') }}
      </ElAlert>
      <ElForm v-if="customRow" label-width="150px">
        <ElFormItem :label="$t('vnetPages.games.customDisplayName')">
          <ElInput v-model="customForm.display_name" :placeholder="customRow.display_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.customIcon')">
          <ElInput v-model="customForm.icon" :placeholder="customRow.icon || customRow.launcher" />
          <div class="text-12px text-gray-500">{{ $t('vnetPages.games.iconHint') }}</div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.clientRootLocal')">
          <ElInput v-model="customForm.client_root" placeholder="E:\Games2" />
          <div class="text-12px text-gray-500">{{ $t('vnetPages.games.clientRootLocalHint') }}</div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.runFirst')">
          <div class="w-full">
            <ElCheckbox v-model="customForm.follow_run_first">{{ $t('vnetPages.games.customRunFirstFollow') }}</ElCheckbox>
            <template v-if="!customForm.follow_run_first">
              <div class="text-12px text-gray-500">{{ $t('vnetPages.games.runFirstHint') }}</div>
              <div
                v-for="(s, i) in customForm.run_first"
                :key="i"
                class="grid grid-cols-[1fr_1fr_auto] mt-8px items-center gap-6px"
              >
                <ElInput v-model="s.launcher" :placeholder="$t('vnetPages.games.stepLauncher')" />
                <ElInput v-model="s.launch_args" :placeholder="$t('vnetPages.games.stepArgs')" />
                <ElButton type="danger" link @click="customForm.run_first.splice(i, 1)">
                  {{ $t('vnetPages.common.delete') }}
                </ElButton>
              </div>
              <ElButton
                v-if="customForm.run_first.length < 5"
                class="mt-8px"
                size="small"
                @click="customForm.run_first.push(emptyStep())"
              >
                {{ $t('vnetPages.games.addStep') }}
              </ElButton>
            </template>
          </div>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="customVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="customSaving" @click="saveCustom">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
