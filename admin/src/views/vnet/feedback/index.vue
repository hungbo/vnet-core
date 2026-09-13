<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const loading = ref(false);
const rows = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

const machines = ref<any[]>([]);
const machineFilter = ref('');
const ratingFilter = ref<number | ''>('');
const onlyWithContent = ref(false);

const summary = ref<any>({
  total: 0,
  average: 0,
  distribution: {},
  with_content: 0
});

// Tỉ lệ để vẽ thanh phân bố. Chia cho mức CAO NHẤT chứ không phải tổng: chia
// cho tổng thì khi điểm dồn vào một mức, các mức khác thành sợi chỉ không đọc được.
const maxCount = computed(() => {
  const vals = Object.values(summary.value.distribution || {}) as number[];
  return Math.max(1, ...vals);
});

function starColor(star: number) {
  if (star >= 4) return '#67c23a';
  if (star === 3) return '#e6a23c';
  return '#f56c6c';
}

async function load() {
  loading.value = true;
  try {
    const params: any = {
      page: page.value,
      page_size: pageSize.value,
      machine_id: machineFilter.value || undefined,
      with_content: onlyWithContent.value || undefined
    };
    if (ratingFilter.value !== '') {
      params.min_rating = ratingFilter.value;
      params.max_rating = ratingFilter.value;
    }
    const [list, sum]: any[] = await Promise.all([
      client.get('/feedback', { params }),
      client.get('/feedback/summary', {
        params: { machine_id: machineFilter.value || undefined }
      })
    ]);
    rows.value = list?.items || [];
    total.value = list?.total || 0;
    summary.value = sum || summary.value;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

async function loadMachines() {
  try {
    const res: any = await client.get('/machines', {
      params: { page_size: 500 }
    });
    machines.value = Array.isArray(res) ? res : res?.items || [];
  } catch {
    machines.value = [];
  }
}

onMounted(() => {
  load();
  loadMachines();
});
</script>

<template>
  <div>
    <ElCard style="margin-bottom: 16px">
      <div style="display: flex; gap: 40px; align-items: center; flex-wrap: wrap">
        <div style="text-align: center; min-width: 120px">
          <!--
 Màu chữ/nền phải lấy theo biến của Element Plus, không viết cứng: ở chế độ
               tối, #303133 là chữ đen trên nền đen — con số quan trọng nhất trang này
               biến mất hoàn toàn. 
-->
          <div style="font-size: 44px; font-weight: 300; line-height: 1; color: var(--el-text-color-primary)">
            {{ summary.total ? summary.average.toFixed(1) : '–' }}
          </div>
          <div style="color: #909399; font-size: 13px; margin-top: 6px">
            {{ $t('vnetPages.feedback.outOf5') }}
          </div>
          <div style="color: #909399; font-size: 13px">
            {{ $t('vnetPages.feedback.totalRatings', { count: summary.total }) }}
          </div>
        </div>

        <div style="flex: 1; min-width: 280px">
          <div
            v-for="star in [5, 4, 3, 2, 1]"
            :key="star"
            style="display: flex; align-items: center; gap: 10px; margin-bottom: 6px"
          >
            <span style="width: 34px; color: #606266; font-size: 13px">{{ star }} ★</span>
            <div style="flex: 1; height: 10px; background: var(--el-fill-color); border-radius: 5px; overflow: hidden">
              <div
                :style="{
                  width: `${((summary.distribution[String(star)] || 0) / maxCount) * 100}%`,
                  height: '100%',
                  background: starColor(star),
                  borderRadius: '5px'
                }"
              />
            </div>
            <span
              style="
                width: 40px;
                text-align: right;
                color: #909399;
                font-size: 13px;
                font-variant-numeric: tabular-nums;
              "
            >
              {{ summary.distribution[String(star)] || 0 }}
            </span>
          </div>
        </div>
      </div>
    </ElCard>

    <ElCard>
      <template #header>
        <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap; justify-content: flex-end">
          <ElSelect
            v-model="machineFilter"
            filterable
            clearable
            :placeholder="$t('vnetPages.feedback.allMachines')"
            style="width: 180px"
            @change="
              page = 1;
              load();
            "
          >
            <ElOption v-for="m in machines" :key="m.id" :label="m.machine_code" :value="m.id" />
          </ElSelect>
          <ElSelect
            v-model="ratingFilter"
            clearable
            :placeholder="$t('vnetPages.feedback.allRatings')"
            style="width: 140px"
            @change="
              page = 1;
              load();
            "
          >
            <ElOption v-for="s in [5, 4, 3, 2, 1]" :key="s" :label="`${s} ★`" :value="s" />
          </ElSelect>
          <ElCheckbox
            v-model="onlyWithContent"
            @change="
              page = 1;
              load();
            "
          >
            {{ $t('vnetPages.feedback.onlyWithContent') }}
          </ElCheckbox>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.feedback.rating')" width="110">
          <template #default="{ row }">
            <span :style="`color:${starColor(row.rating)};font-weight:600`">{{ row.rating }} ★</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.feedback.machine')" width="110">
          <template #default="{ row }">{{ row.machine_code || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.feedback.member')" min-width="150">
          <template #default="{ row }">
            <span v-if="row.member_username">
              {{ row.member_name || row.member_username }}
              <span style="color: #909399">({{ row.member_username }})</span>
            </span>
            <span v-else style="color: #909399">{{ $t('vnetPages.feedback.guest') }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.feedback.order')" width="130">
          <template #default="{ row }">
            <span v-if="row.order_code" style="font-family: ui-monospace, Menlo, Consolas, monospace">
              {{ row.order_code }}
            </span>
            <span v-else style="color: #909399">-</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.feedback.content')" min-width="260">
          <template #default="{ row }">
            <span v-if="row.content">{{ row.content }}</span>
            <span v-else style="color: #909399">{{ $t('vnetPages.feedback.noComment') }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.feedback.at')" width="160">
          <template #default="{ row }">{{ dayjs(row.created_at).format('DD/MM/YYYY HH:mm') }}</template>
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
  </div>
</template>
