<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { useThemeStore } from '../stores/theme';
import { useToast } from '../stores/toast';
import Button from 'primevue/button';
import InputText from 'primevue/inputtext';
import Password from 'primevue/password';
import InputOtp from 'primevue/inputotp';
import Message from 'primevue/message';
import Tag from 'primevue/tag';
import Dialog from 'primevue/dialog';
import appFavicon from '../assets/favicon.png';

const authStore = useAuthStore();
const themeStore = useThemeStore();
const toast = useToast();
const router = useRouter();
const route = useRoute();

// Top-level mode: 'login' | 'register' | 'forgot'
const mode = ref<'login' | 'register' | 'forgot'>('login');
const loading = ref(false);
const errorMsg = ref('');
const successMsg = ref('');

// --- Google Authentication State ---
const googleClientId = (import.meta.env.VITE_GOOGLE_CLIENT_ID as string) || '';
const showGoogleDevModal = ref(false);
const devGoogleEmail = ref('');
const devGoogleName = ref('');
const isGoogleLoading = ref(false);

// --- Login State ---
const loginEmail = ref('');
const loginPassword = ref('');

// --- Registration State ---
const regStep = ref<'form' | 'otp'>('form');
const regName = ref('');
const regEmail = ref('');
const regPassword = ref('');
const regConfirmPassword = ref('');
const regOtp = ref('');
const regResendCooldown = ref(0);
let regTimer: number | null = null;

// --- Forgot Password State ---
const forgotStep = ref<'email' | 'reset'>('email');
const forgotEmail = ref('');
const forgotOtp = ref('');
const forgotNewPassword = ref('');
const forgotConfirmPassword = ref('');
const forgotResendCooldown = ref(0);
let forgotTimer: number | null = null;

const hasInviteQuery = computed(() => !!route.query.invite_trip);

const startRegCooldown = (seconds = 30) => {
  regResendCooldown.value = seconds;
  if (regTimer) clearInterval(regTimer);
  regTimer = window.setInterval(() => {
    if (regResendCooldown.value > 0) {
      regResendCooldown.value--;
    } else if (regTimer) {
      clearInterval(regTimer);
      regTimer = null;
    }
  }, 1000);
};

const startForgotCooldown = (seconds = 30) => {
  forgotResendCooldown.value = seconds;
  if (forgotTimer) clearInterval(forgotTimer);
  forgotTimer = window.setInterval(() => {
    if (forgotResendCooldown.value > 0) {
      forgotResendCooldown.value--;
    } else if (forgotTimer) {
      clearInterval(forgotTimer);
      forgotTimer = null;
    }
  }, 1000);
};

const syncModeWithRoute = () => {
  errorMsg.value = '';
  successMsg.value = '';
  if (route.path === '/register') {
    mode.value = 'register';
  } else if (route.path === '/forgot-password') {
    mode.value = 'forgot';
  } else {
    mode.value = 'login';
  }
};

onMounted(() => {
  syncModeWithRoute();
  if (typeof (window as any).google !== 'undefined' && (window as any).google?.accounts?.id && googleClientId) {
    try {
      (window as any).google.accounts.id.initialize({
        client_id: googleClientId,
        callback: handleGoogleCredentialResponse,
        auto_select: false,
      });
      (window as any).google.accounts.id.prompt();
    } catch {
      // ignore
    }
  }
});

watch(() => route.path, () => {
  syncModeWithRoute();
});

const switchMode = (target: 'login' | 'register' | 'forgot') => {
  errorMsg.value = '';
  successMsg.value = '';
  mode.value = target;
  if (target === 'login') {
    router.push({ path: '/login', query: route.query });
  } else if (target === 'register') {
    router.push({ path: '/register', query: route.query });
  } else if (target === 'forgot') {
    router.push({ path: '/forgot-password', query: route.query });
  }
};

