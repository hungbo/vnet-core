<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import { useAuth } from '@/hooks/business/auth';
import { formatAmount, moneyInput } from '@/utils/money';
import { newIdempotencyKey } from '@/utils/idempotency';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';
import MemberDetailDialog from './modules/member-detail-dialog.vue';
import MemberTopupDialog from './modules/member-topup-dialog.vue';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();

const search = ref('');
// Loại tài khoản: hội viên thường do quầy tạo, hoặc tài khoản combo sinh ra khi
// bán gói. Mặc định chỉ hiện hội viên thường — tài khoản combo nhiều và ngắn hạn.
const roleFilter = ref<'member' | 'combo' | ''>('member');

const dialogVisible = ref(false);
const topupVisible = ref(false);
const detailVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const editingId = ref<number | null>(null);
const currentMember = ref<any>(null);

const detailId = ref<string>('');
const formRef = ref<FormInstance>();

const form = ref({
  username: '',
  password: '',
  full_name: '',
  phone: '',
  email: '',
  group_id: '',
  date_of_birth: '',
  id_card_number: '',
  id_card_image_url: '',
  parent_consent_file_url: '',
  notes: '',
  is_active: true
});

// Tier discounts are applied by the backend from the member's group, and the
// curfew rules need a date of birth. Neither could be entered before, so both
// features were unreachable no matter how they were configured.
const memberGroups = ref<any[]>([]);

async function fetchMemberGroups() {
  try {
    const res: any = await client.get('/member-groups');
    memberGroups.value = Array.isArray(res) ? res : res?.items || [];
  } catch {
    memberGroups.value = [];
  }
}

const rules: FormRules = {
  username: [
    {
      required: true,
      message: $t('vnetPages.members.form.usernameRequired'),
      trigger: 'blur'
    }
  ],
  password: [
    {
      required: true,
      message: $t('vnetPages.members.form.passwordRequired'),
      trigger: 'blur'
    }
  ]
};

