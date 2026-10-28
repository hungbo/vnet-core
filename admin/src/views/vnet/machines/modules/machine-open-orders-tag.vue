<script setup lang="ts">
// Nhãn đỏ cho máy còn đơn chưa thu tiền: "Đang gọi món" khi có đơn khách vừa gọi mà
// quầy chưa duyệt, "Chưa thanh toán" khi món đã làm mà chưa thu tiền. Di chuột vào
// thì thấy tách từng loại. Dùng chung cho trang Máy và Bảng điều khiển.
import { useI18n } from 'vue-i18n';
import { formatPrice } from '@/utils/money';
import type { OpenOrders } from './machine-open-orders';

defineOptions({ name: 'MachineOpenOrdersTag' });

defineProps<{ orders?: OpenOrders | null }>();

const { t: $t } = useI18n();
</script>

<template>
  <ElTooltip
    v-if="orders"
    :content="
      $t('vnetPages.machines.openOrdersHint', {
        pending: orders.pending_count,
        unpaid: orders.unpaid_count,
        amount: formatPrice(orders.amount)
      })
    "
    placement="top"
  >
    <ElTag type="danger" effect="dark" size="small">
      {{ orders.pending_count ? $t('vnetPages.machines.openCalling') : $t('vnetPages.machines.openUnpaid') }}
      · {{ formatPrice(orders.amount) }}
    </ElTag>
  </ElTooltip>
</template>
