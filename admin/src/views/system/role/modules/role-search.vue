<script setup lang="ts">
import { ref } from 'vue';
import { $t } from '@/locales';

defineOptions({ name: 'RoleSearch' });

const activeName = ref(['role-search']);

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const model = defineModel<Api.SystemManage.RoleSearchParams>('model', { required: true });

function reset() {
  emit('reset');
}

function search() {
  emit('search');
}
</script>

<template>
  <ElCard class="card-wrapper">
    <ElCollapse v-model="activeName">
      <ElCollapseItem :title="$t('common.search')" name="role-search">
        <ElForm :model="model" label-position="right" :label-width="80">
          <ElRow :gutter="24">
            <ElCol :lg="6" :md="8" :sm="12">
              <ElFormItem :label="$t('page.manage.role.roleName')" prop="roleName">
                <ElInput v-model="model.roleName" :placeholder="$t('page.manage.role.form.roleName')" />
              </ElFormItem>
            </ElCol>
            <!-- Đã bỏ hai ô "Mã vai trò" và "Trạng thái": bảng roles không có cột
                 code (DTO trả RoleCode = role.Name) và Status bị neo cứng "1".
                 Gõ vào hai ô đó không lọc được gì, mà giao diện vẫn tỏ ra như
                 đang lọc. Muốn dùng thật thì phải thêm cột ở database trước. -->
            <ElCol :lg="6" :md="24" :sm="24">
              <ElSpace class="w-full justify-end" alignment="end">
                <ElButton @click="reset">
                  <template #icon>
                    <icon-ic-round-refresh class="text-icon" />
                  </template>
                  {{ $t('common.reset') }}
                </ElButton>
                <ElButton type="primary" plain @click="search">
                  <template #icon>
                    <icon-ic-round-search class="text-icon" />
                  </template>
                  {{ $t('common.search') }}
                </ElButton>
              </ElSpace>
            </ElCol>
          </ElRow>
        </ElForm>
      </ElCollapseItem>
    </ElCollapse>
  </ElCard>
</template>

<style scoped></style>
