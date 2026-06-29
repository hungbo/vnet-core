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

client.interceptors.response.use(
  response => {
    const { data } = response;
    if (data.code !== 0) {
      return Promise.reject(new Error(data.message || 'Request failed'));
    }
    return data.data;
  },
  error => {
    if (error.response?.status === 401) {
      localStg.remove('token');
      window.location.href = '/login';
    }
    const backendMessage = error.response?.data?.message;
    if (backendMessage) {
      return Promise.reject(new Error(backendMessage));
    }
    return Promise.reject(error);
  }
);

export default client;
