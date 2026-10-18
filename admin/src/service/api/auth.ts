import { request } from '../request';

export function fetchLogin(username: string, password: string) {
  return request<Api.Auth.LoginToken>({
    url: '/auth/login',
    method: 'post',
    data: { username, password }
  });
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
