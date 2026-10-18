import { onBeforeUnmount, onMounted } from 'vue';
import { ElNotification } from 'element-plus';
import { useWebSocketStore } from '@/store/modules/ws';
import { useRouterPush } from '@/hooks/common/router';
import { formatPrice } from '@/utils/money';
import { $t } from '@/locales';

/**
 * Thông báo nổi cho những việc nhân viên KHÔNG được bỏ lỡ, chạy ở mọi trang.
 *
 * Trước đây mỗi trang tự đăng ký handler WebSocket trong onMounted, nên thông
 * báo chỉ hiện khi đang đứng đúng trang đó: khách gửi yêu cầu nạp tiền mà nhân
 * viên đang ở Bảng điều khiển thì không có tiếng, không có chữ, không có gì.
 *
 * Chỗ gọi là base-layout chứ không phải một plugin ở main.ts. Lúc đăng xuất,
 * `wsStore.disconnect()` gọi `handlers.clear()`; plugin chỉ chạy một lần lúc
 * khởi động app nên đăng nhập lại trong cùng tab sẽ mất sạch handler mà không
 * báo lỗi gì. Trang đăng nhập dùng layout.blank, nên base-layout thật sự
 * unmount rồi remount và onMounted dưới đây chạy lại.
 *
 * Chat KHÔNG nằm ở đây: ChatWidget vốn đã mount toàn cục, nên thông báo tin
 * nhắn nằm ngay trong useChatWs. Gom hai thứ vào một "trung tâm thông báo" chỉ
 * thêm một lớp trung gian mà không giải quyết gì.
 */
export function useWsNotify() {
  const wsStore = useWebSocketStore();
  const { routerPushByKey } = useRouterPush();

  function play(src: string) {
    const audio = new Audio(src);
    audio.volume = 0.5;
    // Trình duyệt chặn phát tiếng cho tới khi người dùng bấm vào trang lần đầu.
    // Chữ vẫn hiện, nên nuốt lỗi ở đây là đúng.
    audio.play().catch(() => {});
  }

  function onOrderNew(data: any) {
    const isTopup = data?.order_type === 'topup';
    play(isTopup ? '/audio/deposit.mp3' : '/audio/order.mp3');

    const notification = ElNotification({
      title: $t(isTopup ? 'vnetPages.realtime.topupTitle' : 'vnetPages.realtime.orderTitle'),
      message: $t(isTopup ? 'vnetPages.realtime.topupBody' : 'vnetPages.realtime.orderBody', {
        code: data?.order_code ?? '',
        amount: formatPrice(data?.final_amount)
      }),
      type: isTopup ? 'warning' : 'info',
      duration: 8000,
      onClick: () => {
        notification.close();
        routerPushByKey('vnet_orders');
      }
    });
  }

  function onSessionAutoEnded(data: any) {
    ElNotification({
      title: $t('vnetPages.realtime.sessionEndedTitle'),
      message: $t('vnetPages.realtime.sessionEndedBody', {
        machine: data?.machine_code ?? '',
        reason: $t(`vnetPages.realtime.reasons.${data?.reason ?? 'slot_ended'}` as never)
      }),
      type: 'info',
      duration: 6000
    });
  }

  function onCurfewEnforced(data: any) {
    play('/audio/alarm.mp3');
    ElNotification({
      title: $t('vnetPages.realtime.curfewTitle'),
      message: $t('vnetPages.realtime.curfewBody', { machine: data?.machine_code ?? '' }),
      type: 'warning',
      duration: 0
    });
  }

  onMounted(() => {
    wsStore.on('order:new', onOrderNew);
    wsStore.on('session:auto-ended', onSessionAutoEnded);
    wsStore.on('curfew:enforced', onCurfewEnforced);
  });

  onBeforeUnmount(() => {
    wsStore.off('order:new', onOrderNew);
    wsStore.off('session:auto-ended', onSessionAutoEnded);
    wsStore.off('curfew:enforced', onCurfewEnforced);
  });
}
