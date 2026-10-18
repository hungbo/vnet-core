<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Delete, UploadFilled } from '@element-plus/icons-vue';
import client from '@/api/client';

const props = defineProps<{ modelValue?: string }>();
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();

const uploading = ref(false);

async function handleUpload(options: any) {
  if (uploading.value) return;
  uploading.value = true;
  try {
    const formData = new FormData();
    formData.append('file', options.file);
    const res: any = await client.post('/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    });
    emit('update:modelValue', res.url);
  } catch (e: any) {
    ElMessage.error(e.message || 'Upload thất bại');
  } finally {
    uploading.value = false;
  }
}

function handleRemove() {
  emit('update:modelValue', '');
}
</script>

<template>
  <div class="image-upload">
    <div v-if="modelValue" class="preview">
      <ElImage :src="modelValue" fit="cover" class="preview-img" />
      <div class="overlay">
        <ElButton size="small" type="danger" circle @click="handleRemove">
          <ElIcon><Delete /></ElIcon>
        </ElButton>
      </div>
    </div>
    <ElUpload
      v-else
      :http-request="handleUpload"
      :show-file-list="false"
      accept="image/jpeg,image/png,image/gif,image/webp"
    >
      <ElButton type="primary">
        <ElIcon style="margin-right: 4px"><UploadFilled /></ElIcon>
        {{ $t('vnetPages.products.form.uploadImage') }}
      </ElButton>
    </ElUpload>
  </div>
</template>

<style scoped>
.image-upload {
  display: flex;
  align-items: center;
  gap: 12px;
}
.preview {
  position: relative;
  width: 100px;
  height: 100px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--el-border-color);
}
.preview-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.4);
  opacity: 0;
  transition: opacity 0.2s;
}
.preview:hover .overlay {
  opacity: 1;
}
</style>
