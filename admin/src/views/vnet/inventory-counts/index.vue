<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useFitPageSize } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();

const loading = ref(false);
const sessions = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
// Số dòng/trang vừa khít chiều cao bảng (xem useFitPageSize).
const fitter = useFitPageSize({
  getPageSize: () => pageSize.value,
  setPageSize: size => {
    pageSize.value = size;
    page.value = 1;
    load();
  }
});
// Sắp xếp phía server: bấm tiêu đề cột thì sắp trên toàn bộ phiên, không chỉ trang đang xem.
const sort = ref<string>();
const order = ref<'asc' | 'desc'>();

function onSortChange({ prop, order: o }: { prop: string | null; order: 'ascending' | 'descending' | null }) {
  sort.value = o && prop ? prop : undefined;
  order.value = o ? ({ ascending: 'asc', descending: 'desc' } as const)[o] : undefined;
  page.value = 1;
  load();
}

function formatDate(v: string | null | undefined) {
  return v ? dayjs(v).format('DD/MM/YYYY HH:mm') : '-';
}

function statusTag(status: string) {
  const map: Record<string, 'warning' | 'success' | 'info'> = {
    open: 'warning',
    committed: 'success',
    cancelled: 'info'
  };
  return h(ElTag, { type: map[status] || 'info', size: 'small' }, () => $t(`vnetPages.counts.status.${status}`));
}

async function load() {
  loading.value = true;
  try {
    const res: any = await client.get('/inventory-counts', {
      params: { page: page.value, page_size: pageSize.value, sort: sort.value, order: order.value }
    });
    sessions.value = res?.items || [];
    total.value = res?.total || 0;
    await fitter.afterLoad();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    sessions.value = [];
  } finally {
    loading.value = false;
  }
}

async function openSession() {
  try {
    const { value } = await ElMessageBox.prompt($t('vnetPages.counts.openPrompt'), $t('vnetPages.counts.open'), {
      inputPlaceholder: $t('vnetPages.counts.notePlaceholder'),
      inputValue: ''
    });
    const res: any = await client.post('/inventory-counts', {
      note: value || ''
    });
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.counts.opened', { code: res.code })
    });
    load();
    openDetail(res);
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message);
  }
}

// --- Đếm trong phiên ---------------------------------------------------------

const detailVisible = ref(false);
const detailLoading = ref(false);
const detail = ref<any>(null);
const products = ref<any[]>([]);
const pickProduct = ref('');
const pickQty = ref(0);

const isOpen = computed(() => detail.value?.status === 'open');

async function openDetail(row: any) {
  detailVisible.value = true;
  detailLoading.value = true;
  pickProduct.value = '';
  pickQty.value = 0;
  try {
    const [d]: any[] = await Promise.all([client.get(`/inventory-counts/${row.id}`), loadPickProducts()]);
    detail.value = d;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    detailVisible.value = false;
  } finally {
    detailLoading.value = false;
  }
}

async function loadPickProducts() {
  const p: any = await client.get('/products', { params: { page_size: 500 } });
  const list: any[] = Array.isArray(p) ? p : p?.items || [];
  // Chỉ mặt hàng có theo dõi tồn kho mới kiểm kê được — backend cũng chặn,
  // nhưng để lọt vào danh sách chọn là bẫy người dùng.
  products.value = list.filter(x => x.has_stock);
}

async function refreshDetail() {
  detail.value = await client.get(`/inventory-counts/${detail.value.id}`);
}

// Một phiên có thể nhiều người cùng đếm trên nhiều máy, và chốt/huỷ ở máy khác:
// danh sách và phiên đang mở phải theo kịp. Chỉ ghi đè khi vẫn đang xem đúng
// phiên đó — trong lúc chờ mạng, người dùng có thể đã mở phiên khác.
useRealtimeRefresh(['inventory-counts'], async () => {
  load();
  const id = detail.value?.id;
  if (!detailVisible.value || !id) return;
  try {
    const d: any = await client.get(`/inventory-counts/${id}`);
    if (detailVisible.value && detail.value?.id === id) detail.value = d;
  } catch {}
});

// Ô chọn mặt hàng ghi kèm tồn sổ sách, mà quán vẫn bán trong lúc đếm. Chỉ tải
// lại khi đang đếm dở một phiên — danh sách 500 món không rẻ, và đóng hộp rồi
// mở lại thì openDetail đã tự tải.
useRealtimeRefresh(
  ['products'],
  () => {
    if (detailVisible.value && isOpen.value) loadPickProducts().catch(() => {});
  },
  { events: ['stock:changed'], delay: 2000 }
);

