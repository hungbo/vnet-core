import { useRouter } from 'vue-router';
import { useAuthStore } from '@/store/modules/auth';
import { useRouteStore } from '@/store/modules/route';
import { useTabStore } from '@/store/modules/tab';
import { useRealtimeRefresh } from './realtime';

/**
 * Sửa vai trò hay nhân viên ở trang Phân quyền thì áp ngay cho mọi người đang mở
 * trang quản trị, không bắt đăng nhập lại.
 *
 * API đã theo quyền mới ngay (backend đọc quyền từ database ở mỗi yêu cầu —
 * middleware.LivePermissions). Còn nút bấm (hasAuth) và menu thì giao diện chỉ tải
 * một lần lúc mở trang, nên nghe "systemManage:changed" để tải lại: quyền khác đi
 * thì dựng lại menu. Gắn ở base-layout như useWsNotify để chạy ở mọi trang.
 */
export function usePermissionSync() {
  const authStore = useAuthStore();
  const routeStore = useRouteStore();
  const tabStore = useTabStore();
  const router = useRouter();

  async function sync() {
    if (!(await authStore.refreshPermissions())) return;

    await routeStore.initAuthRoute();

    // Tab của trang vừa mất quyền: đóng, không thì bấm vào là trang 404. Đóng tab
    // đang mở thì tự chuyển sang tab bên cạnh, nên đóng lần lượt từng tab.
    for (const tab of [...tabStore.tabs]) {
      // eslint-disable-next-line no-await-in-loop
      if (!router.hasRoute(tab.routeKey)) await tabStore.removeTab(tab.id);
    }

    // Vẫn đứng ở trang vừa mất quyền (trang không có tab, hoặc tab bên cạnh vừa chuyển
    // tới cũng bị đóng): về trang chủ, thay vì đứng ở trang mà mọi API đều trả 403.
    if (router.resolve(router.currentRoute.value.fullPath).name === 'not-found') {
      await router.replace('/');
    }
  }

  useRealtimeRefresh(['systemManage'], sync);
}
