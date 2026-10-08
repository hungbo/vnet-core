<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { request } from '@/service/request';
import { useAuthStore } from '@/store/modules/auth';
import { $t } from '@/locales';

defineOptions({ name: 'ForcePasswordChange' });

// Máy chủ báo must_change_password khi tài khoản còn dùng mật khẩu mặc định của cmd/seed (lúc đăng
// nhập, và cả lúc tải lại trang). Hộp thoại này không có nút đóng, không đóng bằng Esc hay bấm ra
// ngoài: chỉ đổi xong mật khẩu hoặc đăng xuất mới thoát được. Nó gắn ở layout chứ không ở menu
// avatar để "toàn màn hình nội dung" không ẩn nó đi.
//
// Chỉ chặn giao diện. Token vẫn gọi API bình thường để script cũ đăng nhập bằng mật khẩu mặc
// định tiếp tục chạy.
const authStore = useAuthStore();

const visible = computed(() => Boolean(authStore.userInfo.must_change_password));
const submitting = ref(false);
const form = ref({ old_password: '', new_password: '', confirm: '' });

async function submit() {
  const { old_password: oldPassword, new_password: newPassword, confirm } = form.value;
  if (!oldPassword || !newPassword) {
    ElMessage.warning($t('common.changePasswordRequired'));
    return;
  }
  if (newPassword.length < 8) {
    ElMessage.warning($t('common.changePasswordTooShort'));
    return;
  }
  // Mật khẩu hiện tại của tài khoản này CHÍNH LÀ mật khẩu mặc định, nên "khác mật khẩu hiện tại"
  // cũng là "khác mật khẩu mặc định" mà không phải ghi cứng chuỗi đó vào bundle. Máy chủ vẫn
  // kiểm lại.
  if (newPassword === oldPassword) {
    ElMessage.warning($t('common.changePasswordSameAsOld'));
    return;
  }
  if (newPassword !== confirm) {
    ElMessage.warning($t('common.changePasswordMismatch'));
    return;
  }

  submitting.value = true;
  const { error } = await request({
    url: '/auth/change-password',
    method: 'put',
    data: { old_password: oldPassword, new_password: newPassword }
  });
  submitting.value = false;
  if (error) return;

  form.value = { old_password: '', new_password: '', confirm: '' };
  authStore.userInfo.must_change_password = false;
  ElMessage.success($t('common.changePasswordSuccess'));
}
</script>

<template>
  <ElDialog
    :model-value="visible"
    :title="$t('common.mustChangePasswordTitle')"
    width="420px"
    append-to-body
    :show-close="false"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
  >
    <ElAlert :title="$t('common.mustChangePasswordTip')" type="warning" :closable="false" show-icon class="mb-16px" />
    <ElForm label-width="140px" @submit.prevent>
      <ElFormItem :label="$t('common.changePasswordOld')">
        <ElInput v-model="form.old_password" type="password" show-password autocomplete="current-password" />
      </ElFormItem>
      <ElFormItem :label="$t('common.changePasswordNew')">
        <ElInput v-model="form.new_password" type="password" show-password autocomplete="new-password" />
      </ElFormItem>
      <ElFormItem :label="$t('common.changePasswordConfirm')">
        <ElInput
          v-model="form.confirm"
          type="password"
          show-password
          autocomplete="new-password"
          @keyup.enter="submit"
        />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="authStore.resetStore()">{{ $t('common.logout') }}</ElButton>
      <ElButton type="primary" :loading="submitting" @click="submit">
        {{ $t('common.confirm') }}
      </ElButton>
    </template>
  </ElDialog>
</template>
