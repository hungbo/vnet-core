<script setup lang="ts">
// Điện thoại báo tiền ở quầy (app vnet-bank-notification): trạng thái, ghép bằng mã 6 số,
// gỡ. Ghép lại là thay khoá — điện thoại cũ hết gửi được.
import { computed, onBeforeUnmount, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAuth } from '@/hooks/business/auth';
import { useQRStatus } from '@/hooks/business/qr-status';

defineOptions({ name: 'QrPhonePanel' });

const { t: $t } = useI18n();
const { hasAuth } = useAuth();
const { status, reload } = useQRStatus();

const pairing = ref<{ code: string; expires_at: string } | null>(null);
const now = ref(Date.now());
const tick = setInterval(() => {
  now.value = Date.now();
  if (pairing.value && dayjs(pairing.value.expires_at).valueOf() <= now.value) pairing.value = null;
}, 1000);
onBeforeUnmount(() => clearInterval(tick));

const pairLeft = computed(() => {
  if (!pairing.value) return '';
  const s = Math.max(0, Math.round((dayjs(pairing.value.expires_at).valueOf() - now.value) / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
});

// Điện thoại gõ địa chỉ này: chính địa chỉ trang quản lý đang mở (cùng máy chủ).
const serverAddress = window.location.host;

function when(v: string | null | undefined) {
  return v ? dayjs(v).format('DD/MM HH:mm:ss') : $t('vnetPages.qrPay.never');
}

async function startPairing() {
  try {
    pairing.value = (await client.post('/qr/pairing')) as any;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function unpair() {
  try {
    await ElMessageBox.confirm($t('vnetPages.qrPay.unpairConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return;
  }
  try {
    await client.delete('/qr/device');
    await reload();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}
</script>

<template>
  <ElCard shadow="never" class="mb-16px">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-8px">
        <span>{{ $t('vnetPages.qrPay.phone') }}</span>
        <div class="flex flex-wrap items-center gap-8px">
          <ElTag v-if="!status?.paired" type="info">{{ $t('vnetPages.qrPay.notPaired') }}</ElTag>
          <ElTag v-else :type="status.phone_online ? 'success' : 'danger'">
            {{ status.phone_online ? $t('vnetPages.qrPay.online') : $t('vnetPages.qrPay.offline') }}
          </ElTag>
          <template v-if="hasAuth('settings.edit')">
            <ElButton size="small" type="primary" @click="startPairing">
              {{ status?.paired ? $t('vnetPages.qrPay.repair') : $t('vnetPages.qrPay.pair') }}
            </ElButton>
            <ElButton v-if="status?.paired" size="small" type="danger" plain @click="unpair">
              {{ $t('vnetPages.qrPay.unpair') }}
            </ElButton>
          </template>
        </div>
      </div>
    </template>

    <ElAlert v-if="pairing" type="warning" :closable="false" class="mb-12px">
      <div class="pair-code">{{ pairing.code }}</div>
      <div>
        {{ $t('vnetPages.qrPay.serverAddress') }}:
        <b>{{ serverAddress }}</b>
      </div>
      <div>{{ $t('vnetPages.qrPay.pairCodeHint', { time: pairLeft }) }}</div>
    </ElAlert>

    <ElDescriptions v-if="status?.paired" :column="2" border size="small">
      <ElDescriptionsItem :label="$t('vnetPages.qrPay.lastSeen')">{{ when(status.last_seen) }}</ElDescriptionsItem>
      <ElDescriptionsItem :label="$t('vnetPages.qrPay.lastNotif')">
        {{ when(status.last_notification_at) }}
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="$t('vnetPages.qrPay.appVersion')">{{ status.app_version || '-' }}</ElDescriptionsItem>
      <ElDescriptionsItem :label="$t('vnetPages.qrPay.battery')">
        <template v-if="status.last_seen">
          {{ status.battery }}%
          <span v-if="status.charging">({{ $t('vnetPages.qrPay.charging') }})</span>
        </template>
        <template v-else>-</template>
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="$t('vnetPages.qrPay.listener')">
        <ElTag v-if="status.last_seen" :type="status.listener_ok ? 'success' : 'danger'" size="small">
          {{ status.listener_ok ? $t('vnetPages.qrPay.listenerOk') : $t('vnetPages.qrPay.listenerMissing') }}
        </ElTag>
        <span v-else>-</span>
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="$t('vnetPages.qrPay.queue')">{{ status.queue }}</ElDescriptionsItem>
      <ElDescriptionsItem :label="$t('vnetPages.qrPay.connection')">
        <template v-if="status.phone_online">
          {{ status.ws_connected ? $t('vnetPages.qrPay.connectionLive') : $t('vnetPages.qrPay.connectionHeartbeat') }}
        </template>
        <template v-else>-</template>
      </ElDescriptionsItem>
    </ElDescriptions>
  </ElCard>
</template>

<style scoped>
.pair-code {
  font-size: 32px;
  font-weight: 700;
  letter-spacing: 8px;
  font-family: monospace;
}
</style>
