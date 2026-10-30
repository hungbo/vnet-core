<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
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

const tab = ref<'security' | 'activity' | 'changes'>('security');

function formatTime(v?: string) {
  return v ? dayjs(v).format('DD/MM/YYYY HH:mm:ss') : '';
}

// ---- Tab Boot và bảo mật -------------------------------------------------

interface HardwareRow {
  id: string;
  machine_code: string;
  status: string;
  cpu_name: string;
  gpu_name: string;
  ram_gb: number;
  storage_gb: number;
  boot_mode: string;
  secure_boot: string;
  tpm: string;
  hvci: string;
  virtualization: string;
  bios_version: string;
  display_mode: string;
  pending_changes: number;
}

const rows = ref<HardwareRow[]>([]);
const rowsLoading = ref(false);
const search = ref('');
const onlyNotReady = ref(false);

async function getRows() {
  rowsLoading.value = true;
  try {
    rows.value = (await client.get('/machine-hardware/overview')) as unknown as HardwareRow[];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.hardware.loadFailed'));
  } finally {
    rowsLoading.value = false;
  }
}

// Valorant (Vanguard) trên Windows 11 đòi UEFI + Secure Boot + TPM 2.0. Thiếu một trong ba
// là báo lỗi khi mở game, và nhân viên chỉ biết sau khi khách phàn nàn. "Chưa đo" khi máy
// trạm chưa báo đủ ba giá trị (bản cũ, hoặc đọc không được): không gọi là "chưa sẵn sàng"
// khi chưa biết.
type Readiness = { state: 'ready' | 'notReady' | 'unknown'; reasons: string[] };

// row: any — ElTable truyền DefaultRow (Record<string, any>) vào slot.
function readiness(row: any): Readiness {
  if (!row.boot_mode || !row.secure_boot || !row.tpm) return { state: 'unknown', reasons: [] };
  const reasons: string[] = [];
  if (row.boot_mode !== 'uefi') {
    reasons.push('legacy');
  } else if (row.secure_boot !== 'on') {
    reasons.push('secureBoot');
  }
  if (!row.tpm.startsWith('2')) reasons.push('tpm');
  return { state: reasons.length ? 'notReady' : 'ready', reasons };
}

const summary = computed(() => {
  const s = { ready: 0, notReady: 0, unknown: 0 };
  for (const r of rows.value) s[readiness(r).state] += 1;
  return s;
});

const shownRows = computed(() => {
  const q = search.value.trim().toLowerCase();
  return rows.value.filter(r => {
    if (q && !r.machine_code.toLowerCase().includes(q)) return false;
    if (onlyNotReady.value && readiness(r).state !== 'notReady') return false;
    return true;
  });
});

const pendingTotal = computed(() => rows.value.reduce((n, r) => n + (r.pending_changes || 0), 0));

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

// on/off/uefi/... → thẻ màu. good: giá trị tốt cho game chống gian lận.
function flagType(value: string, good: string): any {
  if (!value) return 'info';
  return value === good ? 'success' : 'warning';
}

function flagText(value: string) {
  if (!value) return '-';
  const key = `vnetPages.hardware.values.${value}`;
  const text = $t(key as any);
  return text === key ? value : text;
}

function tpmType(tpm: string): any {
  if (!tpm) return 'info';
  return tpm.startsWith('2') ? 'success' : 'warning';
}

// ---- Tab Hoạt động -------------------------------------------------------

interface ActivityRow {
  id: string;
  machine_code: string;
  status: string;
  last_heartbeat: string | null;
  active_app: string;
  cpu_temp: number;
  gpu_temp: number;
  cpu_usage: number;
  ram_usage: number;
  disk_usage: number;
  net_down_bps: number;
  net_up_bps: number;
  ping_ms: number;
}

const activity = ref<ActivityRow[]>([]);
const activityLoading = ref(false);
const activitySearch = ref('');

async function getActivity() {
  activityLoading.value = true;
  try {
    activity.value = (await client.get('/machine-hardware/activity')) as unknown as ActivityRow[];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.hardware.loadFailed'));
  } finally {
    activityLoading.value = false;
  }
}

