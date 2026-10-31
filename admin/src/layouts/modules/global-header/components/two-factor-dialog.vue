<script setup lang="ts">
// Tự bật / tắt xác thực 2 bước cho tài khoản đang đăng nhập (mở từ menu tài khoản).
// Bật: tạo mã QR → quét bằng ứng dụng xác thực → gõ mã 6 số để xác nhận → hiện 10
// mã dự phòng đúng một lần. Tắt: gõ lại mật khẩu.
import { ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import {
  fetchTwoFactorDisable,
  fetchTwoFactorEnable,
  fetchTwoFactorSetup,
  fetchTwoFactorStatus
} from '@/service/api';
import { $t } from '@/locales';

defineOptions({ name: 'TwoFactorDialog' });

const visible = defineModel<boolean>('visible', { default: false });

const loading = ref(false);
const status = ref<Api.Auth.TwoFactorStatus | null>(null);
const setup = ref<Api.Auth.TwoFactorSetup | null>(null);
const code = ref('');
const password = ref('');
const recovery = ref<string[]>([]);

async function load() {
  loading.value = true;
  setup.value = null;
  code.value = '';
  password.value = '';
  recovery.value = [];
  const { data, error } = await fetchTwoFactorStatus();
  loading.value = false;
  status.value = error ? null : data;
}

watch(visible, v => {
  if (v) load();
});

async function startSetup() {
  loading.value = true;
  const { data, error } = await fetchTwoFactorSetup();
  loading.value = false;
  if (!error) setup.value = data;
}

async function enable() {
  if (!code.value.trim()) return;
  loading.value = true;
  const { data, error } = await fetchTwoFactorEnable(code.value.trim());
  loading.value = false;
  if (error) return;
  recovery.value = data;
  setup.value = null;
  ElMessage.success($t('common.twoFactor.enabledSuccess'));
}

async function disable() {
  if (!password.value) return;
  loading.value = true;
  const { error } = await fetchTwoFactorDisable(password.value);
  loading.value = false;
  if (error) return;
  ElMessage.success($t('common.twoFactor.disabledSuccess'));
  await load();
}

async function copyRecovery() {
  try {
    await navigator.clipboard.writeText(recovery.value.join('\n'));
    ElMessage.success($t('common.twoFactor.copied'));
  } catch {
    // Trang mở qua http (IP trong quán) thì trình duyệt không cho chép: người dùng tự bôi đen.
  }
}

async function closeRecovery() {
  await load();
}
</script>

<template>
  <ElDialog v-model="visible" :title="$t('common.twoFactor.title')" width="460px" :close-on-click-modal="false">
    <div v-loading="loading" class="min-h-120px">
      <!-- Mã dự phòng vừa tạo: chỉ hiện đúng lần này. -->
      <template v-if="recovery.length">
        <ElAlert type="warning" :closable="false" show-icon :title="$t('common.twoFactor.recoveryTip')" />
        <div class="my-12px grid grid-cols-2 gap-8px font-mono text-15px">
          <span v-for="c in recovery" :key="c">{{ c }}</span>
        </div>
        <div class="flex justify-end gap-8px">
          <ElButton @click="copyRecovery">{{ $t('common.twoFactor.copy') }}</ElButton>
          <ElButton type="primary" @click="closeRecovery">{{ $t('common.twoFactor.saved') }}</ElButton>
        </div>
      </template>

      <template v-else-if="status && status.enabled">
        <p class="mb-12px">
          {{
            $t('common.twoFactor.enabledSince', {
              time: dayjs(status.enabled_at).format('DD/MM/YYYY HH:mm'),
              n: status.recovery_left
            })
          }}
        </p>
        <ElAlert
          v-if="!status.feature_enabled"
          class="mb-12px"
          type="info"
          :closable="false"
          :title="$t('common.twoFactor.featureOffKept')"
        />
        <ElInput
          v-model="password"
          type="password"
          show-password
          :placeholder="$t('common.twoFactor.disablePassword')"
          @keyup.enter="disable"
        />
        <div class="mt-12px flex justify-end">
          <ElButton type="danger" plain :disabled="!password" @click="disable">
            {{ $t('common.twoFactor.disable') }}
          </ElButton>
        </div>
      </template>

      <template v-else-if="status && !status.feature_enabled">
        <ElAlert type="info" :closable="false" show-icon :title="$t('common.twoFactor.featureOff')" />
      </template>

      <template v-else-if="setup">
        <p class="mb-12px">{{ $t('common.twoFactor.scanTip') }}</p>
        <div class="flex flex-col items-center gap-8px">
          <img :src="setup.qr" alt="QR" width="200" height="200" />
          <div class="text-13px">
            {{ $t('common.twoFactor.secretLabel') }}:
            <span class="select-all font-mono">{{ setup.secret }}</span>
          </div>
        </div>
        <ElInput
          v-model="code"
          class="mt-12px"
          inputmode="numeric"
          maxlength="6"
          autocomplete="one-time-code"
          :placeholder="$t('common.twoFactor.codePlaceholder')"
          @keyup.enter="enable"
        />
        <div class="mt-12px flex justify-end">
          <ElButton type="primary" :disabled="code.trim().length !== 6" @click="enable">
            {{ $t('common.twoFactor.confirmEnable') }}
          </ElButton>
        </div>
      </template>

      <template v-else-if="status">
        <p class="mb-12px">{{ $t('common.twoFactor.disabledTip') }}</p>
        <div class="flex justify-end">
          <ElButton type="primary" @click="startSetup">{{ $t('common.twoFactor.start') }}</ElButton>
        </div>
      </template>
    </div>
  </ElDialog>
</template>

<style scoped></style>
