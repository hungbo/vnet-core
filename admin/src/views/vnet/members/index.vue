<script setup lang="ts">
import { h, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const search = ref('');

const dialogVisible = ref(false);
const topupVisible = ref(false);
const detailVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const currentMember = ref<any>(null);

const detailId = ref<string>('');
const detailLoading = ref(false);
const detailMember = ref<any>(null);
const detailTab = ref('info');
const detailTxList = ref<any[]>([]);
const detailTxLoading = ref(false);
const detailTxPage = ref(1);
const detailTxSize = ref(10);
const detailTxTotal = ref(0);
const detailSessions = ref<any[]>([]);
const detailSesLoading = ref(false);
const detailSesPage = ref(1);
const detailSesSize = ref(10);
const detailSesTotal = ref(0);

const formRef = ref<FormInstance>();
const topupFormRef = ref<FormInstance>();

const form = ref({
  username: '',
  password: '',
  full_name: '',
  phone: '',
  email: '',
  is_active: true
});

const topupForm = ref({
  amount: 10000,
  payment_method: 'cash'
});

const rules: FormRules = {
  username: [{ required: true, message: $t('vnetPages.members.form.usernameRequired'), trigger: 'blur' }],
  password: [{ required: true, message: $t('vnetPages.members.form.passwordRequired'), trigger: 'blur' }]
};

const topupRules: FormRules = {
  amount: [{ required: true, message: $t('vnetPages.members.form.amountRequired'), trigger: 'blur' }],
  payment_method: [{ required: true, message: $t('vnetPages.members.form.methodRequired'), trigger: 'change' }]
};

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/members', { params: { page, page_size: pageSize, search: search.value || undefined } }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'username', label: $t('vnetPages.members.username'), width: 120 },
    { prop: 'full_name', label: $t('vnetPages.members.fullName'), minWidth: 160 },
    { prop: 'phone', label: $t('vnetPages.members.phone'), width: 130 },
    {
      prop: 'balance',
      label: $t('vnetPages.members.balance'),
      width: 120,
      formatter: (row: any) => row.balance?.toLocaleString() ?? ''
    },
    {
      prop: 'bonus_balance',
      label: $t('vnetPages.members.bonus'),
      width: 120,
      formatter: (row: any) => row.bonus_balance?.toLocaleString() ?? ''
    },
    {
      prop: 'group',
      label: $t('vnetPages.members.group'),
      width: 100,
      formatter: (row: any) => row.group?.name || '-'
    },
    {
      prop: 'is_active',
      label: $t('vnetPages.members.isActive'),
      width: 90,
      formatter: (row: any) =>
        h(ElTag, { type: row.is_active ? 'success' : 'danger', size: 'small' }, () =>
          row.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no')
        )
    }
  ]
});

function searchData() {
  getData();
}

function openCreate() {
  isEdit.value = false;
  editingId.value = null;
  form.value = { username: '', password: '', full_name: '', phone: '', email: '', is_active: true };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  form.value = {
    username: row.username || '',
    full_name: row.full_name || '',
    phone: row.phone || '',
    email: row.email || '',
    password: '',
    is_active: row.is_active ?? true
  };
  dialogVisible.value = true;
}

function openTopup(row: any) {
  currentMember.value = row;
  topupForm.value = { amount: 10000, payment_method: 'cash' };
  topupVisible.value = true;
}

function viewDetail(row: any) {
  detailId.value = row.id;
  detailVisible.value = true;
  detailTab.value = 'info';
  detailTxPage.value = 1;
  detailSesPage.value = 1;
  fetchMemberDetail(row.id);
  fetchDetailTransactions();
  fetchDetailSessions();
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    const payload = { ...form.value };
    if (!payload.password && isEdit.value) payload.password = undefined as any;
    if (isEdit.value && editingId.value) {
      await client.put(`/members/${editingId.value}`, payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.members.messages.editSuccess')
      });
    } else {
      const { is_active, ...createPayload } = payload;
      await client.post('/members', createPayload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.members.messages.addSuccess')
      });
    }
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.messages.saveError'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.members.messages.deleteConfirm', { name: row.full_name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
    await client.delete(`/members/${row.id}`);
    ElMessage.success($t('vnetPages.members.messages.deleteSuccess'));
    getData();
  } catch {}
}

async function handleResetPassword(row: any) {
  try {
    const { value } = await ElMessageBox.prompt($t('vnetPages.members.resetPasswordConfirm'), $t('vnetPages.common.confirm'), {
      confirmButtonText: $t('vnetPages.common.confirm'),
      cancelButtonText: $t('vnetPages.common.cancel'),
      inputPattern: /.+/,
      inputErrorMessage: $t('vnetPages.members.resetPasswordRequired'),
      type: 'warning'
    });
    await client.post(`/members/${row.id}/reset-password`, { password: value });
    ElMessage.success($t('vnetPages.members.messages.resetPasswordSuccess'));
  } catch {}
}

function formatDate(date: string | null | undefined) {
  if (!date) return '-';
  return dayjs(date).format('DD/MM/YYYY HH:mm');
}

async function fetchMemberDetail(id: string) {
  detailLoading.value = true;
  try {
    const res: any = await client.get(`/members/${id}`);
    detailMember.value = res;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.messages.loadDetailError'));
  } finally {
    detailLoading.value = false;
  }
}

async function fetchDetailTransactions() {
  detailTxLoading.value = true;
  try {
    const res: any = await client.get(`/members/${detailId.value}/transactions`, {
      params: { page: detailTxPage.value, page_size: detailTxSize.value }
    });
    detailTxList.value = res.items || [];
    detailTxTotal.value = res.total || 0;
  } catch {
    detailTxList.value = [];
  } finally {
    detailTxLoading.value = false;
  }
}

