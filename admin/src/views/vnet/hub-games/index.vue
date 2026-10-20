<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAuth } from '@/hooks/business/auth';
import { useRealtimeRefresh } from '@/hooks/common/realtime';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();
const canEdit = computed(() => hasAuth('games.update'));

// ---- dữ liệu ---------------------------------------------------------------

interface LocalState {
  id: string;
  status: string;
  progress: number;
  version: number;
  auto_update: boolean;
  priority: number;
  error: string;
  needs_update: boolean;
}

interface CatalogRow {
  name: string;
  display_name: string;
  category: string;
  source_type: string;
  size_bytes: number;
  approx_size_mb: number;
  available: boolean;
  version: number;
  mandatory: boolean;
  selected: boolean;
  local?: LocalState;
}

interface QueueEntry {
  name: string;
  display_name: string;
  priority: number;
  state: string;
  progress: number;
  bytes_per_sec: number;
  peers: number;
  size_bytes: number;
  retry_at?: string;
  error?: string;
}

interface QueueView {
  current: QueueEntry | null;
  waiting: QueueEntry[];
  in_window: boolean;
  update_window: string;
  limits: { download_mbps: number; upload_mbps: number };
}

const loading = ref(false);
const mode = ref<'all' | 'selected'>('all');
const rows = ref<CatalogRow[]>([]);
const errorText = ref('');
const queue = ref<QueueView | null>(null);

