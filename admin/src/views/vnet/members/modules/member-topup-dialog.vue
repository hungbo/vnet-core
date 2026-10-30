<script setup lang="ts">
// Hộp nạp tiền cho một hội viên. Dùng chung cho trang Hội viên và bảng "Máy
// đang hoạt động" ở Bảng điều khiển: khách đang ngồi máy hỏi nạp thì nhân viên
// nạp ngay tại dòng đó, không phải đi tìm hội viên ở trang khác.
import { ref, watch } from 'vue';
import { ElMessage, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { usePaymentMethods } from '@/hooks/business/payment-methods';
import { newIdempotencyKey } from '@/utils/idempotency';
import { formatAmount, moneyInput } from '@/utils/money';

defineOptions({ name: 'MemberTopupDialog' });

const props = defineProps<{ memberId: string | null; memberName?: string }>();
const emit = defineEmits<{ success: [] }>();
const visible = defineModel<boolean>('visible', { default: false });

const { t: $t } = useI18n();
const { enabledMethods } = usePaymentMethods();

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
    loadPresets();
  }
});

async function submit() {
  if (!props.memberId) return;
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
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
    <ElForm ref="formRef" :model="form" :rules="rules" :label-width="130">
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
        </ElSelect>
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
      <ElButton type="primary" :loading="submitting" @click="submit">
        {{ $t('vnetPages.common.confirm') }}
      </ElButton>
    </template>
  </ElDialog>
</template>
