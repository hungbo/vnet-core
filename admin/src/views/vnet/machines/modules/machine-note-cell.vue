<script setup lang="ts">
// Ô ghi chú của một máy, nhấn vào để sửa tại chỗ rồi lưu ngay. Dùng chung cho
// trang Máy và bảng "Máy đang hoạt động" ở Bảng điều khiển — hai bản chép là hai
// chỗ để lệch nhau.
//
// Enter hoặc bấm ra ngoài thì lưu, Esc thì bỏ. Lưu lỗi thì giữ ô mở cùng chữ
// đang gõ để không mất công viết lại. Bảng tải lại theo thời gian thực nhưng
// ElTable có row-key nên ô này không bị dựng lại giữa lúc đang gõ.
import { nextTick, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

defineOptions({ name: 'MachineNoteCell' });

// readonly: nhân viên không có quyền sửa máy chỉ xem được ghi chú.
const props = defineProps<{ machineId: string; note?: string | null; readonly?: boolean }>();
const emit = defineEmits<{ saved: [note: string] }>();

const { t: $t } = useI18n();

const editing = ref(false);
const saving = ref(false);
const draft = ref('');
const inputRef = ref<any>(null);

async function startEdit() {
  if (editing.value || props.readonly) return;
  draft.value = props.note || '';
  editing.value = true;
  await nextTick();
  inputRef.value?.focus();
}

function cancel() {
  if (saving.value) return;
  editing.value = false;
}

// Gõ tiếng Việt bằng Telex/VNI: Enter chốt chữ đang gạch chân KHÔNG phải lệnh lưu.
// Chrome báo isComposing, Safari báo keyCode 229.
function onEnter(e: Event | KeyboardEvent) {
  const k = e as KeyboardEvent;
  if (k.isComposing || k.keyCode === 229) return;
  save();
}

async function save() {
  // Enter rồi blur (hoặc Esc rồi blur khi ô bị gỡ) không được lưu hai lần.
  if (!editing.value || saving.value) return;
  const next = draft.value.trim();
  if (next === (props.note || '')) {
    editing.value = false;
    return;
  }
  saving.value = true;
  try {
    await client.put(`/machines/${props.machineId}`, { note: next });
    emit('saved', next);
    editing.value = false;
    ElMessage.success($t('vnetPages.machines.noteSaved'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.machines.messages.saveError'));
    await nextTick();
    inputRef.value?.focus();
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <ElInput
    v-if="editing"
    ref="inputRef"
    v-model="draft"
    size="small"
    maxlength="255"
    :disabled="saving"
    :placeholder="$t('vnetPages.machines.notePlaceholder')"
    @keydown.enter="onEnter"
    @keydown.esc="cancel"
    @blur="save"
  />
  <div v-else class="machine-note" :class="{ 'is-readonly': readonly }" @click="startEdit">
    <span v-if="note">{{ note }}</span>
    <span v-else-if="!readonly" class="machine-note-empty">{{ $t('vnetPages.machines.noteAdd') }}</span>
  </div>
</template>

<style scoped>
.machine-note {
  min-height: 24px;
  padding: 2px 6px;
  border: 1px dashed transparent;
  border-radius: 4px;
  line-height: 1.4;
  word-break: break-word;
  cursor: text;
}

.machine-note:hover:not(.is-readonly) {
  border-color: var(--el-border-color);
  background: var(--el-fill-color-light);
}

.machine-note.is-readonly {
  cursor: default;
}

.machine-note-empty {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
</style>
