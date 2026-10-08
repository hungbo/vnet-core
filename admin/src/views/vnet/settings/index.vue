<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { type PaymentMethod, usePaymentMethods } from '@/hooks/business/payment-methods';
import { moneyInput } from '@/utils/money';

const { t: $t } = useI18n();
const { reload: reloadPaymentMethods } = usePaymentMethods();

const loading = ref(false);
const saving = ref(false);
const activeTab = ref('general');
const settings = reactive<Record<string, any>>({});

// Mệnh giá nạp lưu dưới dạng jsonb `{"values": [...]}` nên không gắn thẳng vào
// một ô nhập được; giữ riêng thành mảng số rồi đóng/mở gói lúc đọc/ghi.
const presets = ref<number[]>([]);

// Tab "Thanh toán" đọc qua GET /payment-methods thay vì /settings/payment: API
// đó đã điền sẵn danh sách mặc định khi quán chưa lưu lần nào.
const paymentMethods = ref<PaymentMethod[]>([]);

async function fetchPaymentMethods() {
  loading.value = true;
  try {
    const res: any = await client.get('/payment-methods');
    paymentMethods.value = (Array.isArray(res) ? res : []).map((m: PaymentMethod) => ({ ...m }));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.settings.messages.saveError'));
  } finally {
    loading.value = false;
  }
}

function addPaymentMethod() {
  paymentMethods.value.push({ code: '', name: '', enabled: true });
}

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
  if (activeTab.value === 'payment') {
    await fetchPaymentMethods();
    return;
  }
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
    if (activeTab.value === 'limits') coerceNumbers();
    if (activeTab.value === 'features') applyFeatureDefaults();
    if (activeTab.value === 'client') applyClientDefaults();
  } catch (e: any) {
    // A group with no rows yet is a normal first-run state, not an error:
    // the form stays blank and saving creates the rows.
    if (e?.status !== 404 && e?.message) ElMessage.error(e.message);
    if (activeTab.value === 'features') applyFeatureDefaults();
    if (activeTab.value === 'client') applyClientDefaults();
  } finally {
    loading.value = false;
  }
}

// Giá trị trong cột jsonb trở về dạng CHUỖI. ElInputNumber đòi số: đưa chuỗi
// vào thì Vue cảnh báo sai kiểu ở mọi lần mở tab, và phép tăng/giảm làm việc
// trên một thứ không phải số.
function coerceNumbers() {
  [
    'max_bookings_per_day',
    'max_bookings_per_member',
    'cancel_before_minutes',
    'max_debt',
    'min_session_charge'
  ].forEach(k => {
    const n = Number(settings[k]);
    settings[k] = settings[k] === '' || settings[k] === undefined || Number.isNaN(n) ? undefined : n;
  });
}

// Nhóm "features" chưa tồn tại cho tới lần Lưu đầu tiên, và backend mặc định
// BẬT khi thiếu khoá. Không điền mặc định ở đây thì công tắc hiện TẮT trong khi
// tính năng vẫn đang chạy — hai bên nói ngược nhau.
function applyFeatureDefaults() {
  ['attendance_enabled', 'feedback_enabled'].forEach(k => {
    if (settings[k] !== 'true' && settings[k] !== 'false') settings[k] = 'true';
  });
}

// Nhóm "client" cũng chưa tồn tại cho tới lần Lưu đầu tiên; máy chủ dùng mặc
// định khởi động lại / 60 giây / 300 giây. Điền đúng các mặc định đó để form
// không hiện trống trong khi chính sách vẫn đang chạy.
function applyClientDefaults() {
  if (settings.tamper_action !== 'restart' && settings.tamper_action !== 'shutdown') settings.tamper_action = 'restart';
  const lock = Number(settings.offline_lock_seconds);
  settings.offline_lock_seconds = Number.isFinite(lock) && lock > 0 ? lock : 60;
  const reboot = Number(settings.offline_reboot_seconds);
  settings.offline_reboot_seconds =
    settings.offline_reboot_seconds === undefined || settings.offline_reboot_seconds === '' || !Number.isFinite(reboot)
      ? 300
      : reboot;
  const idle = Number(settings.idle_shutdown_minutes);
  settings.idle_shutdown_minutes =
    settings.idle_shutdown_minutes === undefined || settings.idle_shutdown_minutes === '' || !Number.isFinite(idle)
      ? 5
      : idle;
}

