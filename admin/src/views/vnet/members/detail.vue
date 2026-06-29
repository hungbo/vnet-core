<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

const { t: $t } = useI18n();
const route = useRoute();
const memberId = route.params.id as string;

const member = ref<any>(null);
const loading = ref(false);
const activeTab = ref('info');

const transactions = ref<any[]>([]);
const transLoading = ref(false);
const txPage = ref(1);
const txPageSize = ref(10);
const txTotal = ref(0);

const sessions = ref<any[]>([]);
const sesLoading = ref(false);
const sesPage = ref(1);
const sesPageSize = ref(10);
const sesTotal = ref(0);

const editVisible = ref(false);
const submitting = ref(false);
const formRef = ref<FormInstance>();
const form = ref({
  username: '',
  full_name: '',
  phone: '',
  email: '',
  password: '',
  is_active: true
});

const rules: FormRules = {};

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

async function fetchMember() {
  loading.value = true;
  try {
    const res: any = await client.get(`/members/${memberId}`);
    member.value = res;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.messages.loadDetailError'));
  } finally {
    loading.value = false;
  }
}

async function fetchTransactions() {
  transLoading.value = true;
  try {
    const res: any = await client.get(`/members/${memberId}/transactions`, {
      params: { page: txPage.value, page_size: txPageSize.value }
    });
    transactions.value = res.items || [];
    txTotal.value = res.total || 0;
  } catch {
    transactions.value = [];
  } finally {
    transLoading.value = false;
  }
}

async function fetchSessions() {
  sesLoading.value = true;
  try {
    const res: any = await client.get(`/members/${memberId}/sessions`, {
      params: { page: sesPage.value, page_size: sesPageSize.value }
    });
    sessions.value = res.items || [];
    sesTotal.value = res.total || 0;
  } catch {
    sessions.value = [];
  } finally {
    sesLoading.value = false;
  }
}

function openEdit() {
  form.value = {
    username: member.value?.username || '',
    full_name: member.value?.full_name || '',
    phone: member.value?.phone || '',
    email: member.value?.email || '',
    password: '',
    is_active: member.value?.is_active ?? true
  };
  editVisible.value = true;
}

async function handleEdit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    const payload = { ...form.value };
    if (!payload.password) payload.password = undefined as any;
    await client.put(`/members/${memberId}`, payload);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.members.messages.editSuccess')
    });
    editVisible.value = false;
    fetchMember();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

onMounted(() => {
  fetchMember();
  fetchTransactions();
  fetchSessions();
});
</script>

<template>
  <div>
    <div style="margin-bottom: 16px">
      <ElButton @click="$router.push('/members')">← {{ $t('vnetPages.common.back') }}</ElButton>
    </div>

    <ElCard v-loading="loading">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>{{ $t('vnetPages.members.title') }}: {{ member?.full_name || member?.username }}</span>
          <ElButton type="primary" size="small" @click="openEdit">{{ $t('vnetPages.common.edit') }}</ElButton>
        </div>
      </template>

      <ElTabs v-model="activeTab">
        <ElTabPane :label="$t('vnetPages.members.detail')" name="info">
          <ElDescriptions :column="2" border>
            <ElDescriptionsItem :label="$t('vnetPages.members.username')">{{ member?.username }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.fullName')">{{ member?.full_name }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.phone')">{{ member?.phone }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.email')">{{ member?.email || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.balance')">
              {{ member?.balance?.toLocaleString() }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.bonus')">
              {{ member?.bonus_balance?.toLocaleString() }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.group')">
              {{ member?.group?.name || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.isActive')">
              <ElTag :type="member?.is_active ? 'success' : 'danger'" size="small">
                {{ member?.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no') }}
              </ElTag>
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.createdAt')">
              {{ formatDate(member?.created_at) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.lastLogin')">
              {{ formatDate(member?.last_login) }}
            </ElDescriptionsItem>
          </ElDescriptions>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.members.transactions')" name="transactions">
          <ElTable v-loading="transLoading" :data="transactions" border stripe style="width: 100%">
            <ElTableColumn prop="transaction_type" :label="$t('vnetPages.members.type')" width="100" />
            <ElTableColumn :label="$t('vnetPages.members.amount')" width="120">
              <template #default="{ row }">{{ row.amount?.toLocaleString() }}</template>
            </ElTableColumn>
            <ElTableColumn prop="payment_method" :label="$t('vnetPages.members.method')" width="120" />
            <ElTableColumn prop="reference_id" :label="$t('vnetPages.members.reference')" min-width="140" />
            <ElTableColumn :label="$t('vnetPages.members.createdAt')" width="160">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </ElTableColumn>
          </ElTable>
          <div style="display: flex; justify-content: center; margin-top: 12px">
            <ElPagination
              v-model:current-page="txPage"
              :page-size="txPageSize"
              :total="txTotal"
              layout="prev, pager, next, total"
              small
              @current-change="fetchTransactions"
            />
          </div>
        </ElTabPane>

        <ElTabPane :label="$t('vnetPages.members.sessions')" name="sessions">
          <ElTable v-loading="sesLoading" :data="sessions" border stripe style="width: 100%">
            <ElTableColumn prop="machine_code" :label="$t('vnetPages.sessions.machineCode')" width="100" />
            <ElTableColumn :label="$t('vnetPages.sessions.startTime')" width="160">
              <template #default="{ row }">{{ formatDate(row.started_at) }}</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.sessions.endTime')" width="160">
              <template #default="{ row }">{{ formatDate(row.ended_at) || $t('vnetPages.sessions.running') }}</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.sessions.duration')" width="100">
              <template #default="{ row }">{{ row.duration_minutes }}p</template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.sessions.status')" width="100">
              <template #default="{ row }">
                <ElTag :type="row.is_active ? 'warning' : 'info'" size="small">
                  {{ row.is_active ? $t('vnetPages.sessions.running') : $t('vnetPages.sessions.ended') }}
                </ElTag>
              </template>
            </ElTableColumn>
          </ElTable>
          <div style="display: flex; justify-content: center; margin-top: 12px">
            <ElPagination
              v-model:current-page="sesPage"
              :page-size="sesPageSize"
              :total="sesTotal"
              layout="prev, pager, next, total"
              small
              @current-change="fetchSessions"
            />
          </div>
        </ElTabPane>
      </ElTabs>
    </ElCard>

    <ElDialog v-model="editVisible" :title="$t('vnetPages.members.edit')" width="500px">
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.members.form.username')" prop="username">
          <ElInput v-model="form.username" disabled />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.fullName')" prop="full_name">
          <ElInput v-model="form.full_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.phone')" prop="phone">
          <ElInput v-model="form.phone" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.email')" prop="email">
          <ElInput v-model="form.email" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.password')">
          <ElInput
            v-model="form.password"
            type="password"
            show-password
            :placeholder="$t('vnetPages.members.passwordPlaceholder')"
          />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.isActive')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="editVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleEdit">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
