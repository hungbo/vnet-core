import { useI18n } from 'vue-i18n';

// Tên hiển thị của loại giao dịch hội viên (member_transactions.transaction_type).
// Dùng chung cho trang Sổ giao dịch và tab Giao dịch trong chi tiết hội viên.
// Backend ghi ra nhiều loại; thiếu khoá nào thì nhân viên đọc nguyên mã thô
// ("attendance_bonus" thay vì "Thưởng điểm danh"), nên thêm loại mới thì thêm ở đây.
const TYPE_KEYS: Record<string, string> = {
  topup: 'vnetPages.transactions.topup',
  topup_bonus: 'vnetPages.transactions.topupBonus',
  topup_card: 'vnetPages.transactions.topupCard',
  session_fee: 'vnetPages.transactions.sessionFee',
  refund: 'vnetPages.transactions.refund',
  refund_bonus: 'vnetPages.transactions.refundBonus',
  cancel: 'vnetPages.transactions.cancel',
  combo_purchase: 'vnetPages.transactions.comboPurchase',
  attendance_bonus: 'vnetPages.transactions.attendanceBonus',
  booking_deposit: 'vnetPages.transactions.bookingDeposit',
  deposit_refund: 'vnetPages.transactions.depositRefund',
  order_payment: 'vnetPages.transactions.orderPayment',
  lucky_spin_balance: 'vnetPages.transactions.luckySpinBalance',
  lucky_spin_bonus: 'vnetPages.transactions.luckySpinBonus'
};

export function useTransactionTypeLabel() {
  const { t } = useI18n();
  return (type: string) => (TYPE_KEYS[type] ? t(TYPE_KEYS[type]) : type);
}