async function fetchDetailSessions() {
  detailSesLoading.value = true;
  try {
    const res: any = await client.get(`/members/${detailId.value}/sessions`, {
      params: { page: detailSesPage.value, page_size: detailSesSize.value }
    });
    detailSessions.value = res.items || [];
    detailSesTotal.value = res.total || 0;
  } catch {
    detailSessions.value = [];
  } finally {
    detailSesLoading.value = false;
  }
}

async function handleTopup() {
  const valid = await topupFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    await client.post(`/members/${currentMember.value.id}/topup`, topupForm.value);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.members.messages.topUpSuccess')
    });
    topupVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.messages.topUpError'));
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <div>
    <ElCard>
      <div class="flex items-center justify-between" style="margin-bottom: 16px">
        <div class="flex items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.members.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="searchData"
          />
          <ElButton type="primary" @click="searchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @add="openCreate" @refresh="getData" />
      </div>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="360" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="viewDetail(row)">{{ $t('vnetPages.common.detail') }}</ElButton>
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="warning" @click="handleResetPassword(row)">
              {{ $t('vnetPages.members.resetPassword') }}
            </ElButton>
            <ElButton size="small" type="success" @click="openTopup(row)">{{ $t('vnetPages.members.topUp') }}</ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="mt-16px flex justify-center">
        <ElPagination
          v-if="mobilePagination.total"
          layout="total, sizes, prev, pager, next"
          v-bind="mobilePagination"
          @current-change="mobilePagination['current-change']"
          @size-change="mobilePagination['size-change']"
        />
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.members.edit') : $t('vnetPages.members.add')"
      width="500px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="120">
        <ElFormItem :label="$t('vnetPages.members.form.username')" prop="username">
          <ElInput v-model="form.username" />
        </ElFormItem>
        <ElFormItem v-if="!isEdit" :label="$t('vnetPages.members.form.password')" prop="password">
          <ElInput v-model="form.password" type="password" show-password />
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
        <ElFormItem :label="$t('vnetPages.members.form.isActive')">
          <ElSwitch v-model="form.is_active" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="topupVisible" :title="$t('vnetPages.members.topUpAmount')" width="400px">
      <ElForm ref="topupFormRef" :model="topupForm" :rules="topupRules" :label-width="130">
        <ElFormItem :label="$t('vnetPages.members.amount')" prop="amount">
          <ElInputNumber v-model="topupForm.amount" :min="1000" :step="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.method')" prop="payment_method">
          <ElSelect v-model="topupForm.payment_method" style="width: 100%">
            <ElOption :label="$t('vnetPages.members.cash')" value="cash" />
            <ElOption :label="$t('vnetPages.members.transfer')" value="transfer" />
            <ElOption :label="$t('vnetPages.members.eWallet')" value="ewallet" />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="topupVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleTopup">
          {{ $t('vnetPages.common.confirm') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="detailVisible" :title="$t('vnetPages.members.detail')" width="800px" top="5vh" destroy-on-close>
      <ElCard v-loading="detailLoading" shadow="never" style="border: none">
        <ElTabs v-model="detailTab">
          <ElTabPane :label="$t('vnetPages.members.detail')" name="info">
            <ElDescriptions :column="2" border>
              <ElDescriptionsItem :label="$t('vnetPages.members.username')">
                {{ detailMember?.username }}
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.fullName')">
                {{ detailMember?.full_name }}
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.phone')">{{ detailMember?.phone }}</ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.email')">
                {{ detailMember?.email || '-' }}
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.balance')">
                {{ detailMember?.balance?.toLocaleString() }}
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.bonus')">
                {{ detailMember?.bonus_balance?.toLocaleString() }}
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.group')">
                {{ detailMember?.group?.name || '-' }}
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.isActive')">
                <ElTag :type="detailMember?.is_active ? 'success' : 'danger'" size="small">
                  {{ detailMember?.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no') }}
                </ElTag>
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.createdAt')">
                {{ formatDate(detailMember?.created_at) }}
              </ElDescriptionsItem>
              <ElDescriptionsItem :label="$t('vnetPages.members.lastLogin')">
                {{ formatDate(detailMember?.last_login) }}
              </ElDescriptionsItem>
            </ElDescriptions>
          </ElTabPane>
          <ElTabPane :label="$t('vnetPages.members.transactions')" name="transactions">
            <ElTable v-loading="detailTxLoading" :data="detailTxList" border stripe style="width: 100%">
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
                v-model:current-page="detailTxPage"
                :page-size="detailTxSize"
                :total="detailTxTotal"
                layout="prev, pager, next, total"
                small
                @current-change="fetchDetailTransactions"
              />
            </div>
          </ElTabPane>
          <ElTabPane :label="$t('vnetPages.members.sessions')" name="sessions">
            <ElTable v-loading="detailSesLoading" :data="detailSessions" border stripe style="width: 100%">
              <ElTableColumn prop="machine_code" :label="$t('vnetPages.sessions.machineCode')" width="100" />
              <ElTableColumn :label="$t('vnetPages.sessions.startTime')" width="160">
                <template #default="{ row }">{{ formatDate(row.started_at) }}</template>
              </ElTableColumn>
              <ElTableColumn :label="$t('vnetPages.sessions.endTime')" width="160">
                <template #default="{ row }">
                  {{ formatDate(row.ended_at) || $t('vnetPages.sessions.running') }}
                </template>
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
                v-model:current-page="detailSesPage"
                :page-size="detailSesSize"
                :total="detailSesTotal"
                layout="prev, pager, next, total"
                small
                @current-change="fetchDetailSessions"
              />
            </div>
          </ElTabPane>
        </ElTabs>
      </ElCard>
    </ElDialog>
  </div>
</template>
