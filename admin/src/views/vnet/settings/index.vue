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

async function fetchSettings() {
  loading.value = true;
  try {
    const res: any = await client.get(`/settings/${activeTab.value}`);
    Object.assign(settings, res || {});
  } catch (e: any) {
    if (e.message) ElMessage.error(e.message);
  } finally {
    loading.value = false;
  }
}

async function handleSave() {
  saving.value = true;
  try {
    await client.put(`/settings/${activeTab.value}`, settings);
    ElMessage.success($t('vnetPages.settings.messages.saveSuccess'));
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
    <ElCard>
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
        <ElTabPane :label="$t('vnetPages.settings.pricing')" name="pricing">
          <ElForm :model="settings" label-width="180px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.pricePerHour')">
              <ElInputNumber v-model="settings.hourly_rate" :min="0" :precision="0" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.pricePerMinute')">
              <ElInputNumber v-model="settings.minute_rate" :min="0" :precision="0" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.minMinutes')">
              <ElInputNumber v-model="settings.min_minutes" :min="0" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.hourlyDiscount')">
              <ElInputNumber v-model="settings.hourly_discount" :min="0" :precision="0" style="width: 100%" />
            </ElFormItem>
          </ElForm>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.settings.limits')" name="limits">
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
              <ElInputNumber v-model="settings.max_debt" :min="0" :precision="0" style="width: 100%" />
            </ElFormItem>
          </ElForm>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.settings.printing')" name="printer">
          <ElForm :model="settings" label-width="180px" label-position="left">
            <ElFormItem :label="$t('vnetPages.settings.printerType')">
              <ElSelect v-model="settings.printer_type" style="width: 100%">
                <ElOption :label="$t('vnetPages.settings.thermal')" value="thermal" />
                <ElOption :label="$t('vnetPages.settings.laser')" value="laser" />
                <ElOption :label="$t('vnetPages.settings.inkjet')" value="inkjet" />
              </ElSelect>
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.printerName')">
              <ElInput v-model="settings.printer_name" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.paperSize')">
              <ElInput v-model="settings.paper_size" :placeholder="$t('vnetPages.settings.paperSizePlaceholder')" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.autoPrint')">
              <ElSwitch v-model="settings.auto_print" />
            </ElFormItem>
          </ElForm>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.settings.invoice')" name="invoice">
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
            <ElFormItem :label="$t('vnetPages.settings.invoiceStartNumber')">
              <ElInputNumber v-model="settings.invoice_start_number" :min="1" style="width: 100%" />
            </ElFormItem>
            <ElFormItem :label="$t('vnetPages.settings.showLogo')">
              <ElSwitch v-model="settings.show_logo_on_invoice" />
            </ElFormItem>
          </ElForm>
        </ElTabPane>
      </ElTabs>
    </ElCard>
  </div>
</template>
