<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ElMessage, ElTree } from 'element-plus';
import { fetchGetAllPermissions, fetchGetRolePermissions, fetchUpdateRolePermissions } from '@/service/api';
import { $t } from '@/locales';

defineOptions({ name: 'PermissionAuthModal' });

// Bản mẫu của Soybean đổ ra button1…button10 rồi bấm Lưu chỉ ghi console. Ở đây
// đọc đúng bảng permissions của backend và ghi thật vào role_permissions —
// trước nay bảng đó chỉ được ghi một lần duy nhất bởi cmd/seed.

interface Props {
  /** vai trò đang sửa */
  roleId: string;
}

const props = defineProps<Props>();

const visible = defineModel<boolean>('visible', {
  default: false
});

const title = computed(() => $t('page.manage.role.buttonAuth'));

const treeRef = ref<InstanceType<typeof ElTree>>();
const loading = ref(false);
const submitting = ref(false);
const tree = ref<any[]>([]);
const checked = ref<string[]>([]);

function buildTree(permissions: Api.SystemManage.Permission[]) {
  // Gom theo module để danh sách vài chục mã quyền còn đọc được; tick ở nhánh
  // cha là tick cả module.
  const byModule = new Map<string, Api.SystemManage.Permission[]>();
  permissions.forEach(p => {
    const key = p.module || 'khác';
    if (!byModule.has(key)) byModule.set(key, []);
    byModule.get(key)!.push(p);
  });

  return [...byModule.entries()].map(([module, items]) => ({
    id: `module:${module}`,
    label: module,
    children: items.map(p => ({
      id: p.id,
      label: p.name ? `${p.name} (${p.code})` : p.code
    }))
  }));
}

async function init() {
  if (!props.roleId) return;
  loading.value = true;
  const [all, mine] = await Promise.all([fetchGetAllPermissions(), fetchGetRolePermissions(props.roleId)]);
  loading.value = false;

  if (all.error || mine.error) {
    tree.value = [];
    checked.value = [];
    return;
  }
  tree.value = buildTree(all.data || []);
  checked.value = mine.data || [];
  // setCheckedKeys phải chạy sau khi cây đã dựng xong node.
  requestAnimationFrame(() => treeRef.value?.setCheckedKeys(checked.value, false));
}

async function handleSubmit() {
  // Chỉ lấy node lá: node cha là nhãn module, không phải mã quyền thật.
  const keys = (treeRef.value?.getCheckedKeys(true) || []) as string[];
  submitting.value = true;
  const { error } = await fetchUpdateRolePermissions(props.roleId, keys.map(String));
  submitting.value = false;
  if (error) return;
  // Quyền nằm trong token, nên người bị đổi quyền phải đăng nhập lại mới thấy.
  ElMessage.success($t('page.manage.role.permissionSaved'));
  visible.value = false;
}

watch(visible, () => {
  if (visible.value) init();
});
</script>

<template>
  <ElDialog v-model="visible" :title="title" class="w-560px">
    <ElTree
      ref="treeRef"
      v-loading="loading"
      :data="tree"
      node-key="id"
      show-checkbox
      default-expand-all
      class="h-320px overflow-y-auto"
    />
    <template #footer>
      <ElSpace class="w-full justify-end">
        <ElButton size="small" @click="visible = false">{{ $t('common.cancel') }}</ElButton>
        <ElButton type="primary" size="small" :loading="submitting" @click="handleSubmit">
          {{ $t('common.confirm') }}
        </ElButton>
      </ElSpace>
    </template>
  </ElDialog>
</template>

<style scoped></style>