async function load() {
  loading.value = true;
  errorText.value = '';
  try {
    const res: any = await client.get('/games/hub-catalog');
    mode.value = res.mode === 'selected' ? 'selected' : 'all';
    rows.value = res.items || [];
  } catch (e: any) {
    errorText.value = e?.message || $t('vnetPages.hubGames.loadFailed');
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

// Hàng đợi hỏi riêng và dày hơn: tốc độ và phần trăm đổi từng giây trong khi danh mục thì không.
async function loadQueue() {
  try {
    queue.value = (await client.get('/games/queue')) as any;
  } catch {
    queue.value = null;
  }
}

// Ô trần băng thông là ô riêng chứ không đọc thẳng từ queue: vòng làm mới 3 giây ghi đè, đang gõ dở sẽ mất chữ.
const limits = ref({ download_mbps: 0, upload_mbps: 0 });

async function loadLimits() {
  try {
    limits.value = (await client.get('/games/limits')) as any;
  } catch {
    // Không có trần đã lưu thì để 0 (không giới hạn).
  }
}

async function saveLimits() {
  try {
    await client.put('/games/limits', {
      download_mbps: Number(limits.value.download_mbps) || 0,
      upload_mbps: Number(limits.value.upload_mbps) || 0
    });
    ElMessage.success($t('vnetPages.hubGames.limitsSaved'));
    loadQueue();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.hubGames.actionFailed'));
  }
}

let queueTimer: ReturnType<typeof setInterval> | undefined;
onMounted(() => {
  load();
  loadQueue();
  loadLimits();
  queueTimer = setInterval(() => {
    if (!document.hidden) loadQueue();
  }, 3000);
});
onBeforeUnmount(() => {
  if (queueTimer) clearInterval(queueTimer);
});
// Trang Game, quán đổi lựa chọn, vòng nền tải xong hay lỗi đều phát "games:changed".
useRealtimeRefresh(['games'], () => {
  load();
  loadQueue();
});

// ---- lọc -------------------------------------------------------------------

const search = ref('');
const filterCategory = ref('');
const filterSource = ref('');
const filterState = ref('');
const page = ref(1);
const pageSize = ref(50);

const categories = computed(() => [...new Set(rows.value.map(r => r.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'vi')));
const sources = computed(() => [...new Set(rows.value.map(r => r.source_type).filter(Boolean))]);

// r: any — ElTable truyền DefaultRow (Record<string, any>) vào slot.
function stateOf(r: any): string {
  if (r.local?.status === 'downloading') return 'downloading';
  if (r.local?.status === 'error') return 'error';
  if (r.local?.needs_update) return 'needs_update';
  if (r.local && r.local.version > 0) return 'installed';
  return 'not_installed';
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  return rows.value.filter(r => {
    if (q && ![r.display_name, r.name, r.category].some(s => s.toLowerCase().includes(q))) return false;
    if (filterCategory.value && r.category !== filterCategory.value) return false;
    if (filterSource.value && r.source_type !== filterSource.value) return false;
    switch (filterState.value) {
      case '':
        return true;
      case 'selected':
        return r.selected;
      case 'not_selected':
        return !r.selected;
      case 'available':
        return r.available;
      case 'unavailable':
        return !r.available;
      default:
        return stateOf(r) === filterState.value;
    }
  });
});

const pageRows = computed(() => filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value));

function resetPage() {
  page.value = 1;
}

// ---- hiển thị --------------------------------------------------------------

function formatSize(bytes: number, approxMB: number) {
  const b = bytes > 0 ? bytes : approxMB * 1024 * 1024;
  if (b <= 0) return '-';
  const gb = b / 1024 ** 3;
  const text = gb >= 1 ? `${gb.toFixed(1)} GB` : `${Math.round(b / 1024 ** 2)} MB`;
  return bytes > 0 ? text : `~${text}`;
}

function bitRate(bps: number) {
  const bits = bps * 8;
  if (bits >= 1_000_000) return `${(bits / 1_000_000).toFixed(1)} Mbps`;
  return `${Math.round(bits / 1000)} kbps`;
}

function stateType(s: string): any {
  return ({ installed: 'success', downloading: 'primary', needs_update: 'warning', error: 'danger', not_installed: 'info' } as Record<string, string>)[s] || 'info';
}

function formatTime(v?: string) {
  return v ? dayjs(v).format('HH:mm:ss') : '';
}

// ---- thao tác --------------------------------------------------------------

const selection = ref<CatalogRow[]>([]);
const busy = ref(false);

function onSelectionChange(v: CatalogRow[]) {
  selection.value = v;
}

async function run(action: () => Promise<any>, okMessage: () => string): Promise<any> {
  if (busy.value) return undefined;
  busy.value = true;
  try {
    const res = await action();
    ElMessage.success(okMessage());
    return res;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.hubGames.actionFailed'));
    return undefined;
  } finally {
    busy.value = false;
    load();
    loadQueue();
  }
}

// Chọn thêm game; downloadNow xếp tải ngay (bỏ qua khung giờ cập nhật).
async function choose(names: string[], downloadNow: boolean) {
  const res: any = await run(
    () => client.put('/games/hub-selection', { mode: 'selected', add: names, download_now: downloadNow }),
    () => $t('vnetPages.hubGames.chosen', { n: names.length })
  );
  if (res?.unknown?.length) ElMessage.warning($t('vnetPages.hubGames.unknown', { names: res.unknown.join(', ') }));
}

async function unchoose(names: string[]) {
  try {
    await ElMessageBox.confirm($t('vnetPages.hubGames.unchooseConfirm', { n: names.length }), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return;
  }
  await run(
    () => client.put('/games/hub-selection', { mode: 'selected', remove: names }),
    () => $t('vnetPages.hubGames.unchosen', { n: names.length })
  );
}

// Chuyển từ "nhận mọi game" sang "chỉ game đã chọn": Hub bắt đầu từ các game quán đang có.
async function startChoosing() {
  try {
    await ElMessageBox.confirm($t('vnetPages.hubGames.startChoosingConfirm'), $t('vnetPages.common.confirm'), { type: 'warning' });
  } catch {
    return;
  }
  await run(
    () => client.put('/games/hub-selection', { mode: 'selected' }),
    () => $t('vnetPages.hubGames.modeSelected')
  );
}

async function receiveAll() {
  try {
    await ElMessageBox.confirm($t('vnetPages.hubGames.receiveAllConfirm'), $t('vnetPages.common.confirm'), { type: 'warning' });
  } catch {
    return;
  }
  await run(
    () => client.put('/games/hub-selection', { mode: 'all' }),
    () => $t('vnetPages.hubGames.modeAll')
  );
}

// "Tải ngay" game chưa có và "Sửa chữa" game đã có là cùng một lệnh: máy chủ luôn băm lại cả thư mục, tải phần thiếu hay
// hỏng rồi xoá tệp thừa.
async function syncNow(names: string[]) {
  await run(
    () => client.post('/games/bulk-sync', { names }),
    () => $t('vnetPages.hubGames.queued', { n: names.length })
  );
}

async function repair(row: any) {
  try {
    await ElMessageBox.confirm($t('vnetPages.hubGames.repairConfirm', { name: row.display_name }), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return;
  }
  await syncNow([row.name]);
}

async function toggleAutoUpdate(row: any, on: boolean) {
  if (!row.local) return;
  try {
    await client.put(`/games/${row.local.id}`, { auto_update: on });
    row.local.auto_update = on;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.hubGames.actionFailed'));
  }
}

const selectedNames = computed(() => selection.value.map(r => r.name));
const downloadable = computed(() => selection.value.filter(r => r.available).map(r => r.name));
</script>

<template>
  <div class="flex-col-stretch gap-16px">
    <ElCard>
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-8px">
          <span>{{ $t('vnetPages.hubGames.queueTitle') }}</span>
          <span v-if="queue" class="text-13px text-[var(--el-text-color-secondary)]">
            {{ $t('vnetPages.hubGames.waiting', { n: queue.waiting.length }) }}
            <template v-if="queue.update_window">
              ·
              <ElTag :type="queue.in_window ? 'success' : 'warning'" size="small">
                {{ $t('vnetPages.hubGames.window') }} {{ queue.update_window }}
              </ElTag>
            </template>
          </span>
        </div>
      </template>

      <div v-if="queue?.current" class="mb-12px">
        <div class="mb-4px flex flex-wrap items-center justify-between gap-8px">
          <b>{{ queue.current.display_name }}</b>
          <span class="text-13px">
            {{ bitRate(queue.current.bytes_per_sec) }} · {{ $t('vnetPages.hubGames.peers', { n: queue.current.peers }) }}
          </span>
        </div>
        <ElProgress :percentage="queue.current.progress" :stroke-width="14" />
      </div>
      <div v-else class="mb-12px text-13px text-[var(--el-text-color-secondary)]">{{ $t('vnetPages.hubGames.idle') }}</div>

      <div v-if="queue?.waiting.length" class="mb-12px flex flex-wrap gap-6px">
        <ElTooltip
          v-for="w in queue.waiting.slice(0, 12)"
          :key="w.name"
          :content="
            w.retry_at ? `${$t('vnetPages.hubGames.queueStates.retry')} ${formatTime(w.retry_at)}` : $t(`vnetPages.hubGames.queueStates.${w.state}` as any)
          "
        >
          <ElTag :type="w.state === 'queued' ? 'primary' : w.state === 'retry' ? 'danger' : 'info'" size="small">
            {{ w.display_name }}
          </ElTag>
        </ElTooltip>
        <span v-if="queue.waiting.length > 12" class="text-12px">+{{ queue.waiting.length - 12 }}</span>
      </div>

      <div class="flex flex-wrap items-center gap-12px">
        <span class="text-13px">{{ $t('vnetPages.hubGames.limitDown') }}</span>
        <ElInputNumber v-model="limits.download_mbps" :min="0" :max="100000" :step="10" size="small" :disabled="!canEdit" />
        <span class="text-13px">{{ $t('vnetPages.hubGames.limitUp') }}</span>
        <ElInputNumber v-model="limits.upload_mbps" :min="0" :max="100000" :step="10" size="small" :disabled="!canEdit" />
        <span class="text-12px text-[var(--el-text-color-secondary)]">{{ $t('vnetPages.hubGames.limitHint') }}</span>
        <ElButton v-if="canEdit" size="small" type="primary" plain @click="saveLimits">{{ $t('vnetPages.common.save') }}</ElButton>
      </div>
    </ElCard>

    <ElCard>
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-8px">
          <span>{{ $t('vnetPages.hubGames.title') }}</span>
          <span class="text-13px">
            <ElTag :type="mode === 'all' ? 'success' : 'warning'" size="small">
              {{ mode === 'all' ? $t('vnetPages.hubGames.modeAllLabel') : $t('vnetPages.hubGames.modeSelectedLabel') }}
            </ElTag>
            <ElButton v-if="canEdit && mode === 'all'" size="small" class="ml-8px" @click="startChoosing">
              {{ $t('vnetPages.hubGames.startChoosing') }}
            </ElButton>
            <ElButton v-if="canEdit && mode === 'selected'" size="small" class="ml-8px" @click="receiveAll">
              {{ $t('vnetPages.hubGames.receiveAll') }}
            </ElButton>
          </span>
        </div>
      </template>

      <ElAlert v-if="errorText" type="error" :closable="false" class="mb-12px" :title="errorText" />
      <ElAlert
        v-else
        type="info"
        :closable="false"
        class="mb-12px"
        :title="mode === 'all' ? $t('vnetPages.hubGames.hintAll') : $t('vnetPages.hubGames.hintSelected')"
      />

      <div class="mb-12px flex flex-wrap items-center gap-8px">
        <ElInput
          v-model="search"
          :placeholder="$t('vnetPages.hubGames.searchPlaceholder')"
          clearable
          style="width: 240px"
          @input="resetPage"
        />
        <ElSelect v-model="filterCategory" clearable :placeholder="$t('vnetPages.hubGames.category')" style="width: 150px" @change="resetPage">
          <ElOption v-for="c in categories" :key="c" :label="c" :value="c" />
        </ElSelect>
        <ElSelect v-model="filterSource" clearable :placeholder="$t('vnetPages.hubGames.source')" style="width: 150px" @change="resetPage">
          <ElOption v-for="s in sources" :key="s" :label="$t(`vnetPages.hubGames.sources.${s}` as any)" :value="s" />
        </ElSelect>
        <ElSelect v-model="filterState" clearable :placeholder="$t('vnetPages.hubGames.state')" style="width: 190px" @change="resetPage">
          <ElOption v-for="s in ['selected', 'not_selected', 'available', 'unavailable', 'installed', 'downloading', 'needs_update', 'error', 'not_installed']" :key="s" :label="$t(`vnetPages.hubGames.states.${s}` as any)" :value="s" />
        </ElSelect>
        <ElButton :loading="loading" @click="load">{{ $t('common.refresh') }}</ElButton>
        <span class="ml-auto text-13px text-[var(--el-text-color-secondary)]">{{ $t('vnetPages.hubGames.count', { n: filtered.length }) }}</span>
      </div>

      <div v-if="canEdit && selection.length" class="mb-12px flex flex-wrap items-center gap-8px rounded-4px bg-[var(--el-fill-color-light)] p-8px">
        <span class="text-13px">{{ $t('vnetPages.hubGames.picked', { n: selection.length }) }}</span>
        <ElButton size="small" type="primary" :disabled="busy || mode === 'all'" @click="choose(selectedNames, true)">
          {{ $t('vnetPages.hubGames.chooseAndDownload') }}
        </ElButton>
        <ElButton size="small" :disabled="busy || mode === 'all'" @click="choose(selectedNames, false)">{{ $t('vnetPages.hubGames.choose') }}</ElButton>
        <ElButton size="small" :disabled="busy || !downloadable.length" @click="syncNow(downloadable)">
          {{ $t('vnetPages.hubGames.downloadNow') }}
        </ElButton>
        <ElButton size="small" type="danger" plain :disabled="busy || mode === 'all'" @click="unchoose(selectedNames)">
          {{ $t('vnetPages.hubGames.unchoose') }}
        </ElButton>
      </div>

      <ElTable v-loading="loading" :data="pageRows" row-key="name" style="width: 100%" @selection-change="onSelectionChange">
        <ElTableColumn type="selection" width="44" :selectable="() => canEdit" />
        <ElTableColumn :label="$t('vnetPages.hubGames.game')" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="font-medium">{{ row.display_name }}</span>
            <ElTag v-if="row.mandatory" size="small" type="info" class="ml-6px">{{ $t('vnetPages.hubGames.mandatory') }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="category" :label="$t('vnetPages.hubGames.category')" width="130" show-overflow-tooltip />
        <ElTableColumn :label="$t('vnetPages.hubGames.source')" width="120">
          <template #default="{ row }">
            <span v-if="row.source_type">{{ $t(`vnetPages.hubGames.sources.${row.source_type}` as any) }}</span>
            <span v-else>-</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.hubGames.size')" width="100" align="right">
          <template #default="{ row }">{{ formatSize(row.size_bytes, row.approx_size_mb) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.hubGames.hubStatus')" width="150">
          <template #default="{ row }">
            <ElTag v-if="row.available" type="success" size="small">{{ $t('vnetPages.hubGames.available', { v: row.version }) }}</ElTag>
            <ElTag v-else type="warning" size="small">{{ $t('vnetPages.hubGames.unavailable') }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.hubGames.selected')" width="90" align="center">
          <template #default="{ row }">
            <ElTag :type="row.selected ? 'success' : 'info'" size="small">{{ row.selected ? $t('vnetPages.common.yes') : $t('vnetPages.common.no') }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.hubGames.atCafe')" width="180">
          <template #default="{ row }">
            <ElTag :type="stateType(stateOf(row))" size="small">{{ $t(`vnetPages.hubGames.states.${stateOf(row)}` as any) }}</ElTag>
            <span v-if="row.local?.status === 'downloading'" class="ml-6px text-12px">{{ row.local.progress }}%</span>
            <ElTooltip v-if="row.local?.status === 'error' && row.local.error" :content="row.local.error">
              <span class="ml-6px cursor-help text-12px">?</span>
            </ElTooltip>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.hubGames.autoUpdate')" width="110" align="center">
          <template #default="{ row }">
            <ElSwitch
              v-if="row.local"
              :model-value="row.local.auto_update"
              size="small"
              :disabled="!canEdit"
              @change="(v: any) => toggleAutoUpdate(row, !!v)"
            />
            <span v-else>-</span>
          </template>
        </ElTableColumn>
        <ElTableColumn v-if="canEdit" label="" width="230" fixed="right">
          <template #default="{ row }">
            <ElButton v-if="!row.selected && !row.mandatory" size="small" :disabled="busy" @click="choose([row.name], true)">
              {{ $t('vnetPages.hubGames.choose') }}
            </ElButton>
            <ElButton
              v-else-if="row.available && (!row.local || row.local.version === 0 || row.local.needs_update)"
              size="small"
              type="primary"
              plain
              :disabled="busy || row.local?.status === 'downloading'"
              @click="syncNow([row.name])"
            >
              {{ $t('vnetPages.hubGames.downloadNowOne') }}
            </ElButton>
            <ElButton
              v-if="row.available && row.local && row.local.version > 0"
              size="small"
              :disabled="busy || row.local.status === 'downloading'"
              @click="repair(row)"
            >
              {{ $t('vnetPages.hubGames.repair') }}
            </ElButton>
            <ElButton v-if="row.selected && !row.mandatory && mode === 'selected'" size="small" type="danger" plain :disabled="busy" @click="unchoose([row.name])">
              {{ $t('vnetPages.hubGames.unchooseOne') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
      <div class="mt-16px flex justify-end">
        <ElPagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          layout="total, sizes, prev, pager, next"
          :page-sizes="[25, 50, 100, 200]"
          :total="filtered.length"
        />
      </div>
    </ElCard>
  </div>
</template>
