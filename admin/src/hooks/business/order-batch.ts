import { ElMessage, ElMessageBox } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';

// Thao tác hàng loạt trên đơn hàng, dùng chung cho trang Đơn hàng và hộp "Đơn
// đang gọi" ở Bảng điều khiển.
//
// Không có API hàng loạt: mỗi đơn đi qua đúng API của thao tác đơn lẻ, nên trừ
// kho, trừ số dư, ghi người thực hiện và kiểm trạng thái giống hệt bấm từng đơn.
// Gửi LẦN LƯỢT chứ không song song: các đơn của cùng một hội viên khoá cùng một
// dòng số dư, gửi dồn một lúc chỉ làm chúng chờ nhau rồi hết hạn.

const DANG_XU_LY = new Set(['pending', 'confirmed']);

export const coTheThanhToan = (o: any) => DANG_XU_LY.has(o.status);
export const coTheHuy = (o: any) => DANG_XU_LY.has(o.status);
// Đơn nạp tiền không có bước xác nhận: duyệt là cộng tiền luôn.
export const coTheXacNhan = (o: any) => o.status === 'pending' && o.order_type !== 'topup';

export const tongTien = (orders: any[]) => orders.reduce((sum, o) => sum + (Number(o.final_amount) || 0), 0);

export function useOrderBatch() {
  const { t } = useI18n();

  async function chay(orders: any[], call: (o: any) => Promise<unknown>) {
    const loi: string[] = [];
    let xong = 0;
    for (const o of orders) {
      try {
        // Cố ý tuần tự, xem ghi chú đầu tệp.
        // eslint-disable-next-line no-await-in-loop
        await call(o);
        xong += 1;
      } catch (e: any) {
        loi.push(`${o.order_code}: ${e?.message || t('vnetPages.common.error')}`);
      }
    }
    const tom = t('vnetPages.orders.batchDone', { ok: xong, total: orders.length });
    if (!loi.length) {
      ElMessage.success(tom);
    } else {
      // Đơn lỗi phải hiện đủ từng mã: một toast "3/5" không cho biết đơn nào còn
      // chờ, và nhân viên sẽ tưởng cả năm đơn đã thu tiền.
      ElMessageBox.alert(loi.join('\n'), tom, { type: 'warning', customStyle: { whiteSpace: 'pre-line' } });
    }
    return xong;
  }

  return {
    thanhToan: (orders: any[], form: { payment_method: string; reference_code?: string }) =>
      chay(orders, o =>
        client.post(`/orders/${o.id}/pay`, {
          payment_method: form.payment_method,
          // Máy chủ từ chối số tiền lệch với đơn, nên gửi đúng tổng của chính đơn.
          amount: o.final_amount,
          reference_code: form.reference_code || ''
        })
      ),
    xacNhan: (orders: any[]) => chay(orders, o => client.post(`/orders/${o.id}/status`, { status: 'confirmed' })),
    huy: (orders: any[]) => chay(orders, o => client.post(`/orders/${o.id}/status`, { status: 'cancelled' }))
  };
}
