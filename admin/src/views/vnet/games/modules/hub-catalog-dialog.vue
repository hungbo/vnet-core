<script setup lang="ts">
// Hộp "Tải game" của trang Game (thay trang Danh mục Hub cũ): mọi game Hub phát kèm tình trạng ở quán, hàng đợi tải và
// trần băng thông. Bấm Tải ở game chưa tải thì trang Game hỏi thư mục lưu rồi gọi POST /games/hub-download, máy chủ tự
// chọn game đó trên Hub nếu quán đang "chỉ nhận game đã chọn".
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAuth } from '@/hooks/business/auth';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { khongDau } from '@/utils/text';

defineOptions({ name: 'HubCatalogDialog' });

interface CatalogRow {
  name: string;
  display_name: string;
  category: string;
  size_bytes: number;
  approx_size_mb: number;
  available: boolean;
  version: number;
  mandatory: boolean;
  local_game?: boolean;
  local?: {
    id: string;
    status: string;
    progress: number;
    version: number;
    error: string;
    needs_update: boolean;
    chosen: boolean;
    install_root: string;
  };
}

interface QueueEntry {
  name: string;
  display_name: string;
  state: string;
  progress: number;
  bytes_per_sec: number;
  peers: number;
  retry_at?: string;
}

interface QueueView {
  current: QueueEntry | null;
  waiting: QueueEntry[];
  in_window: boolean;
  update_window: string;
}

const visible = defineModel<boolean>('visible', { default: false });
const emit = defineEmits<{ download: [row: CatalogRow] }>();

const { t: $t } = useI18n();
const { hasAuth } = useAuth();
const canEdit = computed(() => hasAuth('games.update'));

const loading = ref(false);
const rows = ref<CatalogRow[]>([]);
const errorText = ref('');
const queue = ref<QueueView | null>(null);

