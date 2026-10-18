import { useAuthStore } from '@/store/modules/auth';

export function useAuth() {
  const authStore = useAuthStore();

  function hasAuth(codes: string | string[]) {
    if (!authStore.isLogin) {
      return false;
    }

    const perms = authStore.userInfo.permissions || [];
    // "*" là siêu quyền, giống middleware.PermissionRequired bên backend.
    if (perms.includes('*')) {
      return true;
    }

    if (typeof codes === 'string') {
      return perms.includes(codes);
    }

    return codes.some(code => perms.includes(code));
  }

  return {
    hasAuth
  };
}
