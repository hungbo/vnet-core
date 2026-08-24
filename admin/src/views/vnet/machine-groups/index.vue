<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import { formatAmount } from '@/utils/money';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');

// --- Bảng giá -----------------------------------------------------------------
// Máy tính tiền chọn giá theo ba tầng, tầng trước thắng tầng sau:
//   1. giá riêng cho hạng hội viên   2. giá theo khung giờ   3. giá cơ bản ở đây
// Hai tầng đầu trước nay chỉ chèn tay vào database mới có.

const priceVisible = ref(false);
const priceLoading = ref(false);
const priceGroup = ref<any>(null);
const memberGroups = ref<any[]>([]);
const tierPrices = ref<any[]>([]);
const timePrices = ref<any[]>([]);
const priceTab = ref<'tier' | 'time'>('tier');

const DAYS = [0, 1, 2, 3, 4, 5, 6];

function dayLabel(d: number) {
  return $t(`vnetPages.machineGroups.days.${d}`);
}

function clock(v: string) {
  return String(v || '').slice(0, 5);
}

function formatMoney(v: number | null | undefined) {
  return `${new Intl.NumberFormat('vi-VN').format(v || 0)}₫`;
}

async function openPricing(row: any) {
  priceGroup.value = row;
  priceVisible.value = true;
  priceTab.value = 'tier';
  await Promise.all([loadPrices(), loadMemberGroups()]);
}

async function loadMemberGroups() {
  try {
    const res: any = await client.get('/member-groups', {
      params: { page_size: 200 }
    });
    memberGroups.value = Array.isArray(res) ? res : res?.items || [];
  } catch {
    memberGroups.value = [];
  }
}

async function loadPrices() {
  priceLoading.value = true;
  try {
    const [a, b]: any[] = await Promise.all([
      client.get('/machine-prices', {
        params: { machine_group_id: priceGroup.value.id }
      }),
      client.get('/time-pricing', {
        params: { machine_group_id: priceGroup.value.id }
      })
    ]);
    tierPrices.value = Array.isArray(a) ? a : a?.items || [];
    timePrices.value = Array.isArray(b) ? b : b?.items || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    priceLoading.value = false;
  }
}

const tierForm = ref<any>({
  member_group_id: '',
  price_per_hour: 10000,
  min_duration: 1,
  effective_from: '',
  effective_to: ''
});
const timeForm = ref<any>({
  day_of_week: 0,
  start_time: '18:00',
  end_time: '22:00',
  price_per_hour: 15000
});
const savingPrice = ref(false);

async function addTierPrice() {
  if (!tierForm.value.member_group_id) {
    ElMessage.warning($t('vnetPages.machineGroups.pickTier'));
    return;
  }
  savingPrice.value = true;
  try {
    await client.post('/machine-prices', {
      machine_group_id: priceGroup.value.id,
      member_group_id: tierForm.value.member_group_id,
      price_per_hour: tierForm.value.price_per_hour,
      min_duration: tierForm.value.min_duration,
      effective_from: tierForm.value.effective_from || undefined,
      effective_to: tierForm.value.effective_to || undefined
    });
    tierForm.value.member_group_id = '';
    await loadPrices();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    savingPrice.value = false;
  }
}

async function addTimePrice() {
  savingPrice.value = true;
  try {
    await client.post('/time-pricing', {
      machine_group_id: priceGroup.value.id,
      day_of_week: timeForm.value.day_of_week,
      start_time: timeForm.value.start_time,
      end_time: timeForm.value.end_time,
      price_per_hour: timeForm.value.price_per_hour
    });
    await loadPrices();
  } catch (e: any) {
    // Khung chồng nhau bị backend từ chối kèm lý do cụ thể — hiện nguyên văn.
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    savingPrice.value = false;
  }
}