async function load() {
  loading.value = true;
  errorText.value = '';
  try {
    rows.value = ((await client.get('/games/hub-catalog')) as any)?.items || [];
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

// Chỉ hỏi Hub và hàng đợi khi hộp đang mở.
let queueTimer: ReturnType<typeof setInterval> | undefined;
watch(visible, open => {
  clearInterval(queueTimer);
  if (!open) return;
  load();
  loadQueue();
  loadLimits();
  queueTimer = setInterval(() => {
    if (!document.hidden) loadQueue();
  }, 3000);
});
onBeforeUnmount(() => clearInterval(queueTimer));
// Bấm Tải, tải xong hay lỗi đều phát "games:changed".
useRealtimeRefresh(['games'], () => {
  if (!visible.value) return;
  load();
  loadQueue();
});

// ---- lọc -------------------------------------------------------------------

type Tab = 'all' | 'downloaded' | 'downloading' | 'not_downloaded';
const tab = ref<Tab>('all');
const search = ref('');

// Đã tải: có trọn một bản trên ổ (hoặc quán có game cục bộ cùng tên). Đang tải: quán đã bấm Tải mà chưa xong, kể cả đang
// chờ lượt hay lỗi chờ thử lại. Còn lại là chưa tải.
function tabOf(r: CatalogRow): Exclude<Tab, 'all'> {
  if (r.local_game || (r.local && r.local.version > 0)) return 'downloaded';
  if (r.local && (r.local.chosen || ['downloading', 'queued'].includes(r.local.status))) return 'downloading';
  return 'not_downloaded';
}

const counts = computed(() => {
  const c: Record<Tab, number> = { all: rows.value.length, downloaded: 0, downloading: 0, not_downloaded: 0 };
  for (const r of rows.value) c[tabOf(r)] += 1;
  return c;
});

const shown = computed(() => {
  const q = khongDau(search.value.trim());
  return rows.value.filter(
    r =>
      (tab.value === 'all' || tabOf(r) === tab.value) &&
      (!q || khongDau(`${r.display_name} ${r.name} ${r.category}`).includes(q))
  );
});

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

// Nhãn ở cột "Ở quán": đang tải thì nói rõ đang tải, chờ lượt hay lỗi.
function stateOf(r: CatalogRow): { key: string; type: 'success' | 'primary' | 'warning' | 'danger' | 'info' } {
  if (r.local_game) return { key: 'local_game', type: 'success' };
  const t = tabOf(r);
  if (t === 'downloaded') return { key: 'downloaded', type: 'success' };
  if (t === 'not_downloaded') return { key: 'not_downloaded', type: 'info' };
  if (r.local?.status === 'downloading') return { key: 'downloading', type: 'primary' };
  if (r.local?.status === 'error') return { key: 'error', type: 'danger' };
  return { key: 'queued', type: 'warning' };
}

function formatTime(v?: string) {
  return v ? dayjs(v).format('HH:mm:ss') : '';
}
</script>

<template>
  <ElDialog v-model="visible" :title="$t('vnetPages.hubGames.title')" width="1040px" top="5vh">
    <div class="mb-12px rounded-4px bg-[var(--el-fill-color-light)] p-12px">
      <div v-if="queue?.current">
        <div class="mb-4px flex flex-wrap items-center justify-between gap-8px">
          <b>{{ queue.current.display_name }}</b>
          <span class="text-13px">
            {{ bitRate(queue.current.bytes_per_sec) }} ·
            {{ $t('vnetPages.hubGames.peers', { n: queue.current.peers }) }}
          </span>
        </div>
        <ElProgress :percentage="queue.current.progress" :stroke-width="12" />
      </div>
      <div v-else class="text-13px text-[var(--el-text-color-secondary)]">{{ $t('vnetPages.hubGames.idle') }}</div>

      <div v-if="queue" class="mt-8px flex flex-wrap items-center gap-6px text-13px">
        <span>{{ $t('vnetPages.hubGames.waiting', { n: queue.waiting.length }) }}</span>
        <ElTag v-if="queue.update_window" :type="queue.in_window ? 'success' : 'warning'" size="small">
          {{ $t('vnetPages.hubGames.window') }} {{ queue.update_window }}
        </ElTag>
        <ElTooltip
          v-for="w in queue.waiting.slice(0, 10)"
          :key="w.name"
          :content="
            w.retry_at
              ? `${$t('vnetPages.hubGames.queueStates.retry')} ${formatTime(w.retry_at)}`
              : $t(`vnetPages.hubGames.queueStates.${w.state}` as any)
          "
        >
          <ElTag :type="w.state === 'queued' ? 'primary' : w.state === 'retry' ? 'danger' : 'info'" size="small">
            {{ w.display_name }}
          </ElTag>
        </ElTooltip>
        <span v-if="queue.waiting.length > 10" class="text-12px">+{{ queue.waiting.length - 10 }}</span>
      </div>

      <div class="mt-8px flex flex-wrap items-center gap-8px">
        <span class="text-13px">{{ $t('vnetPages.hubGames.limitDown') }}</span>
        <ElInputNumber
          v-model="limits.download_mbps"
          :min="0"
          :max="100000"
          :step="10"
          size="small"
          :disabled="!canEdit"
        />
        <span class="text-13px">{{ $t('vnetPages.hubGames.limitUp') }}</span>
        <ElInputNumber
          v-model="limits.upload_mbps"
          :min="0"
          :max="100000"
          :step="10"
          size="small"
          :disabled="!canEdit"
        />
        <ElButton v-if="canEdit" size="small" type="primary" plain @click="saveLimits">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
        <ElTooltip :content="$t('vnetPages.hubGames.limitHint')">
          <span class="cursor-help text-12px text-[var(--el-text-color-secondary)]">?</span>
        </ElTooltip>
      </div>
    </div>

    <ElAlert v-if="errorText" type="error" :closable="false" class="mb-12px" :title="errorText" />

    <div class="mb-12px flex flex-wrap items-center gap-8px">
      <ElRadioGroup v-model="tab">
        <ElRadioButton v-for="k in ['all', 'downloaded', 'downloading', 'not_downloaded'] as Tab[]" :key="k" :value="k">
          {{ $t(`vnetPages.hubGames.tabs.${k}`) }} ({{ counts[k] }})
        </ElRadioButton>
      </ElRadioGroup>
      <ElInput
        v-model="search"
        :placeholder="$t('vnetPages.hubGames.searchPlaceholder')"
        :aria-label="$t('vnetPages.hubGames.searchPlaceholder')"
        clearable
        class="w-220px"
      />
      <ElButton :loading="loading" @click="load">{{ $t('common.refresh') }}</ElButton>
    </div>

    <ElTable v-loading="loading" :data="shown" row-key="name" height="52vh" class="w-full">
      <ElTableColumn
        prop="display_name"
        :label="$t('vnetPages.hubGames.game')"
        min-width="240"
        show-overflow-tooltip
        sortable
      >
        <template #default="{ row }">
          <span class="font-medium">{{ row.display_name }}</span>
          <ElTag v-if="row.mandatory" size="small" type="info" class="ml-6px">
            {{ $t('vnetPages.hubGames.mandatory') }}
          </ElTag>
        </template>
      </ElTableColumn>
      <ElTableColumn
        prop="category"
        :label="$t('vnetPages.hubGames.category')"
        width="130"
        show-overflow-tooltip
        sortable
      />
      <ElTableColumn
        prop="size_bytes"
        :label="$t('vnetPages.hubGames.size')"
        width="110"
        align="right"
        sortable
        :sort-by="(r: any) => (r.size_bytes > 0 ? r.size_bytes : r.approx_size_mb * 1024 * 1024)"
      >
        <template #default="{ row }">{{ formatSize(row.size_bytes, row.approx_size_mb) }}</template>
      </ElTableColumn>
      <ElTableColumn prop="available" :label="$t('vnetPages.hubGames.hubStatus')" width="140" sortable>
        <template #default="{ row }">
          <ElTag v-if="row.available" type="success" size="small">
            {{ $t('vnetPages.hubGames.available', { v: row.version }) }}
          </ElTag>
          <ElTag v-else type="warning" size="small">{{ $t('vnetPages.hubGames.unavailable') }}</ElTag>
        </template>
      </ElTableColumn>
      <ElTableColumn :label="$t('vnetPages.hubGames.atCafe')" width="190" sortable :sort-by="(r: any) => tabOf(r)">
        <template #default="{ row }">
          <ElTag :type="stateOf(row).type" size="small">
            {{ $t(`vnetPages.hubGames.states.${stateOf(row).key}` as any) }}
          </ElTag>
          <span v-if="row.local?.status === 'downloading'" class="ml-6px text-12px">{{ row.local.progress }}%</span>
          <ElTag v-if="row.local?.needs_update && row.local.version > 0" type="warning" size="small" class="ml-6px">
            {{ $t('vnetPages.hubGames.states.needs_update') }}
          </ElTag>
          <ElTooltip v-if="row.local?.status === 'error' && row.local.error" :content="row.local.error">
            <span class="ml-6px cursor-help text-12px">?</span>
          </ElTooltip>
        </template>
      </ElTableColumn>
      <ElTableColumn v-if="canEdit" :label="$t('vnetPages.common.action')" width="100" fixed="right">
        <template #default="{ row }">
          <ElButton
            v-if="tabOf(row as CatalogRow) === 'not_downloaded' && !row.local_game"
            size="small"
            type="primary"
            :disabled="!row.available"
            @click="emit('download', row as CatalogRow)"
          >
            {{ $t('vnetPages.hubGames.download') }}
          </ElButton>
        </template>
      </ElTableColumn>
    </ElTable>
    <div class="mt-8px text-12px text-[var(--el-text-color-secondary)]">{{ $t('vnetPages.hubGames.hint') }}</div>
  </ElDialog>
</template>
