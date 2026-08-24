<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAppStore } from '@/store/modules/app';
import { useEcharts } from '@/hooks/common/echarts';

// Ô biểu đồ trên Bảng điều khiển trước đây là một <div> cao 300px chỉ chứa dòng
// chữ "Biểu đồ doanh thu". ECharts và hook useEcharts đã có sẵn trong dự án,
// nguồn dữ liệu /reports/daily-revenue cũng đã trả chuỗi thời gian sắp theo ngày.

defineOptions({ name: 'RevenueChart' });

const { t: $t } = useI18n();
const appStore = useAppStore();

const DAYS = 7;
const empty = ref(false);

const { domRef, updateOptions } = useEcharts(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'cross' }
  },
  // Chú thích để mặc định nằm giữa và đè lên nhãn trục ngày; ghim lên đỉnh và
  // chừa khoảng trống tương ứng ở lưới.
  legend: {
    top: 0,
    data: [$t('vnetPages.dashboard.revenueSeries'), $t('vnetPages.dashboard.ordersSeries')]
  },
  grid: { top: 40, left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: [] as string[]
  },
  yAxis: [
    {
      type: 'value',
      // Tiền quán net dễ lên hàng triệu; để nhãn nguyên số sẽ đẩy trục ra rất rộng.
      axisLabel: { formatter: (v: number) => (v >= 1000 ? `${v / 1000}k` : String(v)) }
    },
    { type: 'value', minInterval: 1 }
  ],
  series: [
    {
      color: '#8e9dff',
      name: $t('vnetPages.dashboard.revenueSeries'),
      type: 'line',
      smooth: true,
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0.25, color: '#8e9dff' },
            { offset: 1, color: 'rgba(142, 157, 255, 0)' }
          ]
        }
      },
      emphasis: { focus: 'series' },
      data: [] as number[]
    },
    {
      color: '#26deca',
      name: $t('vnetPages.dashboard.ordersSeries'),
      type: 'bar',
      yAxisIndex: 1,
      barMaxWidth: 18,
      emphasis: { focus: 'series' },
      data: [] as number[]
    }
  ]
}));

async function load() {
  const to = dayjs();
  const from = to.subtract(DAYS - 1, 'day');

  const res: any = await client
    .get('/reports/daily-revenue', {
      params: { date_from: from.format('YYYY-MM-DD'), date_to: to.format('YYYY-MM-DD') }
    })
    .catch(() => null);
  const rows: any[] = Array.isArray(res) ? res : res?.items || [];

  // Cột date là kiểu date của PostgreSQL nên về tới đây có thể là
  // "2026-08-22T00:00:00Z"; cắt lấy 10 ký tự đầu mới ghép được với ngày ở đây.
  const byDate = new Map<string, any>();
  rows.forEach(r => {
    const key = String(r.date || '').slice(0, 10);
    if (key) byDate.set(key, r);
  });

  // Ngày không có đơn nào không xuất hiện trong kết quả; dựng đủ 7 ngày để
  // đường biểu đồ không nhảy cóc qua ngày vắng khách.
  const labels: string[] = [];
  const revenue: number[] = [];
  const orders: number[] = [];
  for (let i = 0; i < DAYS; i += 1) {
    const d = from.add(i, 'day');
    const row = byDate.get(d.format('YYYY-MM-DD'));
    labels.push(d.format('DD/MM'));
    revenue.push(row?.revenue || 0);
    orders.push(row?.total_orders || 0);
  }

  empty.value = revenue.every(v => v === 0);

  updateOptions(opts => {
    opts.xAxis.data = labels;
    opts.series[0].data = revenue;
    opts.series[1].data = orders;
    return opts;
  });
}

function updateLocale() {
  updateOptions((opts, factory) => {
    const origin = factory();
    opts.legend.data = origin.legend.data;
    opts.series[0].name = origin.series[0].name;
    opts.series[1].name = origin.series[1].name;
    return opts;
  });
}

watch(() => appStore.locale, updateLocale);

onMounted(load);

defineExpose({ load });
</script>

<template>
  <div class="relative">
    <div ref="domRef" class="h-300px overflow-hidden"></div>
    <div
      v-if="empty"
      class="pointer-events-none absolute inset-0 flex items-center justify-center text-13px"
      style="color: #909399"
    >
      {{ $t('vnetPages.dashboard.noRevenue') }}
    </div>
  </div>
</template>

<style scoped></style>
