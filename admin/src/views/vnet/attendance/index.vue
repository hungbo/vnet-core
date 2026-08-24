<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage, ElNotification } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const loading = ref(false);
const rows = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const dateFilter = ref('');

function formatDate(v: string | null | undefined) {
  return v ? dayjs(v).format('DD/MM/YYYY HH:mm') : '-';
}

function formatMoney(v: number | null | undefined) {
  return `${new Intl.NumberFormat('vi-VN').format(v || 0)}₫`;
}

async function load() {
  loading.value = true;
  try {
    const res: any = await client.get('/attendance', {
      params: {
        page: page.value,
        page_size: pageSize.value,
        date: dateFilter.value || undefined
      }
    });
    rows.value = res?.items || [];
    total.value = res?.total || 0;
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
}

async function saveConfig() {
  savingCfg.value = true;
  try {
    await client.put('/settings/attendance', {
      daily_bonus: String(cfg.value.daily_bonus),
      streak_bonus: String(cfg.value.streak_bonus),
      streak_every: String(cfg.value.streak_every)
    });
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.common.saved')
    });
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    savingCfg.value = false;
  }
}

onMounted(() => {
  load();
  loadConfig();
});
</script>

<template>
  <div>
    <ElCard style="margin-bottom: 16px">
      <template #header>
        <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.attendance.configHint') }}</span>
      </template>
      <div style="display: flex; gap: 24px; align-items: flex-end; flex-wrap: wrap">
        <div>
          <div style="font-size: 13px; color: #606266; margin-bottom: 6px">
            {{ $t('vnetPages.attendance.dailyBonus') }}
          </div>
          <ElInputNumber v-model="cfg.daily_bonus" :min="0" :step="1000" />
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
          <ElInputNumber v-model="cfg.streak_bonus" :min="0" :step="1000" />
        </div>
        <ElButton type="primary" :loading="savingCfg" @click="saveConfig">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </div>
    </ElCard>

    <ElCard>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.attendance.listHint') }}</span>
          <div style="display: flex; gap: 8px">
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

      <ElTable v-loading="loading" :data="rows" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.attendance.member')" min-width="180">
          <template #default="{ row }">
            {{ row.member_name || '-' }}
            <span style="color: #909399">({{ row.member_username }})</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.attendance.date')" width="130">
          <template #default="{ row }">{{ dayjs(row.checkin_date).format('DD/MM/YYYY') }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.attendance.at')" width="160">
          <template #default="{ row }">{{ formatDate(row.checkin_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.attendance.streak')" width="120">
          <template #default="{ row }">
            <span :style="row.streak_days >= cfg.streak_every ? 'color:#e6a23c;font-weight:600' : ''">
              {{ row.streak_days }}
            </span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.attendance.reward')" width="140">
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
