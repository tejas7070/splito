import { createRouter, createWebHistory } from 'vue-router';
import LandingView from '../views/LandingView.vue';
import LoginView from '../views/LoginView.vue';
import DashboardView from '../views/DashboardView.vue';

const routes = [
  { path: '/', component: LandingView },
  { path: '/login', component: LoginView },
  { path: '/register', component: LoginView },
  { path: '/forgot-password', component: LoginView },
  { path: '/dashboard', component: DashboardView },
  { path: '/:pathMatch(.*)*', redirect: '/' },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

import { useAuthStore } from '../stores/auth';

router.beforeEach(async (to, _, next) => {
  const publicPages = ['/', '/login', '/register', '/forgot-password'];
  const isPublicPage = publicPages.includes(to.path);

  const authStore = useAuthStore();
  if (!authStore.isAuthChecked) {
    await authStore.fetchUser();
  }

  const authenticated = authStore.isAuthenticated();

  // If user is trying to access protected route and is not authenticated
  if (!isPublicPage && !authenticated) {
    if (to.query.invite_trip) {
      next({ path: '/login', query: to.query });
    } else {
      next('/login');
    }
  } else if (authenticated && (to.path === '/login' || to.path === '/register' || to.path === '/forgot-password') && !to.query.invite_trip) {
    // If logged in and visiting login/register without invite query, send to dashboard
    next('/dashboard');
  } else {
    next();
  }
});

export default router;