const navigateToApp = () => {
  const inviteTrip = route.query.invite_trip;
  const inviteToken = route.query.invite_token;
  if (inviteTrip) {
    router.push({ 
      path: '/dashboard', 
      query: { invite_trip: inviteTrip, ...(inviteToken ? { invite_token: inviteToken } : {}) } 
    });
  } else {
    router.push('/dashboard');
  }
};

// ========================
// GOOGLE AUTHENTICATION
// ========================
const handleGoogleCredentialResponse = async (response: any) => {
  if (!response?.credential) {
    toast.error('No Google credentials received');
    return;
  }
  isGoogleLoading.value = true;
  errorMsg.value = '';
  try {
    await authStore.googleAuth(response.credential);
    toast.success('Signed in with Google! Welcome to Splito.');
    navigateToApp();
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Google authentication failed';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    isGoogleLoading.value = false;
  }
};

const handleGoogleSignIn = () => {
  if (googleClientId && typeof (window as any).google !== 'undefined' && (window as any).google?.accounts?.oauth2) {
    try {
      isGoogleLoading.value = true;
      const client = (window as any).google.accounts.oauth2.initTokenClient({
        client_id: googleClientId,
        scope: 'email profile openid',
        callback: async (tokenResponse: any) => {
          if (tokenResponse?.access_token) {
            try {
              await authStore.googleAuth(tokenResponse.access_token);
              toast.success('Signed in with Google! Welcome to Splito.');
              navigateToApp();
            } catch (err: any) {
              const msg = err.response?.data?.error || 'Google authentication failed';
              errorMsg.value = msg;
              toast.error(msg);
            } finally {
              isGoogleLoading.value = false;
            }
          } else {
            isGoogleLoading.value = false;
          }
        },
        error_callback: () => {
          isGoogleLoading.value = false;
        }
      });
      client.requestAccessToken();
    } catch {
      isGoogleLoading.value = false;
      showGoogleDevModal.value = true;
    }
  } else if (googleClientId && typeof (window as any).google !== 'undefined' && (window as any).google?.accounts?.id) {
    try {
      (window as any).google.accounts.id.initialize({
        client_id: googleClientId,
        callback: handleGoogleCredentialResponse,
      });
      (window as any).google.accounts.id.prompt();
    } catch {
      showGoogleDevModal.value = true;
    }
  } else {
    showGoogleDevModal.value = true;
  }
};

const handleSimulatedGoogleAuth = async () => {
  if (!devGoogleEmail.value.trim()) {
    toast.error('Please enter a Google email');
    return;
  }
  isGoogleLoading.value = true;
  errorMsg.value = '';
  try {
    const payload = JSON.stringify({
      email: devGoogleEmail.value.trim(),
      name: devGoogleName.value.trim() || undefined
    });
    await authStore.googleAuth(`dev_google:${payload}`);
    showGoogleDevModal.value = false;
    toast.success('Signed in with Google! Welcome to Splito.');
    navigateToApp();
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Google authentication failed';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    isGoogleLoading.value = false;
  }
};

// ========================
// 1. SIGN IN (LOGIN)
// ========================
const handleLogin = async () => {
  if (!loginEmail.value.trim() || !loginPassword.value) {
    errorMsg.value = 'Please provide both email address and password';
    return;
  }
  loading.value = true;
  errorMsg.value = '';
  try {
    await authStore.login(loginEmail.value.trim(), loginPassword.value);
    toast.success('Signed in successfully! Welcome to Splito.');
    navigateToApp();
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Invalid email or password';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    loading.value = false;
  }
};

// ========================
// 2. REGISTRATION FLOW
// ========================
const handleSendRegOTP = async () => {
  if (!regName.value.trim()) {
    errorMsg.value = 'Please enter your full name';
    return;
  }
  if (!regEmail.value.trim()) {
    errorMsg.value = 'Please enter a valid email address';
    return;
  }
  if (regPassword.value.length < 6) {
    errorMsg.value = 'Password must be at least 6 characters long';
    return;
  }
  if (regPassword.value !== regConfirmPassword.value) {
    errorMsg.value = 'Passwords do not match';
    return;
  }

  loading.value = true;
  errorMsg.value = '';
  try {
    await authStore.requestRegisterOTP(regEmail.value.trim());
    regStep.value = 'otp';
    startRegCooldown();
    toast.success(`Verification code dispatched to ${regEmail.value}`);
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Failed to dispatch registration code';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    loading.value = false;
  }
};

