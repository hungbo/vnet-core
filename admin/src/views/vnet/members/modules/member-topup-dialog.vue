<script setup lang="ts">
// Hộp nạp tiền cho một hội viên. Dùng chung cho trang Hội viên và bảng "Máy
// đang hoạt động" ở Bảng điều khiển: khách đang ngồi máy hỏi nạp thì nhân viên
// nạp ngay tại dòng đó, không phải đi tìm hội viên ở trang khác.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ElMessage, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { usePaymentMethods } from '@/hooks/business/payment-methods';
import { useQRStatus } from '@/hooks/business/qr-status';
import { newIdempotencyKey } from '@/utils/idempotency';
import { formatAmount, moneyInput } from '@/utils/money';

defineOptions({ name: 'MemberTopupDialog' });

const props = defineProps<{ memberId: string | null; memberName?: string }>();
const emit = defineEmits<{ success: [] }>();
const visible = defineModel<boolean>('visible', { default: false });

const { t: $t } = useI18n();
const { enabledMethods } = usePaymentMethods();
// "QR chuyển khoản": máy chủ tạo đơn nạp chờ + mã VietQR; điện thoại báo tiền ở
// quầy thấy tiền về thì đơn tự duyệt (hoặc chờ duyệt ở trang Tiền vào).
const { status: qrStatus } = useQRStatus();
const qrReady = computed(() => Boolean(qrStatus.value?.ready));
const qr = ref<any>(null);
const qrState = ref<'waiting' | 'paid' | 'expired'>('waiting');
const now = ref(Date.now());
let qrTimer: ReturnType<typeof setInterval> | null = null;
const qrLeft = computed(() => {
  if (!qr.value) return '';
  const s = Math.max(0, Math.round((new Date(qr.value.expires_at).getTime() - now.value) / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
});

const formRef = ref<FormInstance>();
const submitting = ref(false);
const form = ref({ amount: 10000, payment_method: 'cash', idempotency_key: newIdempotencyKey() });

const rules: FormRules = {
  amount: [{ required: true, message: $t('vnetPages.members.form.amountRequired'), trigger: 'blur' }],
  payment_method: [{ required: true, message: $t('vnetPages.members.form.methodRequired'), trigger: 'change' }]
};

// Mệnh giá cài ở Cài đặt → Nạp tiền (nhóm "topup", khoá "presets", lưu dạng
// {"values": [...]}) — cùng danh sách khách thấy trên máy trạm. Chưa cài thì
// dùng đúng danh sách mặc định của máy trạm để quầy và khách thấy như nhau.
const DEFAULT_PRESETS = [5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000];
const presets = ref<number[]>(DEFAULT_PRESETS);

function parsePresets(raw: unknown): number[] {
  let v: any = raw;
  if (typeof v === 'string') {
    try {
      v = JSON.parse(v);
    } catch {
      return [];
    }
  }
  const list = Array.isArray(v) ? v : v?.values;
  return Array.isArray(list) ? list.map(Number).filter(n => Number.isFinite(n) && n > 0) : [];
}

async function loadPresets() {
  try {
    const rows: any = await client.get('/settings/topup');
    const row = (Array.isArray(rows) ? rows : []).find((r: any) => r.key === 'presets');
    const list = parsePresets(row?.value);
    presets.value = list.length ? [...list].sort((a, b) => a - b) : DEFAULT_PRESETS;
  } catch {
    presets.value = DEFAULT_PRESETS;
  }
}

// Khoá mới cho mỗi lần mở hộp thoại: đây là một ý định nạp mới. Gửi lại trong
// cùng một lần mở (bấm hai lần, mạng chập) vẫn là MỘT lần nạp.
watch(visible, open => {
  if (open) {
    form.value = { amount: 10000, payment_method: 'cash', idempotency_key: newIdempotencyKey() };
    qr.value = null;
    loadPresets();
  } else {
    stopQRTimer();
  }
});

function stopQRTimer() {
  if (qrTimer) clearInterval(qrTimer);
  qrTimer = null;
}

function qrPaid() {
  qrState.value = 'paid';
  stopQRTimer();
  ElNotification({
    type: 'success',
    title: $t('vnetPages.common.success'),
    message: $t('vnetPages.qrPay.paid', { amount: formatAmount(qr.value?.amount) })
  });
  emit('success');
  visible.value = false;
}

// Mỗi giây chạy đồng hồ đếm ngược; mỗi 5 giây hỏi lại trạng thái phòng khi lỡ
// sự kiện "qr:paid" (mất kết nối WebSocket giữa chừng).
function startQRTimer() {
  stopQRTimer();
  let n = 0;
  qrTimer = setInterval(async () => {
    now.value = Date.now();
    n += 1;
    if (n % 5 !== 0 || !qr.value) return;
    try {
      const r: any = await client.get(`/qr/requests/${qr.value.request_id}`);
      if (r?.status === 'paid') qrPaid();
      else if (r?.status === 'expired' || r?.status === 'cancelled') {
        qrState.value = 'expired';
        stopQRTimer();
      }
    } catch {}
  }, 1000);
}

const wsStore = useWebSocketStore();
function onQRPaid(data: any) {
  if (qr.value && data?.request_id === qr.value.request_id && qrState.value === 'waiting') qrPaid();
}
onMounted(() => wsStore.on('qr:paid', onQRPaid));
onBeforeUnmount(() => {
  wsStore.off('qr:paid', onQRPaid);
  stopQRTimer();
});

async function createQR() {
  submitting.value = true;
  try {
    qr.value = await client.post('/qr/topups', { member_id: props.memberId, amount: form.value.amount });
    qrState.value = 'waiting';
    now.value = Date.now();
    startQRTimer();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.messages.topUpError'));
  } finally {
    submitting.value = false;
  }
}

async function cancelQR() {
  if (!qr.value) return;
  try {
    await client.post(`/qr/requests/${qr.value.request_id}/cancel`);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    return;
  }
  stopQRTimer();
  qr.value = null;
}

async function submit() {
  if (!props.memberId) return;
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  if (form.value.payment_method === 'qr') {
    await createQR();
    return;
  }
  submitting.value = true;
  try {
    await client.post(`/members/${props.memberId}/topup`, form.value);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.members.messages.topUpSuccess')
    });
    visible.value = false;
    emit('success');
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.messages.topUpError'));
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <ElDialog
    v-model="visible"
    :title="memberName ? `${$t('vnetPages.members.topUpAmount')} — ${memberName}` : $t('vnetPages.members.topUpAmount')"
    width="400px"
    append-to-body
  >
    <div v-if="qr" class="qr-box">
      <img :src="qr.qr_image" alt="VietQR" class="qr-img" :class="{ 'qr-img--off': qrState !== 'waiting' }" />
      <div class="qr-amount">{{ formatAmount(qr.amount) }}</div>
      <div>{{ qr.bank_name }} · {{ qr.account_name || qr.account_no }}</div>
      <div>
        {{ $t('vnetPages.qrPay.content') }}:
        <b class="qr-code">{{ qr.code }}</b>
      </div>
      <ElAlert
        v-if="qrState === 'expired'"
        type="error"
        :closable="false"
        :title="$t('vnetPages.qrPay.expired')"
        class="mt-8px"
      />
      <template v-else>
        <div class="qr-wait">{{ $t('vnetPages.qrPay.waiting') }}</div>
        <div class="qr-hint">{{ $t('vnetPages.qrPay.expiresIn', { time: qrLeft }) }}</div>
        <div class="qr-hint">{{ $t('vnetPages.qrPay.scanHint') }}</div>
      </template>
    </div>
    <ElForm v-else ref="formRef" :model="form" :rules="rules" :label-width="130">
      <ElFormItem :label="$t('vnetPages.members.amount')" prop="amount">
        <ElInputNumber v-bind="moneyInput" v-model="form.amount" :min="1000" :step="10000" style="width: 100%" />
        <div class="grid grid-cols-3 mt-8px w-full gap-6px">
          <ElButton
            v-for="a in presets"
            :key="a"
            class="m-0!"
            size="small"
            :type="form.amount === a ? 'primary' : 'default'"
            @click="form.amount = a"
          >
            {{ formatAmount(a) }}
          </ElButton>
        </div>
      </ElFormItem>
      <ElFormItem :label="$t('vnetPages.members.method')" prop="payment_method">
        <ElSelect v-model="form.payment_method" style="width: 100%">
          <ElOption v-for="m in enabledMethods" :key="m.code" :label="m.name" :value="m.code" />
          <ElOption :label="$t('vnetPages.qrPay.methodQr')" value="qr" :disabled="!qrReady" />
        </ElSelect>
        <div v-if="!qrReady && qrStatus?.enabled" class="qr-hint">{{ $t('vnetPages.qrPay.notReady') }}</div>
      </ElFormItem>
    </ElForm>
    <template #footer>
      <template v-if="qr">
        <ElButton v-if="qrState === 'waiting'" type="danger" plain @click="cancelQR">
          {{ $t('vnetPages.qrPay.cancelQr') }}
        </ElButton>
        <ElButton @click="visible = false">{{ $t('common.close') }}</ElButton>
      </template>
      <template v-else>
        <ElButton @click="visible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submit">
          {{ form.payment_method === 'qr' ? $t('vnetPages.qrPay.createQr') : $t('vnetPages.common.confirm') }}
        </ElButton>
      </template>
    </template>
  </ElDialog>
</template>

<style scoped>
.qr-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  text-align: center;
}
.qr-img {
  width: 260px;
  height: 260px;
  image-rendering: pixelated;
}
.qr-img--off {
  opacity: 0.25;
}
.qr-amount {
  font-size: 22px;
  font-weight: 700;
}
.qr-code {
  font-family: monospace;
  letter-spacing: 1px;
}
.qr-wait {
  margin-top: 6px;
  font-weight: 600;
  color: var(--el-color-primary);
}
.qr-hint {
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}
</style>
