<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useBoolean } from '@sa/hooks';
import { fetchAddRole, fetchUpdateRole } from '@/service/api';
import { useForm, useFormRules } from '@/hooks/common/form';
import { $t } from '@/locales';
import PermissionAuthModal from './button-auth-modal.vue';

defineOptions({ name: 'RoleOperateDrawer' });

interface Props {
  /** the type of operation */
  operateType: UI.TableOperateType;
  /** the edit row data */
  rowData?: Api.SystemManage.Role | null;
}

const props = defineProps<Props>();

interface Emits {
  (e: 'submitted'): void;
}

const emit = defineEmits<Emits>();

const visible = defineModel<boolean>('visible', {
  default: false
});

const { formRef, validate, restoreValidation } = useForm();
const { defaultRequiredRule } = useFormRules();
const { bool: permissionAuthVisible, setTrue: openPermissionAuthModal } = useBoolean();

const title = computed(() => {
  const titles: Record<UI.TableOperateType, string> = {
    add: $t('page.manage.role.addRole'),
    edit: $t('page.manage.role.editRole')
  };
  return titles[props.operateType];
});

// Bỏ roleCode và status khỏi biểu mẫu: bảng roles không có cột code (DTO trả
// RoleCode = role.Name) và toRoleManageResponse neo cứng Status = "1". Gõ vào
// hai ô đó thì dữ liệu bị vứt đi mà giao diện vẫn báo "lưu thành công" — nguy
// hiểm nhất ở ô Trạng thái: người quản lý tin rằng mình vừa khoá quyền của cả
// một nhóm nhân viên. Muốn dùng thật thì phải thêm cột ở database trước.
type Model = Pick<Api.SystemManage.Role, 'roleName' | 'roleDesc'>;

const model = ref(createDefaultModel());

function createDefaultModel(): Model {
  return {
    roleName: '',
    roleDesc: ''
  };
}

type RuleKey = Exclude<keyof Model, 'roleDesc'>;

const rules: Record<RuleKey, App.Global.FormRule> = {
  roleName: defaultRequiredRule
};

const roleId = computed(() => String(props.rowData?.id ?? ''));

const isEdit = computed(() => props.operateType === 'edit');

function handleInitModel() {
  model.value = createDefaultModel();

  if (props.operateType === 'edit' && props.rowData) {
    Object.assign(model.value, props.rowData);
  }
}

function closeDrawer() {
  visible.value = false;
}

const submitting = ref(false);

async function handleSubmit() {
  await validate();
  submitting.value = true;
  const body = {
    roleName: model.value.roleName,
    roleDesc: model.value.roleDesc
  };
  // Bản mẫu của Soybean hiện "lưu thành công" rồi không gửi gì. Chỉ báo thành
  // công sau khi backend thực sự nhận.
  const { error } = isEdit.value ? await fetchUpdateRole({ ...body, id: roleId.value }) : await fetchAddRole(body);
  submitting.value = false;
  if (error) return;
  // Thêm mới mà báo "Cập nhật thành công" thì người trực không biết mình vừa
  // tạo hay vừa sửa; khoá 'common.addSuccess' đã có sẵn trong locale.
  window.$message?.success($t(isEdit.value ? 'common.updateSuccess' : 'common.addSuccess'));
  closeDrawer();
  emit('submitted');
}

watch(visible, () => {
  if (visible.value) {
    handleInitModel();
    restoreValidation();
  }
});
</script>

<template>
  <ElDrawer v-model="visible" :title="title" :size="360">
    <ElForm ref="formRef" :model="model" :rules="rules" label-position="top">
      <ElFormItem :label="$t('page.manage.role.roleName')" prop="roleName">
        <ElInput v-model="model.roleName" :placeholder="$t('page.manage.role.form.roleName')" />
      </ElFormItem>
      <ElFormItem :label="$t('page.manage.role.roleDesc')" prop="roleDesc">
        <ElInput v-model="model.roleDesc" :placeholder="$t('page.manage.role.form.roleDesc')" />
      </ElFormItem>
    </ElForm>
    <ElSpace v-if="isEdit">
      <!--
        Nút "Phân quyền menu" của bản mẫu đã gỡ: cây menu là hằng số trong
        internal/service/route.go, không có bảng menu-theo-vai-trò để ghi, nên
        nút đó chỉ có thể hiện thông báo thành công giả.
      -->
      <ElButton @click="openPermissionAuthModal">{{ $t('page.manage.role.buttonAuth') }}</ElButton>
      <PermissionAuthModal v-model:visible="permissionAuthVisible" :role-id="roleId" />
    </ElSpace>
    <template #footer>
      <ElSpace :size="16">
        <ElButton @click="closeDrawer">{{ $t('common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</ElButton>
      </ElSpace>
    </template>
  </ElDrawer>
</template>

<style scoped></style>
