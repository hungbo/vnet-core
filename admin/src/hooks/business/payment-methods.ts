import { computed, ref } from 'vue';
import client from '@/api/client';
import { $t } from '@/locales';

/** Một phương thức ở Cài đặt > Thanh toán (`GET /payment-methods`). */
export interface PaymentMethod {
  code: string;
  name: string;
  enabled: boolean;
}

// Dùng chung cho cả ứng dụng: mỗi trang nhận tiền gọi hook này, nên chỉ tải một
// lần thay vì mỗi hộp thoại một lần.
const all = ref<PaymentMethod[]>([]);
let pending: Promise<void> | null = null;

function load(force = false) {
  if (!pending || force) {
    pending = client
      .get('/payment-methods')
      .then((res: any) => {
        all.value = Array.isArray(res) ? res : [];
      })
      .catch(() => {
        // Để lần gọi sau thử lại thay vì nhớ mãi một lần lỗi mạng.
        pending = null;
      });
  }
  return pending;
}

// Cách trừ tiền nội bộ, không nằm trong cài đặt nhưng vẫn xuất hiện trong lịch sử.
const SYSTEM_LABELS: Record<string, App.I18n.I18nKey> = {
  balance: 'vnetPages.paymentMethods.balance',
  gift_card: 'vnetPages.paymentMethods.giftCard',
  bonus_balance: 'vnetPages.paymentMethods.bonusBalance',
  topup_card: 'vnetPages.paymentMethods.topupCard'
};

export function usePaymentMethods() {
  load();

  /** Các phương thức đang bật — dùng cho ô chọn. */
  const enabledMethods = computed(() => all.value.filter(m => m.enabled));

  /** Tên hiển thị của một mã, kể cả mã đã tắt hoặc mã nội bộ — dùng cho bảng lịch sử. */
  function paymentLabel(code?: string | null) {
    if (!code) return '-';
    const m = all.value.find(x => x.code === code);
    if (m) return m.name;
    const key = SYSTEM_LABELS[code];
    return key ? $t(key) : code;
  }

  return { methods: all, enabledMethods, paymentLabel, reload: () => load(true) };
}
