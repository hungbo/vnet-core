<script setup lang="ts">
import { computed, h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { formatAmount } from '@/utils/money';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const formRef = ref<FormInstance>();

const rewardsVisible = ref(false);
const rewardsLoading = ref(false);
const rewards = ref<any[]>([]);

const form = ref({
  name: '',
  type: '',
  priority: 0,
  is_active: true,
  valid_from: '',
  valid_to: '',
  conditions: [] as Array<{ condition_key: string; condition_value: string }>,
  rewards: [] as Array<{ reward_type: string; reward_value: string }>
});

const rules: FormRules = {
  name: [
    {
      required: true,
      message: $t('vnetPages.promotions.form.nameRequired'),
      trigger: 'blur'
    }
  ],
  type: [
    {
      required: true,
      message: $t('vnetPages.promotions.form.typeRequired'),
      trigger: 'blur'
    }
  ]
};

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

// --- Ô thưởng vòng quay -------------------------------------------------------
// Bảng lucky_spin_rewards trước đây chỉ được ĐỌC lúc quay: không CRUD, seed
// không chèn, nên POST /lucky-spin/spin luôn quay vào tập rỗng.

const rewardForm = ref<any>({
  id: '',
  name: '',
  reward_type: 'balance',
  amount: 10000,
  probability: 10,
  max_per_day: 0,
  is_active: true
});
const savingReward = ref(false);
const spinning = ref(false);

// Xác suất nhập theo % cho dễ hiểu; backend làm việc với 0–1.
const totalProbability = computed(() =>
  rewards.value.filter(r => r.is_active).reduce((sum, r) => sum + (r.probability || 0) * 100, 0)
);

async function fetchRewards() {
  rewardsVisible.value = true;
  rewardsLoading.value = true;
  try {
    // include_inactive: ô đã tắt vẫn phải nhìn thấy để bật lại.
    const res: any = await client.get('/lucky-spin/rewards', {
      params: { include_inactive: true }
    });
    rewards.value = Array.isArray(res) ? res : res?.items || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.promotions.messages.loadRewardsError'));
    rewards.value = [];
  } finally {
    rewardsLoading.value = false;
  }
}

function resetRewardForm() {
  rewardForm.value = {
    id: '',
    name: '',
    reward_type: 'balance',
    amount: 10000,
    probability: 10,
    max_per_day: 0,
    is_active: true
  };
}

function editReward(row: any) {
  rewardForm.value = {
    id: row.id,
    name: row.name,
    reward_type: row.reward_type,
    amount: row.amount ?? 0,
    probability: Math.round((row.probability || 0) * 10000) / 100,
    max_per_day: row.max_per_day ?? 0,
    is_active: row.is_active
  };
}

async function saveReward() {
  if (!rewardForm.value.name) {
    ElMessage.warning($t('vnetPages.promotions.rewardNameRequired'));
    return;
  }
  savingReward.value = true;
  const body = {
    name: rewardForm.value.name,
    reward_type: rewardForm.value.reward_type,
    amount: rewardForm.value.amount,
    probability: rewardForm.value.probability / 100,
    max_per_day: rewardForm.value.max_per_day,
    is_active: rewardForm.value.is_active
  };
  try {
    if (rewardForm.value.id) {
      await client.put(`/lucky-spin/rewards/${rewardForm.value.id}`, body);
    } else {
      await client.post('/lucky-spin/rewards', body);
    }
    resetRewardForm();
    await fetchRewards();
  } catch (e: any) {
    // Backend từ chối kèm lý do cụ thể (tổng xác suất vượt 1, loại chưa hỗ
    // trợ) — hiện nguyên văn thay vì một câu lỗi chung.
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    savingReward.value = false;
  }
}

async function deleteReward(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.promotions.deleteRewardConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      {
        type: 'warning'
      }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/lucky-spin/rewards/${row.id}`);
    await fetchRewards();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

// Quay thử: nhân viên kiểm cấu hình trước khi mở cho khách. Dùng chính hội viên
// đầu tiên trong danh sách vì backend bắt buộc member_id để ghi lịch sử quay.
async function testSpin() {
  spinning.value = true;
  try {
    const list: any = await client.get('/members', {
      params: { page_size: 1 }
    });
    const member = (Array.isArray(list) ? list : list?.items || [])[0];
    if (!member) {
      ElMessage.warning($t('vnetPages.promotions.spinNeedsMember'));
      return;
    }
    const res: any = await client.post('/lucky-spin/spin', {
      member_id: member.id
    });
    ElNotification({
      type: res?.is_win ? 'success' : 'info',
      title: $t('vnetPages.promotions.testSpin'),
      message: res?.is_win
        ? $t('vnetPages.promotions.spinWin', {
            name: res.reward?.name,
            member: member.full_name || member.username
          })
        : $t('vnetPages.promotions.spinLose')
    });
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    spinning.value = false;
  }
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/promotions', {
      params: {
        page,
        page_size: pageSize,
        search: search.value || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.promotions.name'), minWidth: 160 },
    { prop: 'type', label: $t('vnetPages.promotions.type'), width: 120 },
    {
      prop: 'priority',
      label: $t('vnetPages.promotions.priority'),
      width: 80
    },
    {
      prop: 'is_active',
      label: $t('vnetPages.promotions.isActive'),
      width: 90,
      formatter: (row: any) =>
        h(ElTag, { type: row.is_active ? 'success' : 'danger', size: 'small' }, () =>
          row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no')
        )
    },
    {
      prop: 'valid_period',
      label: $t('vnetPages.promotions.validPeriod'),
      minWidth: 240,
      formatter: (row: any) => `${formatDate(row.valid_from)} → ${formatDate(row.valid_to)}`
    }
  ]
});

function searchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = {
    name: '',
    type: '',
    priority: 0,
    is_active: true,
    valid_from: '',
    valid_to: '',
    conditions: [],
    rewards: []
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    type: row.type || '',
    priority: row.priority ?? 0,
    is_active: row.is_active ?? true,
    valid_from: row.valid_from || '',
    valid_to: row.valid_to || '',
    // Values come back as raw JSON; render them back as the text the editor
    // shows, keeping plain strings unquoted so they read naturally.
    conditions: (row.conditions || []).map((c: any) => ({
      condition_key: c.condition_key || '',
      condition_value: fromJsonValue(c.condition_value)
    })),
    rewards: (row.rewards || []).map((r: any) => ({
      reward_type: r.reward_type || '',
      reward_value: fromJsonValue(r.reward_value)
    }))
  };
  dialogVisible.value = true;
}

// The API takes a list of typed entries, not one free-form object. Sending an
// object made the request fail to deserialise, which is why conditions and
// rewards could never actually be saved.
//
// condition_value is raw JSON on the wire: a number stays a number, and plain
// text becomes a JSON string rather than invalid JSON.
function toJsonValue(raw: string): any {
  const text = String(raw ?? '').trim();
  if (text === '') return '';
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function fromJsonValue(raw: any): string {
  if (raw === null || raw === undefined) return '';
  if (typeof raw === 'string') return raw;
  return JSON.stringify(raw);
}

// Khoá điều kiện và loại thưởng phải khớp đúng chuỗi backend hiểu. Trước đây
// đây là hai ô nhập tự do: gõ sai một ký tự thì khuyến mãi lưu thành công nhưng
// không bao giờ chạy. Backend nay từ chối khoá lạ, và danh sách này giữ cho
// hai đầu không lệch nhau.
const CONDITION_KEYS = [
  { value: 'min_amount', hint: '50000' },
  { value: 'min_quantity', hint: '2' },
  { value: 'member_group', hint: '["<id nhóm>"]' },
  { value: 'day_of_week', hint: '[0,6]' },
  { value: 'time_range', hint: '{"from":"18:00","to":"22:00"}' },
  { value: 'product_category', hint: '["<id danh mục>"]' }
];

const REWARD_TYPES = [
  { value: 'discount_percent', hint: '{"percent":10,"max_discount":50000}' },
  { value: 'discount_amount', hint: '{"amount":20000}' }
];

function conditionHint(key: string) {
  return CONDITION_KEYS.find(k => k.value === key)?.hint || $t('vnetPages.promotions.conditionValue');
}

function rewardHint(type: string) {
  return REWARD_TYPES.find(r => r.value === type)?.hint || $t('vnetPages.promotions.rewardValue');
}

function addCondition() {
  form.value.conditions.push({ condition_key: '', condition_value: '' });
}

function removeCondition(i: number) {
  form.value.conditions.splice(i, 1);
}

function addReward() {
  form.value.rewards.push({ reward_type: '', reward_value: '' });
}

function removeReward(i: number) {
  form.value.rewards.splice(i, 1);
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    const payload: any = {
      name: form.value.name,
      type: form.value.type,
      priority: form.value.priority,
      is_active: form.value.is_active,
      valid_from: form.value.valid_from || null,
      valid_to: form.value.valid_to || null
    };
    payload.conditions = form.value.conditions
      .filter(c => c.condition_key)
      .map(c => ({
        condition_key: c.condition_key,
        condition_value: toJsonValue(c.condition_value)
      }));
    payload.rewards = form.value.rewards
      .filter(r => r.reward_type)
      .map(r => ({
        reward_type: r.reward_type,
        reward_value: toJsonValue(r.reward_value)
      }));
    if (isEdit.value && editingId.value) {
      await client.put(`/promotions/${editingId.value}`, payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.promotions.messages.editSuccess')
      });
    } else {
      await client.post('/promotions', payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.promotions.messages.addSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.promotions.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.promotions.messages.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/promotions/${row.id}`);
    ElMessage.success($t('vnetPages.promotions.messages.deleteSuccess'));
    getData();
  } catch (e: any) {
    // ElMessageBox từ chối bằng chuỗi 'cancel'/'close' khi người dùng bấm Huỷ —
    // đó không phải lỗi. Còn lại là backend từ chối (ràng buộc dữ liệu, quy tắc
    // nghiệp vụ) và phải nói ra; `catch {}` rỗng làm nút bấm vào im lặng.
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.promotions.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="searchData"
          />
          <ElButton type="primary" @click="searchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :loading="loading"
          :show-delete="false"
          @add="openCreate"
          @refresh="getData"
        >
          <template #prefix>
            <ElButton @click="fetchRewards">{{ $t('vnetPages.promotions.luckySpinRewards') }}</ElButton>
          </template>
        </TableHeaderOperation>
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="200" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="mt-16px flex justify-center">
        <ElPagination
          v-if="mobilePagination.total"
          layout="total, sizes, prev, pager, next"
          v-bind="mobilePagination"
          @current-change="mobilePagination['current-change']"
          @size-change="mobilePagination['size-change']"
        />
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.promotions.edit') : $t('vnetPages.promotions.add')"
      width="650px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.promotions.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.type')" prop="type">
          <ElInput v-model="form.type" :placeholder="$t('vnetPages.promotions.form.typePlaceholder')" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.priority')" prop="priority">
          <ElInputNumber v-model="form.priority" :min="0" :max="999" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.isActive')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.validFrom')">
          <ElDatePicker
            v-model="form.valid_from"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            style="width: 100%"
          />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.validTo')">
          <ElDatePicker
            v-model="form.valid_to"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            style="width: 100%"
          />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.conditions')">
          <div style="width: 100%">
            <div v-for="(c, i) in form.conditions" :key="`c${i}`" style="display: flex; gap: 8px; margin-bottom: 8px">
              <ElSelect
                v-model="c.condition_key"
                :placeholder="$t('vnetPages.promotions.conditionKey')"
                style="flex: 1; min-width: 180px"
              >
                <ElOption
                  v-for="k in CONDITION_KEYS"
                  :key="k.value"
                  :label="$t(`vnetPages.promotions.conditionKeys.${k.value}`)"
                  :value="k.value"
                />
              </ElSelect>
              <ElInput v-model="c.condition_value" :placeholder="conditionHint(c.condition_key)" style="flex: 1" />
              <ElButton type="danger" plain @click="removeCondition(i)">
                {{ $t('vnetPages.promotions.remove') }}
              </ElButton>
            </div>
            <ElButton size="small" @click="addCondition">{{ $t('vnetPages.promotions.addCondition') }}</ElButton>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.promotions.rewards')">
          <div style="width: 100%">
            <div v-for="(r, i) in form.rewards" :key="`r${i}`" style="display: flex; gap: 8px; margin-bottom: 8px">
              <ElSelect
                v-model="r.reward_type"
                :placeholder="$t('vnetPages.promotions.rewardType')"
                style="flex: 1; min-width: 180px"
              >
                <ElOption
                  v-for="t in REWARD_TYPES"
                  :key="t.value"
                  :label="$t(`vnetPages.promotions.rewardTypes.${t.value}`)"
                  :value="t.value"
                />
              </ElSelect>
              <ElInput v-model="r.reward_value" :placeholder="rewardHint(r.reward_type)" style="flex: 1" />
              <ElButton type="danger" plain @click="removeReward(i)">{{ $t('vnetPages.promotions.remove') }}</ElButton>
            </div>
            <ElButton size="small" @click="addReward">{{ $t('vnetPages.promotions.addReward') }}</ElButton>
          </div>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="rewardsVisible" :title="$t('vnetPages.promotions.luckySpinRewards')" width="900px">
      <ElAlert
        :type="totalProbability > 100 ? 'error' : 'info'"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      >
        {{
          $t('vnetPages.promotions.probabilityNote', {
            total: totalProbability.toFixed(2),
            lose: Math.max(0, 100 - totalProbability).toFixed(2)
          })
        }}
      </ElAlert>

      <div style="display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; align-items: center">
        <ElInput v-model="rewardForm.name" :placeholder="$t('vnetPages.promotions.rewardName')" style="width: 170px" />
        <ElSelect v-model="rewardForm.reward_type" style="width: 150px">
          <ElOption :label="$t('vnetPages.promotions.rewardTypes.balance')" value="balance" />
          <ElOption :label="$t('vnetPages.promotions.rewardTypes.bonus_points')" value="bonus_points" />
        </ElSelect>
        <ElInputNumber v-model="rewardForm.amount" :min="1" :step="1000" style="width: 140px" />
        <div style="display: flex; align-items: center; gap: 6px">
          <ElInputNumber v-model="rewardForm.probability" :min="0.01" :max="100" :precision="2" style="width: 120px" />
          <span style="color: #909399">%</span>
        </div>
        <ElSwitch v-model="rewardForm.is_active" :active-text="$t('vnetPages.promotions.isActive')" />
        <ElButton type="primary" :loading="savingReward" @click="saveReward">
          {{ rewardForm.id ? $t('vnetPages.common.save') : $t('vnetPages.common.add') }}
        </ElButton>
        <ElButton v-if="rewardForm.id" @click="resetRewardForm">{{ $t('vnetPages.common.cancel') }}</ElButton>
      </div>

      <ElTable v-loading="rewardsLoading" :data="rewards" border stripe size="small" style="width: 100%">
        <ElTableColumn prop="name" :label="$t('vnetPages.promotions.name')" min-width="150" />
        <ElTableColumn :label="$t('vnetPages.promotions.type')" width="130">
          <template #default="{ row }">{{ $t(`vnetPages.promotions.rewardTypes.${row.reward_type}`) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.promotions.value')" width="120" align="right">
          <template #default="{ row }">{{ formatAmount(row.amount || 0) }}₫</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.promotions.probability')" width="100" align="right">
          <template #default="{ row }">{{ ((row.probability || 0) * 100).toFixed(2) }}%</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.promotions.isActive')" width="90">
          <template #default="{ row }">
            <ElTag :type="row.is_active ? 'success' : 'info'" size="small">
              {{ row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no') }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="130">
          <template #default="{ row }">
            <ElButton size="small" @click="editReward(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" @click="deleteReward(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <template #footer>
        <ElButton :loading="spinning" :disabled="!rewards.length" @click="testSpin">
          {{ $t('vnetPages.promotions.testSpin') }}
        </ElButton>
        <ElButton @click="rewardsVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
