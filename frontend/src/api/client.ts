import axios from 'axios';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '',
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
    'X-Requested-With': 'Splito',
    'Bypass-Tunnel-Reminder': 'true',
  },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token');
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 401) {
      // 1. Never redirect on probe/session-check calls like /auth/me or public invite info
      const requestUrl = error.config?.url || '';
      if (requestUrl.includes('/auth/me') || requestUrl.includes('/trips/invite-info')) {
        return Promise.reject(error);
      }

      // 2. Don't redirect if on public pages like landing (/), login, register, forgot-password, or invite link (/invite/...)
      const path = window.location.pathname;
      const isPublicPath = 
        path === '/' || 
        path.startsWith('/login') || 
        path.startsWith('/register') || 
        path.startsWith('/forgot-password') || 
        path.startsWith('/invite');

      if (!isPublicPath) {
        localStorage.removeItem('auth_token');
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        window.location.href = '/';
      }
    }
    return Promise.reject(error);
  }
);

export default api;
