import { defineStore } from 'pinia';
import { ref } from 'vue';
import { authService, type User } from '../api/services';

export const useAuthStore = defineStore('auth', () => {
  // In-memory only state (NO PII, NO JWT in localStorage)
  const user = ref<User | null>(null);
  const isAuthChecked = ref(false);

  const isAuthenticated = () => !!user.value;

  const setAuth = (accessToken: string, userData: User) => {
    user.value = userData;
    if (accessToken) {
      localStorage.setItem('auth_token', accessToken);
    }
    // Clean up any old residual localStorage items
    localStorage.removeItem('token');
    localStorage.removeItem('user');
  };

  const login = async (email: string, password: string) => {
    const res = await authService.login({ email, password });
    setAuth(res.data.access_token, res.data.user);
    return res.data;
  };

  const requestRegisterOTP = async (email: string) => {
    return await authService.requestRegisterOTP(email);
  };

  const register = async (name: string, email: string, password: string, otp: string) => {
    const res = await authService.register({ name, email, password, otp });
    setAuth(res.data.access_token, res.data.user);
    return res.data;
  };

  const requestForgotPasswordOTP = async (email: string) => {
    return await authService.requestForgotPasswordOTP(email);
  };

  const resetPassword = async (email: string, otp: string, newPassword: string) => {
    const res = await authService.resetPassword({ email, otp, new_password: newPassword });
    if (res.data.user) {
      setAuth(res.data.access_token, res.data.user);
    }
    return res.data;
  };

  const googleAuth = async (credential: string) => {
    const res = await authService.googleAuth({ credential });
    setAuth(res.data.access_token, res.data.user);
    return res.data;
  };

  const fetchUser = async () => {
    try {
      const res = await authService.getMe();
      user.value = res.data;
      isAuthChecked.value = true;
      return res.data;
    } catch (e) {
      user.value = null;
      isAuthChecked.value = true;
      return null;
    }
  };

  const updateProfile = async (data: { name: string; phone?: string }) => {
    const res = await authService.updateProfile(data);
    user.value = res.data;
    return res.data;
  };

  const requestPasswordOTP = async () => {
    const res = await authService.requestPasswordOTP();
    return res.data;
  };

  const changePassword = async (data: { current_password?: string; otp?: string; new_password: string }) => {
    const res = await authService.changePassword(data);
    return res.data;
  };

  const logout = async () => {
    user.value = null;
    isAuthChecked.value = true;
    localStorage.removeItem('auth_token');
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    try {
      await authService.logout();
    } catch (e) {
      // ignore network errors on logout
    }
  };

  // Legacy
  const requestOTP = async (email: string) => {
    await authService.requestOTP(email);
  };

  const verifyOTP = async (email: string, otp: string) => {
    const res = await authService.verifyOTP(email, otp);
    setAuth(res.data.access_token, res.data.user);
  };

  return {
    user,
    isAuthChecked,
    isAuthenticated,
    setAuth,
    login,
    fetchUser,
    updateProfile,
    requestPasswordOTP,
    changePassword,
    requestRegisterOTP,
    register,
    requestForgotPasswordOTP,
    resetPassword,
    googleAuth,
    logout,
    requestOTP,
    verifyOTP,
  };
});