const handleResendRegOTP = async () => {
  if (regResendCooldown.value > 0) return;
  loading.value = true;
  errorMsg.value = '';
  try {
    await authStore.requestRegisterOTP(regEmail.value.trim());
    startRegCooldown();
    toast.success('New verification code dispatched to your inbox');
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Failed to resend verification code';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    loading.value = false;
  }
};

const handleCompleteRegistration = async () => {
  if (!regOtp.value || regOtp.value.length < 6) {
    errorMsg.value = 'Please enter the 6-digit verification code';
    return;
  }
  loading.value = true;
  errorMsg.value = '';
  try {
    await authStore.register(
      regName.value.trim(),
      regEmail.value.trim(),
      regPassword.value,
      regOtp.value
    );
    toast.success('Account created successfully! Welcome to Splito.');
    navigateToApp();
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Registration failed. Check your OTP code';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    loading.value = false;
  }
};

// ========================
// 3. FORGOT PASSWORD FLOW
// ========================
const handleSendForgotOTP = async () => {
  if (!forgotEmail.value.trim()) {
    errorMsg.value = 'Please enter your registered email address';
    return;
  }
  loading.value = true;
  errorMsg.value = '';
  try {
    await authStore.requestForgotPasswordOTP(forgotEmail.value.trim());
    forgotStep.value = 'reset';
    startForgotCooldown();
    toast.success(`Password reset code dispatched to ${forgotEmail.value}`);
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Failed to dispatch reset code';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    loading.value = false;
  }
};

const handleResendForgotOTP = async () => {
  if (forgotResendCooldown.value > 0) return;
  loading.value = true;
  errorMsg.value = '';
  try {
    await authStore.requestForgotPasswordOTP(forgotEmail.value.trim());
    startForgotCooldown();
    toast.success('New reset code dispatched to your inbox');
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Failed to resend reset code';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    loading.value = false;
  }
};

