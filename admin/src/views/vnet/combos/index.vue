<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const formRef = ref<FormInstance>();

const form = ref({
  name: '',
  type: 'fixed_slot',
  price: 0,
  minutes: 60,
  slots: [{ start: null, end: null }],
  is_active: true
});

const rules: FormRules = {
  name: [{ required: true, message: $t('vnetPages.combos.form.nameRequired'), trigger: 'blur' }],
  type: [{ required: true, message: $t('vnetPages.combos.form.typeRequired'), trigger: 'change' }],
  price: [{ required: true, message: $t('vnetPages.combos.form.priceRequired'), trigger: 'blur' }],
  minutes: [{ required: true, message: $t('vnetPages.combos.form.minutesRequired'), trigger: 'blur' }]
};

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/combos', { params: { page, page_size: pageSize, search: search.value || undefined } }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.combos.name'), minWidth: 160 },
    {
      prop: 'type',
      label: $t('vnetPages.combos.type'),
      width: 110,
      formatter: (row: any) =>
        h(ElTag, { type: row.type === 'fixed_slot' ? 'warning' : 'success', size: 'small' }, () =>
          row.type === 'fixed_slot' ? $t('vnetPages.combos.fixedSlot') : $t('vnetPages.combos.prepaid')
        )
    },
    {
      prop: 'price',
      label: $t('vnetPages.combos.price'),
      width: 110,
      formatter: (row: any) => row.price?.toLocaleString() ?? ''
    },
    {
      prop: 'is_active',
      label: $t('vnetPages.combos.isActive'),
      width: 90,
      formatter: (row: any) =>
        h(ElTag, { type: row.is_active ? 'success' : 'danger', size: 'small' }, () =>
          row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no')
        )
    }
  ]
});

function fetchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = {
    name: '',
    type: 'fixed_slot',
    price: 0,
    minutes: 60,
    slots: [{ start: null, end: null }],
    is_active: true
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    type: row.type || 'fixed_slot',
    price: row.price ?? 0,
    minutes: row.minutes ?? 60,
    slots: row.slots?.length
      ? row.slots.map((s: any) => ({ start: s.start, end: s.end }))
      : [{ start: null, end: null }],
    is_active: row.is_active ?? true
  };
  dialogVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    const payload: any = {
      name: form.value.name,
      type: form.value.type,
      price: form.value.price,
      is_active: form.value.is_active
    };
    if (form.value.type === 'fixed_slot') {
      payload.slot_start = form.value.slots?.[0]?.start ? dayjs(form.value.slots[0].start).format('HH:mm') : '';
      payload.slot_end = form.value.slots?.[0]?.end ? dayjs(form.value.slots[0].end).format('HH:mm') : '';
    } else {
      payload.total_minutes = form.value.minutes;
    }
    if (isEdit.value && editingId.value) {
      await client.put(`/combos/${editingId.value}`, payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.combos.messages.editSuccess')
      });
    } else {
      await client.post('/combos', payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.combos.messages.addSuccess')
      });
    }
    dialogVisible.value = false;
    fetchData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.combos.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.combos.messages.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/combos/${row.id}`);
    ElMessage.success($t('vnetPages.combos.messages.deleteSuccess'));
    fetchData();
  } catch {}
}
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.combos.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="fetchData"
          />
          <ElButton type="primary" @click="fetchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @add="openCreate" @refresh="getData" />
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
      :title="isEdit ? $t('vnetPages.combos.edit') : $t('vnetPages.combos.add')"
      width="550px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.combos.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.type')" prop="type">
          <ElSelect v-model="form.type" style="width: 100%">
            <ElOption :label="$t('vnetPages.combos.fixedSlot')" value="fixed_slot" />
            <ElOption :label="$t('vnetPages.combos.prepaid')" value="prepaid" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.combos.price')" prop="price">
          <ElInputNumber v-model="form.price" :min="0" :step="10000" style="width: 100%" />
        </ElFormItem>
        <template v-if="form.type === 'fixed_slot'">
          <ElFormItem v-for="(slot, idx) in form.slots" :key="idx" :label="`Slot ${idx + 1}`">
            <div style="display: flex; gap: 8px; width: 100%">
              <ElTimePicker
                v-model="slot.start"
                format="HH:mm"
                :placeholder="$t('vnetPages.combos.from')"
                style="flex: 1"
              />
              <ElTimePicker
                v-model="slot.end"
                format="HH:mm"
                :placeholder="$t('vnetPages.combos.to')"
                style="flex: 1"
              />
              <ElButton icon="Delete" @click="form.slots.splice(idx, 1)" />
            </div>
          </ElFormItem>
          <ElFormItem label=" ">
            <ElButton type="primary" link @click="form.slots.push({ start: null, end: null })">
              {{ $t('vnetPages.combos.addSlot') }}
            </ElButton>
          </ElFormItem>
        </template>
        <template v-if="form.type === 'prepaid'">
          <ElFormItem :label="$t('vnetPages.combos.minutes')" prop="minutes">
            <ElInputNumber v-model="form.minutes" :min="1" :step="30" style="width: 100%" />
          </ElFormItem>
        </template>
        <ElFormItem :label="$t('vnetPages.combos.isActive')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
