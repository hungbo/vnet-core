<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { usePaymentMethods } from '@/hooks/business/payment-methods';
import { useFitPageSize } from '@/hooks/common/table';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { useTodayRange } from '@/hooks/common/today-filter';
import { formatPrice } from '@/utils/money';

const { t: $t } = useI18n();
const { methods, paymentLabel } = usePaymentMethods();

// Một trang, một bộ lọc: thẻ tổng, bảng từng dòng và mọi cách "Xem theo" đều
// đọc GET /reports với cùng các ô lọc, nên số ở thẻ luôn khớp với bảng.
const { range: dateRange, follow: followToday, reset: resetToday } = useTodayRange();
const kind = ref('');
const employeeId = ref('');
const machineId = ref('');
const paymentMethod = ref('');
const productId = ref('');
const search = ref('');
// Chỉ đặt được khi bấm vào một dòng nhóm, hiện thành thẻ có nút xoá.
const member = ref<{ id: string; label: string } | null>(null);
const promotion = ref<{ id: string; label: string } | null>(null);

const groupBy = ref('');
const totals = ref<any>({});
const rows = ref<any[]>([]);
const total = ref(0);
const loading = ref(false);
const page = ref(1);
const pageSize = ref(20);
const sort = ref<string>();
const order = ref<'asc' | 'desc'>();

const fitter = useFitPageSize({
  getPageSize: () => pageSize.value,
  setPageSize: size => {
    pageSize.value = size;
    page.value = 1;
    fetchData();
  },
  isManual: () => groupBy.value !== ''
});

const kindOptions = computed(() =>
  ['sale', 'topup', 'combo', 'refund', 'session'].map(k => ({ value: k, label: kindLabel(k) }))
);
const kindTag: Record<string, any> = {
  sale: 'primary',
  topup: 'success',
  combo: 'warning',
  refund: 'danger',
  session: 'info'
};
function kindLabel(k: string) {
  return $t(`vnetPages.reports.kinds.${k}` as any);
}

const views = computed(() => [
  { value: '', label: $t('vnetPages.reports.lines') },
  { value: 'day', label: $t('vnetPages.reports.date') },
  { value: 'month', label: $t('vnetPages.reports.month') },
  { value: 'employee', label: $t('vnetPages.reports.employee') },
  { value: 'member', label: $t('vnetPages.reports.member') },
  { value: 'machine', label: $t('vnetPages.reports.machine') },
  { value: 'product', label: $t('vnetPages.reports.product') },
  { value: 'payment_method', label: $t('vnetPages.reports.paymentMethod') },
  { value: 'kind', label: $t('vnetPages.reports.kind') },
  { value: 'promotion', label: $t('vnetPages.reports.promotion') }
]);

// Mốc tính lúc bấm, không lúc mở trang: trang quầy để mở qua đêm.
const range = (from: dayjs.Dayjs, to: dayjs.Dayjs) => [from.toDate(), to.toDate()];
const shortcuts = computed(() => [
  { text: $t('vnetPages.reports.today'), value: () => range(dayjs(), dayjs()) },
  {
    text: $t('vnetPages.reports.yesterday'),
    value: () => range(dayjs().subtract(1, 'day'), dayjs().subtract(1, 'day'))
  },
  { text: $t('vnetPages.reports.last7Days'), value: () => range(dayjs().subtract(6, 'day'), dayjs()) },
  { text: $t('vnetPages.reports.thisMonth'), value: () => range(dayjs().startOf('month'), dayjs()) },
  {
    text: $t('vnetPages.reports.lastMonth'),
    value: () => range(dayjs().subtract(1, 'month').startOf('month'), dayjs().subtract(1, 'month').endOf('month'))
  }
]);

function filterParams() {
  return {
    date_from: dateRange.value?.[0] || undefined,
    date_to: dateRange.value?.[1] || undefined,
    kind: kind.value || undefined,
    employee_id: employeeId.value || undefined,
    machine_id: machineId.value || undefined,
    payment_method: paymentMethod.value || undefined,
    product_id: productId.value || undefined,
    member_id: member.value?.id,
    promotion_id: promotion.value?.id,
    search: search.value.trim() || undefined
  };
}