const handleResetPassword = async () => {
  if (!forgotOtp.value || forgotOtp.value.length < 6) {
    errorMsg.value = 'Please enter the 6-digit reset code';
    return;
  }
  if (forgotNewPassword.value.length < 6) {
    errorMsg.value = 'New password must be at least 6 characters long';
    return;
  }
  if (forgotNewPassword.value !== forgotConfirmPassword.value) {
    errorMsg.value = 'Passwords do not match';
    return;
  }

  loading.value = true;
  errorMsg.value = '';
  try {
    await authStore.resetPassword(
      forgotEmail.value.trim(),
      forgotOtp.value,
      forgotNewPassword.value
    );
    toast.success('Password updated successfully! Welcome back.');
    navigateToApp();
  } catch (err: any) {
    const msg = err.response?.data?.error || 'Password reset failed';
    errorMsg.value = msg;
    toast.error(msg);
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <div class="auth-wrapper">
    <!-- Top Bar Controls -->
    <header class="top-bar">
      <Button 
        @click="themeStore.toggleTheme" 
        :label="themeStore.isDark ? 'Light' : 'Dark'"
        :icon="themeStore.isDark ? 'pi pi-sun' : 'pi pi-moon'"
        rounded
        size="small"
        severity="secondary"
        text
        aria-label="Toggle theme"
      />
    </header>

    <div class="auth-card">
      <!-- App Brand & Identity -->
      <div class="brand-block">
        <div class="brand-badge-cluster">
          <div class="brand-symbol">
            <img :src="appFavicon" alt="Splito Logo" class="brand-logo-img" />
          </div>
          <span class="logo-text brand-font">Splito</span>
        </div>
        <p class="brand-tagline">Expedition Treasury Protocol & Debt Settlement</p>
      </div>

      <!-- Invite Context Alert -->
      <div v-if="hasInviteQuery" class="invite-alert-banner">
        <Tag value="Expedition Invitation" severity="warn" icon="pi pi-envelope" rounded />
        <span class="invite-alert-text">Sign in or create an account to access this shared trip invitation.</span>
      </div>

      <!-- Mode Selector Pills (Tabs) -->
      <div v-if="mode !== 'forgot'" class="auth-mode-pills">
        <button 
          type="button" 
          @click="switchMode('login')"
          :class="['mode-tab-btn', { active: mode === 'login' }]"
        >
          <i class="pi pi-sign-in"></i>
          <span>Sign In</span>
        </button>
        <button 
          type="button" 
          @click="switchMode('register')"
          :class="['mode-tab-btn', { active: mode === 'register' }]"
        >
          <i class="pi pi-user-plus"></i>
          <span>Register</span>
        </button>
      </div>

      <!-- Error / Success Alert Messages -->
      <Message v-if="errorMsg" severity="error" :closable="true" @close="errorMsg = ''" class="auth-alert">
        {{ errorMsg }}
      </Message>
      <Message v-if="successMsg" severity="success" :closable="true" @close="successMsg = ''" class="auth-alert">
        {{ successMsg }}
      </Message>

      <!-- ============================================== -->
      <!-- MODE 1: SIGN IN (EMAIL + PASSWORD)             -->
      <!-- ============================================== -->
      <div v-if="mode === 'login'" class="auth-section">
        <div class="section-heading">
          <h2 class="auth-title brand-font">Welcome back</h2>
          <p class="auth-sub">Enter your credentials to access your trip manifests.</p>
        </div>

        <form @submit.prevent="handleLogin" class="form-body">
          <div class="input-block">
            <label class="input-label" for="login-email">Email Address</label>
            <InputText 
              id="login-email"
              v-model="loginEmail" 
              type="email" 
              required 
              placeholder="name@adventure.com"
              class="prime-field"
              autofocus
              fluid
            />
          </div>

          <div class="input-block">
            <div class="label-row">
              <label class="input-label" for="login-password">Password</label>
              <button 
                type="button" 
                @click="switchMode('forgot')" 
                class="link-forgot-btn"
              >
                Forgot Password?
              </button>
            </div>
            <Password 
              id="login-password"
              v-model="loginPassword" 
              required 
              placeholder="••••••••"
              :feedback="false"
              toggleMask
              fluid
            />
          </div>

          <Button 
            type="submit" 
            :loading="loading"
            label="Sign In"
            icon="pi pi-arrow-right"
            iconPos="right"
            severity="primary"
            class="prime-action-btn"
            fluid
          />
        </form>

        <!-- Divider -->
        <div class="oauth-divider">
          <span class="oauth-line"></span>
          <span class="oauth-text">OR CONTINUE WITH</span>
          <span class="oauth-line"></span>
        </div>

        <!-- Google Auth Button (Synchronous & Full Width) -->
        <button 
          type="button" 
          @click="handleGoogleSignIn"
          class="google-auth-btn"
          :disabled="loading || isGoogleLoading"
        >
          <svg class="google-icon" viewBox="0 0 24 24" width="20" height="20">
            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
            <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
            <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
          </svg>
          <span class="google-btn-text">
            {{ isGoogleLoading ? 'Connecting to Google...' : 'Continue with Google' }}
          </span>
          <i v-if="isGoogleLoading" class="pi pi-spin pi-spinner ml-2"></i>
        </button>

        <div class="bottom-switch-row">
          <span class="switch-caption">New to Splito?</span>
          <button type="button" @click="switchMode('register')" class="switch-action-link">
            Create an account &rarr;
          </button>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- MODE 2: REGISTRATION (NAME + EMAIL + OTP)      -->
      <!-- ============================================== -->
      <div v-else-if="mode === 'register'" class="auth-section">
        <!-- Step A: User Details Input -->
        <div v-if="regStep === 'form'">
          <div class="section-heading">
            <h2 class="auth-title brand-font">Create Account</h2>
            <p class="auth-sub">Step 1 of 2: Set your account credentials.</p>
          </div>

          <form @submit.prevent="handleSendRegOTP" class="form-body">
            <div class="input-block">
              <label class="input-label" for="reg-name">Full Name</label>
              <InputText 
                id="reg-name"
                v-model="regName" 
                type="text" 
                required 
                placeholder="Rohit Sharma"
                class="prime-field"
                autofocus
                fluid
              />
            </div>

            <div class="input-block">
              <label class="input-label" for="reg-email">Email Address</label>
              <InputText 
                id="reg-email"
                v-model="regEmail" 
                type="email" 
                required 
                placeholder="rohit@expedition.com"
                class="prime-field"
                fluid
              />
            </div>

            <div class="input-block">
              <label class="input-label" for="reg-password">Password (min 6 characters)</label>
              <Password 
                id="reg-password"
                v-model="regPassword" 
                required 
                placeholder="••••••••"
                :feedback="false"
                toggleMask
                fluid
              />
            </div>

            <div class="input-block">
              <label class="input-label" for="reg-confirm">Confirm Password</label>
              <Password 
                id="reg-confirm"
                v-model="regConfirmPassword" 
                required 
                placeholder="••••••••"
                :feedback="false"
                toggleMask
                fluid
              />
            </div>

            <Button 
              type="submit" 
              :loading="loading"
              label="Send Verification Code"
              icon="pi pi-send"
              iconPos="right"
              severity="primary"
              class="prime-action-btn"
              fluid
            />
          </form>

          <!-- Divider -->
          <div class="oauth-divider">
            <span class="oauth-line"></span>
            <span class="oauth-text">OR SIGN UP WITH</span>
            <span class="oauth-line"></span>
          </div>

          <!-- Google Auth Button (Synchronous & Full Width) -->
          <button 
            type="button" 
            @click="handleGoogleSignIn"
            class="google-auth-btn"
            :disabled="loading || isGoogleLoading"
          >
            <svg class="google-icon" viewBox="0 0 24 24" width="20" height="20">
              <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
              <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
              <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
              <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
            </svg>
            <span class="google-btn-text">
              {{ isGoogleLoading ? 'Connecting to Google...' : 'Sign up with Google' }}
            </span>
            <i v-if="isGoogleLoading" class="pi pi-spin pi-spinner ml-2"></i>
          </button>

          <div class="bottom-switch-row">
            <span class="switch-caption">Already have an account?</span>
            <button type="button" @click="switchMode('login')" class="switch-action-link">
              Sign In &rarr;
            </button>
          </div>
        </div>

        <!-- Step B: OTP Verification & Finalize -->
        <div v-else>
          <div class="section-heading">
            <h2 class="auth-title brand-font">Verify Email</h2>
            <p class="auth-sub">
              Step 2 of 2: Enter the 6-digit security code dispatched to <strong>{{ regEmail }}</strong>
            </p>
          </div>

          <form @submit.prevent="handleCompleteRegistration" class="form-body">
            <div class="input-block">
              <div class="otp-center-box">
                <InputOtp 
                  v-model="regOtp" 
                  :length="6" 
                  integerOnly 
                  autofocus 
                  class="prime-otp"
                />
              </div>
            </div>

            <Button 
              type="submit" 
              :loading="loading"
              label="Complete Registration & Launch"
              icon="pi pi-check"
              iconPos="right"
              severity="primary"
              class="prime-action-btn"
              fluid
            />

            <div class="auth-aux-links">
              <button 
                type="button" 
                @click="handleResendRegOTP" 
                :disabled="loading || regResendCooldown > 0"
                class="ghost-link"
              >
                {{ regResendCooldown > 0 ? `Resend code in ${regResendCooldown}s` : 'Resend code' }}
              </button>
              <span class="aux-sep">•</span>
              <button 
                type="button" 
                @click="regStep = 'form'; regOtp = ''; errorMsg = ''"
                class="ghost-link"
              >
                Edit details
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- MODE 3: FORGOT PASSWORD (OTP RESET)            -->
      <!-- ============================================== -->
      <div v-else class="auth-section">
        <!-- Step A: Enter Registered Email -->
        <div v-if="forgotStep === 'email'">
          <div class="section-heading">
            <h2 class="auth-title brand-font">Reset Password</h2>
            <p class="auth-sub">Enter your email and we'll send a 6-digit verification code to reset your password.</p>
          </div>

          <form @submit.prevent="handleSendForgotOTP" class="form-body">
            <div class="input-block">
              <label class="input-label" for="forgot-email">Registered Email Address</label>
              <InputText 
                id="forgot-email"
                v-model="forgotEmail" 
                type="email" 
                required 
                placeholder="name@adventure.com"
                class="prime-field"
                autofocus
                fluid
              />
            </div>

            <Button 
              type="submit" 
              :loading="loading"
              label="Send Password Reset Code"
              icon="pi pi-send"
              iconPos="right"
              severity="primary"
              class="prime-action-btn"
              fluid
            />
          </form>

          <div class="bottom-switch-row">
            <button type="button" @click="switchMode('login')" class="switch-action-link">
              &larr; Back to Sign In
            </button>
          </div>
        </div>

        <!-- Step B: Enter OTP & Set New Password -->
        <div v-else>
          <div class="section-heading">
            <h2 class="auth-title brand-font">Set New Password</h2>
            <p class="auth-sub">
              Enter the 6-digit code sent to <strong>{{ forgotEmail }}</strong> and choose a secure password.
            </p>
          </div>

          <form @submit.prevent="handleResetPassword" class="form-body">
            <div class="input-block">
              <label class="input-label">Verification Security Code</label>
              <div class="otp-center-box">
                <InputOtp 
                  v-model="forgotOtp" 
                  :length="6" 
                  integerOnly 
                  autofocus 
                  class="prime-otp"
                />
              </div>
            </div>

            <div class="input-block">
              <label class="input-label" for="forgot-new-pwd">New Password (min 6 characters)</label>
              <Password 
                id="forgot-new-pwd"
                v-model="forgotNewPassword" 
                required 
                placeholder="••••••••"
                :feedback="false"
                toggleMask
                fluid
              />
            </div>

            <div class="input-block">
              <label class="input-label" for="forgot-confirm-pwd">Confirm New Password</label>
              <Password 
                id="forgot-confirm-pwd"
                v-model="forgotConfirmPassword" 
                required 
                placeholder="••••••••"
                :feedback="false"
                toggleMask
                fluid
              />
            </div>

            <Button 
              type="submit" 
              :loading="loading"
              label="Update Password & Sign In"
              icon="pi pi-lock"
              iconPos="right"
              severity="primary"
              class="prime-action-btn"
              fluid
            />

            <div class="auth-aux-links">
              <button 
                type="button" 
                @click="handleResendForgotOTP" 
                :disabled="loading || forgotResendCooldown > 0"
                class="ghost-link"
              >
                {{ forgotResendCooldown > 0 ? `Resend code in ${forgotResendCooldown}s` : 'Resend code' }}
              </button>
              <span class="aux-sep">•</span>
              <button 
                type="button" 
                @click="forgotStep = 'email'; forgotOtp = ''; errorMsg = ''"
                class="ghost-link"
              >
                Change email
              </button>
            </div>
          </form>

          <div class="bottom-switch-row">
            <button type="button" @click="switchMode('login')" class="switch-action-link">
              &larr; Back to Sign In
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Google Auth Dev / Fallback Dialog -->
    <Dialog 
      v-model:visible="showGoogleDevModal" 
      modal 
      header="Google Authentication" 
      :style="{ width: '92vw', maxWidth: '440px' }"
    >
      <div class="google-modal-content">
        <div class="google-modal-header">
          <div class="google-avatar-box">
            <svg viewBox="0 0 24 24" width="28" height="28">
              <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
              <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
              <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
              <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
            </svg>
          </div>
          <div>
            <h4 class="google-modal-title">Continue with Google</h4>
            <p class="google-modal-desc">
              {{ googleClientId ? 'Verify your identity or test with local Google profile' : 'Local Dev Mode: Instant Google registration & login' }}
            </p>
          </div>
        </div>

        <div v-if="!googleClientId" class="google-env-hint">
          <i class="pi pi-info-circle"></i>
          <span>To connect live Google accounts, configure <code>VITE_GOOGLE_CLIENT_ID</code> in <code>frontend/.env</code>. For immediate local testing, enter any name and email below:</span>
        </div>

        <form @submit.prevent="handleSimulatedGoogleAuth" class="google-sim-form">
          <div class="input-block">
            <label class="input-label" for="sim-google-name">Account Name</label>
            <InputText 
              id="sim-google-name" 
              v-model="devGoogleName" 
              placeholder="e.g. Vikram Batra" 
              class="prime-field" 
              fluid 
            />
          </div>
          <div class="input-block">
            <label class="input-label" for="sim-google-email">Google Email Address</label>
            <InputText 
              id="sim-google-email" 
              v-model="devGoogleEmail" 
              type="email" 
              required 
              placeholder="vikram@gmail.com" 
              class="prime-field" 
              fluid 
            />
          </div>

          <div class="modal-actions-row">
            <Button 
              type="button" 
              label="Cancel" 
              severity="secondary" 
              text 
              @click="showGoogleDevModal = false" 
            />
            <Button 
              type="submit" 
              label="Authenticate with Google" 
              icon="pi pi-check" 
              :loading="isGoogleLoading" 
              severity="primary" 
            />
          </div>
        </form>
      </div>
    </Dialog>
  </div>
</template>

<style scoped>
.auth-wrapper {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background-color: var(--bg);
  position: relative;
}

.top-bar {
  position: absolute;
  top: 20px;
  right: 20px;
}

.auth-card {
  width: 100%;
  max-width: 440px;
  background-color: var(--surface);
  box-shadow: var(--shadow-flat-lg);
  border: 1px solid var(--border-subtle);
  border-radius: 24px;
  padding: 36px 32px;
  display: flex;
  flex-direction: column;
}

.brand-block {
  text-align: center;
  margin-bottom: 22px;
}

.brand-badge-cluster {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.brand-symbol {
  width: 38px;
  height: 38px;
  border-radius: 12px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 4px;
}

.brand-symbol .brand-logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 8px;
}

.logo-text {
  font-size: 28px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-main);
}

.brand-tagline {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.invite-alert-banner {
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.25);
  border-radius: 12px;
  padding: 10px 14px;
  margin-bottom: 18px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.invite-alert-text {
  font-size: 12px;
  color: var(--text-main);
  line-height: 1.4;
}

/* Auth Mode Selector Pills */
.auth-mode-pills {
  display: flex;
  background: var(--active-pill, rgba(0, 0, 0, 0.04));
  border-radius: 12px;
  padding: 4px;
  margin-bottom: 22px;
  gap: 4px;
}

.mode-tab-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 9px 12px;
  border: none;
  border-radius: 9px;
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.18s ease;
  font-family: inherit;
}

.mode-tab-btn:hover {
  color: var(--text-main);
}

.mode-tab-btn.active {
  background: var(--surface);
  color: var(--accent);
  box-shadow: var(--shadow-flat-sm);
}

.auth-alert {
  margin-bottom: 18px;
}

.section-heading {
  margin-bottom: 20px;
}

.auth-title {
  margin: 0 0 6px 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--text-main);
}