function addPreset() {
  presets.value.push(10000);
}

function removePreset(idx: number) {
  presets.value.splice(idx, 1);
}

async function handleSave() {
  saving.value = true;
  let body: Record<string, any> = { ...settings };
  if (activeTab.value === 'topup') {
    // Máy khách đọc đúng khoá `presets` của nhóm `topup`; giữ nguyên dạng
    // {"values": [...]} mà seed đang lưu để không phải chạy lại seed.
    body = { presets: { values: [...presets.value].sort((a, b) => a - b) } };
  } else if (activeTab.value === 'payment') {
    body = { methods: paymentMethods.value };
  }
  try {
    await client.put(`/settings/${activeTab.value}`, body);
    ElMessage.success($t('vnetPages.settings.messages.saveSuccess'));
    if (activeTab.value === 'client') settings.local_admin_password = '';
    if (activeTab.value === 'topup') await fetchSettings();
    if (activeTab.value === 'payment') {
      // Máy chủ sinh mã cho dòng mới; tải lại để thấy mã, và để các trang
      // nhận tiền đang mở dùng ngay danh sách mới.
      await fetchPaymentMethods();
      await reloadPaymentMethods();
    }
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
            <!-- Đã bỏ hai ô "Email cửa hàng" và "Múi giờ": không nơi nào đọc
                 chúng. Hoá đơn chỉ in tên/địa chỉ/điện thoại, còn toàn bộ mốc
                 thời gian của backend neo cứng Asia/Ho_Chi_Minh trong
                 utils.VietnamLocation(). Ô Múi giờ nguy hiểm hơn vì trông như
                 có tác dụng: đổi xong người vận hành tưởng báo cáo đã đổi theo. -->
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
              <ElInputNumber
                v-bind="moneyInput"
                v-model="settings.max_debt"
                :min="0"
                :step="10000"
                :precision="0"
                style="width: 100%"
              />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.minSessionCharge')">
              <ElInputNumber
                v-bind="moneyInput"
                v-model="settings.min_session_charge"
                :min="0"
                :step="1000"
                :precision="0"
                style="width: 100%"
              />
              <div style="color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5">
                {{ $t('vnetPages.settings.minSessionChargeHint') }}
              </div>
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
                  <ElInputNumber
                    v-bind="moneyInput"
                    v-model="presets[idx]"
                    :min="1000"
                    :step="10000"
                    style="width: 150px"
                  />
                  <ElButton type="danger" plain @click="removePreset(idx)">
                    {{ $t('vnetPages.common.delete') }}
                  </ElButton>
                </div>
                <ElButton @click="addPreset">{{ $t('vnetPages.settings.addPreset') }}</ElButton>
              </div>
            </ElFormItem>
          </ElForm>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.settings.payment')" name="payment">
          <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
            {{ $t('vnetPages.settings.paymentHint') }}
          </ElAlert>
          <ElTable :data="paymentMethods" border style="width: 100%; margin-bottom: 12px">
            <ElTableColumn :label="$t('vnetPages.settings.paymentName')" min-width="200">
              <template #default="{ row }">
                <ElInput v-model="row.name" maxlength="50" />
              </template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.settings.paymentCode')" width="180">
              <template #default="{ row }">
                <span v-if="row.code">{{ row.code }}</span>
                <span v-else style="color: var(--el-text-color-secondary)">
                  {{ $t('vnetPages.settings.paymentCodeAuto') }}
                </span>
              </template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.settings.paymentEnabled')" width="110" align="center">
              <template #default="{ row }">
                <ElSwitch v-model="row.enabled" :disabled="row.code === 'cash'" />
              </template>
            </ElTableColumn>
            <ElTableColumn width="100" align="center">
              <!--
                Chỉ xoá được dòng chưa lưu. Dòng đã lưu có thể đã nằm trong lịch
                sử giao dịch; xoá đi thì lịch sử chỉ còn hiện mã. Muốn ngừng dùng
                thì tắt.
              -->
              <template #default="{ row, $index }">
                <ElButton v-if="!row.code" type="danger" plain size="small" @click="paymentMethods.splice($index, 1)">
                  {{ $t('vnetPages.common.delete') }}
                </ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
          <ElButton @click="addPaymentMethod">{{ $t('vnetPages.settings.addPaymentMethod') }}</ElButton>
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
              <!--
                ElTabs dựng sẵn mọi pane, nên hai công tắc này tồn tại cả khi
                đang mở tab khác — lúc đó khoá chưa nạp và ElSwitch cảnh báo
                "model-value must be active-value or inactive-value". Mặc định
                "true" khớp với mặc định BẬT của backend.
              -->
              <ElSwitch
                :model-value="settings.attendance_enabled ?? 'true'"
                active-value="true"
                inactive-value="false"
                @update:model-value="(v: any) => (settings.attendance_enabled = v)"
              />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.feedbackEnabled')">
              <ElSwitch
                :model-value="settings.feedback_enabled ?? 'true'"
                active-value="true"
                inactive-value="false"
                @update:model-value="(v: any) => (settings.feedback_enabled = v)"
              />
            </ElFormItem>
          </ElForm>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.settings.client')" name="client">
          <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
            {{ $t('vnetPages.settings.clientHint') }}
          </ElAlert>
          <ElForm label-width="260px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.tamperAction')">
              <!-- Mặc định "restart" khi khoá chưa nạp: ElTabs dựng sẵn mọi pane. -->
              <ElRadioGroup
                :model-value="settings.tamper_action ?? 'restart'"
                @update:model-value="(v: any) => (settings.tamper_action = v)"
              >
                <ElRadio value="restart">{{ $t('vnetPages.settings.tamperRestart') }}</ElRadio>
                <ElRadio value="shutdown">{{ $t('vnetPages.settings.tamperShutdown') }}</ElRadio>
              </ElRadioGroup>
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.offlineLockSeconds')">
              <ElInputNumber v-model="settings.offline_lock_seconds" :min="30" :step="10" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.offlineRebootSeconds')">
              <ElInputNumber v-model="settings.offline_reboot_seconds" :min="0" :step="30" style="width: 100%" />
              <div style="color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5">
                {{ $t('vnetPages.settings.offlineRebootHint') }}
              </div>
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.idleShutdownMinutes')">
              <ElInputNumber v-model="settings.idle_shutdown_minutes" :min="0" :max="1440" :step="1" style="width: 100%" />
              <div style="color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5">
                {{ $t('vnetPages.settings.idleShutdownHint') }}
              </div>
            </ElFormItem>
            <ElDivider content-position="left">{{ $t('vnetPages.settings.blockedAppsTitle') }}</ElDivider>
            <ElFormItem :label="$t('vnetPages.settings.blockedApps')">
              <ElInput
                v-model="settings.blocked_apps"
                type="textarea"
                :rows="5"
                :placeholder="$t('vnetPages.settings.blockedAppsPlaceholder')"
              />
              <div style="color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5">
                {{ $t('vnetPages.settings.blockedAppsHint') }}
              </div>
            </ElFormItem>
            <ElDivider content-position="left">{{ $t('vnetPages.settings.hiddenShortcutsTitle') }}</ElDivider>
            <ElFormItem :label="$t('vnetPages.settings.hiddenShortcuts')">
              <ElInput
                v-model="settings.hidden_shortcuts"
                type="textarea"
                :rows="3"
                :placeholder="$t('vnetPages.settings.hiddenShortcutsPlaceholder')"
              />
              <div style="color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5">
                {{ $t('vnetPages.settings.hiddenShortcutsHint') }}
              </div>
            </ElFormItem>
            <ElDivider content-position="left">{{ $t('vnetPages.settings.localAdminTitle') }}</ElDivider>
            <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
              {{ $t('vnetPages.settings.localAdminHint') }}
            </ElAlert>
            <ElFormItem :label="$t('vnetPages.settings.localAdminUsername')">
              <ElInput v-model="settings.local_admin_username" autocomplete="off" clearable />
            </ElFormItem>
            <!-- Máy chủ chỉ giữ băm, không bao giờ trả mật khẩu về: để trống là giữ nguyên. -->
            <ElFormItem :label="$t('vnetPages.settings.localAdminPassword')">
              <ElInput
                v-model="settings.local_admin_password"
                type="password"
                show-password
                autocomplete="new-password"
                :placeholder="$t('vnetPages.settings.localAdminPasswordPlaceholder')"
              />
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