async function addLine() {
  if (!pickProduct.value) {
    ElMessage.warning($t('vnetPages.counts.pickProduct'));
    return;
  }
  try {
    await client.post(`/inventory-counts/${detail.value.id}/lines`, {
      product_id: pickProduct.value,
      actual_qty: pickQty.value
    });
    pickProduct.value = '';
    pickQty.value = 0;
    await refreshDetail();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function removeLine(line: any) {
  try {
    await client.delete(`/inventory-counts/${detail.value.id}/lines/${line.product_id}`);
    await refreshDetail();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function commit() {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.counts.commitConfirm', { count: detail.value.diff_count }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    const res: any = await client.post(`/inventory-counts/${detail.value.id}/commit`);
    detail.value = res;
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.counts.committed')
    });
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function cancelSession() {
  try {
    await ElMessageBox.confirm($t('vnetPages.counts.cancelConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return;
  }
  try {
    await client.post(`/inventory-counts/${detail.value.id}/cancel`);
    detailVisible.value = false;
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

onMounted(() => {
  if (!fitter.fit()) load();
});
</script>

<template>
  <div class="page-fill">
    <ElCard class="fill-card">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.counts.hint') }}</span>
          <ElButton v-if="hasAuth('inventory_counts.open')" type="primary" @click="openSession">
            {{ $t('vnetPages.counts.open') }}
          </ElButton>
        </div>
      </template>

      <ElTable
        v-loading="loading"
        :data="sessions"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn prop="code" :label="$t('vnetPages.counts.code')" width="140" sortable="custom">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace">{{ row.code }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="status" :label="$t('vnetPages.counts.statusLabel')" width="130" sortable="custom">
          <template #default="{ row }"><component :is="statusTag(row.status)" /></template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.counts.lines')" width="110">
          <template #default="{ row }">{{ row.line_count }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.counts.diffs')" width="120">
          <template #default="{ row }">
            <span :style="row.diff_count ? 'color:#e6a23c;font-weight:600' : ''">{{ row.diff_count }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="note" :label="$t('vnetPages.counts.note')" min-width="180" sortable="custom">
          <template #default="{ row }">{{ row.note || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="opened_at" :label="$t('vnetPages.counts.openedAt')" width="160" sortable="custom">
          <template #default="{ row }">{{ formatDate(row.opened_at) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="committed_at" :label="$t('vnetPages.counts.committedAt')" width="160" sortable="custom">
          <template #default="{ row }">{{ formatDate(row.committed_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="110" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openDetail(row)">
              {{ row.status === 'open' ? $t('vnetPages.counts.count') : $t('vnetPages.common.detail') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="mt-16px flex justify-end">
        <ElPagination
          v-if="total"
          v-model:current-page="page"
          v-model:page-size="pageSize"
          layout="total, prev, pager, next"
          :total="total"
          @current-change="load"
        />
      </div>
    </ElCard>

    <ElDialog
      v-model="detailVisible"
      :title="$t('vnetPages.counts.sessionTitle', { code: detail?.code })"
      width="760px"
    >
      <div v-loading="detailLoading">
        <div v-if="isOpen && hasAuth('inventory_counts.count')" style="display: flex; gap: 8px; margin-bottom: 16px">
          <ElSelect v-model="pickProduct" filterable :placeholder="$t('vnetPages.counts.pickProduct')" style="flex: 1">
            <ElOption
              v-for="p in products"
              :key="p.id"
              :label="`${p.name} — ${$t('vnetPages.counts.book')} ${p.current_stock}`"
              :value="p.id"
            />
          </ElSelect>
          <ElInputNumber v-model="pickQty" :min="0" :precision="3" :step="1" style="width: 160px" />
          <ElButton type="primary" @click="addLine">{{ $t('vnetPages.counts.addLine') }}</ElButton>
        </div>

        <ElTable :data="detail?.lines || []" border stripe size="small" style="width: 100%">
          <ElTableColumn prop="product_name" :label="$t('vnetPages.counts.product')" min-width="180" sortable />
          <ElTableColumn prop="expected_qty" :label="$t('vnetPages.counts.expected')" width="110" sortable>
            <template #default="{ row }">{{ row.expected_qty }}</template>
          </ElTableColumn>
          <ElTableColumn prop="actual_qty" :label="$t('vnetPages.counts.actual')" width="110" sortable>
            <template #default="{ row }">{{ row.actual_qty }}</template>
          </ElTableColumn>
          <ElTableColumn prop="difference_qty" :label="$t('vnetPages.counts.difference')" width="120" sortable>
            <template #default="{ row }">
              <span v-if="row.difference_qty === 0" style="color: #909399">0</span>
              <span v-else :style="`color:${row.difference_qty < 0 ? '#f56c6c' : '#67c23a'};font-weight:600`">
                {{ row.difference_qty > 0 ? '+' : '' }}{{ row.difference_qty }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn v-if="isOpen" :label="$t('vnetPages.common.action')" width="90">
            <template #default="{ row }">
              <ElButton
                v-if="hasAuth('inventory_counts.count')"
                size="small"
                type="danger"
                plain
                @click="removeLine(row)"
              >
                {{ $t('vnetPages.counts.remove') }}
              </ElButton>
            </template>
          </ElTableColumn>
        </ElTable>

        <div v-if="detail" style="margin-top: 16px; color: #606266; font-size: 13px">
          {{
            $t('vnetPages.counts.summary', {
              lines: detail.line_count,
              diffs: detail.diff_count,
              short: detail.total_short,
              over: detail.total_over
            })
          }}
        </div>
        <ElAlert v-if="isOpen" type="info" :closable="false" show-icon style="margin-top: 12px">
          {{ $t('vnetPages.counts.deltaNote') }}
        </ElAlert>
      </div>
      <template #footer>
        <ElButton v-if="isOpen && hasAuth('inventory_counts.cancel')" type="danger" plain @click="cancelSession">
          {{ $t('vnetPages.counts.cancelSession') }}
        </ElButton>
        <ElButton @click="detailVisible = false">{{ $t('common.close') }}</ElButton>
        <ElButton v-if="isOpen && hasAuth('inventory_counts.commit')" type="primary" @click="commit">
          {{ $t('vnetPages.counts.commit') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