.auth-sub {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.5;
}

.form-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.input-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.label-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.input-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main);
}

.link-forgot-btn {
  background: none;
  border: none;
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
  transition: opacity 0.15s ease;
  font-family: inherit;
}

.link-forgot-btn:hover {
  opacity: 0.8;
  text-decoration: underline;
}

.otp-center-box {
  display: flex;
  justify-content: center;
  padding: 10px 0;
}

.prime-otp {
  gap: 8px;
}

.prime-action-btn {
  border-radius: 12px;
  padding: 12px 18px;
  font-weight: 700;
  font-size: 14px;
  margin-top: 6px;
}

.bottom-switch-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px solid var(--border-subtle);
}

.switch-caption {
  font-size: 13px;
  color: var(--text-muted);
}

.switch-action-link {
  background: none;
  border: none;
  color: var(--accent);
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
  padding: 0;
  font-family: inherit;
  transition: opacity 0.15s ease;
}

.switch-action-link:hover {
  text-decoration: underline;
  opacity: 0.85;
}

.auth-aux-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-top: 8px;
}

.ghost-link {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  padding: 4px 6px;
  font-family: inherit;
  transition: color 0.15s ease;
}

.ghost-link:hover:not(:disabled) {
  color: var(--text-main);
  text-decoration: underline;
}

