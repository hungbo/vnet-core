<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const ASSET_TYPES = ['monitor', 'keyboard', 'mouse', 'headset', 'chair', 'pc', 'other'];
const STATUSES = ['good', 'worn', 'broken', 'missing'];

const loading = ref(false);
const rows = ref<any[]>([]);
const machines = ref<any[]>([]);
const machineFilter = ref('');

const machineCode = computed(() => (id: string) => machines.value.find(m => m.id === id)?.machine_code || '-');

function statusTag(status: string) {
  // Không có tình trạng thì đừng tra khoá: $t('...status.undefined') không tồn
  // tại ở cả ba ngôn ngữ nên nó in nguyên khoá thô ra ô cho người dùng đọc.
  if (!status) return h('span', { style: 'color:#909399' }, '-');

  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    good: 'success',
    worn: 'warning',
    broken: 'danger',
    missing: 'info'
  };
  return h(ElTag, { type: map[status] || 'info', size: 'small' }, () => $t(`vnetPages.assets.status.${status}`));
}

async function load() {
  loading.value = true;
  try {
    const [a, m]: any[] = await Promise.all([
      client.get('/machine-assets', {
        params: { machine_id: machineFilter.value || undefined }
      }),
      client.get('/machines', { params: { page_size: 500 } })
    ]);
    rows.value = Array.isArray(a) ? a : a?.items || [];
    machines.value = Array.isArray(m) ? m : m?.items || [];
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
const form = ref<any>({
  machine_id: '',
  asset_type: 'monitor',
  brand: '',
  model: '',
  serial: '',
  status: 'good',
  notes: '',
  check_photos: [] as string[]
});

function openCreate() {
  editingId.value = '';
  form.value = {
    machine_id: machineFilter.value || '',
    asset_type: 'monitor',
    brand: '',
    model: '',
    serial: '',
    status: 'good',
    notes: '',
    check_photos: []
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  form.value = {
    machine_id: row.machine_id,
    asset_type: row.asset_type,
    brand: row.brand || '',
    model: row.model || '',
    serial: row.serial || '',
    status: row.status || 'good',
    notes: row.notes || '',
    check_photos: [...(row.check_photos || [])]
  };
  dialogVisible.value = true;
}

// Ảnh kiểm tra: tải lên qua /upload rồi giữ đường dẫn. Trường này có trong model
// từ đầu nhưng backend không nhận, nên trước đây không lưu được ảnh nào.
const uploading = ref(false);

async function uploadPhoto(file: File) {
  uploading.value = true;
  try {
    const fd = new FormData();
    fd.append('file', file);
    const res: any = await client.post('/upload', fd, {
      headers: { 'Content-Type': 'multipart/form-data' }
    });
    const url = res?.url || res?.path || res?.file_url;
    if (url) form.value.check_photos.push(url);
    else ElMessage.warning($t('vnetPages.assets.uploadNoUrl'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    uploading.value = false;
  }
  return false;
}

function removePhoto(i: number) {
  form.value.check_photos.splice(i, 1);
}

async function submit() {
  if (!form.value.machine_id) {
    ElMessage.warning($t('vnetPages.assets.pickMachine'));
    return;
  }
  saving.value = true;
  try {
    if (editingId.value) {
      await client.put(`/machine-assets/${editingId.value}`, form.value);
    } else {
      await client.post('/machine-assets', form.value);
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
      $t('vnetPages.assets.deleteConfirm', {
        type: $t(`vnetPages.assets.types.${row.asset_type}`)
      }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/machine-assets/${row.id}`);
    ElMessage.success($t('vnetPages.common.deleted'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

onMounted(load);
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.assets.hint') }}</span>
          <div style="display: flex; gap: 8px">
            <ElSelect
              v-model="machineFilter"
              filterable
              clearable
              :placeholder="$t('vnetPages.assets.allMachines')"
              style="width: 200px"
              @change="load"
            >
              <ElOption v-for="m in machines" :key="m.id" :label="m.machine_code" :value="m.id" />
            </ElSelect>
            <ElButton type="primary" @click="openCreate">{{ $t('vnetPages.assets.add') }}</ElButton>
          </div>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.assets.machine')" width="120">
          <template #default="{ row }">{{ machineCode(row.machine_id) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.assets.type')" width="130">
          <!--
 row rỗng: el-table vẫn dựng ô mẫu một lần khi bảng chưa có dữ liệu, và
               $t('...types.undefined') in thẳng khoá thô ra ô. 
-->
          <template #default="{ row }">
            {{ row.asset_type ? $t(`vnetPages.assets.types.${row.asset_type}`) : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.assets.brandModel')" min-width="180">
          <template #default="{ row }">{{ [row.brand, row.model].filter(Boolean).join(' ') || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.assets.serial')" min-width="140">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace">{{ row.serial || '-' }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.assets.statusLabel')" width="120">
          <template #default="{ row }"><component :is="statusTag(row.status)" /></template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.assets.photos')" width="110">
          <template #default="{ row }">
            <span v-if="(row.check_photos || []).length">{{ row.check_photos.length }} 📷</span>
            <span v-else style="color: #909399">-</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.assets.checkedAt')" width="160">
          <template #default="{ row }">
            {{ row.checked_at ? dayjs(row.checked_at).format('DD/MM/YYYY HH:mm') : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="150" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.assets.check') }}</ElButton>
            <ElButton size="small" type="danger" plain @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? $t('vnetPages.assets.check') : $t('vnetPages.assets.add')"
      width="560px"
    >
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.assets.machine')">
          <ElSelect v-model="form.machine_id" filterable style="width: 100%">
            <ElOption v-for="m in machines" :key="m.id" :label="m.machine_code" :value="m.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.assets.type')">
          <ElSelect v-model="form.asset_type" style="width: 100%">
            <ElOption v-for="tp in ASSET_TYPES" :key="tp" :label="$t(`vnetPages.assets.types.${tp}`)" :value="tp" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.assets.brand')">
          <ElInput v-model="form.brand" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.assets.model')">
          <ElInput v-model="form.model" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.assets.serial')">
          <ElInput v-model="form.serial" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.assets.statusLabel')">
          <ElSelect v-model="form.status" style="width: 100%">
            <ElOption v-for="st in STATUSES" :key="st" :label="$t(`vnetPages.assets.status.${st}`)" :value="st" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.assets.photos')">
          <div style="width: 100%">
            <div v-if="form.check_photos.length" style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px">
              <div v-for="(p, i) in form.check_photos" :key="i" style="position: relative">
                <ElImage :src="p" fit="cover" style="width: 72px; height: 72px; border-radius: 4px" />
                <ElButton
                  size="small"
                  type="danger"
                  circle
                  style="position: absolute; top: -6px; right: -6px"
                  @click="removePhoto(Number(i))"
                >
                  ×
                </ElButton>
              </div>
            </div>
            <ElUpload :show-file-list="false" :before-upload="uploadPhoto" accept="image/*">
              <ElButton size="small" :loading="uploading">{{ $t('vnetPages.assets.addPhoto') }}</ElButton>
            </ElUpload>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.assets.notes')">
          <ElInput v-model="form.notes" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="submit">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
