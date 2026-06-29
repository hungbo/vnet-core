<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import { Camera, ChatDotSquare, Lock, Refresh, SwitchButton } from '@element-plus/icons-vue';
import client from '@/api/client';

const { t: $t } = useI18n();
const route = useRoute();
const machineId = route.params.id as string;

const machine = ref<any>(null);
const loading = ref(false);
const activeTab = ref('info');
const assets = ref<any[]>([]);
const assetLoading = ref(false);

const msgVisible = ref(false);
const msgText = ref('');
const sending = ref(false);

const screenshotUrl = ref('');

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

async function fetchMachine() {
  loading.value = true;
  try {
    const res: any = await client.get(`/machines/${machineId}`);
    machine.value = res;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machines.messages.loadError'));
  } finally {
    loading.value = false;
  }
}

async function fetchAssets() {
  assetLoading.value = true;
  try {
    const res: any = await client.get(`/machines/${machineId}/hardware`);
    assets.value = Array.isArray(res) ? res : res?.items || [];
  } catch {
    assets.value = [];
  } finally {
    assetLoading.value = false;
  }
}

async function remoteAction(action: string) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.machines.remote.confirmAction', { action, code: machine.value?.machine_code }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    const res: any = await client.post(`/machines/${machineId}/remote/${action}`);
    if (action === 'screenshot') {
      screenshotUrl.value = res?.url || res?.screenshot_url || '';
    }
    if (res) {
      ElMessage.success($t('vnetPages.machines.remote.actionSent', { action }));
    } else {
      ElMessage.info($t('vnetPages.machines.remote.actionSent', { action }));
    }
  } catch {}
}

function remoteMessage() {
  msgText.value = '';
  msgVisible.value = true;
}

async function sendMessage() {
  if (!msgText.value) return;
  sending.value = true;
  try {
    await client.post(`/machines/${machineId}/remote/message`, { message: msgText.value });
    ElMessage.success($t('vnetPages.machines.remote.notificationSent'));
    msgVisible.value = false;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machines.remote.notificationError'));
  } finally {
    sending.value = false;
  }
}

onMounted(() => {
  fetchMachine();
  fetchAssets();
});
</script>

<template>
  <div>
    <div style="margin-bottom: 16px">
      <ElButton @click="$router.push('/machines')">← {{ $t('vnetPages.common.back') }}</ElButton>
    </div>

    <ElCard v-loading="loading">
      <template #header>
        <span>{{ $t('vnetPages.machines.title') }}: {{ machine?.machine_code }}</span>
      </template>

      <ElTabs v-model="activeTab">
        <ElTabPane :label="$t('vnetPages.machines.tabs.info')" name="info">
          <ElDescriptions :column="2" border>
            <ElDescriptionsItem :label="$t('vnetPages.machines.code')">{{ machine?.machine_code }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.group')">
              {{ machine?.group?.name || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.common.status')">
              <ElTag :type="statusType(machine?.status)" size="small">
                {{ statusLabel(machine?.status) }}
              </ElTag>
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.ip')">
              {{ machine?.ip_address || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.cpu')">
              {{ machine?.cpu_name || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.gpu')">
              {{ machine?.gpu_name || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.ram')">{{ machine?.ram || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.disk')">{{ machine?.disk || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.os')">{{ machine?.os || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.lastSeen')">
              {{ formatDate(machine?.last_heartbeat) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.createdAt')">
              {{ formatDate(machine?.created_at) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.machines.updatedAt')">
              {{ formatDate(machine?.updated_at) }}
            </ElDescriptionsItem>
          </ElDescriptions>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.machines.tabs.sessions')" name="sessions">
          <div style="padding: 20px; text-align: center; color: #909399">
            {{ $t('vnetPages.common.comingSoon') }}
          </div>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.machines.tabs.assets')" name="assets">
          <ElTable v-loading="assetLoading" :data="assets" border stripe style="width: 100%">
            <ElTableColumn prop="name" :label="$t('vnetPages.machines.asset.name')" min-width="160" />
            <ElTableColumn prop="type" :label="$t('vnetPages.machines.asset.type')" width="100" />
            <ElTableColumn prop="serial" :label="$t('vnetPages.machines.asset.serial')" width="140" />
            <ElTableColumn prop="status" :label="$t('vnetPages.machines.asset.status')" width="100">
              <template #default="{ row }">
                <ElTag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
                  {{ row.status }}
                </ElTag>
              </template>
            </ElTableColumn>
          </ElTable>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.machines.tabs.remote')" name="remote">
          <div style="display: flex; flex-wrap: wrap; gap: 12px">
            <ElButton type="danger" @click="remoteAction('shutdown')">
              <ElIcon><SwitchButton /></ElIcon>
              {{ $t('vnetPages.machines.remote.shutdown') }}
            </ElButton>
            <ElButton type="warning" @click="remoteAction('restart')">
              <ElIcon><Refresh /></ElIcon>
              {{ $t('vnetPages.machines.remote.restart') }}
            </ElButton>
            <ElButton type="primary" @click="remoteAction('lock')">
              <ElIcon><Lock /></ElIcon>
              {{ $t('vnetPages.machines.remote.lock') }}
            </ElButton>
            <ElButton type="info" @click="remoteMessage">
              <ElIcon><ChatDotSquare /></ElIcon>
              {{ $t('vnetPages.machines.remote.message') }}
            </ElButton>
            <ElButton type="success" @click="remoteAction('screenshot')">
              <ElIcon><Camera /></ElIcon>
              {{ $t('vnetPages.machines.remote.screenshot') }}
            </ElButton>
          </div>

          <div v-if="screenshotUrl" style="margin-top: 16px">
            <ElImage :src="screenshotUrl" style="max-width: 100%" fit="contain" />
          </div>
        </ElTabPane>
      </ElTabs>
    </ElCard>

    <ElDialog v-model="msgVisible" :title="$t('vnetPages.machines.remote.sendNotification')" width="400px">
      <ElInput
        v-model="msgText"
        type="textarea"
        :rows="4"
        :placeholder="$t('vnetPages.machines.remote.notificationPlaceholder')"
      />
      <template #footer>
        <ElButton @click="msgVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="sending" @click="sendMessage">
          {{ $t('vnetPages.machines.remote.send') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
