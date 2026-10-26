import { createRouter, createWebHistory } from 'vue-router';
import LoginView from '../views/LoginView.vue';
import DashboardView from '../views/DashboardView.vue';
import InviteView from '../views/InviteView.vue';
import { useAuthStore } from '../stores/auth';

const routes = [
  { path: '/', redirect: '/login' },
  { path: '/login', component: LoginView },
  { path: '/register', component: LoginView },
  { path: '/forgot-password', component: LoginView },
  { path: '/dashboard', component: DashboardView },
  { path: '/invite/:code', component: InviteView },
  { path: '/invite', component: InviteView },
  { path: '/:pathMatch(.*)*', redirect: '/login' },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach(async (to, _, next) => {
  const publicPages = ['/login', '/register', '/forgot-password'];
  const isPublicPage = publicPages.includes(to.path) || to.path.startsWith('/invite');

  // 1. Capture invite_code from route params, query, or path and persist reliably
  let inviteFromRoute = (to.params.code as string) || (to.query.invite_code as string) || (to.query.code as string);
  if (!inviteFromRoute && to.path.startsWith('/invite/')) {
    const parts = to.path.split('/');
    if (parts.length >= 3 && parts[2]) {
      inviteFromRoute = parts[2];
    }
  }

  if (inviteFromRoute) {
    try {
      sessionStorage.setItem('splito_invite_code', inviteFromRoute);
      localStorage.setItem('splito_pending_invite', inviteFromRoute);
    } catch (_) {}
  }

  let pendingInviteCode = inviteFromRoute;
  if (!pendingInviteCode) {
    try {
      pendingInviteCode = sessionStorage.getItem('splito_invite_code') || localStorage.getItem('splito_pending_invite') || '';
    } catch (_) {}
  }

  // 2. Fallback for bare '/invite' route
  if (to.path === '/invite' || to.path === '/invite/') {
    if (pendingInviteCode) {
      next(`/invite/${pendingInviteCode}`);
      return;
    }
  }

  const authStore = useAuthStore();
  if (!authStore.isAuthChecked) {
    await authStore.fetchUser();
  }

  const authenticated = authStore.isAuthenticated();

  // 3. Protected route guard
  if (!isPublicPage && !authenticated) {
    if (to.query.invite_trip) {
      next({ path: '/login', query: to.query });
      return;
    }
    if (pendingInviteCode) {
      next({ path: '/login', query: { invite_code: pendingInviteCode } });
      return;
    }
    next('/login');
    return;
  }

  // 4. If authenticated user visits login, register, or forgot-password
  if (authenticated && (to.path === '/login' || to.path === '/register' || to.path === '/forgot-password')) {
    if (pendingInviteCode) {
      next(`/invite/${pendingInviteCode}`);
      return;
    }
    if (to.query.invite_trip) {
      next({ path: '/dashboard', query: to.query });
      return;
    }
    next('/dashboard');
    return;
  }

  next();
});

export default router;
