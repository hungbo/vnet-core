import { request } from '../request';

export function fetchLogin(username: string, password: string) {
  return request<Api.Auth.LoginToken>({
    url: '/auth/login',
    method: 'post',
    data: { username, password }
  });
}

/** Bước 2 của đăng nhập khi /auth/login trả need_totp: vé + mã 6 số (hoặc mã dự phòng). */
export function fetchLoginTotp(challenge: string, code: string) {
  return request<Api.Auth.LoginToken>({
    url: '/auth/login/totp',
    method: 'post',
    data: { challenge, code }
  });
}

export function fetchTwoFactorStatus() {
  return request<Api.Auth.TwoFactorStatus>({ url: '/auth/2fa' });
}

export function fetchTwoFactorSetup() {
  return request<Api.Auth.TwoFactorSetup>({ url: '/auth/2fa/setup', method: 'post' });
}

/** Trả 10 mã dự phòng, chỉ lần này. */
export function fetchTwoFactorEnable(code: string) {
  return request<string[]>({ url: '/auth/2fa/enable', method: 'post', data: { code } });
}

export function fetchTwoFactorDisable(password: string) {
  return request({ url: '/auth/2fa/disable', method: 'post', data: { password } });
}

export function fetchGetUserInfo() {
  return request<Api.Auth.UserInfo>({ url: '/auth/me' });
}

export function fetchRefreshToken(refreshToken: string) {
  return request<Api.Auth.LoginToken>({
    url: '/auth/refresh',
    method: 'post',
    data: { refresh_token: refreshToken }
  });
}

export function fetchCustomBackendError(code: string, msg: string) {
  return request({ url: '/auth/error', params: { code, msg } });
}
