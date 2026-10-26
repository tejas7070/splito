<template>
  <div class="invite-page-root">
    <!-- Top Branding Bar -->
    <header class="invite-header">
      <router-link to="/" class="brand-link">
        <img src="../assets/favicon.png" alt="Splito" class="brand-logo" />
        <div class="brand-text">
          <span class="brand-title brand-font">Splito</span>
          <span class="brand-tagline">Expedition Treasury Protocol</span>
        </div>
      </router-link>
    </header>

    <main class="invite-container">
      <!-- Loading State -->
      <div v-if="loading" class="invite-card glass-panel loading-box">
        <i class="pi pi-spin pi-spinner text-4xl text-sky-400"></i>
        <p class="loading-label">Loading expedition invitation...</p>
      </div>

      <!-- Error State -->
      <div v-else-if="errorMessage" class="invite-card glass-panel error-box">
        <div class="icon-avatar error">
          <i class="pi pi-exclamation-triangle"></i>
        </div>
        <h2 class="card-title brand-font">Invitation Unavailable</h2>
        <p class="card-desc">{{ errorMessage }}</p>
        <div class="card-actions">
          <router-link to="/dashboard">
            <Button label="Go to Dashboard" icon="pi pi-home" severity="secondary" />
          </router-link>
        </div>
      </div>

      <!-- Valid Invite Content -->
      <div v-else-if="inviteInfo" class="invite-card glass-panel">
        <!-- Top Status Pill -->
        <div class="card-top-badge">
          <Tag 
            v-if="userStatus === 'active' || inviteInfo.is_munimji"
            value="Active Participant" 
            severity="success" 
            icon="pi pi-check-circle" 
            rounded 
          />
          <Tag 
            v-else-if="userStatus === 'requested'"
            value="Request Pending Approval" 
            severity="warn" 
            icon="pi pi-clock" 
            rounded 
          />
          <Tag 
            v-else-if="userStatus === 'rejected'"
            value="Invitation Declined" 
            severity="danger" 
            icon="pi pi-times-circle" 
            rounded 
          />
          <Tag 
            v-else
            value="Expedition Invitation" 
            severity="info" 
            icon="pi pi-send" 
            rounded 
          />
        </div>

        <div class="trip-headline-deck">
          <span class="kicker-label">GROUP EXPEDITION</span>
          <h1 class="trip-name brand-font">{{ inviteInfo.title }}</h1>
        </div>

        <!-- Manifest Route Box -->
        <div class="route-manifest-box">
          <div class="route-node-row">
            <div class="dot start"></div>
            <div class="node-details">
              <span class="node-label">Start Point</span>
              <span class="node-val">{{ inviteInfo.start_point }}</span>
            </div>
          </div>
          <div class="route-line-connector">
            <i class="pi pi-arrow-down arrow-down"></i>
          </div>
          <div class="route-node-row">
            <div class="dot destination"></div>
            <div class="node-details">
              <span class="node-label">Destination</span>
              <span class="node-val">{{ inviteInfo.destination }}</span>
            </div>
          </div>
        </div>

        <!-- Meta Details Grid -->
        <div class="trip-specs-grid">
          <div class="spec-pill">
            <i class="pi pi-calendar text-sky-400"></i>
            <div>
              <span class="spec-label">Timeline</span>
              <span class="spec-val">{{ formatDate(inviteInfo.start_at) }} — {{ formatDate(inviteInfo.end_at) }}</span>
            </div>
          </div>

          <div class="spec-pill">
            <i class="pi pi-crown text-amber-400"></i>
            <div>
              <span class="spec-label">Munimji (Treasurer)</span>
              <span class="spec-val">{{ inviteInfo.munimji_name || 'Trip Leader' }}</span>
            </div>
          </div>

          <div class="spec-pill">
            <i class="pi pi-users text-emerald-400"></i>
            <div>
              <span class="spec-label">Participants</span>
              <span class="spec-val">{{ inviteInfo.member_count }} Active</span>
            </div>
          </div>

          <div class="spec-pill" v-if="inviteInfo.estimated_budget > 0">
            <i class="pi pi-wallet text-indigo-400"></i>
            <div>
              <span class="spec-label">Est. Budget</span>
              <span class="spec-val">₹{{ Number(inviteInfo.estimated_budget).toLocaleString('en-IN') }}</span>
            </div>
          </div>
        </div>

        <!-- Condition A: User NOT logged in -->
        <div v-if="!isAuthenticated" class="auth-gate-section">
          <div class="gate-notice-box">
            <i class="pi pi-lock gate-icon"></i>
            <div class="gate-notice-text">
              <strong>Account Required</strong>
              <p>Please sign in or register to review this group and submit your request to join.</p>
            </div>
          </div>

          <div class="card-actions-row">
            <Button 
              @click="handleGoToLogin" 
              label="Sign In to Join" 
              icon="pi pi-sign-in" 
              severity="primary" 
              size="large" 
              class="w-full"
            />
            <Button 
              @click="handleGoToRegister" 
              label="Create Account" 
              icon="pi pi-user-plus" 
              severity="secondary" 
              size="large" 
              outlined
              class="w-full"
            />
          </div>
        </div>

        <!-- Condition B: User IS logged in -->
        <div v-else class="authenticated-decision-section">
          <!-- State 1: Active Member / Munimji -->
          <div v-if="userStatus === 'active' || inviteInfo.is_munimji" class="state-result-box active">
            <div class="state-icon-badge">
              <i class="pi pi-check"></i>
            </div>
            <h3>You are already in this expedition!</h3>
            <p>You have full access to view live balances, log pool cash, and submit expense claims.</p>
            <Button 
              @click="handleGoToDashboard" 
              label="Enter Trip Workspace" 
              icon="pi pi-arrow-right" 
              iconPos="right" 
              severity="primary" 
              size="large"
            />
          </div>

          <!-- State 2: Request Pending -->
          <div v-else-if="userStatus === 'requested'" class="state-result-box pending">
            <div class="state-icon-badge warn">
              <i class="pi pi-clock"></i>
            </div>
            <h3>Join Request Under Review</h3>
            <p>
              Your request has been delivered to Munimji (<strong>{{ inviteInfo.munimji_name }}</strong>). 
              You will receive an email once your membership is approved.
            </p>
            <Button 
              @click="handleGoToDashboard" 
              label="Back to Dashboard" 
              icon="pi pi-home" 
              severity="secondary" 
              outlined
            />
          </div>

          <!-- State 3: Rejected / Declined -->
          <div v-else-if="userStatus === 'rejected'" class="state-result-box rejected">
            <div class="state-icon-badge danger">
              <i class="pi pi-times"></i>
            </div>
            <h3>Invitation No Longer Available</h3>
            <p>This trip invitation is closed or was previously declined for your account.</p>
            <Button 
              @click="handleGoToDashboard" 
              label="Return to Dashboard" 
              icon="pi pi-home" 
              severity="secondary" 
              outlined
            />
          </div>

          <!-- State 4: Eligible (First time clicking invite link) -->
          <div v-else class="decision-prompt-box">
            <p class="decision-lead-text">
              You are logged in as <strong>{{ authStore.user?.full_name || authStore.user?.email }}</strong>. 
              Would you like to request to join this expedition?
            </p>

            <div class="card-actions-row">
              <Button 
                @click="handleDecision('reject')" 
                :loading="submitting && actionType === 'reject'"
                label="Decline" 
                icon="pi pi-times" 
                severity="danger" 
                outlined 
                size="large"
                class="flex-1"
              />
              <Button 
                @click="handleDecision('request')" 
                :loading="submitting && actionType === 'request'"
                label="Request to Join" 
                icon="pi pi-user-plus" 
                severity="primary" 
                size="large"
                class="flex-1"
              />
            </div>
            <span class="fineprint-note">
              Munimji will receive an email to approve your request.
            </span>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import Button from 'primevue/button';
