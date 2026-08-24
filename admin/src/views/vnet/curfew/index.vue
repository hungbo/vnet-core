<script setup lang="ts">
import { h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const DAYS = [0, 1, 2, 3, 4, 5, 6];

const loading = ref(false);
const rows = ref<any[]>([]);

function dayLabel(d: number) {
  return $t(`vnetPages.curfew.days.${d}`);
}

function clock(v: string) {
  return String(v || '').slice(0, 5);
}

async function load() {
  loading.value = true;
  try {
    const res: any = await client.get('/curfew');
    const list: any[] = Array.isArray(res) ? res : res?.items || [];
    // Xếp theo thứ để đọc như một lịch tuần, không phải theo lúc tạo.
    rows.value = list.sort((a, b) => a.day_of_week - b.day_of_week);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

const dialogVisible = ref(false);
const saving = ref(false);
const editingId = ref('');
const form = ref({
  day_of_week: 0,
  curfew_start: '22:00',
  curfew_end: '06:00',
  max_minor_hours: 2,
  is_active: true
});

function openCreate() {
  editingId.value = '';
  form.value = {
    day_of_week: 0,
    curfew_start: '22:00',
    curfew_end: '06:00',
    max_minor_hours: 2,
    is_active: true
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  form.value = {
    day_of_week: row.day_of_week,
    curfew_start: clock(row.curfew_start),
    curfew_end: clock(row.curfew_end),
    max_minor_hours: row.max_minor_hours ?? 2,
    is_active: row.is_active
  };
  dialogVisible.value = true;
}

async function submit() {
  saving.value = true;
  try {
    if (editingId.value) {
      await client.put(`/curfew/${editingId.value}`, form.value);
    } else {
      await client.post('/curfew', form.value);
    }
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.common.saved')
    });
    dialogVisible.value = false;
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.curfew.deleteConfirm', { day: dayLabel(row.day_of_week) }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/curfew/${row.id}`);
    ElMessage.success($t('vnetPages.common.deleted'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

// Miễn trừ ghi lại AI đã cho phép và VÌ SAO. Đây là quy định về trẻ vị thành
// niên, nên phải truy được người chịu trách nhiệm.
async function handleOverride(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      $t('vnetPages.curfew.overridePrompt', { day: dayLabel(row.day_of_week) }),
      $t('vnetPages.curfew.override'),
      {
        inputPlaceholder: $t('vnetPages.curfew.overridePlaceholder'),
        inputPattern: /\S/,
        inputErrorMessage: $t('vnetPages.curfew.reasonRequired')
      }
    );
    await client.post('/curfew/override', {
      policy_id: row.id,
      override_reason: value
    });
    ElNotification({
      type: 'warning',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.curfew.overridden')
    });
    load();
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message);
  }
}

onMounted(load);
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.curfew.hint') }}</span>
          <ElButton type="primary" @click="openCreate">{{ $t('vnetPages.curfew.add') }}</ElButton>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.curfew.day')" width="140">
          <template #default="{ row }">{{ dayLabel(row.day_of_week) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.curfew.window')" width="180">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace">
              {{ clock(row.curfew_start) }} – {{ clock(row.curfew_end) }}
            </span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.curfew.maxHours')" width="160">
          <template #default="{ row }">{{ row.max_minor_hours }}h</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.curfew.active')" width="110">
          <template #default="{ row }">
            <component
              :is="
                h(ElTag, { type: row.is_active ? 'success' : 'info', size: 'small' }, () =>
                  row.is_active ? $t('vnetPages.curfew.on') : $t('vnetPages.curfew.off')
                )
              "
            />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.curfew.overrideCol')" min-width="240">
          <template #default="{ row }">
            <div v-if="row.override_at">
              <div style="color: #e6a23c; font-weight: 600">
                {{ row.override_reason }}
              </div>
              <div style="color: #909399; font-size: 12px">
                {{ dayjs(row.override_at).format('DD/MM/YYYY HH:mm') }}
              </div>
            </div>
            <span v-else style="color: #909399">-</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="220" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="warning" plain @click="handleOverride(row)">
              {{ $t('vnetPages.curfew.override') }}
            </ElButton>
            <ElButton size="small" type="danger" plain @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <ElAlert type="info" :closable="false" show-icon style="margin-top: 16px">
        {{ $t('vnetPages.curfew.note') }}
      </ElAlert>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? $t('vnetPages.curfew.edit') : $t('vnetPages.curfew.add')"
      width="460px"
    >
      <ElForm label-width="170px">
        <ElFormItem :label="$t('vnetPages.curfew.day')">
          <ElSelect v-model="form.day_of_week" style="width: 100%">
            <ElOption v-for="d in DAYS" :key="d" :label="dayLabel(d)" :value="d" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.curfew.window')">
          <div style="display: flex; gap: 8px; align-items: center; width: 100%">
            <ElTimePicker v-model="form.curfew_start" format="HH:mm" value-format="HH:mm" style="width: 120px" />
            <span>–</span>
            <ElTimePicker v-model="form.curfew_end" format="HH:mm" value-format="HH:mm" style="width: 120px" />
          </div>
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 4px">
            {{ $t('vnetPages.curfew.windowHint') }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.curfew.maxHours')">
          <ElInputNumber v-model="form.max_minor_hours" :min="0" :max="24" />
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 4px">
            {{ $t('vnetPages.curfew.maxHoursHint') }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.curfew.active')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="submit">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
