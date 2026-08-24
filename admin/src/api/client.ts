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

client.interceptors.response.use(
  response => {
    const { data } = response;
    if (data.code !== 0) {
      return Promise.reject(apiError(data.message || 'Request failed', data.code, response.status));
    }
    return data.data;
  },
  error => {
    const status = error.response?.status;
    if (status === 401) {
      localStg.remove('token');
      window.location.href = '/login';
    }
    const body = error.response?.data;
    return Promise.reject(apiError(body?.message || error.message || 'Request failed', body?.code, status));
  }
);

export default client;
