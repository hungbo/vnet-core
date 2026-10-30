<script setup lang="ts">
// Hộp chi tiết một hội viên: thông tin, giao dịch, phiên chơi và đơn hàng. Dùng
// chung cho trang Hội viên (bấm tên đăng nhập) và bảng "Máy đang hoạt động" ở
// Bảng điều khiển (bấm tên hội viên đang ngồi máy).
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAppStore } from '@/store/modules/app';
import { useWebSocketStore } from '@/store/modules/ws';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { usePaymentMethods } from '@/hooks/business/payment-methods';
import { useTransactionTypeLabel } from '@/hooks/business/transaction-types';
import { formatAmount } from '@/utils/money';

defineOptions({ name: 'MemberDetailDialog' });

const props = defineProps<{ memberId: string | null }>();
const visible = defineModel<boolean>('visible', { default: false });

const { t: $t } = useI18n();
const { paymentLabel } = usePaymentMethods();
const txTypeLabel = useTransactionTypeLabel();
const appStore = useAppStore();

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
// Tab "Đơn hàng": đơn gọi món và đơn nạp tiền của hội viên, mới nhất trước.
const detailOrders = ref<any[]>([]);
const detailOrdLoading = ref(false);
const detailOrdPage = ref(1);
const detailOrdSize = ref(10);
const detailOrdTotal = ref(0);
// Sắp xếp phía máy chủ cho ba bảng phân trang trong hộp chi tiết: gửi kèm sort/order.
type DetailSort = { sort?: string; order?: 'asc' | 'desc' };
const detailTxSort = ref<DetailSort>({});
const detailSesSort = ref<DetailSort>({});
const detailOrdSort = ref<DetailSort>({});

// ElTable báo 'ascending' | 'descending' | null; bỏ sắp xếp thì cả hai là undefined.
function toDetailSort({ prop, order }: { prop: string | null; order: string | null }): DetailSort {
  if (!prop || !order) return {};
  return { sort: prop, order: order === 'ascending' ? 'asc' : 'desc' };
}

// total_played_minutes tính bằng PHÚT (cột cũ tên ...hours chưa bao giờ được
// ghi); đổi sang "Xg Yp" cho dễ đọc.
// Số phút của một phiên: phiên đã đóng có duration_minutes; phiên đang chơi thì
// tính tới bây giờ.
function sessionMinutes(row: any) {
  if (typeof row.duration_minutes === 'number') return row.duration_minutes;
  return row.started_at ? Math.max(0, dayjs().diff(dayjs(row.started_at), 'minute')) : 0;
}

function formatPlayed(minutes: number | null | undefined) {
  const m = minutes || 0;
  if (m < 60) return `${m}p`;
  return `${Math.floor(m / 60)}g ${m % 60}p`;
}

// Mở hộp (hoặc đổi người khi đang mở) thì tải lại từ đầu.
watch(
  () => (visible.value ? props.memberId : null),
  id => {
    if (!id) return;
    detailId.value = id;
    detailTab.value = 'info';
    detailTxPage.value = 1;
    detailSesPage.value = 1;
    detailOrdPage.value = 1;
    // Hộp destroy-on-close nên bảng mở lại không còn dấu sắp xếp: bỏ luôn ở đây.
    detailTxSort.value = {};
    detailSesSort.value = {};
    detailOrdSort.value = {};
    fetchMemberDetail(id);
    fetchDetailTransactions();
    fetchDetailSessions();
    fetchDetailOrders();
  },
  { immediate: true }
);

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
      params: { page: detailTxPage.value, page_size: detailTxSize.value, ...detailTxSort.value }
    });
    detailTxList.value = res.items || [];
    detailTxTotal.value = res.total || 0;
  } catch {
    detailTxList.value = [];
  } finally {
    detailTxLoading.value = false;
  }
}

function onDetailTxSort(e: { prop: string | null; order: string | null }) {
  detailTxSort.value = toDetailSort(e);
  detailTxPage.value = 1;
  fetchDetailTransactions();
}