import Tag from 'primevue/tag';
import { tripService, type InviteInfo } from '../api/services';
import { useAuthStore } from '../stores/auth';
import { useToast } from '../stores/toast';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const toast = useToast();

const loading = ref(true);
const errorMessage = ref('');
const inviteInfo = ref<InviteInfo | null>(null);
const userStatus = ref<string>('');
const submitting = ref(false);
const actionType = ref<'request' | 'reject' | null>(null);

const inviteCode = computed(() => {
  return (route.params.code as string) || (route.query.code as string) || (route.query.invite_code as string) || sessionStorage.getItem('splito_invite_code') || localStorage.getItem('splito_pending_invite') || '';
});

const isAuthenticated = computed(() => authStore.isAuthenticated());

const formatDate = (dateStr?: string) => {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric'
  });
};

const loadInviteDetails = async () => {
  if (!inviteCode.value) {
    errorMessage.value = 'No invite code provided in the link.';
    loading.value = false;
    return;
  }

  loading.value = true;
  errorMessage.value = '';

  try {
    const res = await tripService.getInviteInfo(inviteCode.value);
    inviteInfo.value = res.data;
    userStatus.value = res.data.user_status || '';
  } catch (err: any) {
    errorMessage.value = err.response?.data?.error || 'Invalid or expired invitation link.';
  } finally {
    loading.value = false;
  }
};