const shownActivity = computed(() => {
  const q = activitySearch.value.trim().toLowerCase();
  return activity.value.filter(r => !q || r.machine_code.toLowerCase().includes(q));
});

// Máy tắt thì số đo cuối cùng là số cũ: hiện "-" thay vì để chủ quán tưởng đang chạy.
function live(row: any) {
  return row.status !== 'offline';
}

// byte/giây → Mbps hoặc kbps, như cách quán nói về đường truyền.
function bitRate(bps: number) {
  const bits = bps * 8;
  if (bits >= 1_000_000) return `${(bits / 1_000_000).toFixed(1)} Mbps`;
  return `${Math.round(bits / 1000)} kbps`;
}

function usageType(v: number): any {
  if (v >= 90) return 'danger';
  if (v >= 75) return 'warning';
  return 'success';
}

function pingType(ms: number): any {
  if (ms >= 100) return 'danger';
  if (ms >= 30) return 'warning';
  return 'success';
}

// Số đo đổi mỗi nhịp tim (15 giây) nhưng không qua DataChanged (middleware bỏ route nhịp tim,
// không thì mọi trang tải lại liên tục), nên tab này hỏi lại định kỳ khi đang mở.
let activityTimer: ReturnType<typeof setInterval> | undefined;

// ---- Tab Thay đổi --------------------------------------------------------

const onlyPending = ref(true);
const filterComponent = ref<string | null>(null);
const machineSearch = ref('');

const componentKeys = [
  'cpu',
  'gpu',
  'ram',
  'storage',
  'monitor',
  'keyboard',
  'mouse',
  'sound',
  'display',
  'boot_mode',
  'secure_boot',
  'tpm',
  'hvci',
  'virtualization',
  'bios'
];

function componentLabel(key: string) {
  const k = `vnetPages.hardware.components.${key}`;
  const text = $t(k as any);
  return text === k ? key : text;
}

// Thiết lập Boot đổi giữa các giá trị cố định (on/off/uefi…): hiện bằng chữ của giao diện
// như ở bảng bên kia, không phải mã thô.
const flagComponents = new Set(['boot_mode', 'secure_boot', 'tpm', 'hvci', 'virtualization']);

function changeText(row: any) {
  const show = (v: string) => (flagComponents.has(row.component) ? flagText(v) : v);
  if (row.action === 'added') return `+ ${row.new_value}`;
  if (row.action === 'removed') return `− ${row.old_value}`;
  return `${show(row.old_value)} → ${show(row.new_value)}`;
}

