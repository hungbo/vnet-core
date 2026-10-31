<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { useAuthStore } from '@/store/modules/auth';
import { useForm, useFormRules } from '@/hooks/common/form';
import { $t } from '@/locales';

defineOptions({ name: 'PwdLogin' });

const authStore = useAuthStore();
const { formRef, validate } = useForm();

interface FormModel {
  userName: string;
  password: string;
}

// Điền sẵn tài khoản seed chỉ để tiện khi dev. Bản build KHÔNG được mang mật khẩu mặc định:
// ai mở trang đăng nhập từ máy khách chỉ cần bấm Xác nhận là vào được. `import.meta.env.DEV` là
// hằng số lúc build nên nhánh này bị loại hẳn khỏi bundle production.
const model = ref<FormModel>(
  import.meta.env.DEV ? { userName: 'admin', password: 'admin123' } : { userName: '', password: '' }
);

const rules = computed<Record<keyof FormModel, App.Global.FormRule[]>>(() => {
  const { formRules } = useFormRules();

  return {
    userName: formRules.userName,
    password: formRules.pwd
  };
});

async function handleSubmit() {
  await validate();
  await authStore.login(model.value.userName, model.value.password);
}

// Bước 2 khi tài khoản bật xác thực 2 bước: mã 6 số, hoặc một mã dự phòng "abcd-efgh".
// Quán bắt buộc mà tài khoản chưa cài thì quét QR trước, mã đúng là cài xong và hiện
// 10 mã dự phòng trước khi vào trong.
const code = ref('');
const codeInput = ref<{ focus: () => void } | null>(null);

watch(
  () => authStore.totpChallenge,
  async v => {
    code.value = '';
    if (v) {
      await nextTick();
      codeInput.value?.focus();
    }
  }
);

async function handleCode() {
  if (!code.value.trim()) return;
  await authStore.loginTotp(code.value.trim());
}

async function copyRecovery() {
  try {
    await navigator.clipboard.writeText(authStore.recoveryCodes.join('\n'));
    ElMessage.success($t('common.twoFactor.copied'));
  } catch {
    // Trang mở qua http (IP trong quán) thì trình duyệt không cho chép: người dùng tự bôi đen.
  }
}
</script>

<template>
  <div v-if="authStore.recoveryCodes.length">
    <ElAlert type="warning" :closable="false" show-icon :title="$t('common.twoFactor.recoveryTip')" />
    <div class="grid grid-cols-2 my-16px gap-8px text-15px font-mono">
      <span v-for="c in authStore.recoveryCodes" :key="c">{{ c }}</span>
    </div>
    <ElSpace direction="vertical" :size="16" class="w-full" fill>
      <ElButton
        type="primary"
        size="large"
        round
        block
        :loading="authStore.loginLoading"
        @click="authStore.confirmRecovery()"
      >
        {{ $t('common.twoFactor.saved') }}
      </ElButton>
      <ElButton size="large" round block @click="copyRecovery">{{ $t('common.twoFactor.copy') }}</ElButton>
    </ElSpace>
  </div>
  <div v-else-if="authStore.totpChallenge">
    <template v-if="authStore.totpSetup">
      <p class="mb-12px text-14px text-gray-500">{{ $t('common.twoFactor.setupLoginTip') }}</p>
      <div class="mb-16px flex flex-col items-center gap-8px">
        <img :src="authStore.totpSetup.qr" alt="QR" width="180" height="180" />
        <div class="text-13px">
          {{ $t('common.twoFactor.secretLabel') }}:
          <span class="select-all font-mono">{{ authStore.totpSetup.secret }}</span>
        </div>
      </div>
    </template>
    <p v-else class="mb-16px text-14px text-gray-500">{{ $t('common.twoFactor.loginTip') }}</p>
    <ElForm size="large" :show-label="false" @submit.prevent="handleCode">
      <ElFormItem>
        <ElInput
          ref="codeInput"
          v-model="code"
          autocomplete="one-time-code"
          inputmode="numeric"
          :maxlength="authStore.totpSetup ? 6 : 9"
          :placeholder="$t('common.twoFactor.codePlaceholder')"
        />
      </ElFormItem>
      <ElSpace direction="vertical" :size="16" class="w-full" fill>
        <ElButton type="primary" size="large" round block :loading="authStore.loginLoading" @click="handleCode">
          {{ $t('common.confirm') }}
        </ElButton>
        <ElButton size="large" round block @click="authStore.cancelTotp()">
          {{ $t('page.login.common.back') }}
        </ElButton>
      </ElSpace>
    </ElForm>
  </div>
  <ElForm
    v-else
    ref="formRef"
    :model="model"
    :rules="rules"
    size="large"
    :show-label="false"
    @keyup.enter="handleSubmit"
  >
    <ElFormItem prop="userName">
      <ElInput v-model="model.userName" :placeholder="$t('page.login.common.userNamePlaceholder')" />
    </ElFormItem>
    <ElFormItem prop="password">
      <ElInput
        v-model="model.password"
        type="password"
        show-password-on="click"
        :placeholder="$t('page.login.common.passwordPlaceholder')"
      />
    </ElFormItem>
    <ElSpace direction="vertical" :size="24" class="w-full" fill>
      <ElButton type="primary" size="large" round block :loading="authStore.loginLoading" @click="handleSubmit">
        {{ $t('common.confirm') }}
      </ElButton>
    </ElSpace>
  </ElForm>
</template>

<style scoped></style>
