<script setup lang="ts">
import { h, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import { useWebSocketStore } from '@/store/modules/ws';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();
const router = useRouter();
const wsStore = useWebSocketStore();

const search = ref('');

const dialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const formRef = ref<FormInstance>();

const groups = ref<any[]>([]);

const form = ref({
  machine_code: '',
  group_id: null as number | null,
  cpu_name: '',
  gpu_name: '',
  ram_gb: 8,
  storage_gb: 256,
  os_info: ''
});

const rules: FormRules = {
  machine_code: [{ required: true, message: $t('vnetPages.machines.form.codeRequired'), trigger: 'blur' }],
  group_id: [{ required: true, message: $t('vnetPages.machines.form.groupRequired'), trigger: 'change' }]
};

function statusType(status: string): any {
  const map: Record<string, string> = {
    offline: 'danger',
    available: 'success',
    in_use: 'warning',
    maintenance: 'info'
  };
  return map[status] || 'info';
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    offline: $t('vnetPages.machines.statusLabels.offline'),
    available: $t('vnetPages.machines.statusLabels.available'),
    in_use: $t('vnetPages.machines.statusLabels.inUse'),
    maintenance: $t('vnetPages.machines.statusLabels.maintenance')
  };
  return map[status] || status;
}

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

async function fetchGroups() {
  try {
    const res: any = await client.get('/machine-groups', { params: { page_size: 200 } });
    groups.value = Array.isArray(res) ? res : res?.items || [];
  } catch {
    groups.value = [];
  }
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/machines', { params: { page, page_size: pageSize, search: search.value || undefined } }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'machine_code', label: $t('vnetPages.machines.code'), width: 130 },
    {
      prop: 'group',
      label: $t('vnetPages.machines.group'),
      width: 120,
      formatter: (row: any) => row.group?.name || '-'
    },
    {
      prop: 'status',
      label: $t('vnetPages.common.status'),
      width: 110,
      formatter: (row: any) => h(ElTag, { type: statusType(row.status), size: 'small' }, () => statusLabel(row.status))
    },
    { prop: 'cpu_name', label: $t('vnetPages.machines.cpu'), minWidth: 160 },
    { prop: 'gpu_name', label: $t('vnetPages.machines.gpu'), minWidth: 160 },
    {
      prop: 'last_heartbeat',
      label: $t('vnetPages.machines.lastHeartbeat'),
      width: 160,
      formatter: (row: any) => formatDate(row.last_heartbeat)
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
    machine_code: '',
    group_id: null,
    cpu_name: '',
    gpu_name: '',
    ram_gb: 8,
    storage_gb: 256,
    os_info: ''
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    machine_code: row.machine_code || '',
    group_id: row.group?.id ?? row.group_id ?? null,
    cpu_name: row.cpu_name || '',
    gpu_name: row.gpu_name || '',
    ram_gb: row.ram_gb || 8,
    storage_gb: row.storage_gb || 256,
    os_info: row.os_info || ''
  };
  dialogVisible.value = true;
}

function viewDetail(row: any) {
  router.push(`/machines/${row.id}`);
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    if (isEdit.value && editingId.value) {
      await client.put(`/machines/${editingId.value}`, form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machines.messages.editSuccess')
      });
    } else {
      await client.post('/machines', form.value);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.machines.messages.addSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machines.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.machines.messages.deleteConfirm', { code: row.machine_code }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/machines/${row.id}`);
    ElMessage.success($t('vnetPages.machines.messages.deleteSuccess'));
    getData();
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e?.message || $t('vnetPages.machines.messages.deleteError'));
    }
  }
}

onMounted(() => {
  wsStore.on('machine:status', () => { getData(); });
});

onBeforeUnmount(() => {
  wsStore.off('machine:status');
});
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.machines.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="searchData"
          />
          <ElButton type="primary" @click="searchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @add="openCreate" @refresh="getData" />
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="260" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="viewDetail(row)">{{ $t('vnetPages.common.detail') }}</ElButton>
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
      :title="isEdit ? $t('vnetPages.machines.edit') : $t('vnetPages.machines.add')"
      width="600px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.machines.code')" prop="machine_code">
          <ElInput v-model="form.machine_code" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.group')" prop="group_id">
          <ElSelect v-model="form.group_id" style="width: 100%" filterable>
            <ElOption v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.cpu')" prop="cpu_name">
          <ElInput v-model="form.cpu_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.gpu')" prop="gpu_name">
          <ElInput v-model="form.gpu_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.ram')" prop="ram_gb">
          <ElInputNumber v-model="form.ram_gb" :min="1" :max="1024" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.disk')" prop="storage_gb">
          <ElInputNumber v-model="form.storage_gb" :min="0" :max="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.machines.os')" prop="os_info">
          <ElInput v-model="form.os_info" :placeholder="$t('vnetPages.machines.osPlaceholder')" />
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
