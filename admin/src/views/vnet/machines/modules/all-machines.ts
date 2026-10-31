import client from '@/api/client';

// Toàn bộ máy: API trả tối đa 100 máy một trang, quán lớn hơn thì tải nốt các
// trang còn lại, nếu không thẻ online/tổng và bảng máy sẵn sàng thiếu máy.
// Dùng ở Bảng điều khiển và nút "Thông báo tất cả máy đang bật" của trang Máy.
export async function fetchAllMachines(): Promise<{ items: any[]; total: number } | null> {
  const pageSize = 100;
  const first: any = await (
    client.get('/machines', { params: { page: 1, page_size: pageSize } }) as Promise<any>
  ).catch(() => null);
  if (!first) return null;
  const items: any[] = Array.isArray(first) ? first : first.items || [];
  const total: number = first.total ?? items.length;
  const pages = Math.ceil(total / pageSize);
  if (pages > 1) {
    const rest = await Promise.all(
      Array.from({ length: pages - 1 }, (_, i) =>
        (client.get('/machines', { params: { page: i + 2, page_size: pageSize } }) as Promise<any>).catch(() => null)
      )
    );
    for (const r of rest) items.push(...(r?.items || []));
  }
  return { items, total };
}
