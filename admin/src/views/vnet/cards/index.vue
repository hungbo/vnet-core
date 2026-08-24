<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

type CardKind = 'topup' | 'gift';
const kind = ref<CardKind>('topup');
const endpoint = computed(() => (kind.value === 'topup' ? '/topup-cards' : '/gift-cards'));

const loading = ref(false);
const rows = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const search = ref('');
const statusFilter = ref('');

function formatMoney(v: number | null | undefined) {
  return `${new Intl.NumberFormat('vi-VN').format(v || 0)}₫`;
}

function formatDate(v: string | null | undefined) {
  return v ? dayjs(v).format('DD/MM/YYYY HH:mm') : '-';
}

function statusTag(status: string) {
  const map: Record<string, 'success' | 'info' | 'danger'> = {
    active: 'success',
    used: 'info',
    cancelled: 'danger'
  };
  return h(ElTag, { type: map[status] || 'info', size: 'small' }, () => $t(`vnetPages.cards.status.${status}`));
}

async function load() {
  loading.value = true;
  try {
    const res: any = await client.get(endpoint.value, {
      params: {
        page: page.value,
        page_size: pageSize.value,
        search: search.value || undefined,
        status: statusFilter.value || undefined
      }
    });
    rows.value = res?.items || [];
    total.value = res?.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    rows.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function switchKind(k: CardKind) {
  kind.value = k;
  page.value = 1;
  search.value = '';
  statusFilter.value = '';
  load();
}

// --- Sinh thẻ ----------------------------------------------------------------
// Mã bí mật chỉ trả về ĐÚNG MỘT LẦN trong phản hồi này. Backend lưu băm, nên
// đóng hộp thoại mà chưa lưu là mất luôn cả lô thẻ.

const genVisible = ref(false);
const generating = ref(false);
const genForm = ref({
  count: 10,
  face_value: 100000,
  bonus_value: 0,
  expires_at: ''
});

const generated = ref<any[]>([]);
const resultVisible = ref(false);

function openGenerate() {
  genForm.value = {
    count: 10,
    face_value: 100000,
    bonus_value: 0,
    expires_at: ''
  };
  genVisible.value = true;
}

async function submitGenerate() {
  generating.value = true;
  try {
    const body: any = {
      count: genForm.value.count,
      expires_at: genForm.value.expires_at || undefined
    };
    if (kind.value === 'topup') {
      body.face_value = genForm.value.face_value;
      body.bonus_value = genForm.value.bonus_value || 0;
    } else {
      body.value = genForm.value.face_value;
    }
    const res: any = await client.post(`${endpoint.value}/generate`, body);
    generated.value = res?.cards || [];
    genVisible.value = false;
    resultVisible.value = true;
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    generating.value = false;
  }
}

const generatedCsv = computed(() =>
  ['seri,ma_bi_mat,gia_tri', ...generated.value.map(c => `${c.serial},${c.secret},${c.value}`)].join('\n')
);

function downloadCsv() {
  const blob = new Blob([`﻿${generatedCsv.value}`], {
    type: 'text/csv;charset=utf-8'
  });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `${kind.value === 'topup' ? 'the-nap' : 'the-qua-tang'}-${dayjs().format('YYYYMMDD-HHmm')}.csv`;
  a.click();
  URL.revokeObjectURL(a.href);
}

async function copyCsv() {
  try {
    await navigator.clipboard.writeText(generatedCsv.value);
    ElMessage.success($t('vnetPages.cards.copied'));
  } catch {
    ElMessage.warning($t('vnetPages.cards.copyFailed'));
  }
}

async function closeResult() {
  try {
    await ElMessageBox.confirm($t('vnetPages.cards.closeWarning'), $t('vnetPages.common.confirm'), { type: 'warning' });
  } catch {
    return;
  }
  resultVisible.value = false;
  generated.value = [];
}

// --- Huỷ thẻ -----------------------------------------------------------------

async function handleCancel(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.cards.cancelConfirm', { serial: row.code || row.serial }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.post(`${endpoint.value}/${row.id}/cancel`);
    ElMessage.success($t('vnetPages.cards.cancelled'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

// --- Đổi thẻ nạp / tra thẻ quà tặng -------------------------------------------
// POST /topup-cards/redeem và POST /gift-cards/check đã có từ trước nhưng trang
// này chỉ sinh thẻ và huỷ thẻ — nhân viên không có cách nào nạp hộ khách tại
// quầy, cũng không tra được thẻ quà tặng còn bao nhiêu.

const redeemVisible = ref(false);
const redeeming = ref(false);
const redeemForm = ref({ serial: '', secret: '', member_id: '' });
const redeemMembers = ref<any[]>([]);
const memberLoading = ref(false);

async function searchMembers(query: string) {
  if (!query) {
    redeemMembers.value = [];
    return;
  }
  memberLoading.value = true;
  try {
    const res: any = await client.get('/members', { params: { page: 1, page_size: 20, search: query } });
    redeemMembers.value = res?.items || [];
  } catch {
    redeemMembers.value = [];
  } finally {
    memberLoading.value = false;
  }
}

function openRedeem() {
  redeemForm.value = { serial: '', secret: '', member_id: '' };
  redeemMembers.value = [];
  redeemVisible.value = true;
}

async function submitRedeem() {
  const f = redeemForm.value;
  if (!f.serial || !f.secret) {
    ElMessage.warning($t('vnetPages.cards.needSerialSecret'));
    return;
  }
  if (!f.member_id) {
    ElMessage.warning($t('vnetPages.cards.needMember'));
    return;
  }
  redeeming.value = true;
  try {
    const res: any = await client.post('/topup-cards/redeem', {
      serial: f.serial,
      secret: f.secret,
      member_id: f.member_id
    });
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.cards.redeemSuccess', {
        value: formatMoney((res?.face_value || 0) + (res?.bonus_value || 0)),
        balance: formatMoney(res?.balance_after)
      })
    });
    redeemVisible.value = false;
    load();
  } catch (e: any) {
    // Thẻ sai / đã dùng / hết hạn đều trả về cùng một câu để không lộ thẻ nào có thật.
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    redeeming.value = false;
  }
}

// --- Bán thẻ ở quầy -----------------------------------------------------------
// sold_to/sold_at khai từ đầu nhưng KHÔNG endpoint nào ghi. Chúng khác
// used_by/used_at: bán là lúc quán đưa tấm thẻ giấy cho khách, nạp là lúc ai đó
// gõ mã vào tài khoản — hai người có thể khác nhau (mua tặng).

const sellVisible = ref(false);
const selling = ref(false);
const sellCard = ref<any>(null);
const sellMemberId = ref('');

function openSell(row: any) {
  sellCard.value = row;
  sellMemberId.value = '';
  redeemMembers.value = [];
  sellVisible.value = true;
}

async function submitSell() {
  if (!sellMemberId.value) {
    ElMessage.warning($t('vnetPages.cards.needMember'));
    return;
  }
  selling.value = true;
  try {
    await client.post(`/topup-cards/${sellCard.value.id}/sell`, { member_id: sellMemberId.value });
    const member = redeemMembers.value.find(m => m.id === sellMemberId.value);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.cards.sellSuccess', {
        serial: sellCard.value.code || sellCard.value.serial,
        member: member?.full_name || member?.username || ''
      })
    });
    sellVisible.value = false;
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    selling.value = false;
  }
}