async function fetchDetailOrders() {
  detailOrdLoading.value = true;
  try {
    const res: any = await client.get(`/members/${detailId.value}/orders`, {
      params: { page: detailOrdPage.value, page_size: detailOrdSize.value, ...detailOrdSort.value }
    });
    detailOrders.value = res.items || [];
    detailOrdTotal.value = res.total || 0;
  } catch {
    detailOrders.value = [];
  } finally {
    detailOrdLoading.value = false;
  }
}

function onDetailOrdSort(e: { prop: string | null; order: string | null }) {
  detailOrdSort.value = toDetailSort(e);
  detailOrdPage.value = 1;
  fetchDetailOrders();
}

const ORDER_STATUS_TAG: Record<string, 'info' | 'primary' | 'success' | 'danger'> = {
  pending: 'info',
  confirmed: 'primary',
  completed: 'success',
  cancelled: 'danger'
};

// Tóm tắt món trong đơn cho một ô: "Mì Hảo Hảo x2, Coca x1". Đơn nạp tiền không có món.
function orderItemsSummary(row: any) {
  if (row.order_type === 'topup') return $t('vnetPages.orders.topup');
  const items: any[] = row.items || [];
  return items.map(i => `${i.product_name} x${i.quantity}`).join(', ') || '-';
}

async function fetchDetailSessions() {
  detailSesLoading.value = true;
  try {
    const res: any = await client.get(`/members/${detailId.value}/sessions`, {
      params: { page: detailSesPage.value, page_size: detailSesSize.value, ...detailSesSort.value }
    });
    detailSessions.value = res.items || [];
    detailSesTotal.value = res.total || 0;
  } catch {
    detailSessions.value = [];
  } finally {
    detailSesLoading.value = false;
  }
}

function onDetailSesSort(e: { prop: string | null; order: string | null }) {
  detailSesSort.value = toDetailSort(e);
  detailSesPage.value = 1;
  fetchDetailSessions();
}

// Hộp chi tiết đang mở: thông tin, giao dịch và phiên của người đó. Phiên mở/đóng
// không qua HTTP (khách đăng nhập máy trạm, tác vụ nền tự đóng) thì nghe sự kiện
// phiên; số dư đổi thì onMemberUpdated gọi scheduleDetail.
function reloadDetail() {
  if (!visible.value || !detailId.value) return;
  fetchMemberDetail(detailId.value);
  fetchDetailTransactions();
  fetchDetailSessions();
  fetchDetailOrders();
}

const { schedule: scheduleDetail } = useRealtimeRefresh(
  ['members', 'member-groups', 'sessions', 'combos', 'orders'],
  reloadDetail,
  { events: ['session:started', 'session:ended', 'session:auto-ended'] }
);

// Số dư người đang xem vừa đổi (quầy bên cạnh, điện thoại của khách, đơn nạp vừa
// duyệt) thì cả số dư lẫn lịch sử giao dịch phải khớp theo. Qua scheduleDetail để
// gộp với "members:changed" của cùng một lần nạp thành một lần tải.
const wsStore = useWebSocketStore();

function onMemberUpdated(payload: any) {
  if (visible.value && payload?.member_id && detailId.value === payload.member_id) scheduleDetail();
}

onMounted(() => {
  wsStore.on('member:updated', onMemberUpdated);
});

onBeforeUnmount(() => {
  wsStore.off('member:updated', onMemberUpdated);
});
</script>

