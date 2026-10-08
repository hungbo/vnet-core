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
import { moneyInput } from '@/utils/money';

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

// Khoá mới cho mỗi lần mở hộp thoại: đây là một ý định nạp mới. Gửi lại trong
// cùng một lần mở (bấm hai lần, mạng chập) vẫn là MỘT lần nạp.
watch(visible, open => {
  if (open) form.value = { amount: 10000, payment_method: 'cash', idempotency_key: newIdempotencyKey() };
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
