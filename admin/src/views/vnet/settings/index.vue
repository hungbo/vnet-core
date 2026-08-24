<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();

const loading = ref(false);
const saving = ref(false);
const activeTab = ref('general');
const settings = reactive<Record<string, any>>({});

// Mệnh giá nạp lưu dưới dạng jsonb `{"values": [...]}` nên không gắn thẳng vào
// một ô nhập được; giữ riêng thành mảng số rồi đóng/mở gói lúc đọc/ghi.
const presets = ref<number[]>([]);

function parsePresets(raw: unknown): number[] {
  if (Array.isArray(raw)) return raw.map(Number).filter(n => Number.isFinite(n));
  if (typeof raw !== 'string' || !raw) return [];
  try {
    const parsed = JSON.parse(raw);
    const list = Array.isArray(parsed) ? parsed : parsed?.values;
    return Array.isArray(list) ? list.map(Number).filter(n => Number.isFinite(n)) : [];
  } catch {
    return [];
  }
}

async function fetchSettings() {
  loading.value = true;
  // Mỗi tab là một nhóm cài đặt riêng; xoá sạch khoá của tab trước để không
  // lẫn khoá nhóm này sang nhóm kia lúc bấm Lưu.
  Object.keys(settings).forEach(k => Reflect.deleteProperty(settings, k));
  presets.value = [];
  try {
    // The API returns a list of {key, value} rows, not an object. Assigning the
    // list straight onto a reactive object produced numeric indices and left
    // every named field undefined, so the form always rendered empty.
    const res: any = await client.get(`/settings/${activeTab.value}`);
    const rows: any[] = Array.isArray(res) ? res : res?.items || [];
    rows.forEach(row => {
      if (row?.key !== undefined) settings[row.key] = row.value;
    });
    if (activeTab.value === 'topup') presets.value = parsePresets(settings.presets);
    if (activeTab.value === 'features') applyFeatureDefaults();
  } catch (e: any) {
    // A group with no rows yet is a normal first-run state, not an error:
    // the form stays blank and saving creates the rows.
    if (e?.status !== 404 && e?.message) ElMessage.error(e.message);
    if (activeTab.value === 'features') applyFeatureDefaults();
  } finally {
    loading.value = false;
  }
}

// Nhóm "features" chưa tồn tại cho tới lần Lưu đầu tiên, và backend mặc định
// BẬT khi thiếu khoá. Không điền mặc định ở đây thì công tắc hiện TẮT trong khi
// tính năng vẫn đang chạy — hai bên nói ngược nhau.
function applyFeatureDefaults() {
  ['attendance_enabled', 'feedback_enabled'].forEach(k => {
    if (settings[k] !== 'true' && settings[k] !== 'false') settings[k] = 'true';
  });
}

function addPreset() {
  presets.value.push(10000);
}

function removePreset(idx: number) {
  presets.value.splice(idx, 1);
}

async function handleSave() {
  saving.value = true;
  const body: Record<string, any> =
    activeTab.value === 'topup'
      ? // Máy khách đọc đúng khoá `presets` của nhóm `topup`; giữ nguyên dạng
        // {"values": [...]} mà seed đang lưu để không phải chạy lại seed.
        { presets: { values: [...presets.value].sort((a, b) => a - b) } }
      : { ...settings };
  try {
    await client.put(`/settings/${activeTab.value}`, body);
    ElMessage.success($t('vnetPages.settings.messages.saveSuccess'));
    if (activeTab.value === 'topup') await fetchSettings();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.settings.messages.saveError'));
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  fetchSettings();
});
</script>