<template>
  <ElDialog
    v-model="visible"
    :title="$t('vnetPages.members.detail')"
    width="min(1000px, 96vw)"
    top="5vh"
    destroy-on-close
  >
    <ElCard v-loading="detailLoading" shadow="never" style="border: none">
      <ElTabs v-model="detailTab">
        <ElTabPane :label="$t('vnetPages.members.detail')" name="info">
          <ElDescriptions :column="appStore.isMobile ? 1 : 2" border>
            <ElDescriptionsItem :label="$t('vnetPages.members.username')">
              {{ detailMember?.username }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.fullName')">
              {{ detailMember?.full_name || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.phone')">
              {{ detailMember?.phone || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.email')">
              {{ detailMember?.email || '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.balance')">
              {{ formatAmount(detailMember?.balance) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.bonus')">
              {{ formatAmount(detailMember?.bonus_balance) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.group')">
              {{ detailMember?.group?.name || '-' }}
            </ElDescriptionsItem>
            <!-- Hạng xếp theo tổng chi tiêu nên phải nhìn thấy được con số này. -->
            <ElDescriptionsItem :label="$t('vnetPages.reports.totalSpent')">
              {{ formatAmount(detailMember?.total_spent) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.form.dateOfBirth')">
              {{ detailMember?.date_of_birth ? dayjs(detailMember.date_of_birth).format('DD/MM/YYYY') : '-' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.form.idCardNumber')">
              {{ detailMember?.id_card_number || '-' }}
            </ElDescriptionsItem>
            <!--
              Hai ô này trước đây không hiện được: total_played_hours và
              last_visit_at khai trong model nhưng không nơi nào ghi, nên
              luôn là 0 và rỗng.
            -->
            <ElDescriptionsItem :label="$t('vnetPages.members.playedTime')">
              {{ formatPlayed(detailMember?.total_played_minutes) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.lastVisit')">
              {{ detailMember?.last_visit_at ? formatDate(detailMember.last_visit_at) : $t('vnetPages.members.never') }}
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.isActive')">
              <ElTag :type="detailMember?.is_active ? 'success' : 'danger'" size="small">
                {{ detailMember?.is_active ? $t('vnetPages.common.yes') : $t('vnetPages.common.no') }}
              </ElTag>
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="$t('vnetPages.members.createdAt')">
              {{ formatDate(detailMember?.created_at) }}
            </ElDescriptionsItem>
            <!--
              "Lần cuối đăng nhập" đọc last_login, trường mà máy chủ không có
              (hội viên không lưu giờ đăng nhập) nên luôn là "-". Lần chơi gần
              nhất đã có ở ô "Lần ghé gần nhất".
            -->
            <ElDescriptionsItem :label="$t('vnetPages.members.form.notes')" :span="2">
              {{ detailMember?.notes || '-' }}
            </ElDescriptionsItem>
          </ElDescriptions>
        </ElTabPane>
        <ElTabPane :label="$t('vnetPages.members.transactions')" name="transactions">
          <ElTable
            v-loading="detailTxLoading"
            :data="detailTxList"
            border
            stripe
            class="w-full"
            @sort-change="onDetailTxSort"
          >
            <ElTableColumn prop="transaction_type" :label="$t('vnetPages.members.type')" width="150" sortable="custom">
              <template #default="{ row }">{{ txTypeLabel(row.transaction_type) }}</template>
            </ElTableColumn>
            <ElTableColumn
              prop="amount"
              :label="$t('vnetPages.members.amount')"
              width="110"
              align="right"
              sortable="custom"
            >
              <template #default="{ row }">{{ formatAmount(row.amount) }}</template>
            </ElTableColumn>
            <ElTableColumn
              prop="balance_after"
              :label="$t('vnetPages.transactions.balanceAfter')"
              width="110"
              align="right"
              sortable="custom"
            >
              <template #default="{ row }">{{ formatAmount(row.balance_after) }}</template>
            </ElTableColumn>
            <ElTableColumn prop="payment_method" :label="$t('vnetPages.members.method')" width="140" sortable="custom">
              <template #default="{ row }">{{ paymentLabel(row.payment_method) }}</template>
            </ElTableColumn>
            <!-- Mô tả thay cho reference_id: mã UUID của đơn/phiên không ai đọc được. -->
            <ElTableColumn
              prop="description"
              :label="$t('vnetPages.transactions.description')"
              min-width="200"
              sortable="custom"
            />
            <ElTableColumn prop="created_at" :label="$t('vnetPages.members.createdAt')" width="140" sortable="custom">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </ElTableColumn>
          </ElTable>
          <div class="mt-12px flex justify-center">
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
          <ElTable
            v-loading="detailSesLoading"
            :data="detailSessions"
            border
            stripe
            class="w-full"
            @sort-change="onDetailSesSort"
          >
            <ElTableColumn :label="$t('vnetPages.sessions.machineCode')" width="110">
              <template #default="{ row }">{{ row.machine_code || '-' }}</template>
            </ElTableColumn>
            <ElTableColumn prop="started_at" :label="$t('vnetPages.sessions.startTime')" width="150" sortable="custom">
              <template #default="{ row }">{{ formatDate(row.started_at) }}</template>
            </ElTableColumn>
            <ElTableColumn prop="ended_at" :label="$t('vnetPages.sessions.endTime')" width="150" sortable="custom">
              <template #default="{ row }">
                {{ row.ended_at ? formatDate(row.ended_at) : $t('vnetPages.sessions.running') }}
              </template>
            </ElTableColumn>
            <!-- Phiên đang chơi chưa có duration_minutes: tính từ giờ bắt đầu tới lúc mở hộp. -->
            <ElTableColumn
              prop="duration_minutes"
              :label="$t('vnetPages.sessions.duration')"
              width="110"
              sortable="custom"
            >
              <template #default="{ row }">{{ formatPlayed(sessionMinutes(row)) }}</template>
            </ElTableColumn>
            <ElTableColumn
              prop="total_cost"
              :label="$t('vnetPages.sessions.cost')"
              width="120"
              align="right"
              sortable="custom"
            >
              <template #default="{ row }">
                {{ typeof row.total_cost === 'number' ? formatAmount(row.total_cost) : '-' }}
              </template>
            </ElTableColumn>
            <ElTableColumn prop="is_active" :label="$t('vnetPages.sessions.status')" min-width="100" sortable="custom">
              <template #default="{ row }">
                <ElTag :type="row.is_active ? 'warning' : 'info'" size="small">
                  {{ row.is_active ? $t('vnetPages.sessions.running') : $t('vnetPages.sessions.ended') }}
                </ElTag>
              </template>
            </ElTableColumn>
          </ElTable>
          <div class="mt-12px flex justify-center">
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
        <ElTabPane :label="$t('vnetPages.members.orders')" name="orders">
          <ElTable
            v-loading="detailOrdLoading"
            :data="detailOrders"
            border
            stripe
            class="w-full"
            @sort-change="onDetailOrdSort"
          >
            <ElTableColumn prop="order_code" :label="$t('vnetPages.orders.orderCode')" width="110" sortable="custom" />
            <ElTableColumn :label="$t('vnetPages.orders.products')" min-width="200">
              <template #default="{ row }">{{ orderItemsSummary(row) }}</template>
            </ElTableColumn>
            <ElTableColumn
              prop="final_amount"
              :label="$t('vnetPages.orders.finalAmount')"
              width="110"
              sortable="custom"
            >
              <template #default="{ row }">{{ formatAmount(row.final_amount) }}</template>
            </ElTableColumn>
            <ElTableColumn
              prop="payment_method"
              :label="$t('vnetPages.orders.paymentMethod')"
              width="140"
              sortable="custom"
            >
              <template #default="{ row }">
                {{ row.payment_method ? paymentLabel(row.payment_method) : '-' }}
              </template>
            </ElTableColumn>
            <ElTableColumn prop="status" :label="$t('vnetPages.orders.status')" width="120" sortable="custom">
              <template #default="{ row }">
                <ElTag :type="ORDER_STATUS_TAG[row.status] || 'info'" size="small">
                  {{ $t(`vnetPages.orders.statusLabels.${row.status}`) }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn :label="$t('vnetPages.orders.machineCode')" width="90">
              <template #default="{ row }">{{ row.machine_code || '-' }}</template>
            </ElTableColumn>
            <ElTableColumn prop="created_at" :label="$t('vnetPages.orders.createdAt')" width="140" sortable="custom">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </ElTableColumn>
          </ElTable>
          <div class="mt-12px flex justify-center">
            <ElPagination
              v-model:current-page="detailOrdPage"
              :page-size="detailOrdSize"
              :total="detailOrdTotal"
              layout="prev, pager, next, total"
              small
              @current-change="fetchDetailOrders"
            />
          </div>
        </ElTabPane>
      </ElTabs>
    </ElCard>
  </ElDialog>
</template>
