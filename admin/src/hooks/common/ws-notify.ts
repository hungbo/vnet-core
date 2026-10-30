import { onBeforeUnmount, onMounted, watch } from 'vue';
import { ElNotification } from 'element-plus';
import type { NotificationHandle } from 'element-plus';
import client from '@/api/client';
import { useWebSocketStore } from '@/store/modules/ws';
import { useAuth } from '@/hooks/business/auth';
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
  const { hasAuth } = useAuth();
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
    // Lượt nhắc đầu sau 5 giây: tiếng và thông báo của chính đơn này vừa phát xong.
    henNhac();
  }

  // Đơn chờ duyệt (khách gọi món, khách xin nạp tiền) chỉ kêu một lần lúc đơn tới
  // thì nhân viên không đứng ở quầy là lỡ. Còn đơn "pending" thì cứ 5 giây kêu lại
  // và giữ một thông báo không tự tắt, tới khi không còn đơn nào chờ.
  //
  // Mỗi lượt đếm lại bằng API chứ không cộng trừ theo sự kiện WebSocket: nhân viên
  // khác duyệt ở máy khác, hay rớt mạng lỡ sự kiện, thì vẫn tắt đúng lúc. Hết đơn
  // chờ thì thôi hỏi; đơn mới tới hay WebSocket nối lại thì hỏi lại.
  const NHAC_DON_CHO_MS = 5000;
  let nhacTimer: ReturnType<typeof setTimeout> | null = null;
  let nhacHop: NotificationHandle | null = null;
  let nhacChu = '';
  let dangMo = false;

  function henNhac() {
    if (dangMo && !nhacTimer) nhacTimer = setTimeout(() => nhacDonCho(true), NHAC_DON_CHO_MS);
  }

  // Kiểm ngay, không kêu (lúc mở trang, lúc nối lại). Đã có lượt hẹn thì để lượt đó lo.
  function kiemNgay() {
    if (!nhacTimer) nhacDonCho(false);
  }

  function dongNhac() {
    nhacHop?.close();
    nhacHop = null;
    nhacChu = '';
  }

  async function nhacDonCho(keu: boolean) {
    nhacTimer = null;
    if (!dangMo || !hasAuth('orders.view')) return;
    let data: any;
    try {
      data = await client.get('/orders', { params: { status: 'pending', page: 1, page_size: 100 } });
    } catch {
      henNhac();
      return;
    }
    if (!dangMo) return;
    const tong: number = data?.total ?? 0;
    if (!tong) {
      dongNhac();
      return;
    }
    const napTien = (data?.items ?? []).filter((o: any) => o.order_type === 'topup').length;
    const goiMon = tong - napTien;
    if (keu) play(napTien ? '/audio/deposit.mp3' : '/audio/order.mp3');

    const chu = [
      goiMon ? $t('vnetPages.realtime.pendingOrders', { count: goiMon }) : '',
      napTien ? $t('vnetPages.realtime.pendingTopups', { count: napTien }) : ''
    ]
      .filter(Boolean)
      .join(' ');
    // Nhân viên đóng thông báo mà đơn vẫn chờ thì lượt sau hiện lại.
    if (chu !== nhacChu || !nhacHop) {
      nhacHop?.close();
      nhacChu = chu;
      const hop = ElNotification({
        title: $t('vnetPages.realtime.pendingTitle'),
        message: chu,
        type: 'warning',
        duration: 0,
        onClose: () => {
          if (nhacHop === hop) nhacHop = null;
        },
        onClick: () => {
          hop.close();
          routerPushByKey('vnet_orders');
        }
      });
      nhacHop = hop;
    }
    henNhac();
  }

  watch(
    () => wsStore.connected,
    now => {
      if (now && dangMo) kiemNgay();
    }
  );

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

  // Khách rời máy (trả máy, hết tiền, tắt máy ngang) mà còn đơn chưa thu tiền.
  // Kêu lặp tới khi nhân viên đóng thông báo, tối đa ba lượt (~15 giây): kêu một
  // lần dễ lẫn vào tiếng ồn của quán, mà lỡ lúc này là khách ra khỏi cửa.
  function onUnpaidLeft(data: any) {
    const audio = new Audio('/audio/alarm.mp3');
    audio.volume = 0.5;
    let luot = 1;
    audio.onended = () => {
      if (luot >= 3) return;
      luot += 1;
      audio.play().catch(() => {});
    };
    audio.play().catch(() => {});

    const offline = data?.reason === 'offline';
    const notification = ElNotification({
      title: $t('vnetPages.realtime.unpaidLeftTitle', { machine: data?.machine_code ?? '' }),
      message: $t(offline ? 'vnetPages.realtime.unpaidLeftOffline' : 'vnetPages.realtime.unpaidLeftEnded', {
        member: data?.member_name || $t('vnetPages.realtime.guest'),
        count: data?.order_count ?? 0,
        amount: formatPrice(data?.amount)
      }),
      type: 'error',
      duration: 0,
      onClose: () => {
        luot = 3;
        audio.pause();
      },
      onClick: () => {
        notification.close();
        routerPushByKey('vnet_orders');
      }
    });
  }

  onMounted(() => {
    wsStore.on('order:new', onOrderNew);
    wsStore.on('session:auto-ended', onSessionAutoEnded);
    wsStore.on('curfew:enforced', onCurfewEnforced);
    wsStore.on('order:unpaid-left', onUnpaidLeft);
    dangMo = true;
    kiemNgay();
  });

  onBeforeUnmount(() => {
    wsStore.off('order:new', onOrderNew);
    wsStore.off('session:auto-ended', onSessionAutoEnded);
    wsStore.off('curfew:enforced', onCurfewEnforced);
    wsStore.off('order:unpaid-left', onUnpaidLeft);
    dangMo = false;
    if (nhacTimer) clearTimeout(nhacTimer);
    nhacTimer = null;
    dongNhac();
  });
}
