<script setup lang="ts">
import { h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const loading = ref(false);
const rows = ref<any[]>([]);

function formatSize(v: number) {
  if (!v) return '-';
  return `${(v / 1024 / 1024).toFixed(1)} MB`;
}

async function load() {
  loading.value = true;
  try {
    const res: any = await client.get('/app-updates', {
      params: { page_size: 100 }
    });
    rows.value = res?.items || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

const dialogVisible = ref(false);
const saving = ref(false);
const form = ref({
  version: '',
  platform: 'windows-amd64',
  file_url: '',
  checksum: '',
  file_size: 0,
  changelog: '',
  is_required: false
});

function openCreate() {
  form.value = {
    version: '',
    platform: 'windows-amd64',
    file_url: '',
    checksum: '',
    file_size: 0,
    changelog: '',
    is_required: false
  };
  dialogVisible.value = true;
}

async function submit() {
  saving.value = true;
  try {
    await client.post('/app-updates', form.value);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.appUpdates.published', {
        version: form.value.version
      })
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
    await client.put(`/app-updates/${row.id}/active`, {
      is_active: !row.is_active
    });
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.appUpdates.deleteConfirm', { version: row.version }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/app-updates/${row.id}`);
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
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.appUpdates.hint') }}</span>
          <ElButton type="primary" @click="openCreate">{{ $t('vnetPages.appUpdates.publish') }}</ElButton>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" border stripe style="width: 100%">
        <ElTableColumn :label="$t('vnetPages.appUpdates.version')" width="110">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace">{{ row.version }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.appUpdates.platform')" width="150">
          <template #default="{ row }">{{ row.platform }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.appUpdates.size')" width="100">
          <template #default="{ row }">{{ formatSize(row.file_size) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.appUpdates.checksum')" min-width="180">
          <template #default="{ row }">
            <span style="font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 12px" :title="row.checksum">
              {{ String(row.checksum).slice(0, 16) }}…
            </span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.appUpdates.required')" width="110">
          <template #default="{ row }">
            <component
              :is="
                row.is_required
                  ? h(ElTag, { type: 'warning', size: 'small' }, () => $t('vnetPages.appUpdates.requiredTag'))
                  : h('span', '-')
              "
            />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.appUpdates.active')" width="100">
          <template #default="{ row }">
            <ElSwitch :model-value="row.is_active" @change="toggleActive(row)" />
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.appUpdates.publishedAt')" width="160">
          <template #default="{ row }">{{ dayjs(row.created_at).format('DD/MM/YYYY HH:mm') }}</template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="90" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" type="danger" plain @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog v-model="dialogVisible" :title="$t('vnetPages.appUpdates.publish')" width="560px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.appUpdates.checksumHint') }}
      </ElAlert>
      <ElForm label-width="150px">
        <ElFormItem :label="$t('vnetPages.appUpdates.version')">
          <ElInput v-model="form.version" placeholder="1.2.0" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.appUpdates.platform')">
          <ElSelect v-model="form.platform" style="width: 100%">
            <ElOption label="windows-amd64" value="windows-amd64" />
            <ElOption label="windows-arm64" value="windows-arm64" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.appUpdates.fileUrl')">
          <ElInput v-model="form.file_url" placeholder="https://.../vnet-client-amd64.exe" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.appUpdates.checksum')">
          <ElInput v-model="form.checksum" placeholder="sha256 hex, 64 ký tự" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.appUpdates.size')">
          <ElInputNumber v-model="form.file_size" :min="0" :step="1048576" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.appUpdates.changelog')">
          <ElInput v-model="form.changelog" type="textarea" :rows="3" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.appUpdates.required')">
          <ElSwitch v-model="form.is_required" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="submit">
          {{ $t('vnetPages.appUpdates.publish') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