const handleGoToLogin = () => {
  router.push({
    path: '/login',
    query: { invite_code: inviteCode.value }
  });
};

const handleGoToRegister = () => {
  router.push({
    path: '/register',
    query: { invite_code: inviteCode.value }
  });
};

const handleGoToDashboard = () => {
  if (userStatus.value === 'active' && inviteInfo.value?.trip_id) {
    router.push({
      path: '/dashboard',
      query: { trip: inviteInfo.value.trip_id }
    });
  } else {
    router.push('/dashboard');
  }
};

const handleDecision = async (action: 'request' | 'reject') => {
  if (!inviteCode.value) return;

  submitting.value = true;
  actionType.value = action;

  try {
    const res = await tripService.respondInviteLink({
      invite_code: inviteCode.value,
      action
    });

    if (action === 'request') {
      userStatus.value = 'requested';
      toast.success(res.data.message || 'Join request sent to Munimji via email!');
    } else {
      userStatus.value = 'rejected';
      sessionStorage.removeItem('splito_invite_code');
      localStorage.removeItem('splito_pending_invite');
      toast.info('Invitation declined.');
    }
  } catch (err: any) {
    toast.error(err.response?.data?.error || `Failed to ${action} invitation`);
  } finally {
    submitting.value = false;
    actionType.value = null;
  }
};

onMounted(async () => {
  if (inviteCode.value) {
    try {
      sessionStorage.setItem('splito_invite_code', inviteCode.value);
      localStorage.setItem('splito_pending_invite', inviteCode.value);
    } catch (_) {}
  }
  if (!authStore.isAuthChecked) {
    await authStore.fetchUser();
  }
  await loadInviteDetails();
});

watch([() => route.params.code, () => authStore.user], async () => {
  await loadInviteDetails();
});
</script>

<style scoped>
.invite-page-root {
  min-height: 100vh;
  background-color: #0b1120;
  color: #f8fafc;
  display: flex;
  flex-direction: column;
}

.invite-header {
  padding: 24px 32px;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(12px);
}

.brand-link {
  display: flex;
  align-items: center;
  gap: 12px;
  text-decoration: none;
}

.brand-logo {
  width: 40px;
  height: 40px;
  border-radius: 10px;
}

.brand-text {
  display: flex;
  flex-direction: column;
}

.brand-title {
  font-size: 20px;
  font-weight: 800;
  color: #ffffff;
  letter-spacing: -0.5px;
}

.brand-tagline {
  font-size: 11px;
  color: #94a3b8;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  font-weight: 600;
}