const { columns, columnChecks, data, getData, loading, mobilePagination, onSortChange } = useUIPaginatedTable({
  // Hook gọi api({ page, pageSize, sort, order }) nhưng khai kiểu là () => …; tham số tuỳ chọn khớp được cả hai.
  api: (p?: { page: number; pageSize: number; sort?: string; order?: string }) =>
    client.get('/machine-hardware/changes', {
      params: {
        page: p?.page,
        page_size: p?.pageSize,
        sort: p?.sort,
        order: p?.order,
        unacknowledged: onlyPending.value ? 'true' : undefined,
        component: filterComponent.value || undefined,
        search: machineSearch.value.trim() || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    {
      prop: 'detected_at',
      label: $t('vnetPages.hardware.detectedAt'),
      width: 170,
      sortable: 'custom',
      formatter: (row: any) => formatTime(row.detected_at)
    },
    { prop: 'machine_code', label: $t('vnetPages.hardware.machine'), width: 150, sortable: 'custom' },
    {
      prop: 'component',
      label: $t('vnetPages.hardware.component'),
      width: 150,
      sortable: 'custom',
      formatter: (row: any) => componentLabel(row.component)
    },
    { prop: 'change', label: $t('vnetPages.hardware.change'), minWidth: 280, showOverflowTooltip: true },
    { prop: 'acknowledged', label: $t('vnetPages.hardware.acknowledged'), width: 190, sortable: 'custom' },
    { prop: 'actions', label: '', width: 110, fixed: 'right' }
  ]
});

async function ack(body: { ids?: string[]; all?: boolean }) {
  try {
    await client.post('/machine-hardware/changes/ack', body);
    ElMessage.success($t('vnetPages.hardware.ackSuccess'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.hardware.ackFailed'));
  }
}

// Máy trạm báo thay đổi qua nhịp tim (hub.Changed "machine-hardware"); xác nhận qua
// POST /machine-hardware/... cũng phát cùng chủ đề (middleware DataChanged), nên mọi
// tab quản trị tự tải lại — không cần gọi lại thủ công sau khi xác nhận.
useRealtimeRefresh(['machine-hardware', 'machines'], () => {
  getRows();
  getData();
  if (tab.value === 'activity') getActivity();
});

onMounted(() => {
  getRows();
  getActivity();
  activityTimer = setInterval(() => {
    if (tab.value === 'activity' && !document.hidden) getActivity();
  }, 15000);
});

onBeforeUnmount(() => {
  if (activityTimer) clearInterval(activityTimer);
});
</script>

<template>
  <div>
    <ElCard>
      <ElTabs v-model="tab">
        <ElTabPane name="security" :label="$t('vnetPages.hardware.tabSecurity')">
          <div class="mb-12px flex flex-wrap items-center gap-8px">
            <ElInput
              v-model="search"
              :placeholder="$t('vnetPages.hardware.searchPlaceholder')"
              clearable
              style="width: 260px"
            />
            <ElCheckbox v-model="onlyNotReady">{{ $t('vnetPages.hardware.onlyNotReady') }}</ElCheckbox>
            <ElButton :loading="rowsLoading" @click="getRows">{{ $t('common.refresh') }}</ElButton>
            <span class="ml-auto text-13px">
              <ElTag type="success" size="small">{{ $t('vnetPages.hardware.ready') }} {{ summary.ready }}</ElTag>
              <ElTag type="warning" size="small" class="ml-4px">
                {{ $t('vnetPages.hardware.notReady') }} {{ summary.notReady }}
              </ElTag>
              <ElTag type="info" size="small" class="ml-4px">
                {{ $t('vnetPages.hardware.unknown') }} {{ summary.unknown }}
              </ElTag>
            </span>
          </div>
          <ElAlert type="info" :closable="false" class="mb-12px" :title="$t('vnetPages.hardware.vanguardHint')" />
          <ElTable v-loading="rowsLoading" :data="shownRows" row-key="id" style="width: 100%">
            <ElTableColumn prop="machine_code" :label="$t('vnetPages.hardware.machine')" width="150" fixed sortable />
            <ElTableColumn prop="status" :label="$t('vnetPages.hardware.status')" width="110" sortable>
              <template #default="{ row }">
                <ElTag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.hardware.vanguard')" width="170">
              <template #default="{ row }">
                <ElTooltip
                  v-if="readiness(row).state === 'notReady'"
                  :content="readiness(row).reasons.map(r => $t(`vnetPages.hardware.reasons.${r}` as any)).join('; ')"
                >
                  <ElTag type="warning" size="small">{{ $t('vnetPages.hardware.notReady') }}</ElTag>
                </ElTooltip>
                <ElTag v-else-if="readiness(row).state === 'ready'" type="success" size="small">
                  {{ $t('vnetPages.hardware.ready') }}
                </ElTag>
                <ElTag v-else type="info" size="small">{{ $t('vnetPages.hardware.unknown') }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="boot_mode" :label="$t('vnetPages.hardware.components.boot_mode')" width="110" sortable>
              <template #default="{ row }">
                <ElTag :type="flagType(row.boot_mode, 'uefi')" size="small">{{ flagText(row.boot_mode) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn
              prop="secure_boot"
              :label="$t('vnetPages.hardware.components.secure_boot')"
              width="120"
              sortable
            >
              <template #default="{ row }">
                <ElTag :type="flagType(row.secure_boot, 'on')" size="small">{{ flagText(row.secure_boot) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="tpm" :label="$t('vnetPages.hardware.components.tpm')" width="100" sortable>
              <template #default="{ row }">
                <ElTag :type="tpmType(row.tpm)" size="small">{{ flagText(row.tpm) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="hvci" :label="$t('vnetPages.hardware.components.hvci')" width="120" sortable>
              <template #default="{ row }">
                <ElTag type="info" size="small">{{ flagText(row.hvci) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn
              prop="virtualization"
              :label="$t('vnetPages.hardware.components.virtualization')"
              width="120"
              sortable
            >
              <template #default="{ row }">
                <ElTag type="info" size="small">{{ flagText(row.virtualization) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn
              prop="bios_version"
              :label="$t('vnetPages.hardware.components.bios')"
              width="140"
              show-overflow-tooltip
              sortable
            />
            <ElTableColumn prop="display_mode" :label="$t('vnetPages.hardware.display')" width="150" sortable />
            <ElTableColumn
              prop="pending_changes"
              :label="$t('vnetPages.hardware.pending')"
              width="120"
              fixed="right"
              sortable
            >
              <template #default="{ row }">
                <ElButton v-if="row.pending_changes" size="small" type="warning" plain @click="tab = 'changes'">
                  {{ row.pending_changes }}
                </ElButton>
                <span v-else>-</span>
              </template>
            </ElTableColumn>
          </ElTable>
        </ElTabPane>

        <ElTabPane name="activity" :label="$t('vnetPages.hardware.tabActivity')">
          <div class="mb-12px flex flex-wrap items-center gap-8px">
            <ElInput
              v-model="activitySearch"
              :placeholder="$t('vnetPages.hardware.searchPlaceholder')"
              clearable
              style="width: 260px"
            />
            <ElButton :loading="activityLoading" @click="getActivity">{{ $t('common.refresh') }}</ElButton>
          </div>
          <ElAlert type="info" :closable="false" class="mb-12px" :title="$t('vnetPages.hardware.activityHint')" />
          <ElTable v-loading="activityLoading && !activity.length" :data="shownActivity" row-key="id" style="width: 100%">
            <ElTableColumn prop="machine_code" :label="$t('vnetPages.hardware.machine')" width="150" fixed sortable />
            <ElTableColumn prop="status" :label="$t('vnetPages.hardware.status')" width="110" sortable>
              <template #default="{ row }">
                <ElTag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn
              prop="active_app"
              :label="$t('vnetPages.hardware.app')"
              min-width="220"
              show-overflow-tooltip
              sortable
            >
              <template #default="{ row }">{{ live(row) && row.active_app ? row.active_app : '-' }}</template>
            </ElTableColumn>
            <ElTableColumn prop="cpu_temp" :label="$t('vnetPages.hardware.temp')" width="150" sortable>
              <template #default="{ row }">
                {{ live(row) && (row.cpu_temp || row.gpu_temp) ? `${row.cpu_temp}° / ${row.gpu_temp}°` : '-' }}
              </template>
            </ElTableColumn>
            <ElTableColumn prop="cpu_usage" :label="$t('vnetPages.hardware.cpuLoad')" width="100" sortable>
              <template #default="{ row }">
                <ElTag v-if="live(row)" :type="usageType(row.cpu_usage)" size="small">{{ Math.round(row.cpu_usage) }}%</ElTag>
                <span v-else>-</span>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="ram_usage" :label="$t('vnetPages.hardware.ramLoad')" width="100" sortable>
              <template #default="{ row }">
                <ElTag v-if="live(row)" :type="usageType(row.ram_usage)" size="small">{{ Math.round(row.ram_usage) }}%</ElTag>
                <span v-else>-</span>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="disk_usage" :label="$t('vnetPages.hardware.diskLoad')" width="100" sortable>
              <template #default="{ row }">
                <ElTag v-if="live(row)" :type="usageType(row.disk_usage)" size="small">
                  {{ Math.round(row.disk_usage) }}%
                </ElTag>
                <span v-else>-</span>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="net_down_bps" :label="$t('vnetPages.hardware.down')" width="120" sortable>
              <template #default="{ row }">{{ live(row) ? bitRate(row.net_down_bps) : '-' }}</template>
            </ElTableColumn>
            <ElTableColumn prop="net_up_bps" :label="$t('vnetPages.hardware.up')" width="120" sortable>
              <template #default="{ row }">{{ live(row) ? bitRate(row.net_up_bps) : '-' }}</template>
            </ElTableColumn>
            <ElTableColumn prop="ping_ms" :label="$t('vnetPages.hardware.ping')" width="140" sortable>
              <template #default="{ row }">
                <ElTag v-if="live(row) && row.ping_ms" :type="pingType(row.ping_ms)" size="small">{{ row.ping_ms }} ms</ElTag>
                <span v-else>-</span>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="last_heartbeat" :label="$t('vnetPages.hardware.lastSeen')" width="170" sortable>
              <template #default="{ row }">{{ formatTime(row.last_heartbeat || undefined) || '-' }}</template>
            </ElTableColumn>
          </ElTable>
        </ElTabPane>

        <ElTabPane name="changes">
          <template #label>
            <ElBadge :value="pendingTotal" :hidden="!pendingTotal" :offset="[8, 0]">
              {{ $t('vnetPages.hardware.tabChanges') }}
            </ElBadge>
          </template>
          <div class="mb-12px flex flex-wrap items-center justify-between gap-8px">
            <span class="text-13px text-[var(--el-text-color-secondary)]">{{ $t('vnetPages.hardware.changesHint') }}</span>
            <TableHeaderOperation
              v-model:columns="columnChecks"
              :loading="loading"
              :show-add="false"
              :show-delete="false"
              @refresh="getData"
            >
              <template #prefix>
                <ElInput
                  v-model="machineSearch"
                  :placeholder="$t('vnetPages.hardware.searchPlaceholder')"
                  clearable
                  style="width: 200px"
                  @keyup.enter="getData"
                  @clear="getData"
                />
                <ElSelect
                  v-model="filterComponent"
                  :placeholder="$t('vnetPages.hardware.component')"
                  clearable
                  style="width: 160px"
                  @change="getData"
                >
                  <ElOption v-for="k in componentKeys" :key="k" :label="componentLabel(k)" :value="k" />
                </ElSelect>
                <ElCheckbox v-model="onlyPending" @change="getData">{{ $t('vnetPages.hardware.onlyPending') }}</ElCheckbox>
                <ElButton
                  v-if="hasAuth('machines.update') && pendingTotal"
                  type="primary"
                  plain
                  @click="ack({ all: true })"
                >
                  {{ $t('vnetPages.hardware.ackAll') }}
                </ElButton>
              </template>
            </TableHeaderOperation>
          </div>
          <ElTable v-loading="loading" :data="data" row-key="id" style="width: 100%" @sort-change="onSortChange">
            <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col">
              <template v-if="col.prop === 'change'" #default="{ row }">
                <ElTag
                  :type="row.action === 'added' ? 'success' : row.action === 'removed' ? 'danger' : 'warning'"
                  size="small"
                  class="mr-6px"
                >
                  {{ $t(`vnetPages.hardware.actions.${row.action}` as any) }}
                </ElTag>
                {{ changeText(row) }}
              </template>
              <template v-else-if="col.prop === 'acknowledged'" #default="{ row }">
                <span v-if="row.acknowledged_at" class="text-12px">
                  {{ row.acknowledged_by }} · {{ formatTime(row.acknowledged_at) }}
                </span>
                <ElTag v-else type="warning" size="small">{{ $t('vnetPages.hardware.notAcknowledged') }}</ElTag>
              </template>
              <template v-else-if="col.prop === 'actions'" #default="{ row }">
                <ElButton
                  v-if="!row.acknowledged_at && hasAuth('machines.update')"
                  size="small"
                  type="success"
                  plain
                  @click="ack({ ids: [row.id] })"
                >
                  {{ $t('vnetPages.hardware.ack') }}
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
        </ElTabPane>
      </ElTabs>
    </ElCard>
  </div>
</template>