.ghost-link:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.aux-sep {
  color: var(--text-muted);
  font-size: 11px;
  opacity: 0.5;
}

@media (max-width: 480px) {
  .auth-wrapper {
    padding: 16px 12px;
  }

  .top-bar {
    top: 12px;
    right: 12px;
  }

  .auth-card {
    padding: 24px 18px;
    border-radius: 20px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.05);
  }

  .brand-symbol {
    width: 34px;
    height: 34px;
    font-size: 16px;
    border-radius: 10px;
  }

  .logo-text {
    font-size: 24px;
  }

  .brand-tagline {
    font-size: 12px;
  }

  .auth-mode-pills {
    margin-bottom: 16px;
    gap: 3px;
  }

  .mode-tab-btn {
    padding: 8px 6px;
    font-size: 12px;
    gap: 5px;
  }

  .auth-title {
    font-size: 18px;
  }

  .auth-sub {
    font-size: 12px;
  }

  .prime-otp {
    gap: 6px;
  }

  :deep(.p-inputotp-input) {
    width: 38px !important;
    height: 42px !important;
    font-size: 18px !important;
    border-radius: 8px !important;
  }
}

@media (max-width: 360px) {
  .auth-card {
    padding: 20px 14px;
  }

  .prime-otp {
    gap: 4px;
  }

  :deep(.p-inputotp-input) {
    width: 32px !important;
    height: 38px !important;
    font-size: 16px !important;
  }
}

