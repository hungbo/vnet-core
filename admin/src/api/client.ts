import axios from 'axios';
import { localStg } from '@/utils/storage';

const client = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' }
});

client.interceptors.request.use(
  config => {
    const token = localStg.get('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  error => Promise.reject(error)
);

/** Error carrying the backend response code, so callers can react to a specific
 * failure instead of matching on message text. */
export interface ApiError extends Error {
  /** Business code from the response envelope. */
  code?: number;
  /** HTTP status, when the request itself failed. */
  status?: number;
}

function apiError(message: string, code?: number, status?: number): ApiError {
  const err = new Error(message) as ApiError;
  err.code = code;
  err.status = status;
  return err;
}

// Mã 9999 nghĩa là access token hết hạn — phải LÀM MỚI rồi gửi lại, đúng giao
// kèo ghi trong admin/.env (VITE_SERVICE_EXPIRED_TOKEN_CODES). Bản cũ coi mọi
// 401 là "đăng xuất ngay": access token sống 24 giờ, nên cứ mỗi ngày nhân viên
// đang thao tác dở là bị văng ra màn hình đăng nhập, dù refresh token còn hạn 7
// ngày. Client Soybean (src/service/request) đã làm đúng từ lâu; 26 trang VNET
// dùng client này thì không.
//
// Một lần làm mới dùng chung cho mọi request đang chờ: nhiều bảng cùng gọi API
// lúc token hết hạn thì chỉ một lệnh /auth/refresh được gửi đi.
let refreshPromise: Promise<boolean> | null = null;

async function refreshToken(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      const rToken = localStg.get('refreshToken');
      if (!rToken) return false;
      try {
        const res = await axios.post('/api/auth/refresh', { refresh_token: rToken });
        const data = res.data?.data;
        if (res.data?.code !== 0 || !data?.access_token) return false;
        localStg.set('token', data.access_token);
        localStg.set('refreshToken', data.refresh_token);
        return true;
      } catch {
        return false;
      }
    })();
    // Thả sau khi xong để lần hết hạn kế tiếp lại làm mới được.
    refreshPromise.finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

function forceLogout() {
  localStg.remove('token');
  localStg.remove('refreshToken');
  window.location.href = '/login';
}

client.interceptors.response.use(
  response => {
    const { data } = response;
    if (data.code !== 0) {
      return Promise.reject(apiError(data.message || 'Request failed', data.code, response.status));
    }
    return data.data;
  },
  async error => {
    const status = error.response?.status;
    const body = error.response?.data;
    const code = Number(body?.code);

    if (status === 401) {
      // 9999: làm mới rồi gửi lại đúng một lần. Cờ daGuiLai chặn vòng lặp khi
      // token mới cũng bị từ chối.
      if (code === 9999 && !error.config?.daGuiLai) {
        const ok = await refreshToken();
        if (ok) {
          const cfg = { ...error.config, daGuiLai: true };
          cfg.headers = { ...cfg.headers, Authorization: `Bearer ${localStg.get('token')}` };
          return client.request(cfg);
        }
      }
      forceLogout();
    }
    return Promise.reject(apiError(body?.message || error.message || 'Request failed', body?.code, status));
  }
);

export default client;