const { columns, columnChecks, data, getData, loading, mobilePagination, onSortChange } = useUIPaginatedTable({
  api: ({ page, pageSize, sort, order }) =>
    client.get('/members', {
      params: {
        page,
        page_size: pageSize,
        sort,
        order,
        search: search.value || undefined,
        role: roleFilter.value || undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    {
      prop: 'username',
      label: $t('vnetPages.members.username'),
      width: 160,
      sortable: 'custom',
      // Bấm tên đăng nhập để xem chi tiết — thay cho nút "Chi tiết".
      formatter: (row: any) =>
        h(ElLink, { type: 'primary', underline: false, onClick: () => viewDetail(row) }, () => row.username)
    },
    {
      prop: 'full_name',
      label: $t('vnetPages.members.fullName'),
      minWidth: 160,
      sortable: 'custom'
    },
    { prop: 'phone', label: $t('vnetPages.members.phone'), width: 130, sortable: 'custom' },
    {
      prop: 'balance',
      label: $t('vnetPages.members.balance'),
      width: 120,
      sortable: 'custom',
      formatter: (row: any) => formatAmount(row.balance)
    },
    {
      prop: 'bonus_balance',
      label: $t('vnetPages.members.bonus'),
      width: 120,
      sortable: 'custom',
      formatter: (row: any) => formatAmount(row.bonus_balance)
    },
    {
      prop: 'group',
      label: $t('vnetPages.members.group'),
      width: 100,
      formatter: (row: any) => row.group?.name || memberGroups.value.find(g => g.id === row.group_id)?.name || '-'
    },
    {
      prop: 'is_active',
      label: $t('vnetPages.members.isActive'),
      width: 90,
      sortable: 'custom',
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
  form.value = {
    username: '',
    password: '',
    full_name: '',
    phone: '',
    email: '',
    group_id: '',
    date_of_birth: '',
    id_card_number: '',
    id_card_image_url: '',
    parent_consent_file_url: '',
    notes: '',
    is_active: true
  };
  fetchMemberGroups();
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
    group_id: row.group_id || '',
    date_of_birth: row.date_of_birth ? String(row.date_of_birth).slice(0, 10) : '',
    id_card_number: row.id_card_number || '',
    id_card_image_url: row.id_card_image_url || '',
    parent_consent_file_url: row.parent_consent_file_url || '',
    notes: row.notes || '',
    is_active: row.is_active ?? true
  };
  fetchMemberGroups();
  dialogVisible.value = true;
}

// --- Ảnh giấy tờ --------------------------------------------------------------
// Hai cột id_card_image_url và parent_consent_file_url lưu được từ đầu nhưng
// KHÔNG DTO nào nhận, nên chúng luôn rỗng. Giấy đồng ý của phụ huynh gắn thẳng
// với giới nghiêm: không có nó thì quán không chứng minh được vì sao cho trẻ vị
// thành niên ngồi máy.

// Tác vụ nền xếp lại hạng mỗi 15 phút. Sau khi quán vừa sửa ngưỡng chi tiêu của
// một hạng, chờ 15 phút mới thấy kết quả là quá lâu — và không có cách nào biết
// hàm đó có chạy hay không.
const refreshingTiers = ref(false);

async function handleUnlockLogin(row: any) {
  try {
    await client.post(`/members/${row.id}/unlock-login`);
    ElMessage.success($t('vnetPages.members.unlockLoginSuccess'));
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleRefreshTiers() {
  try {
    await ElMessageBox.confirm($t('vnetPages.members.refreshTiersConfirm'), $t('vnetPages.common.confirm'), {
      type: 'info'
    });
  } catch {
    return;
  }
  refreshingTiers.value = true;
  try {
    const res: any = await client.post('/members/refresh-tiers', {});
    const moved = res?.moved ?? 0;
    ElMessage.success(
      moved > 0 ? $t('vnetPages.members.refreshTiersDone', { count: moved }) : $t('vnetPages.members.refreshTiersNone')
    );
    await getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    refreshingTiers.value = false;
  }
}

const uploading = ref('');

async function uploadDoc(field: 'id_card_image_url' | 'parent_consent_file_url', file: File) {
  const body = new FormData();
  body.append('file', file);
  uploading.value = field;
  try {
    const res: any = await client.post('/upload', body, {
      headers: { 'Content-Type': 'multipart/form-data' }
    });
    form.value[field] = res?.url || '';
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.members.form.uploadFailed'));
  } finally {
    uploading.value = '';
  }
  // Trả false để ElUpload không tự gửi thêm lần nữa.
  return false;
}

// Hoàn tiền là mặt còn lại của nạp tiền và trước đây không có lối vào nào,
// nên nhân viên phải sửa số dư bằng SQL khi cần trả lại tiền cho khách.
const refundVisible = ref(false);
const refunding = ref(false);
const refundForm = ref({ amount: 0, is_bonus: false, description: '', idempotency_key: newIdempotencyKey() });

function openRefund(row: any) {
  currentMember.value = row;
  refundForm.value = { amount: 0, is_bonus: false, description: '', idempotency_key: newIdempotencyKey() };
  refundVisible.value = true;
}

async function handleRefund() {
  if (!currentMember.value || refundForm.value.amount <= 0) return;
  refunding.value = true;
  try {
    await client.post(`/members/${currentMember.value.id}/refund`, {
      amount: refundForm.value.amount,
      is_bonus: refundForm.value.is_bonus,
      description: refundForm.value.description,
      idempotency_key: refundForm.value.idempotency_key
    });
    ElMessage.success($t('vnetPages.members.refundSuccess'));
    refundVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.common.error'));
  } finally {
    refunding.value = false;
  }
}

// Tặng tiền: cộng vào ví KHUYẾN MÃI qua đường nạp nội bộ "bonus_balance".
// Máy chủ ghi giao dịch topup_bonus — báo cáo doanh thu chỉ đếm topup/refund,
// tiền két chỉ đếm khoản thu tiền mặt, nên tiền tặng không lọt vào cả hai.
const giftVisible = ref(false);
const gifting = ref(false);
const giftForm = ref({ amount: 10000, description: '', idempotency_key: newIdempotencyKey() });

function openGift(row: any) {
  currentMember.value = row;
  giftForm.value = { amount: 10000, description: '', idempotency_key: newIdempotencyKey() };
  giftVisible.value = true;
}

async function handleGift() {
  if (!currentMember.value || giftForm.value.amount <= 0) return;
  gifting.value = true;
  try {
    await client.post(`/members/${currentMember.value.id}/topup`, {
      amount: giftForm.value.amount,
      payment_method: 'bonus_balance',
      description: giftForm.value.description,
      idempotency_key: giftForm.value.idempotency_key
    });
    ElMessage.success($t('vnetPages.members.giftSuccess'));
    giftVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.common.error'));
  } finally {
    gifting.value = false;
  }
}

function openTopup(row: any) {
  currentMember.value = row;
  topupVisible.value = true;
}

function viewDetail(row: any) {
  detailId.value = row.id;
  detailVisible.value = true;
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    // group_id bị bấm × thành undefined và rơi khỏi JSON; "" mới là "bỏ chọn".
    const payload = { ...form.value, group_id: form.value.group_id || '' };
    if (!payload.password && isEdit.value) payload.password = undefined as any;
    if (isEdit.value && editingId.value) {
      await client.put(`/members/${editingId.value}`, payload);
      ElNotification({
        type: 'success',
        title: $t('vnetPages.common.success'),
        message: $t('vnetPages.members.messages.editSuccess')
      });
    } else {
      await client.post('/members', payload);
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

function onMoreCommand(cmd: string, row: any) {
  if (cmd === 'reset') handleResetPassword(row);
  else if (cmd === 'delete') handleDelete(row);
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
  } catch (e: any) {
    // ElMessageBox từ chối bằng chuỗi 'cancel'/'close' khi người dùng bấm Huỷ —
    // đó không phải lỗi. Còn lại là backend từ chối (ràng buộc dữ liệu, quy tắc
    // nghiệp vụ) và phải nói ra; `catch {}` rỗng làm nút bấm vào im lặng.
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleResetPassword(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      $t('vnetPages.members.resetPasswordConfirm'),
      $t('vnetPages.common.confirm'),
      {
        confirmButtonText: $t('vnetPages.common.confirm'),
        cancelButtonText: $t('vnetPages.common.cancel'),
        inputPattern: /.+/,
        inputErrorMessage: $t('vnetPages.members.resetPasswordRequired'),
        type: 'warning'
      }
    );
    await client.post(`/members/${row.id}/reset-password`, { password: value });
    ElMessage.success($t('vnetPages.members.messages.resetPasswordSuccess'));
  } catch (e: any) {
    // ElMessageBox từ chối bằng chuỗi 'cancel'/'close' khi người dùng bấm Huỷ —
    // đó không phải lỗi. Còn lại là backend từ chối (ràng buộc dữ liệu, quy tắc
    // nghiệp vụ) và phải nói ra; `catch {}` rỗng làm nút bấm vào im lặng.
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

// Bảng hội viên: sửa/nạp/hoàn/xoá (members), đổi tên hạng (member-groups), bán
// gói sinh tài khoản combo (combos). Số dư đổi ở nơi khác thì member:updated vá
// tại chỗ ở dưới, không cần tải lại. Đang mở biểu mẫu thì hoãn, đóng mới tải bù.
const formOpen = computed(() => dialogVisible.value || topupVisible.value || refundVisible.value || giftVisible.value);
const { resume } = useRealtimeRefresh(['members', 'member-groups', 'combos', 'sessions'], getData, {
  paused: () => formOpen.value
});
watch(formOpen, open => {
  if (!open) resume();
});

const wsStore = useWebSocketStore();

/**
 * Số dư một hội viên vừa đổi ở đâu đó — quầy bên cạnh, điện thoại của chính
 * người đang ngồi đây, hay một đơn nạp vừa được duyệt.
 *
 * Vá tại chỗ thay vì gọi lại cả trang: nhân viên đang gõ ô tìm kiếm hoặc đang ở
 * trang 3 thì một cú getData() làm bảng nhảy về đầu.
 */
function onMemberUpdated(payload: any) {
  if (!payload?.member_id) return;

  const row = data.value.find((m: any) => m.id === payload.member_id);
  if (row) {
    row.balance = payload.balance;
    row.bonus_balance = payload.bonus_balance;
  }
}

onMounted(() => {
  wsStore.on('member:updated', onMemberUpdated);
});

onBeforeUnmount(() => {
  wsStore.off('member:updated', onMemberUpdated);
});
</script>

<template>
  <div class="page-fill">
    <ElCard class="fill-card">
      <div class="flex flex-wrap items-center justify-between gap-8px" style="margin-bottom: 16px">
        <div class="flex flex-wrap items-center gap-8px">
          <ElInput
            v-model="search"
            :placeholder="$t('vnetPages.members.searchPlaceholder')"
            clearable
            style="width: 300px"
            @keyup.enter="searchData"
          />
          <ElSelect v-model="roleFilter" class="w-160px" @change="searchData">
            <ElOption :label="$t('vnetPages.members.roleMember')" value="member" />
            <ElOption :label="$t('vnetPages.members.roleCombo')" value="combo" />
            <ElOption :label="$t('vnetPages.members.roleAll')" value="" />
          </ElSelect>
          <ElButton type="primary" @click="searchData">{{ $t('vnetPages.common.search') }}</ElButton>
        </div>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :loading="loading"
          :show-add="hasAuth('members.create')"
          :show-delete="false"
          @add="openCreate"
          @refresh="getData"
        >
          <template #prefix>
            <ElButton v-if="hasAuth('members.refresh_tiers')" :loading="refreshingTiers" @click="handleRefreshTiers">
              {{ $t('vnetPages.members.refreshTiers') }}
            </ElButton>
          </template>
        </TableHeaderOperation>
      </div>

      <ElTable
        v-loading="loading"
        :data="data"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <!--
          Các việc với tiền (Nạp, Trừ tiền, Tặng tiền) và Sửa nằm ngoài; đặt lại mật
          khẩu và xoá gom vào "Khác". Xem chi tiết: bấm vào tên đăng nhập.
        -->
        <ElTableColumn :label="$t('vnetPages.common.action')" width="400" fixed="right" align="center">
          <template #default="{ row }">
            <div class="row-actions">
              <!-- Chỉ hiện khi đang bị tạm khoá vì nhập sai mật khẩu (locked_until còn hiệu lực). -->
              <ElButton
                v-if="row.locked_until && hasAuth('members.update')"
                size="small"
                type="warning"
                :title="$t('vnetPages.members.lockedUntil', { time: dayjs(row.locked_until).format('HH:mm:ss') })"
                @click="handleUnlockLogin(row)"
              >
                {{ $t('vnetPages.members.unlockLogin') }}
              </ElButton>
              <!-- Tài khoản combo do hệ thống sinh khi bán gói: chỉ được đặt lại mật khẩu. -->
              <ElButton
                v-if="row.role === 'combo' && hasAuth('members.reset_password')"
                size="small"
                @click="handleResetPassword(row)"
              >
                {{ $t('vnetPages.members.resetPassword') }}
              </ElButton>
              <template v-else-if="row.role !== 'combo'">
                <ElButton v-if="hasAuth('members.topup')" size="small" type="success" @click="openTopup(row)">
                  {{ $t('vnetPages.members.topUp') }}
                </ElButton>
                <ElButton v-if="hasAuth('members.refund')" size="small" type="warning" plain @click="openRefund(row)">
                  {{ $t('vnetPages.members.refund') }}
                </ElButton>
                <!-- Tặng tiền gọi POST /members/:id/topup (payment_method bonus_balance). -->
                <ElButton v-if="hasAuth('members.topup')" size="small" type="primary" plain @click="openGift(row)">
                  {{ $t('vnetPages.members.gift') }}
                </ElButton>
                <ElButton v-if="hasAuth('members.update')" size="small" @click="openEdit(row)">
                  {{ $t('vnetPages.common.edit') }}
                </ElButton>
                <ElDropdown
                  v-if="hasAuth(['members.reset_password', 'members.delete'])"
                  trigger="click"
                  @command="(cmd: string) => onMoreCommand(cmd, row)"
                >
                  <ElButton size="small">{{ $t('vnetPages.members.more') }}</ElButton>
                  <template #dropdown>
                    <ElDropdownMenu>
                      <ElDropdownItem v-if="hasAuth('members.reset_password')" command="reset">
                        {{ $t('vnetPages.members.resetPassword') }}
                      </ElDropdownItem>
                      <ElDropdownItem v-if="hasAuth('members.delete')" command="delete" divided class="danger-item">
                        {{ $t('vnetPages.common.delete') }}
                      </ElDropdownItem>
                    </ElDropdownMenu>
                  </template>
                </ElDropdown>
              </template>
            </div>
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
        <ElFormItem :label="$t('vnetPages.members.form.group')">
          <ElSelect
            v-model="form.group_id"
            clearable
            :placeholder="$t('vnetPages.members.form.noGroup')"
            style="width: 100%"
          >
            <ElOption v-for="g in memberGroups" :key="g.id" :label="g.name" :value="g.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.dateOfBirth')">
          <ElDatePicker v-model="form.date_of_birth" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.idCardNumber')">
          <ElInput v-model="form.id_card_number" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.idCardImage')">
          <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap">
            <ElUpload
              :show-file-list="false"
              accept="image/*"
              :before-upload="(f: File) => uploadDoc('id_card_image_url', f)"
            >
              <ElButton :loading="uploading === 'id_card_image_url'">
                {{ $t('vnetPages.members.form.uploadImage') }}
              </ElButton>
            </ElUpload>
            <template v-if="form.id_card_image_url">
              <ElLink type="primary" :href="form.id_card_image_url" target="_blank" rel="noopener noreferrer">
                {{ $t('vnetPages.members.form.viewFile') }}
              </ElLink>
              <ElButton link type="danger" @click="form.id_card_image_url = ''">
                {{ $t('vnetPages.members.form.removeFile') }}
              </ElButton>
            </template>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.parentConsent')">
          <div style="width: 100%">
            <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap">
              <ElUpload
                :show-file-list="false"
                accept="image/*"
                :before-upload="(f: File) => uploadDoc('parent_consent_file_url', f)"
              >
                <ElButton :loading="uploading === 'parent_consent_file_url'">
                  {{ $t('vnetPages.members.form.uploadImage') }}
                </ElButton>
              </ElUpload>
              <template v-if="form.parent_consent_file_url">
                <ElLink type="primary" :href="form.parent_consent_file_url" target="_blank" rel="noopener noreferrer">
                  {{ $t('vnetPages.members.form.viewFile') }}
                </ElLink>
                <ElButton link type="danger" @click="form.parent_consent_file_url = ''">
                  {{ $t('vnetPages.members.form.removeFile') }}
                </ElButton>
              </template>
            </div>
            <div style="font-size: 12px; color: #909399; margin-top: 4px">
              {{ $t('vnetPages.members.form.parentConsentHint') }}
            </div>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.form.notes')">
          <ElInput v-model="form.notes" type="textarea" :rows="2" />
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

    <MemberTopupDialog
      v-model:visible="topupVisible"
      :member-id="currentMember?.id ?? null"
      :member-name="currentMember?.full_name || currentMember?.username"
      @success="getData"
    />

    <MemberDetailDialog v-model:visible="detailVisible" :member-id="detailId || null" />
    <ElDialog v-model="giftVisible" :title="$t('vnetPages.members.gift')" width="420px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        {{ $t('vnetPages.members.giftHint') }}
      </ElAlert>
      <ElForm label-width="130px">
        <ElFormItem :label="$t('vnetPages.members.giftAmount')">
          <ElInputNumber v-bind="moneyInput" v-model="giftForm.amount" :min="1000" :step="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.giftReason')">
          <ElInput v-model="giftForm.description" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="giftVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="gifting" @click="handleGift">
          {{ $t('vnetPages.members.gift') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="refundVisible" :title="$t('vnetPages.members.refund')" width="420px">
      <ElForm label-width="170px">
        <ElFormItem :label="$t('vnetPages.members.refundAmount')">
          <ElInputNumber v-bind="moneyInput" v-model="refundForm.amount" :min="0" :step="10000" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.refundBonus')">
          <ElSwitch v-model="refundForm.is_bonus" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.members.refundReason')">
          <ElInput v-model="refundForm.description" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="refundVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="warning" :loading="refunding" @click="handleRefund">
          {{ $t('vnetPages.members.refund') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
.row-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: 6px;
}

.row-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

:global(.danger-item) {
  color: var(--el-color-danger) !important;
}
</style>