/* OAuth & Google Auth */
.oauth-divider {
  display: flex;
  align-items: center;
  margin: 18px 0 14px 0;
  gap: 12px;
}

.oauth-line {
  flex: 1;
  height: 1px;
  background-color: var(--border);
}

.oauth-text {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  text-transform: uppercase;
}

.google-auth-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 11px 18px;
  border-radius: 12px;
  border: 1px solid #dadce0 !important;
  background-color: #ffffff !important;
  color: #1f2937 !important;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.google-auth-btn:hover:not(:disabled) {
  background-color: #f8fafd !important;
  border-color: #c7c9cc !important;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.google-auth-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.google-icon {
  flex-shrink: 0;
}

.google-btn-text {
  font-family: inherit;
  color: #3c4043 !important;
  font-weight: 600;
  font-size: 14px;
}

/* Google Dialog */
.google-modal-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.google-modal-header {
  display: flex;
  align-items: center;
  gap: 14px;
}

.google-avatar-box {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background-color: var(--surface-ground, rgba(0, 0, 0, 0.03));
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.google-modal-title {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--text);
}

.google-modal-desc {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

.google-env-hint {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  background-color: rgba(59, 130, 246, 0.08);
  border: 1px solid rgba(59, 130, 246, 0.2);
  font-size: 12px;
  color: var(--text);
  line-height: 1.4;
}

.google-env-hint i {
  color: #3b82f6;
  margin-top: 2px;
  flex-shrink: 0;
}

.google-env-hint code {
  font-family: monospace;
  font-size: 11px;
  background-color: rgba(0, 0, 0, 0.05);
  padding: 1px 4px;
  border-radius: 4px;
}

.google-sim-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.modal-actions-row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 6px;
}
</style>

