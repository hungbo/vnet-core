<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue';
import { ElMessage, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useTodayDate } from '@/hooks/common/today-filter';
import { useFitPageSize } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';
import { moneyInput } from '@/utils/money';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();

const loading = ref(false);
const rows = ref<any[]>([]);
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
// Mặc định hôm nay; xoá ô ngày để xem mọi ngày.
const { date: dateFilter, follow: followToday } = useTodayDate();
// Sắp xếp phía máy chủ trên toàn bộ lượt điểm danh: prop của cột bấm và 'asc' | 'desc'.
const sort = ref<string>();
const order = ref<'asc' | 'desc'>();

function onSortChange(e: { prop: string | null; order: 'ascending' | 'descending' | null }) {
  if (e.order && e.prop) {
    sort.value = e.prop;
    order.value = e.order === 'ascending' ? 'asc' : 'desc';
  } else {
    sort.value = undefined;
    order.value = undefined;
  }
  page.value = 1;
  load();
}

function formatDate(v: string | null | undefined) {
  return v ? dayjs(v).format('DD/MM/YYYY HH:mm') : '-';
}

function formatMoney(v: number | null | undefined) {
  return `${new Intl.NumberFormat('vi-VN').format(v || 0)}₫`;
}

async function load() {
  followToday();
  loading.value = true;
  try {
    const res: any = await client.get('/attendance', {
      params: {
        page: page.value,
        page_size: pageSize.value,
        sort: sort.value,
        order: order.value,
        date: dateFilter.value || undefined
      }
    });
    rows.value = res?.items || [];
    total.value = res?.total || 0;
    await fitter.afterLoad();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

// --- Cấu hình thưởng ---------------------------------------------------------
// Để 0 thì điểm danh vẫn ghi nhận nhưng không tặng gì — quán không muốn tặng
// tiền vẫn dùng được tính năng.

const cfg = ref({ daily_bonus: 0, streak_bonus: 0, streak_every: 7 });
const savingCfg = ref(false);
// Cấu hình đúng như lúc vừa nạp; khác đi là đang sửa dở, chưa được đè.
let savedCfg = JSON.stringify(cfg.value);

async function loadConfig() {
  try {
    const res: any = await client.get('/settings/attendance');
    const list: any[] = Array.isArray(res) ? res : res?.items || [];
    for (const row of list) {
      const n = Number(row.value);
      if (!Number.isNaN(n) && row.key in cfg.value) (cfg.value as any)[row.key] = n;
    }
  } catch {
    // Nhóm cài đặt chưa tồn tại là trạng thái lần đầu bình thường, không phải lỗi.
  }
  // Chụp mốc SAU khi ô nhập đã vẽ: ElInputNumber kẹp/làm tròn giá trị ở nhịp DOM
  // kế tiếp, chụp trước đó thì trang tưởng đang sửa dở và hoãn tải lại mãi.
  await nextTick();
  savedCfg = JSON.stringify(cfg.value);
}

// Điểm danh chỉ ghi qua POST /attendance/checkin; tên hội viên lấy từ bảng
// members. Cấu hình thưởng là nhóm cài đặt "attendance" (PUT /settings/:group).
useRealtimeRefresh(['attendance', 'members'], load);
const { resume: resumeConfig } = useRealtimeRefresh(['settings'], loadConfig, {
  paused: () => savingCfg.value || JSON.stringify(cfg.value) !== savedCfg
});

async function saveConfig() {
  savingCfg.value = true;
  try {
    await client.put('/settings/attendance', {
      daily_bonus: String(cfg.value.daily_bonus),
      streak_bonus: String(cfg.value.streak_bonus),
      streak_every: String(cfg.value.streak_every)
    });
    savedCfg = JSON.stringify(cfg.value);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.common.saved')
    });
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    savingCfg.value = false;
    // Sự kiện tới trong lúc lưu đã bị hoãn: tải bù.
    resumeConfig();
  }
}

onMounted(() => {
  if (!fitter.fit()) load();
  loadConfig();
});
</script>

<template>
  <div class="page-fill">
    <ElCard style="margin-bottom: 16px">
      <template #header>
        <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.attendance.configHint') }}</span>
      </template>
      <div style="display: flex; gap: 24px; align-items: flex-end; flex-wrap: wrap">
        <div>
          <div style="font-size: 13px; color: #606266; margin-bottom: 6px">
            {{ $t('vnetPages.attendance.dailyBonus') }}
          </div>
          <ElInputNumber v-bind="moneyInput" v-model="cfg.daily_bonus" :min="0" :step="1000" />
        </div>
        <div>
          <div style="font-size: 13px; color: #606266; margin-bottom: 6px">
            {{ $t('vnetPages.attendance.streakEvery') }}
          </div>
          <ElInputNumber v-model="cfg.streak_every" :min="1" :max="365" />
        </div>
        <div>
          <div style="font-size: 13px; color: #606266; margin-bottom: 6px">
            {{ $t('vnetPages.attendance.streakBonus') }}
          </div>
          <ElInputNumber v-bind="moneyInput" v-model="cfg.streak_bonus" :min="0" :step="1000" />
        </div>
        <!-- Lưu cấu hình thưởng = PUT /settings/attendance. -->
        <ElButton v-if="hasAuth('settings.edit')" type="primary" :loading="savingCfg" @click="saveConfig">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </div>
    </ElCard>

    <ElCard class="fill-card">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.attendance.listHint') }}</span>
          <div style="display: flex; flex-wrap: wrap; gap: 8px">
            <ElDatePicker
              v-model="dateFilter"
              type="date"
              value-format="YYYY-MM-DD"
              :placeholder="$t('vnetPages.attendance.allDays')"
              clearable
              @change="
                page = 1;
                load();
              "
            />
            <ElButton @click="load">{{ $t('common.refresh') }}</ElButton>
          </div>
        </div>
      </template>

      <ElTable
        v-loading="loading"
        :data="rows"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn :label="$t('vnetPages.attendance.member')" min-width="180">
          <template #default="{ row }">
            {{ row.member_name || '-' }}
            <span style="color: #909399">({{ row.member_username }})</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="checkin_date" :label="$t('vnetPages.attendance.date')" width="130" sortable="custom">
          <template #default="{ row }">{{ dayjs(row.checkin_date).format('DD/MM/YYYY') }}</template>
        </ElTableColumn>
        <ElTableColumn prop="checkin_at" :label="$t('vnetPages.attendance.at')" width="160" sortable="custom">
          <template #default="{ row }">{{ formatDate(row.checkin_at) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="streak_days" :label="$t('vnetPages.attendance.streak')" width="120" sortable="custom">
          <template #default="{ row }">
            <span :style="row.streak_days >= cfg.streak_every ? 'color:#e6a23c;font-weight:600' : ''">
              {{ row.streak_days }}
            </span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="reward_amount" :label="$t('vnetPages.attendance.reward')" width="140" sortable="custom">
          <template #default="{ row }">
            {{ row.reward_amount ? formatMoney(row.reward_amount) : '-' }}
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
  </div>
</template>