async function fetchData() {
  followToday();
  loading.value = true;
  try {
    const res: any = await client.get('/reports', {
      params: {
        ...filterParams(),
        group_by: groupBy.value || undefined,
        page: page.value,
        page_size: pageSize.value,
        sort: sort.value,
        order: order.value
      }
    });
    totals.value = res.totals || {};
    rows.value = res.rows || [];
    total.value = res.total || 0;
    if (!groupBy.value) await fitter.afterLoad();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.reports.messages.loadError'));
  } finally {
    loading.value = false;
  }
}

function reload() {
  page.value = 1;
  fetchData();
}

function onSortChange(e: { prop: string | null; order: 'ascending' | 'descending' | null }) {
  if (e.order && e.prop) {
    sort.value = e.prop;
    order.value = e.order === 'ascending' ? 'asc' : 'desc';
  } else {
    sort.value = undefined;
    order.value = undefined;
  }
  reload();
}

function changeView() {
  sort.value = undefined;
  order.value = undefined;
  reload();
}

function clearFilters() {
  resetToday();
  kind.value = '';
  employeeId.value = '';
  machineId.value = '';
  paymentMethod.value = '';
  productId.value = '';
  search.value = '';
  member.value = null;
  promotion.value = null;
  reload();
}

// Ô lọc nhân viên/máy: tải một lần. Ô món: các món đã bán trong khoảng ngày
// đang chọn — danh sách món đầy đủ bị giới hạn 100 dòng mỗi lần gọi.
const employees = ref<any[]>([]);
const machines = ref<any[]>([]);
const products = ref<any[]>([]);
async function loadOptions() {
  const [nv, may]: any[] = await Promise.all([
    client.get('/reports/employees').catch(() => []),
    client.get('/machines', { params: { page_size: 100, sort: 'machine_code', order: 'asc' } }).catch(() => null)
  ]);
  employees.value = nv || [];
  machines.value = may?.items || [];
}
async function loadProducts(open: boolean) {
  if (!open) return;
  const res: any = await client
    .get('/reports', {
      params: { date_from: dateRange.value?.[0], date_to: dateRange.value?.[1], group_by: 'product' }
    })
    .catch(() => null);
  products.value = res?.rows || [];
}

// Phương thức đã tắt vẫn có trong lịch sử, nên lấy cả danh sách chứ không chỉ
// phương thức đang bật; thêm các cách trừ nội bộ.
const paymentOptions = computed(() => [
  ...methods.value.map(m => ({ value: m.code, label: m.name })),
  ...['balance', 'bonus_balance'].map(code => ({ value: code, label: paymentLabel(code) }))
]);

const hours = (minutes: number) => `${Math.round(((minutes || 0) / 60) * 10) / 10}`;

const cards = computed(() => {
  const t = totals.value;
  return [
    {
      label: $t('vnetPages.reports.moneyIn'),
      value: t.money_in,
      hint: $t('vnetPages.reports.moneyInHint'),
      main: true
    },
    {
      label: kindLabel('sale'),
      value: t.sales,
      sub: $t('vnetPages.reports.saleCount', { count: t.sale_count || 0 })
    },
    {
      label: kindLabel('topup'),
      value: t.topups,
      sub: $t('vnetPages.reports.topupCount', { count: t.topup_count || 0 })
    },
    {
      label: kindLabel('combo'),
      value: t.combos,
      sub: $t('vnetPages.reports.comboCount', { count: t.combo_count || 0 })
    },
    { label: kindLabel('refund'), value: t.refunds },
    {
      label: $t('vnetPages.reports.paidByBalance'),
      value: t.paid_by_balance,
      hint: $t('vnetPages.reports.paidByBalanceHint')
    },
    {
      label: kindLabel('session'),
      value: t.session_fees,
      sub: $t('vnetPages.reports.sessionCount', { count: t.session_count || 0, hours: hours(t.minutes) })
    },
    { label: $t('vnetPages.reports.discount'), value: t.discount }
  ];
});