const checkVisible = ref(false);
const checking = ref(false);
const checkForm = ref({ serial: '', secret: '' });
const checkResult = ref<any>(null);

function openCheck() {
  checkForm.value = { serial: '', secret: '' };
  checkResult.value = null;
  checkVisible.value = true;
}

async function submitCheck() {
  if (!checkForm.value.serial || !checkForm.value.secret) {
    ElMessage.warning($t('vnetPages.cards.needSerialSecret'));
    return;
  }
  checking.value = true;
  checkResult.value = null;
  try {
    checkResult.value = await client.post('/gift-cards/check', checkForm.value);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    checking.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px">
          <ElRadioGroup :model-value="kind" @change="(v: any) => switchKind(v)">
            <ElRadioButton value="topup">{{ $t('vnetPages.cards.topupTab') }}</ElRadioButton>
            <ElRadioButton value="gift">{{ $t('vnetPages.cards.giftTab') }}</ElRadioButton>
          </ElRadioGroup>
          <div style="display: flex; gap: 8px; align-items: center">
            <ElInput
              v-model="search"
              :placeholder="$t('vnetPages.cards.searchSerial')"
              clearable
              style="width: 200px"
              @keyup.enter="
                page = 1;
                load();
              "
            />
            <ElSelect
              v-model="statusFilter"
              :placeholder="$t('vnetPages.cards.statusLabel')"
              clearable
              style="width: 140px"
              @change="
                page = 1;
                load();
              "
            >
              <ElOption :label="$t('vnetPages.cards.status.active')" value="active" />
              <ElOption :label="$t('vnetPages.cards.status.used')" value="used" />
              <ElOption :label="$t('vnetPages.cards.status.cancelled')" value="cancelled" />
            </ElSelect>
            <ElButton
              @click="
                page = 1;
                load();
              "
            >
              {{ $t('vnetPages.common.search') }}
            </ElButton>
            <ElButton v-if="kind === 'topup'" @click="openRedeem">{{ $t('vnetPages.cards.redeem') }}</ElButton>
            <ElButton v-else @click="openCheck">{{ $t('vnetPages.cards.check') }}</ElButton>
            <ElButton type="primary" @click="openGenerate">{{ $t('vnetPages.cards.generate') }}</ElButton>
          </div>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.cards.serial')" min-width="160">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace">{{ row.code || row.serial }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn v-if="kind === 'topup'" :label="$t('vnetPages.cards.faceValue')" width="130">
          <template #default="{ row }">{{ formatMoney(row.face_value) }}</template>
        </ElTableColumn>
        <ElTableColumn v-if="kind === 'topup'" :label="$t('vnetPages.cards.bonusValue')" width="130">
          <template #default="{ row }">{{ row.bonus_value ? formatMoney(row.bonus_value) : '-' }}</template>
        </ElTableColumn>
        <ElTableColumn v-if="kind === 'gift'" :label="$t('vnetPages.cards.balance')" width="140">
          <template #default="{ row }">
            {{ formatMoney(row.balance) }}
            <span style="color: #909399">/ {{ formatMoney(row.initial_balance) }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.cards.statusLabel')" width="120">
          <template #default="{ row }"><component :is="statusTag(row.status)" /></template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.cards.expiresAt')" width="160">
          <template #default="{ row }">{{ formatDate(row.expires_at) }}</template>
        </ElTableColumn>
        <ElTableColumn v-if="kind === 'topup'" :label="$t('vnetPages.cards.usedAt')" width="160">
          <template #default="{ row }">{{ formatDate(row.used_at) }}</template>
        </ElTableColumn>
        <ElTableColumn v-if="kind === 'topup'" :label="$t('vnetPages.cards.soldTo')" min-width="140">
          <template #default="{ row }">
            <span v-if="row.sold_to">{{ formatDate(row.sold_at) }}</span>
            <span v-else style="color: #909399">{{ $t('vnetPages.cards.notSold') }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="190" fixed="right">
          <template #default="{ row }">
            <ElButton
              v-if="kind === 'topup' && row.status === 'active' && !row.sold_to"
              size="small"
              @click="openSell(row)"
            >
              {{ $t('vnetPages.cards.sell') }}
            </ElButton>
            <ElButton v-if="row.status === 'active'" size="small" type="danger" plain @click="handleCancel(row)">
              {{ $t('vnetPages.cards.cancel') }}
            </ElButton>
            <span v-if="row.status !== 'active'">-</span>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="mt-16px flex justify-end">
        <ElPagination
          v-if="total"
          v-model:current-page="page"
          v-model:page-size="pageSize"
          layout="total, sizes, prev, pager, next"
          :total="total"
          @current-change="load"
          @size-change="
            page = 1;
            load();
          "
        />
      </div>
    </ElCard>

    <ElDialog v-model="sellVisible" :title="$t('vnetPages.cards.sell')" width="460px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        {{ $t('vnetPages.cards.sellHint') }}
      </ElAlert>
      <ElForm label-width="140px">
        <ElFormItem :label="$t('vnetPages.cards.serial')">
          <strong style="font-family: ui-monospace, Menlo, Consolas, monospace">
            {{ sellCard?.code || sellCard?.serial }}
          </strong>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.cards.faceValue')">
          {{ formatMoney(sellCard?.face_value) }}
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.cards.sellTo')">
          <ElSelect
            v-model="sellMemberId"
            filterable
            remote
            :remote-method="searchMembers"
            :loading="memberLoading"
            style="width: 100%"
          >
            <ElOption
              v-for="m in redeemMembers"
              :key="m.id"
              :label="`${m.full_name || m.username} — ${m.phone || ''}`"
              :value="m.id"
            />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="sellVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="selling" @click="submitSell">
          {{ $t('vnetPages.cards.sell') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="redeemVisible" :title="$t('vnetPages.cards.redeem')" width="480px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        {{ $t('vnetPages.cards.redeemHint') }}
      </ElAlert>
      <ElForm label-width="140px">
        <ElFormItem :label="$t('vnetPages.cards.serial')">
          <ElInput v-model="redeemForm.serial" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.cards.secret')">
          <ElInput v-model="redeemForm.secret" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.cards.redeemMember')">
          <ElSelect
            v-model="redeemForm.member_id"
            filterable
            remote
            :remote-method="searchMembers"
            :loading="memberLoading"
            style="width: 100%"
          >
            <ElOption
              v-for="m in redeemMembers"
              :key="m.id"
              :label="`${m.full_name || m.username} — ${formatMoney(m.balance)}`"
              :value="m.id"
            />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="redeemVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="redeeming" @click="submitRedeem">
          {{ $t('vnetPages.cards.redeem') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="checkVisible" :title="$t('vnetPages.cards.check')" width="440px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        {{ $t('vnetPages.cards.checkHint') }}
      </ElAlert>
      <ElForm label-width="120px">
        <ElFormItem :label="$t('vnetPages.cards.serial')">
          <ElInput v-model="checkForm.serial" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.cards.secret')">
          <ElInput v-model="checkForm.secret" @keyup.enter="submitCheck" />
        </ElFormItem>
      </ElForm>

      <ElDescriptions v-if="checkResult" :column="1" border size="small">
        <ElDescriptionsItem :label="$t('vnetPages.cards.serial')">{{ checkResult.serial }}</ElDescriptionsItem>
        <ElDescriptionsItem :label="$t('vnetPages.cards.balance')">
          {{ formatMoney(checkResult.balance) }}
        </ElDescriptionsItem>
        <ElDescriptionsItem :label="$t('vnetPages.cards.statusLabel')">
          <component :is="statusTag(checkResult.status)" />
        </ElDescriptionsItem>
        <ElDescriptionsItem :label="$t('vnetPages.cards.expiresAt')">
          {{ checkResult.expires_at ? formatDate(checkResult.expires_at) : $t('vnetPages.cards.noExpiry') }}
        </ElDescriptionsItem>
      </ElDescriptions>

      <template #footer>
        <ElButton @click="checkVisible = false">{{ $t('common.close') }}</ElButton>
        <ElButton type="primary" :loading="checking" @click="submitCheck">
          {{ $t('vnetPages.common.search') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="genVisible" :title="$t('vnetPages.cards.generate')" width="440px">
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.cards.count')">
          <ElInputNumber v-model="genForm.count" :min="1" :max="500" />
        </ElFormItem>
        <ElFormItem :label="kind === 'topup' ? $t('vnetPages.cards.faceValue') : $t('vnetPages.cards.value')">
          <ElInputNumber v-model="genForm.face_value" :min="1000" :step="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem v-if="kind === 'topup'" :label="$t('vnetPages.cards.bonusValue')">
          <ElInputNumber v-model="genForm.bonus_value" :min="0" :step="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.cards.expiresAt')">
          <ElDatePicker
            v-model="genForm.expires_at"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            :placeholder="$t('vnetPages.cards.noExpiry')"
            style="width: 100%"
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="genVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="generating" @click="submitGenerate">
          {{ $t('vnetPages.cards.generate') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="resultVisible"
      :title="$t('vnetPages.cards.resultTitle', { count: generated.length })"
      width="580px"
      :close-on-click-modal="false"
      :show-close="false"
    >
      <ElAlert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.cards.onlyOnce') }}
      </ElAlert>
      <div style="max-height: 340px; overflow-y: auto">
        <table
          style="
            width: 100%;
            border-collapse: collapse;
            font-family: ui-monospace, Menlo, Consolas, monospace;
            font-size: 13px;
          "
        >
          <thead>
            <tr style="text-align: left; color: #909399">
              <th style="padding: 6px 8px">
                {{ $t('vnetPages.cards.serial') }}
              </th>
              <th style="padding: 6px 8px">
                {{ $t('vnetPages.cards.secret') }}
              </th>
              <th style="padding: 6px 8px; text-align: right">
                {{ $t('vnetPages.cards.value') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="cd in generated" :key="cd.id" style="border-top: 1px solid #ebeef5">
              <td style="padding: 6px 8px">{{ cd.serial }}</td>
              <td style="padding: 6px 8px; font-weight: 600">
                {{ cd.secret }}
              </td>
              <td style="padding: 6px 8px; text-align: right">
                {{ formatMoney(cd.value) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #footer>
        <ElButton @click="copyCsv">{{ $t('vnetPages.cards.copy') }}</ElButton>
        <ElButton type="primary" @click="downloadCsv">{{ $t('vnetPages.cards.download') }}</ElButton>
        <ElButton @click="closeResult">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