<template>
  <div>
    <ElCard v-loading="loading">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>{{ $t('vnetPages.settings.title') }}</span>
          <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
        </div>
      </template>
      <ElTabs v-model="activeTab" @tab-change="fetchSettings">
        <ElTabPane :label="$t('vnetPages.settings.general')" name="general">
          <ElForm :model="settings" label-width="180px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.storeName')">
              <ElInput v-model="settings.store_name" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.address')">
              <ElInput v-model="settings.store_address" type="textarea" :rows="2" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.phone')">
              <ElInput v-model="settings.store_phone" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.email')">
              <ElInput v-model="settings.store_email" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.timezone')">
              <ElSelect v-model="settings.timezone" style="width: 100%">
                <ElOption label="Asia/Ho_Chi_Minh (UTC+7)" value="Asia/Ho_Chi_Minh" />
                <ElOption label="Asia/Ha_Noi (UTC+7)" value="Asia/Ha_Noi" />
              </ElSelect>
            </ElFormItem>
          </ElForm>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.settings.limits')" name="limits">
          <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
            {{ $t('vnetPages.settings.limitsHint') }}
          </ElAlert>
          <ElForm :model="settings" label-width="180px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.maxBookingsPerDay')">
              <ElInputNumber v-model="settings.max_bookings_per_day" :min="0" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.maxBookingsPerMember')">
              <ElInputNumber v-model="settings.max_bookings_per_member" :min="0" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.cancelBeforeMinutes')">
              <ElInputNumber v-model="settings.cancel_before_minutes" :min="0" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.maxDebt')">
              <ElInputNumber v-model="settings.max_debt" :min="0" :step="10000" :precision="0" style="width: 100%" />
            </ElFormItem>
          </ElForm>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.settings.invoice')" name="invoice">
          <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
            {{ $t('vnetPages.settings.invoiceHint') }}
          </ElAlert>
          <ElForm :model="settings" label-width="180px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.invoiceTitle')">
              <ElInput v-model="settings.invoice_title" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.invoiceFooter')">
              <ElInput v-model="settings.invoice_footer" type="textarea" :rows="2" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.taxCode')">
              <ElInput v-model="settings.tax_code" />
            </ElFormItem>
          </ElForm>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.settings.topup')" name="topup">
          <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
            {{ $t('vnetPages.settings.topupHint') }}
          </ElAlert>
          <ElForm label-width="180px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.topupPresets')">
              <div style="display: flex; flex-wrap: wrap; gap: 8px; width: 100%">
                <div v-for="(_, idx) in presets" :key="idx" style="display: flex; gap: 4px">
                  <ElInputNumber v-model="presets[idx]" :min="1000" :step="10000" style="width: 150px" />
                  <ElButton type="danger" plain @click="removePreset(idx)">
                    {{ $t('vnetPages.common.delete') }}
                  </ElButton>
                </div>
                <ElButton @click="addPreset">{{ $t('vnetPages.settings.addPreset') }}</ElButton>
              </div>
            </ElFormItem>
          </ElForm>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.settings.features')" name="features">
          <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
            {{ $t('vnetPages.settings.featuresHint') }}
          </ElAlert>
          <ElForm label-width="220px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.attendanceEnabled')">
              <!--
                active-value/inactive-value là CHUỖI, không phải boolean. Giá trị
                lưu trong cột jsonb trở về dạng chuỗi, nên "false" là chuỗi khác
                rỗng: bind kiểu boolean thì công tắc luôn hiện "bật" và không bao
                giờ tắt được gì.
              -->
              <ElSwitch v-model="settings.attendance_enabled" active-value="true" inactive-value="false" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.feedbackEnabled')">
              <ElSwitch v-model="settings.feedback_enabled" active-value="true" inactive-value="false" />
            </ElFormItem>
          </ElForm>
        </ElTabPane>
      </ElTabs>

      <!--
        Tab "Máy in" đã gỡ: bốn ô ở đó không service nào đọc, và chúng mâu thuẫn
        với trang Máy in vốn đọc từ bảng printer_configs — khai một loại máy in ở
        đây rồi in ra máy khác là chuyện đã xảy ra được.
      -->
      <ElAlert type="info" :closable="false" show-icon style="margin-top: 8px">
        {{ $t('vnetPages.settings.printerMoved') }}
      </ElAlert>
    </ElCard>
  </div>
</template>