// Cột số của bảng nhóm. Cột nào cả bảng đều 0 thì ẩn, để "Xem theo máy" không
// kèm một dãy cột nạp tiền trống.
const groupColumns = computed(() => {
  if (groupBy.value === 'product') {
    return [
      { prop: 'quantity', label: $t('vnetPages.reports.quantity'), width: 110 },
      { prop: 'sales', label: kindLabel('sale'), money: true },
      { prop: 'sale_count', label: $t('vnetPages.reports.orders'), width: 110 }
    ];
  }
  const all = [
    { prop: 'money_in', label: $t('vnetPages.reports.moneyIn'), money: true, always: true },
    { prop: 'sales', label: kindLabel('sale'), money: true },
    { prop: 'sale_count', label: $t('vnetPages.reports.orders'), width: 85 },
    { prop: 'topups', label: kindLabel('topup'), money: true },
    { prop: 'topup_count', label: $t('vnetPages.reports.topupTimes'), width: 85 },
    { prop: 'combos', label: kindLabel('combo'), money: true },
    { prop: 'refunds', label: kindLabel('refund'), money: true },
    { prop: 'paid_by_balance', label: $t('vnetPages.reports.paidByBalance'), money: true },
    { prop: 'session_fees', label: kindLabel('session'), money: true },
    { prop: 'session_count', label: $t('vnetPages.reports.sessions'), width: 85 },
    { prop: 'minutes', label: $t('vnetPages.reports.hours'), width: 85, hours: true },
    { prop: 'discount', label: $t('vnetPages.reports.discount'), money: true }
  ];
  return all.filter(c => c.always || rows.value.some(r => Number(r[c.prop]) !== 0));
});

function groupLabel(row: any) {
  switch (groupBy.value) {
    case 'day':
      return dayjs(row.key).format('DD/MM/YYYY');
    case 'month':
      return dayjs(`${row.key}-01`).format('MM/YYYY');
    case 'payment_method':
      return row.key ? paymentLabel(row.key) : '—';
    case 'kind':
      return kindLabel(row.key);
    case 'employee':
      return row.label || $t('vnetPages.reports.noEmployee');
    case 'member':
      return row.label || $t('vnetPages.reports.noMember');
    case 'machine':
      return row.label || $t('vnetPages.reports.noMachine');
    case 'promotion':
      return row.label || $t('vnetPages.reports.noPromotion');
    default:
      return row.label || '—';
  }
}

// Bấm một dòng nhóm: lọc đúng nhóm ấy rồi xem từng dòng.
function drill(row: any) {
  const key = row.key;
  switch (groupBy.value) {
    case 'day':
      dateRange.value = [key, key];
      break;
    case 'month': {
      const d = dayjs(`${key}-01`);
      dateRange.value = [d.format('YYYY-MM-DD'), d.endOf('month').format('YYYY-MM-DD')];
      break;
    }
    case 'employee':
      employeeId.value = key || 'none';
      break;
    case 'member':
      if (!key) return;
      member.value = { id: key, label: row.label };
      break;
    case 'machine':
      if (!key) return;
      machineId.value = key;
      break;
    case 'product':
      products.value = [row];
      productId.value = key;
      break;
    case 'payment_method':
      paymentMethod.value = key;
      break;
    case 'kind':
      kind.value = key;
      break;
    case 'promotion':
      if (!key) return;
      promotion.value = { id: key, label: row.label };
      break;
    default:
      return;
  }
  groupBy.value = '';
  changeView();
}

onMounted(() => {
  loadOptions();
  if (!fitter.fit()) fetchData();
});

// Báo cáo đọc đơn hàng (orders), sổ ví hội viên (nạp/hoàn ở members, mua gói ở
// combos), phiên đã kết thúc (sessions, hay tắt máy từ xa ở machines) và tên ở
// members/machines/promotions/systemManage. Phiên do tác vụ nền đóng (hết giờ,
// hết tiền, máy mất tín hiệu) không qua HTTP nhưng vẫn phát session:ended. Gộp
// 2 giây để một loạt thao tác chỉ chạy truy vấn tổng hợp một lần.
useRealtimeRefresh(['orders', 'members', 'combos', 'sessions', 'machines', 'promotions', 'systemManage'], fetchData, {
  events: ['session:ended'],
  delay: 2000
});
</script>

