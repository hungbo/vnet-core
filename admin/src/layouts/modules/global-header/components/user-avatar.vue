<script setup lang="ts">
import { computed, ref } from 'vue';
import type { VNode } from 'vue';
import { ElMessage } from 'element-plus';
import { useAuthStore } from '@/store/modules/auth';
import { request } from '@/service/request';
import { useRouterPush } from '@/hooks/common/router';
import { useSvgIcon } from '@/hooks/common/icon';
import { $t } from '@/locales';

defineOptions({ name: 'UserAvatar' });

const authStore = useAuthStore();
const { routerPushByKey, toLogin } = useRouterPush();
const { SvgIconVNode } = useSvgIcon();

function loginOrRegister() {
  toLogin();
}

type DropdownKey = 'user-center' | 'change-password' | 'logout';

type DropdownOption = {
  key: DropdownKey;
  label: string;
  icon?: () => VNode;
};

const options = computed(() => {
  const opts: DropdownOption[] = [
    {
      label: $t('common.userCenter'),
      key: 'user-center',
      icon: SvgIconVNode({ icon: 'ph:user-circle', fontSize: 18 })
    },
    {
      label: $t('common.changePassword'),
      key: 'change-password',
      icon: SvgIconVNode({ icon: 'ph:key', fontSize: 18 })
    },
    {
      label: $t('common.logout'),
      key: 'logout',
      icon: SvgIconVNode({ icon: 'ph:sign-out', fontSize: 18 })
    }
  ];

  return opts;
});

function logout() {
  window.$messageBox
    ?.confirm($t('common.logoutConfirm'), $t('common.tip'), {
      confirmButtonText: $t('common.confirm'),
      cancelButtonText: $t('common.cancel'),
      type: 'warning'
    })
    .then(() => {
      authStore.resetStore();
    });
}

// --- Đổi mật khẩu -------------------------------------------------------------
// PUT /auth/change-password đã có từ lâu và chạy đúng cho cả nhân viên lẫn hội
// viên, nhưng trong toàn bộ admin không có một chỗ nào gọi tới — người dùng
// không có đường nào tự đổi mật khẩu.

const pwdVisible = ref(false);
const pwdSubmitting = ref(false);
const pwdForm = ref({ old_password: '', new_password: '', confirm: '' });

function openChangePassword() {
  pwdForm.value = { old_password: '', new_password: '', confirm: '' };
  pwdVisible.value = true;
}

async function submitChangePassword() {
  const { old_password: oldPassword, new_password: newPassword, confirm } = pwdForm.value;
  if (!oldPassword || !newPassword) {
    ElMessage.warning($t('common.changePasswordRequired'));
    return;
  }
  if (newPassword.length < 6) {
    ElMessage.warning($t('common.changePasswordTooShort'));
    return;
  }
  if (newPassword !== confirm) {
    ElMessage.warning($t('common.changePasswordMismatch'));
    return;
  }

  pwdSubmitting.value = true;
  const { error } = await request({
    url: '/auth/change-password',
    method: 'put',
    data: { old_password: oldPassword, new_password: newPassword }
  });
  pwdSubmitting.value = false;
  if (error) return;

  ElMessage.success($t('common.changePasswordSuccess'));
  pwdVisible.value = false;
}

function handleDropdown(key: DropdownKey) {
  if (key === 'logout') {
    logout();
  } else if (key === 'change-password') {
    openChangePassword();
  } else {
    // If your other options are jumps from other routes, they will be directly supported here
    routerPushByKey(key);
  }
}
</script>

<template>
  <ElButton v-if="!authStore.isLogin" text @click="loginOrRegister">
    {{ $t('page.login.common.loginOrRegister') }}
  </ElButton>

  <ElDropdown class="px-14px" trigger="click" @command="handleDropdown">
    <template #dropdown>
      <ElDropdownMenu>
        <ElDropdownItem
          v-for="{ key, label, icon } in options"
          :key="key"
          class="mx-4px my-1px rounded-6px"
          :icon="icon"
          :command="key"
        >
          {{ label }}
        </ElDropdownItem>
      </ElDropdownMenu>
    </template>
    <div class="flex items-center">
      <SvgIcon icon="ph:user-circle" class="mr-5px text-icon-large" />
      <span class="text-16px font-medium">{{ authStore.userInfo.username }}</span>
    </div>
  </ElDropdown>

  <ElDialog v-model="pwdVisible" :title="$t('common.changePassword')" width="420px">
    <ElForm label-width="140px">
      <ElFormItem :label="$t('common.changePasswordOld')">
        <ElInput v-model="pwdForm.old_password" type="password" show-password />
      </ElFormItem>
      <ElFormItem :label="$t('common.changePasswordNew')">
        <ElInput v-model="pwdForm.new_password" type="password" show-password />
      </ElFormItem>
      <ElFormItem :label="$t('common.changePasswordConfirm')">
        <ElInput v-model="pwdForm.confirm" type="password" show-password @keyup.enter="submitChangePassword" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="pwdVisible = false">{{ $t('common.cancel') }}</ElButton>
      <ElButton type="primary" :loading="pwdSubmitting" @click="submitChangePassword">
        {{ $t('common.confirm') }}
      </ElButton>
    </template>
  </ElDialog>
</template>

<style scoped></style>
