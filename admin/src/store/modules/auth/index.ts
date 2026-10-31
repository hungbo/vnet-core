import { computed, reactive, ref } from 'vue';
import { useRoute } from 'vue-router';
import { defineStore } from 'pinia';
import { useLoading } from '@sa/hooks';
import { fetchGetUserInfo, fetchLogin, fetchLoginTotp } from '@/service/api';
import { useRouterPush } from '@/hooks/common/router';
import { localStg } from '@/utils/storage';
import { SetupStoreId } from '@/enum';
import { $t } from '@/locales';
import { useRouteStore } from '../route';
import { useTabStore } from '../tab';
import { useWebSocketStore } from '../ws';
import { clearAuthStorage, getToken } from './shared';

export const useAuthStore = defineStore(SetupStoreId.Auth, () => {
  const route = useRoute();
  const authStore = useAuthStore();
  const routeStore = useRouteStore();
  const tabStore = useTabStore();
  const wsStore = useWebSocketStore();
  const { toLogin, redirectFromLogin } = useRouterPush(false);
  const { loading: loginLoading, startLoading, endLoading } = useLoading();

  const token = ref(getToken());

  /** Vé của bước nhập mã 2 bước; rỗng là đang ở bước mật khẩu. */
  const totpChallenge = ref('');
  /** Quán bắt buộc 2 bước mà tài khoản chưa cài: mã QR để quét ngay ở trang đăng nhập. */
  const totpSetup = ref<Api.Auth.TwoFactorSetup | null>(null);
  /** Vừa cài xong lúc đăng nhập: mã dự phòng hiện trước, bấm "Đã lưu mã" mới vào trong. */
  const recoveryCodes = ref<string[]>([]);
  let pendingLogin: { token: Api.Auth.LoginToken; redirect: boolean } | null = null;

  const userInfo: Api.Auth.UserInfo = reactive({
    id: '',
    username: '',
    full_name: '',
    email: '',
    phone: '',
    avatar_url: '',
    role: '',
    permissions: []
  });

  const roles = computed(() => (userInfo.role ? [userInfo.role] : []));

  /** is super role in static route */
  const isStaticSuper = computed(() => {
    const { VITE_AUTH_ROUTE_MODE, VITE_STATIC_SUPER_ROLE } = import.meta.env;

    return VITE_AUTH_ROUTE_MODE === 'static' && roles.value.includes(VITE_STATIC_SUPER_ROLE);
  });

  /** Is login */
  const isLogin = computed(() => Boolean(token.value));

  /** Reset auth store */
  async function resetStore() {
    recordUserId();

    clearAuthStorage();

    authStore.$reset();

    wsStore.disconnect();

    if (!route.meta.constant) {
      await toLogin();
    }

    tabStore.cacheTabs();
    routeStore.resetStore();
  }

  /** Record the user ID of the previous login session Used to compare with the current user ID on next login */
  function recordUserId() {
    if (!userInfo.id) {
      return;
    }

    localStg.set('lastLoginUserId', userInfo.id);
  }

  function checkTabClear(): boolean {
    if (!userInfo.id) {
      return false;
    }

    const lastLoginUserId = localStg.get('lastLoginUserId');

    if (lastLoginUserId !== userInfo.id) {
      localStg.remove('globalTabs');
      tabStore.clearTabs();

      return true;
    }

    return false;
  }

  /**
   * Login
   *
   * @param userName User name
   * @param password Password
   * @param [redirect=true] Whether to redirect after login. Default is `true`
   */
  async function login(userName: string, password: string, redirect = true) {
    startLoading();

    const { data: loginToken, error } = await fetchLogin(userName, password);

    if (!error) {
      // Tài khoản bật xác thực 2 bước (hoặc quán bắt buộc mà chưa cài): trang đăng nhập
      // chuyển sang ô nhập mã, kèm mã QR nếu chưa cài.
      if ((loginToken.need_totp || loginToken.need_setup) && loginToken.challenge) {
        totpChallenge.value = loginToken.challenge;
        totpSetup.value = loginToken.need_setup ? (loginToken.setup ?? null) : null;
      } else {
        await finishLogin(loginToken, redirect);
      }
    } else {
      window.$message?.error(error?.message || 'Login failed');
    }

    endLoading();
  }

  async function loginTotp(code: string, redirect = true) {
    startLoading();

    const { data: loginToken, error } = await fetchLoginTotp(totpChallenge.value, code);

    if (!error) {
      totpChallenge.value = '';
      totpSetup.value = null;
      if (loginToken.recovery_codes?.length) {
        recoveryCodes.value = loginToken.recovery_codes;
        pendingLogin = { token: loginToken, redirect };
      } else {
        await finishLogin(loginToken, redirect);
      }
    }

    endLoading();
  }

  async function confirmRecovery() {
    const p = pendingLogin;
    pendingLogin = null;
    recoveryCodes.value = [];
    if (p) await finishLogin(p.token, p.redirect);
  }

  function cancelTotp() {
    totpChallenge.value = '';
    totpSetup.value = null;
  }

  async function finishLogin(loginToken: Api.Auth.LoginToken, redirect: boolean) {
    const pass = await loginByToken(loginToken);

    if (pass) {
      // Check if the tab needs to be cleared
      const isClear = checkTabClear();
      let needRedirect = redirect;

      if (isClear) {
        // If the tab needs to be cleared,it means we don't need to redirect.
        needRedirect = false;
      }
      await redirectFromLogin(needRedirect);

      window.$notification?.success({
        title: $t('page.login.common.loginSuccess'),
        message: $t('page.login.common.welcomeBack', { userName: userInfo.username }),
        duration: 4500
      });
    }
  }

  async function loginByToken(loginToken: Api.Auth.LoginToken) {
    localStg.set('token', loginToken.access_token);
    localStg.set('refreshToken', loginToken.refresh_token);

    if (loginToken.user) {
      Object.assign(userInfo, loginToken.user);
    }

    token.value = loginToken.access_token;

    wsStore.connect();

    return true;
  }

  async function getUserInfo() {
    const { data: info, error } = await fetchGetUserInfo();

    if (!error) {
      // update store
      Object.assign(userInfo, info);

      return true;
    }

    return false;
  }

  /** Tải lại quyền sau khi trang Phân quyền ghi. Trả về true nếu quyền đã khác, để tải lại menu theo. */
  async function refreshPermissions() {
    const before = [...userInfo.permissions].sort().join(',');

    if (!(await getUserInfo())) return false;

    return [...userInfo.permissions].sort().join(',') !== before;
  }

  async function initUserInfo() {
    const hasToken = getToken();

    if (hasToken) {
      const pass = await getUserInfo();

      if (!pass) {
        resetStore();
      }
    }
  }

  return {
    token,
    userInfo,
    roles,
    isStaticSuper,
    isLogin,
    loginLoading,
    resetStore,
    login,
    totpChallenge,
    totpSetup,
    recoveryCodes,
    loginTotp,
    confirmRecovery,
    cancelTotp,
    initUserInfo,
    refreshPermissions
  };
});