<template>
  <div class="page-fill">
    <ElCard class="fill-card">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center; gap: 8px">
          <span style="white-space: nowrap">{{ $t('vnetPages.reports.title') }}</span>
          <div style="display: flex; flex-wrap: wrap; gap: 8px; align-items: center; justify-content: flex-end">
            <ElDatePicker
              v-model="dateRange"
              type="daterange"
              range-separator="→"
              :start-placeholder="$t('vnetPages.reports.from')"
              :end-placeholder="$t('vnetPages.reports.to')"
              value-format="YYYY-MM-DD"
              :shortcuts="shortcuts"
              style="width: 240px"
              @change="reload"
            />
            <ElSelect
              v-model="kind"
              :placeholder="$t('vnetPages.reports.kind')"
              clearable
              style="width: 120px"
              @change="reload"
            >
              <ElOption v-for="o in kindOptions" :key="o.value" :label="o.label" :value="o.value" />
            </ElSelect>
            <ElSelect
              v-model="employeeId"
              :placeholder="$t('vnetPages.reports.employee')"
              clearable
              filterable
              style="width: 150px"
              @change="reload"
            >
              <ElOption v-for="e in employees" :key="e.id" :label="e.name" :value="e.id" />
              <ElOption :label="$t('vnetPages.reports.noEmployee')" value="none" />
            </ElSelect>
            <ElSelect
              v-model="machineId"
              :placeholder="$t('vnetPages.reports.machine')"
              clearable
              filterable
              style="width: 110px"
              @change="reload"
            >
              <ElOption v-for="m in machines" :key="m.id" :label="m.machine_code" :value="m.id" />
            </ElSelect>
            <ElSelect
              v-model="paymentMethod"
              :placeholder="$t('vnetPages.reports.paymentMethod')"
              clearable
              style="width: 130px"
              @change="reload"
            >
              <ElOption v-for="p in paymentOptions" :key="p.value" :label="p.label" :value="p.value" />
            </ElSelect>
            <ElSelect
              v-model="productId"
              :placeholder="$t('vnetPages.reports.product')"
              clearable
              filterable
              style="width: 160px"
              @visible-change="loadProducts"
              @change="reload"
            >
              <ElOption v-for="p in products" :key="p.key" :label="p.label" :value="p.key" />
            </ElSelect>
            <ElInput
              v-model="search"
              :placeholder="$t('vnetPages.reports.searchPlaceholder')"
              clearable
              style="width: 220px"
              @keyup.enter="reload"
              @clear="reload"
            />
            <ElButton type="primary" @click="reload">{{ $t('vnetPages.common.search') }}</ElButton>
            <ElButton @click="clearFilters">{{ $t('vnetPages.reports.clearFilters') }}</ElButton>
          </div>
        </div>
      </template>

      <div v-if="member || promotion" style="display: flex; gap: 8px; margin-bottom: 8px">
        <ElTag
          v-if="member"
          closable
          @close="
            member = null;
            reload();
          "
        >
          {{ $t('vnetPages.reports.member') }}: {{ member.label }}
        </ElTag>
        <ElTag
          v-if="promotion"
          closable
          @close="
            promotion = null;
            reload();
          "
        >
          {{ $t('vnetPages.reports.promotion') }}: {{ promotion.label || $t('vnetPages.reports.noPromotion') }}
        </ElTag>
      </div>

      <div class="report-cards">
        <div v-for="c in cards" :key="c.label" class="report-card" :class="{ 'report-card--main': c.main }">
          <div class="report-card__label">
            {{ c.label }}
            <ElTooltip v-if="c.hint" :content="c.hint" placement="top">
              <span class="report-card__hint">?</span>
            </ElTooltip>
          </div>
          <div class="report-card__value">{{ formatPrice(c.value || 0) }}</div>
          <div v-if="c.sub" class="report-card__sub">{{ c.sub }}</div>
        </div>
      </div>

      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin: 12px 0 8px">
        <span style="color: var(--el-text-color-secondary); font-size: 13px">{{ $t('vnetPages.reports.viewBy') }}</span>
        <ElRadioGroup v-model="groupBy" size="small" @change="changeView">
          <ElRadioButton v-for="v in views" :key="v.value" :value="v.value">{{ v.label }}</ElRadioButton>
        </ElRadioGroup>
        <span v-if="groupBy" style="color: var(--el-text-color-secondary); font-size: 12px">
          {{ $t('vnetPages.reports.drillHint') }}
        </span>
      </div>

      <ElTable
        v-if="!groupBy"
        v-loading="loading"
        :data="rows"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%"
        @sort-change="onSortChange"
      >
        <ElTableColumn prop="at" :label="$t('vnetPages.reports.time')" width="110" sortable="custom">
          <template #default="{ row }">{{ dayjs(row.at).format('DD/MM HH:mm') }}</template>
        </ElTableColumn>
        <ElTableColumn prop="kind" :label="$t('vnetPages.reports.kind')" width="100" sortable="custom">
          <template #default="{ row }">
            <ElTag :type="kindTag[row.kind]" size="small">{{ kindLabel(row.kind) }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="code" :label="$t('vnetPages.reports.code')" width="110" sortable="custom" />
        <ElTableColumn prop="member_name" :label="$t('vnetPages.reports.member')" min-width="110" sortable="custom">
          <template #default="{ row }">{{ row.member_name || '—' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="machine_code" :label="$t('vnetPages.reports.machine')" width="85" sortable="custom">
          <template #default="{ row }">{{ row.machine_code || '—' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="employee_name" :label="$t('vnetPages.reports.employee')" width="110" sortable="custom">
          <template #default="{ row }">{{ row.employee_name || '—' }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="payment_method"
          :label="$t('vnetPages.reports.paymentMethod')"
          width="115"
          sortable="custom"
        >
          <template #default="{ row }">{{ row.payment_method ? paymentLabel(row.payment_method) : '—' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="note" :label="$t('vnetPages.reports.note')" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.kind === 'session' ? `${hours(row.minutes)} ${$t('vnetPages.reports.hourUnit')}` : row.note }}
          </template>
        </ElTableColumn>
        <ElTableColumn
          prop="amount"
          :label="$t('vnetPages.reports.amount')"
          width="110"
          align="right"
          sortable="custom"
        >
          <template #default="{ row }">{{ formatPrice(row.amount) }}</template>
        </ElTableColumn>
        <ElTableColumn
          prop="money_in"
          :label="$t('vnetPages.reports.moneyIn')"
          width="120"
          align="right"
          sortable="custom"
        >
          <template #default="{ row }">
            <span :style="{ color: row.money_in ? undefined : 'var(--el-text-color-placeholder)' }">
              {{ formatPrice(row.money_in) }}
            </span>
          </template>
        </ElTableColumn>
      </ElTable>

      <ElTable
        v-else
        v-loading="loading"
        :data="rows"
        border
        stripe
        class="fill-table"
        height="100%"
        style="width: 100%; cursor: pointer"
        :default-sort="groupBy === 'day' || groupBy === 'month' ? { prop: 'key', order: 'ascending' } : undefined"
        @row-click="drill"
      >
        <ElTableColumn
          :key="groupBy"
          prop="key"
          :label="views.find(v => v.value === groupBy)?.label"
          min-width="140"
          :sortable="groupBy === 'day' || groupBy === 'month'"
        >
          <template #default="{ row }">{{ groupLabel(row) }}</template>
        </ElTableColumn>
        <ElTableColumn
          v-for="c in groupColumns"
          :key="c.prop"
          :prop="c.prop"
          :label="c.label"
          :min-width="c.width ? undefined : 110"
          :width="c.width"
          align="right"
          sortable
        >
          <template #default="{ row }">
            {{ c.money ? formatPrice(row[c.prop]) : c.hours ? hours(row[c.prop]) : row[c.prop] }}
          </template>
        </ElTableColumn>
      </ElTable>

      <div v-if="!groupBy" style="display: flex; justify-content: center; margin-top: 12px">
        <ElPagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next, total"
          @current-change="fetchData"
        />
      </div>
    </ElCard>
  </div>
</template>

<style scoped>
.report-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 8px;
}

.report-card {
  padding: 8px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
}

.report-card--main {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.report-card__label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.report-card__hint {
  display: inline-block;
  width: 14px;
  height: 14px;
  margin-left: 2px;
  border-radius: 50%;
  border: 1px solid currentColor;
  font-size: 10px;
  line-height: 12px;
  text-align: center;
  cursor: help;
}

.report-card__value {
  margin-top: 2px;
  font-size: 18px;
  font-weight: 600;
}

.report-card--main .report-card__value {
  color: var(--el-color-primary);
}

.report-card__sub {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