async function removePrice(kind: 'tier' | 'time', row: any) {
  try {
    await client.delete(`${kind === 'tier' ? '/machine-prices' : '/time-pricing'}/${row.id}`);
    await loadPrices();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<string | null>(null);
const formRef = ref<FormInstance>();

const form = ref({
  name: '',
  color: '',
  price_per_hour: 0,
  sort_order: 0,
  description: ''
});

const rules: FormRules = {
  name: [
    {
      required: true,
      message: $t('vnetPages.machineGroups.nameRequired'),
      trigger: 'blur'
    }
  ]
};

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: () =>
    client.get('/machine-groups', {
      params: { search: search.value || undefined }
    }),
  transform: vnetSimpleTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.machineGroups.name'), minWidth: 150 },
    {
      prop: 'color',
      label: $t('vnetPages.machineGroups.color'),
      width: 100,
      formatter: (row: any) =>
        row.color
          ? h(ElTag, { color: row.color, style: 'color:#fff;border:none' }, () => row.color)
          : (h('span', '-') as any)
    },
    {
      prop: 'price_per_hour',
      label: $t('vnetPages.machineGroups.pricePerHour'),
      width: 130,
      align: 'center',
      formatter: (row: any) => (row.price_per_hour != null ? `${formatAmount(row.price_per_hour)}đ/h` : '-')
    },
    {
      prop: 'sort_order',
      label: $t('vnetPages.machineGroups.sortOrder'),
      width: 100,
      align: 'center'
    },
    {
      prop: 'description',
      label: $t('vnetPages.machineGroups.description'),
      minWidth: 200
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
    color: '',
    price_per_hour: 0,
    sort_order: 0,
    description: ''
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    color: row.color || '',
    price_per_hour: row.price_per_hour ?? 0,
    sort_order: row.sort_order ?? 0,
    description: row.description || ''
  };
  dialogVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    if (isEdit.value && editingId.value) {
      await client.put(`/machine-groups/${editingId.value}`, form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machineGroups.editSuccess')
      });
    } else {
      await client.post('/machine-groups', form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machineGroups.createSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machineGroups.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.machineGroups.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/machine-groups/${row.id}`);
    ElMessage.success($t('vnetPages.machineGroups.deleteSuccess'));
    getData();
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e?.message || 'Delete failed');
    }
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
            :placeholder="$t('vnetPages.common.searchPlaceholder')"
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
        />
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="270" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" type="primary" plain @click="openPricing(row)">
              {{ $t('vnetPages.machineGroups.pricing') }}
            </ElButton>
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.machineGroups.edit') : $t('vnetPages.machineGroups.create')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.machineGroups.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.color')" prop="color">
          <ElInput v-model="form.color" :placeholder="$t('vnetPages.machineGroups.colorPlaceholder')">
            <template #prefix>
              <div
                v-if="form.color"
                :style="{
                  width: '14px',
                  height: '14px',
                  borderRadius: '2px',
                  backgroundColor: form.color,
                  marginTop: '9px'
                }"
              />
            </template>
          </ElInput>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.pricePerHour')" prop="price_per_hour">
          <ElInputNumber v-model="form.price_per_hour" :min="0" :max="999999" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.sortOrder')" prop="sort_order">
          <ElInputNumber v-model="form.sort_order" :min="0" :max="999" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machineGroups.description')" prop="description">
          <ElInput v-model="form.description" type="textarea" :rows="3" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="priceVisible"
      :title="$t('vnetPages.machineGroups.pricingTitle', { name: priceGroup?.name })"
      width="880px"
    >
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{
          $t('vnetPages.machineGroups.pricingNote', {
            base: formatMoney(priceGroup?.price_per_hour)
          })
        }}
      </ElAlert>

      <ElRadioGroup v-model="priceTab" style="margin-bottom: 16px">
        <ElRadioButton value="tier">{{ $t('vnetPages.machineGroups.tierTab') }}</ElRadioButton>
        <ElRadioButton value="time">{{ $t('vnetPages.machineGroups.timeTab') }}</ElRadioButton>
      </ElRadioGroup>

      <div v-loading="priceLoading">
        <template v-if="priceTab === 'tier'">
          <div style="display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap">
            <ElSelect
              v-model="tierForm.member_group_id"
              :placeholder="$t('vnetPages.machineGroups.pickTier')"
              style="width: 160px"
            >
              <ElOption v-for="g in memberGroups" :key="g.id" :label="g.name" :value="g.id" />
            </ElSelect>
            <ElInputNumber v-model="tierForm.price_per_hour" :min="1000" :step="1000" style="width: 150px" />
            <div style="display: flex; align-items: center; gap: 6px">
              <span style="color: #909399; font-size: 13px; white-space: nowrap">
                {{ $t('vnetPages.machineGroups.minDurationShort') }}
              </span>
              <ElInputNumber
                v-model="tierForm.min_duration"
                :min="1"
                :max="1440"
                :title="$t('vnetPages.machineGroups.minDuration')"
                style="width: 110px"
              />
            </div>
            <ElDatePicker
              v-model="tierForm.effective_from"
              type="date"
              value-format="YYYY-MM-DD"
              :placeholder="$t('vnetPages.machineGroups.fromToday')"
              style="width: 150px"
            />
            <ElDatePicker
              v-model="tierForm.effective_to"
              type="date"
              value-format="YYYY-MM-DD"
              :placeholder="$t('vnetPages.machineGroups.noEnd')"
              style="width: 150px"
            />
            <ElButton type="primary" :loading="savingPrice" @click="addTierPrice">
              {{ $t('vnetPages.common.add') }}
            </ElButton>
          </div>

          <ElTable :data="tierPrices" border stripe size="small" style="width: 100%">
            <ElTableColumn :label="$t('vnetPages.machineGroups.tier')" min-width="120">
              <template #default="{ row }">{{ row.member_group_name || '-' }}</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.machineGroups.pricePerHour')" width="130">
              <template #default="{ row }">{{ formatMoney(row.price_per_hour) }}</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.machineGroups.minDurationShort')" width="90">
              <template #default="{ row }">{{ row.min_duration }}p</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.machineGroups.effective')" min-width="180">
              <template #default="{ row }">
                {{ row.effective_from }} →
                {{ row.effective_to || $t('vnetPages.machineGroups.noEnd') }}
              </template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.machineGroups.inUse')" width="110">
              <template #default="{ row }">
                <ElTag v-if="row.is_current" type="success" size="small">
                  {{ $t('vnetPages.machineGroups.current') }}
                </ElTag>
                <span v-else style="color: #909399">-</span>
              </template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.common.action')" width="80">
              <template #default="{ row }">
                <ElButton size="small" type="danger" plain @click="removePrice('tier', row)">
                  {{ $t('vnetPages.common.delete') }}
                </ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 8px">
            {{ $t('vnetPages.machineGroups.tierNote') }}
          </div>
        </template>

        <template v-else>
          <div style="display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; align-items: center">
            <ElSelect v-model="timeForm.day_of_week" style="width: 130px">
              <ElOption v-for="d in DAYS" :key="d" :label="dayLabel(d)" :value="d" />
            </ElSelect>
            <ElTimePicker v-model="timeForm.start_time" format="HH:mm" value-format="HH:mm" style="width: 110px" />
            <span>–</span>
            <ElTimePicker v-model="timeForm.end_time" format="HH:mm" value-format="HH:mm" style="width: 110px" />
            <ElInputNumber v-model="timeForm.price_per_hour" :min="1000" :step="1000" style="width: 150px" />
            <ElButton type="primary" :loading="savingPrice" @click="addTimePrice">
              {{ $t('vnetPages.common.add') }}
            </ElButton>
          </div>

          <ElTable :data="timePrices" border stripe size="small" style="width: 100%">
            <ElTableColumn :label="$t('vnetPages.machineGroups.day')" width="120">
              <template #default="{ row }">{{ dayLabel(row.day_of_week) }}</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.machineGroups.window')" min-width="150">
              <template #default="{ row }">
                <span style="font-family: ui-monospace, Menlo, Consolas, monospace">
                  {{ clock(row.start_time) }} – {{ clock(row.end_time) }}
                </span>
              </template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.machineGroups.pricePerHour')" width="130">
              <template #default="{ row }">{{ formatMoney(row.price_per_hour) }}</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.common.action')" width="80">
              <template #default="{ row }">
                <ElButton size="small" type="danger" plain @click="removePrice('time', row)">
                  {{ $t('vnetPages.common.delete') }}
                </ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 8px">
            {{ $t('vnetPages.machineGroups.timeNote') }}
          </div>
        </template>
      </div>

      <template #footer>
        <ElButton type="primary" @click="priceVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
