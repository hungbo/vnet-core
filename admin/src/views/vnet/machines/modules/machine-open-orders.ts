// Đơn món khách đã gọi mà quầy chưa thu tiền, gom theo máy (GET /machines/open-orders).
// Trang Máy trạm và bảng "Máy đang hoạt động" ở Bảng điều khiển tô máy có đơn bằng
// một màu khác (lớp CSS bên dưới) và hiện nhãn MachineOpenOrdersTag.
import client from '@/api/client';

export interface OpenOrders {
  machine_id: string;
  /** Khách đang gọi, quầy chưa duyệt. */
  pending_count: number;
  /** Đã duyệt/làm món, chưa thanh toán. */
  unpaid_count: number;
  amount: number;
}

/** Lớp của dòng bảng cho máy có đơn chưa thu tiền — xem style ở hai trang. */
export const OPEN_ORDERS_ROW_CLASS = 'machine-row--open-orders';

/** machine_id → đơn chưa thu tiền. Lỗi thì coi như không máy nào có đơn: bảng máy vẫn phải hiện. */
export async function fetchOpenOrders(): Promise<Map<string, OpenOrders>> {
  const rows: any = await client.get('/machines/open-orders').catch(() => []);
  return new Map((Array.isArray(rows) ? rows : []).map((r: OpenOrders) => [r.machine_id, r]));
}
