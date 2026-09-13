<script setup lang="ts">
import { h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const tab = ref<'rules' | 'violations'>('rules');

// --- Luật --------------------------------------------------------------------

const loading = ref(false);
const rules = ref<any[]>([]);
const groups = ref<any[]>([]);
const search = ref('');

const DAYS = [0, 1, 2, 3, 4, 5, 6];

function dayLabel(d: number) {
  return $t(`vnetPages.webblock.days.${d}`);
}

function scheduleText(row: any) {
  const list: any[] = row.schedules || [];
  if (!list.length) return $t('vnetPages.webblock.always');
  return list
    .map(s => {
      const days =
        (s.day_of_week || []).length === 7 || !(s.day_of_week || []).length
          ? $t('vnetPages.webblock.everyDay')
          : (s.day_of_week || []).map(dayLabel).join(', ');
      return `${days} ${String(s.start_time).slice(0, 5)}–${String(s.end_time).slice(0, 5)}`;
    })
    .join(' · ');
}

function groupText(row: any) {
  const ids: string[] = row.machine_group_ids || [];
  if (!ids.length) return $t('vnetPages.webblock.allMachines');
  return ids.map(id => groups.value.find(g => g.id === id)?.name || '?').join(', ');
}

async function load() {
  loading.value = true;
  try {
    const [r, g]: any[] = await Promise.all([
      client.get('/website-rules', {
        params: { page_size: 200, search: search.value || undefined }
      }),
      client.get('/machine-groups', { params: { page_size: 200 } })
    ]);
    rules.value = r?.items || [];
    groups.value = Array.isArray(g) ? g : g?.items || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    rules.value = [];
  } finally {
    loading.value = false;
  }
}

const dialogVisible = ref(false);
const saving = ref(false);
const editingId = ref('');
const form = ref<any>({
  pattern: '',
  rule_type: 'block',
  category: '',
  description: '',
  is_active: true,
  machine_group_ids: [] as string[],
  schedules: [] as any[]
});

function openCreate() {
  editingId.value = '';
  form.value = {
    pattern: '',
    rule_type: 'block',
    category: '',
    description: '',
    is_active: true,
    machine_group_ids: [],
    schedules: []
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  form.value = {
    pattern: row.pattern,
    rule_type: row.rule_type,
    category: row.category || '',
    description: row.description || '',
    is_active: row.is_active,
    machine_group_ids: [...(row.machine_group_ids || [])],
    schedules: (row.schedules || []).map((s: any) => ({
      day_of_week: [...(s.day_of_week || [])],
      start_time: String(s.start_time).slice(0, 5),
      end_time: String(s.end_time).slice(0, 5)
    }))
  };
  dialogVisible.value = true;
}

function addSchedule() {
  form.value.schedules.push({
    day_of_week: [...DAYS],
    start_time: '08:00',
    end_time: '22:00'
  });
}

function removeSchedule(i: number) {
  form.value.schedules.splice(i, 1);
}

async function submit() {
  if (!form.value.pattern.trim()) {
    ElMessage.warning($t('vnetPages.webblock.patternRequired'));
    return;
  }
  saving.value = true;
  try {
    const body = {
      pattern: form.value.pattern,
      rule_type: form.value.rule_type,
      category: form.value.category,
      description: form.value.description,
      is_active: form.value.is_active
    };
    const id = editingId.value
      ? (await client.put(`/website-rules/${editingId.value}`, body), editingId.value)
      : ((await client.post('/website-rules', body)) as any).id;

    // Lịch và phạm vi máy lưu riêng: cả hai đều là "thay toàn bộ", nên gửi kèm
    // mảng rỗng cũng có nghĩa (24/7 · mọi máy).
    await client.put(`/website-rules/${id}/schedules`, {
      schedules: form.value.schedules
    });
    await client.put(`/website-rules/${id}/groups`, {
      machine_group_ids: form.value.machine_group_ids
    });

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

async function toggleActive(row: any) {
  try {
    await client.put(`/website-rules/${row.id}`, { is_active: !row.is_active });
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.webblock.deleteConfirm', { pattern: row.pattern }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/website-rules/${row.id}`);
    ElMessage.success($t('vnetPages.common.deleted'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

// --- Vi phạm -----------------------------------------------------------------

const vLoading = ref(false);
const violations = ref<any[]>([]);
const vTotal = ref(0);
const vPage = ref(1);

async function loadViolations() {
  vLoading.value = true;
  try {
    const res: any = await client.get('/website-violations', {
      params: { page: vPage.value, page_size: 20 }
    });
    violations.value = res?.items || [];
    vTotal.value = res?.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    violations.value = [];
  } finally {
    vLoading.value = false;
  }
}

function switchTab(v: any) {
  tab.value = v;
  if (v === 'violations') loadViolations();
  else load();
}

onMounted(load);
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap">
          <ElRadioGroup :model-value="tab" @change="switchTab">
            <ElRadioButton value="rules">{{ $t('vnetPages.webblock.rulesTab') }}</ElRadioButton>
            <ElRadioButton value="violations">{{ $t('vnetPages.webblock.violationsTab') }}</ElRadioButton>
          </ElRadioGroup>
          <div v-if="tab === 'rules'" style="display: flex; gap: 8px">
            <ElInput
              v-model="search"
              :placeholder="$t('vnetPages.webblock.searchDomain')"
              clearable
              style="width: 200px"
              @keyup.enter="load"
              @clear="load"
            />
            <ElButton @click="load">{{ $t('vnetPages.common.search') }}</ElButton>
            <ElButton type="primary" @click="openCreate">{{ $t('vnetPages.webblock.addRule') }}</ElButton>
          </div>
        </div>
      </template>

      <ElTable v-if="tab === 'rules'" v-loading="loading" :data="rules" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.webblock.pattern')" min-width="180">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace">{{ row.pattern }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.webblock.ruleType')" width="110">
          <template #default="{ row }">
            <component
              :is="
                h(
                  ElTag,
                  {
                    type: row.rule_type === 'allow' ? 'success' : 'danger',
                    size: 'small'
                  },
                  () => $t(`vnetPages.webblock.type.${row.rule_type}`)
                )
              "
            />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.webblock.category')" width="130">
          <template #default="{ row }">{{ row.category || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.webblock.schedule')" min-width="220">
          <template #default="{ row }">{{ scheduleText(row) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.webblock.scope')" min-width="150">
          <template #default="{ row }">{{ groupText(row) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.webblock.active')" width="100">
          <template #default="{ row }">
            <ElSwitch :model-value="row.is_active" @change="toggleActive(row)" />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="150" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" plain @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <template v-else>
        <ElTable v-loading="vLoading" :data="violations" border stripe style="width: 100%">
          <ElTableColumn :label="$t('vnetPages.webblock.machine')" width="140">
            <template #default="{ row }">{{ row.machine_code || '-' }}</template>
          </ElTableColumn>
          <ElTableColumn :label="$t('vnetPages.webblock.domain')" min-width="200">
            <template #default="{ row }">
              <span style="font-family: ui-monospace, Menlo, Consolas, monospace">{{ row.domain }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="$t('vnetPages.webblock.process')" width="150">
            <template #default="{ row }">{{ row.process_name || '-' }}</template>
          </ElTableColumn>
          <ElTableColumn :label="$t('vnetPages.webblock.at')" width="170">
            <template #default="{ row }">{{ dayjs(row.blocked_at).format('DD/MM/YYYY HH:mm:ss') }}</template>
          </ElTableColumn>
        </ElTable>
        <div class="mt-16px flex justify-end">
          <ElPagination
            v-if="vTotal"
            v-model:current-page="vPage"
            layout="total, prev, pager, next"
            :page-size="20"
            :total="vTotal"
            @current-change="loadViolations"
          />
        </div>
      </template>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? $t('vnetPages.webblock.editRule') : $t('vnetPages.webblock.addRule')"
      width="620px"
    >
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.webblock.pattern')">
          <ElInput v-model="form.pattern" placeholder="facebook.com" />
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 4px">
            {{ $t('vnetPages.webblock.patternHint') }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.webblock.ruleType')">
          <ElRadioGroup v-model="form.rule_type">
            <ElRadioButton value="block">{{ $t('vnetPages.webblock.type.block') }}</ElRadioButton>
            <ElRadioButton value="allow">{{ $t('vnetPages.webblock.type.allow') }}</ElRadioButton>
          </ElRadioGroup>
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 4px">
            {{ $t('vnetPages.webblock.typeHint') }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.webblock.category')">
          <ElInput v-model="form.category" :placeholder="$t('vnetPages.webblock.categoryPlaceholder')" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.webblock.scope')">
          <ElSelect
            v-model="form.machine_group_ids"
            multiple
            clearable
            style="width: 100%"
            :placeholder="$t('vnetPages.webblock.allMachines')"
          >
            <ElOption v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </ElSelect>
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 4px">
            {{ $t('vnetPages.webblock.scopeHint') }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.webblock.schedule')">
          <div style="width: 100%">
            <div
              v-for="(s, i) in form.schedules"
              :key="i"
              style="border: 1px solid #ebeef5; border-radius: 4px; padding: 12px; margin-bottom: 8px"
            >
              <ElCheckboxGroup v-model="s.day_of_week" style="margin-bottom: 8px">
                <ElCheckbox v-for="d in DAYS" :key="d" :value="d">{{ dayLabel(d) }}</ElCheckbox>
              </ElCheckboxGroup>
              <div style="display: flex; gap: 8px; align-items: center">
                <ElTimePicker v-model="s.start_time" format="HH:mm" value-format="HH:mm" style="width: 120px" />
                <span>–</span>
                <ElTimePicker v-model="s.end_time" format="HH:mm" value-format="HH:mm" style="width: 120px" />
                <ElButton type="danger" plain size="small" @click="removeSchedule(Number(i))">
                  {{ $t('vnetPages.webblock.removeSchedule') }}
                </ElButton>
              </div>
            </div>
            <ElButton size="small" @click="addSchedule">{{ $t('vnetPages.webblock.addSchedule') }}</ElButton>
            <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 6px">
              {{ $t('vnetPages.webblock.scheduleHint') }}
            </div>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.webblock.active')">
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
