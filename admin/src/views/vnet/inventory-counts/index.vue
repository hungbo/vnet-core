<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const loading = ref(false);
const sessions = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

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
      params: { page: page.value, page_size: pageSize.value }
    });
    sessions.value = res?.items || [];
    total.value = res?.total || 0;
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
    const [d, p]: any[] = await Promise.all([
      client.get(`/inventory-counts/${row.id}`),
      client.get('/products', { params: { page_size: 500 } })
    ]);
    detail.value = d;
    const list: any[] = Array.isArray(p) ? p : p?.items || [];
    // Chỉ mặt hàng có theo dõi tồn kho mới kiểm kê được — backend cũng chặn,
    // nhưng để lọt vào danh sách chọn là bẫy người dùng.
    products.value = list.filter(x => x.has_stock);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    detailVisible.value = false;
  } finally {
    detailLoading.value = false;
  }
}

async function refreshDetail() {
  detail.value = await client.get(`/inventory-counts/${detail.value.id}`);
}

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

onMounted(load);
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.counts.hint') }}</span>
          <ElButton type="primary" @click="openSession">{{ $t('vnetPages.counts.open') }}</ElButton>
        </div>
      </template>

      <ElTable v-loading="loading" :data="sessions" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.counts.code')" width="140">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace">{{ row.code }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.counts.statusLabel')" width="130">
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
        <ElTableColumn :label="$t('vnetPages.counts.note')" min-width="180">
          <template #default="{ row }">{{ row.note || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.counts.openedAt')" width="160">
          <template #default="{ row }">{{ formatDate(row.opened_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.counts.committedAt')" width="160">
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
        <div v-if="isOpen" style="display: flex; gap: 8px; margin-bottom: 16px">
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
          <ElTableColumn prop="product_name" :label="$t('vnetPages.counts.product')" min-width="180" />
          <ElTableColumn :label="$t('vnetPages.counts.expected')" width="110">
            <template #default="{ row }">{{ row.expected_qty }}</template>
          </ElTableColumn>
          <ElTableColumn :label="$t('vnetPages.counts.actual')" width="110">
            <template #default="{ row }">{{ row.actual_qty }}</template>
          </ElTableColumn>
          <ElTableColumn :label="$t('vnetPages.counts.difference')" width="120">
            <template #default="{ row }">
              <span v-if="row.difference_qty === 0" style="color: #909399">0</span>
              <span v-else :style="`color:${row.difference_qty < 0 ? '#f56c6c' : '#67c23a'};font-weight:600`">
                {{ row.difference_qty > 0 ? '+' : '' }}{{ row.difference_qty }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn v-if="isOpen" :label="$t('vnetPages.common.action')" width="90">
            <template #default="{ row }">
              <ElButton size="small" type="danger" plain @click="removeLine(row)">
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
        <ElButton v-if="isOpen" type="danger" plain @click="cancelSession">
          {{ $t('vnetPages.counts.cancelSession') }}
        </ElButton>
        <ElButton @click="detailVisible = false">{{ $t('common.close') }}</ElButton>
        <ElButton v-if="isOpen" type="primary" @click="commit">{{ $t('vnetPages.counts.commit') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