.invite-container {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.invite-card {
  width: 100%;
  max-width: 560px;
  background: #111827;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
  padding: 36px 32px;
  box-shadow: 0 20px 45px rgba(0, 0, 0, 0.4);
}

.card-top-badge {
  display: flex;
  justify-content: center;
  margin-bottom: 18px;
}

.trip-headline-deck {
  text-align: center;
  margin-bottom: 24px;
}

.kicker-label {
  font-size: 11px;
  letter-spacing: 1.5px;
  font-weight: 800;
  color: #38bdf8;
  text-transform: uppercase;
}

.trip-name {
  font-size: 28px;
  font-weight: 800;
  color: #ffffff;
  margin: 6px 0 0 0;
  letter-spacing: -0.5px;
}

/* Route Box */
.route-manifest-box {
  background: #1e293b;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  padding: 18px 20px;
  margin-bottom: 24px;
}

.route-node-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-top: 5px;
  flex-shrink: 0;
}

.dot.start {
  background-color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.5);
}

.dot.destination {
  background-color: #10b981;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.5);
}

.node-details {
  display: flex;
  flex-direction: column;
}

.node-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #94a3b8;
  font-weight: 700;
}

.node-val {
  font-size: 15px;
  font-weight: 600;
  color: #f1f5f9;
  line-height: 1.4;
}

.route-line-connector {
  padding-left: 5px;
  margin: 4px 0;
  color: #64748b;
  font-size: 12px;
}

/* Specs Grid */
.trip-specs-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin-bottom: 28px;
}

.spec-pill {
  background: rgba(30, 41, 59, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 14px;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.spec-pill i {
  font-size: 18px;
}

.spec-label {
  display: block;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  color: #94a3b8;
  letter-spacing: 0.5px;
}

.spec-val {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
  margin-top: 2px;
}

/* Auth Gate */
.auth-gate-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.gate-notice-box {
  background: rgba(37, 99, 235, 0.1);
  border: 1px solid rgba(37, 99, 235, 0.25);
  border-radius: 14px;
  padding: 16px;
  display: flex;
  gap: 12px;
  align-items: center;
}

.gate-icon {
  font-size: 24px;
  color: #38bdf8;
}

.gate-notice-text strong {
  display: block;
  font-size: 14px;
  color: #ffffff;
}

.gate-notice-text p {
  margin: 2px 0 0 0;
  font-size: 12px;
  color: #94a3b8;
  line-height: 1.4;
}

/* Action Rows */
.card-actions-row {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 8px;
}

.fineprint-note {
  display: block;
  text-align: center;
  font-size: 12px;
  color: #64748b;
  margin-top: 10px;
}

/* States */
.state-result-box {
  text-align: center;
  padding: 20px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.state-icon-badge {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  background: rgba(16, 185, 129, 0.2);
  color: #10b981;
}

.state-icon-badge.warn {
  background: rgba(245, 158, 11, 0.2);
  color: #f59e0b;
}

.state-icon-badge.danger {
  background: rgba(239, 68, 68, 0.2);
  color: #ef4444;
}

.state-result-box h3 {
  font-size: 18px;
  font-weight: 700;
  color: #ffffff;
  margin: 0;
}

.state-result-box p {
  font-size: 13px;
  color: #94a3b8;
  max-width: 400px;
  line-height: 1.5;
  margin: 0 0 10px 0;
}

.decision-prompt-box {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.decision-lead-text {
  font-size: 14px;
  color: #cbd5e1;
  text-align: center;
  line-height: 1.5;
  margin: 0;
}

.decision-lead-text strong {
  color: #ffffff;
}

/* Loading / Error */
.loading-box, .error-box {
  text-align: center;
  padding: 48px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.loading-label {
  font-size: 14px;
  color: #94a3b8;
}

.icon-avatar.error {
  width: 60px;
  height: 60px;
  border-radius: 50%;
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
}

.card-title {
  font-size: 22px;
  font-weight: 700;
  color: #ffffff;
  margin: 0;
}

.card-desc {
  font-size: 14px;
  color: #94a3b8;
  max-width: 380px;
  line-height: 1.5;
  margin: 0;
}

@media (max-width: 640px) {
  .invite-card {
    padding: 24px 20px;
  }
  .trip-specs-grid {
    grid-template-columns: 1fr;
  }
  .card-actions-row {
    flex-direction: column;
  }
}
</style>
