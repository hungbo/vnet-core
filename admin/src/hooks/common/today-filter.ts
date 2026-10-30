import { ref } from 'vue';
import dayjs from 'dayjs';

const homNay = () => dayjs().format('YYYY-MM-DD');

/**
 * Ô khoảng ngày mặc định hôm nay. Chưa ai đổi thì ô tự sang ngày mới: trang quầy
 * mở qua đêm (tab giữ trang sống), không thì sau 0 giờ đơn và giao dịch mới bị
 * lọc mất vì ô vẫn đứng ở hôm qua. Gọi follow() trước mỗi lần tải.
 */
export function useTodayRange() {
  let ngay = homNay();
  const range = ref<any>([ngay, ngay]);

  function follow() {
    const v = range.value;
    if (v?.[0] === ngay && v?.[1] === ngay && ngay !== homNay()) {
      ngay = homNay();
      range.value = [ngay, ngay];
    }
  }

  function reset() {
    ngay = homNay();
    range.value = [ngay, ngay];
  }

  return { range, follow, reset };
}

/** Như useTodayRange, cho ô chọn một ngày. */
export function useTodayDate() {
  let ngay = homNay();
  const date = ref(ngay);

  function follow() {
    if (date.value === ngay && ngay !== homNay()) {
      ngay = homNay();
      date.value = ngay;
    }
  }

  return { date, follow };
}
