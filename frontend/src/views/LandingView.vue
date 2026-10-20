<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { useThemeStore } from '../stores/theme';
import Button from 'primevue/button';
import Tag from 'primevue/tag';
import appFavicon from '../assets/favicon.png';

const router = useRouter();
const authStore = useAuthStore();
const themeStore = useThemeStore();

const isLoggedIn = computed(() => authStore.isAuthenticated());

const handleGetStarted = () => {
  if (isLoggedIn.value) {
    router.push('/dashboard');
  } else {
    router.push('/register');
  }
};

const handleLogin = () => {
  if (isLoggedIn.value) {
    router.push('/dashboard');
  } else {
    router.push('/login');
  }
};
</script>

<template>
  <div class="landing-page">
    <!-- Top Modern Minimal Header -->
    <header class="landing-header">
      <div class="header-inner">
        <div class="brand-cluster" @click="router.push('/')">
          <div class="brand-icon-box">
            <img :src="appFavicon" alt="Splito Logo" class="brand-logo" />
          </div>
          <span class="brand-title brand-font">Splito</span>
        </div>

        <div class="header-actions">
          <button 
            class="neu-icon-btn" 
            @click="themeStore.toggleTheme" 
            :title="themeStore.isDark ? 'Switch to Light Theme' : 'Switch to Dark Theme'"
          >
            <i :class="themeStore.isDark ? 'pi pi-sun' : 'pi pi-moon'"></i>
          </button>

          <template v-if="isLoggedIn">
            <Button 
              label="Go to Dashboard" 
              icon="pi pi-arrow-right" 
              iconPos="right" 
              size="small" 
              @click="router.push('/dashboard')"
              class="primary-btn"
            />
          </template>
          <template v-else>
            <Button 
              label="Log In" 
              text 
              size="small" 
              @click="handleLogin"
              class="login-text-btn"
            />
            <Button 
              label="Get Started" 
              icon="pi pi-arrow-right" 
              iconPos="right" 
              size="small" 
              @click="handleGetStarted"
              class="primary-btn"
            />
          </template>
        </div>
      </div>
    </header>

    <!-- Main Hero Section -->
    <main class="landing-main">
      <div class="hero-container">
        <!-- Hero Left / Content -->
        <div class="hero-content">
          <div class="hero-badge-wrap">
            <Tag severity="info" rounded class="hero-tag">
              <i class="pi pi-compass mr-1"></i> Smart Trip Ledger & Treasury Protocol
            </Tag>
          </div>

          <h1 class="hero-heading brand-font">
            Trip expenses split without the <span class="gradient-text">group drama.</span>
          </h1>

          <p class="hero-subtext">
            Meet <strong>Splito</strong> — the automated expedition treasury with a dedicated <strong>Munimji</strong>. Collect pool funds, audit member claims with receipts, and settle peer balances with mathematical precision.
          </p>

          <div class="hero-cta-group">
            <Button 
              :label="isLoggedIn ? 'Enter Dashboard' : 'Plan Your Trip Free'" 
              icon="pi pi-sparkles" 
              size="large" 
              @click="handleGetStarted"
              class="hero-primary-btn"
            />
            <Button 
              v-if="!isLoggedIn"
              label="Sign In" 
              icon="pi pi-sign-in" 
              size="large" 
              outlined
              @click="handleLogin"
              class="hero-secondary-btn"
            />
          </div>

          <!-- Trust Badges -->
          <div class="trust-pill-row">
            <div class="trust-item">
              <i class="pi pi-shield"></i>
              <span>Anti-Tamper Cryptographic Ledgers</span>
            </div>
            <div class="trust-dot">•</div>
            <div class="trust-item">
              <i class="pi pi-receipt"></i>
              <span>Live Receipt Attachments</span>
            </div>
            <div class="trust-dot">•</div>
            <div class="trust-item">
              <i class="pi pi-bolt"></i>
              <span>1-Click Munimji Settlement</span>
            </div>
          </div>
        </div>

        <!-- Hero Right: Interactive Neumorphic Trip Ticket Widget -->
        <div class="hero-visual">
          <div class="boarding-pass-card">
            <div class="pass-top">
              <div class="pass-brand-row">
                <div class="pass-badge">
                  <img :src="appFavicon" alt="Splito" class="pass-icon" />
                  <span class="pass-title">Splito Ledger</span>
                </div>
                <span class="pass-live-tag">LIVE TRIP</span>
              </div>
              <div class="pass-expedition-title brand-font">Goa Monsoon Roadtrip 2026</div>
              <div class="pass-status-pill">
                <i class="pi pi-users text-xs"></i> 4 Members &nbsp;•&nbsp; <i class="pi pi-shield text-xs"></i> Munimji: Tejas
              </div>
            </div>

            <!-- Route Nodes -->
            <div class="pass-route-row">
              <div class="route-point">
                <span class="point-code">PNVL</span>
                <span class="point-name">Panvel</span>
              </div>
              <div class="route-line-box">
                <i class="pi pi-arrow-right"></i>
                <span class="route-dist">540 KM</span>
              </div>
              <div class="route-point text-right">
                <span class="point-code">GOA</span>
                <span class="point-name">Vagator</span>
              </div>
            </div>

            <!-- Treasury Stats Pill Grid -->
            <div class="pass-stats-grid">
              <div class="pass-stat-box">
                <span class="stat-label">CENTRAL TREASURY</span>
                <span class="stat-val font-mono">₹24,000</span>
                <span class="stat-sub">Advance Inflow</span>
              </div>
              <div class="pass-stat-box">
                <span class="stat-label">EXPENSES LOGGED</span>
                <span class="stat-val font-mono">₹16,400</span>
                <span class="stat-sub">Food, Fuel & Tolls</span>
              </div>
            </div>

            <!-- Live Settlement Preview Banner -->
            <div class="pass-settlement-footer">
              <div class="settle-notice">
                <i class="pi pi-check-circle text-emerald-500"></i>
                <div class="settle-text">
                  <span class="font-bold">Zero-Math Settlement Ready</span>
                  <span class="text-xs text-muted">Surplus refunds & direct peer dues calculated automatically.</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 3 Key Features Section -->
      <section class="features-section">
        <div class="features-header text-center">
          <span class="section-badge">DESIGNED FOR GROUPS</span>
          <h2 class="features-heading brand-font">Everything you need for zero-confusion trips</h2>
        </div>

        <div class="features-grid">
          <!-- Feature 1 -->
          <div class="feature-card">
            <div class="feature-icon-box">
              <i class="pi pi-wallet"></i>
            </div>
            <h3 class="feature-title brand-font">1. Central Treasury (Munimji)</h3>
            <p class="feature-desc">
              Collect advance pooled funds before the trip starts. Munimji logs common expenses with clean receipt uploads, keeping everyone informed in real-time.
            </p>
          </div>

          <!-- Feature 2 -->
          <div class="feature-card">
            <div class="feature-icon-box">
              <i class="pi pi-receipt"></i>
            </div>
            <h3 class="feature-title brand-font">2. Peer Expense Claims</h3>
            <p class="feature-desc">
              Did a member pay toll or dinner from their personal UPI? Submit a claim with proof. Munimji or designated Co-Reviewers audit and approve on the fly.
            </p>
          </div>

          <!-- Feature 3 -->
          <div class="feature-card">
            <div class="feature-icon-box">
              <i class="pi pi-sliders-h"></i>
            </div>
            <h3 class="feature-title brand-font">3. Automated Settlement Engine</h3>
            <p class="feature-desc">
              When the expedition ends, Splito equalizes per-person fair share, distributes surplus treasury refunds, and computes minimal peer-to-peer transfers.
            </p>
          </div>
        </div>
      </section>

      <!-- Bottom Call To Action Banner -->
      <section class="bottom-cta-banner">
        <div class="cta-inner">
          <h2 class="cta-title brand-font">Ready to hit the road?</h2>
          <p class="cta-desc">Join friends traveling smarter with transparent shared ledgers.</p>
          <Button 
            :label="isLoggedIn ? 'Go to Dashboard' : 'Get Started Now — It\'s Free'" 
            icon="pi pi-arrow-right" 
            iconPos="right" 
            size="large" 
            @click="handleGetStarted"
            class="cta-btn"
          />
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="landing-footer">
      <div class="footer-inner">
        <div class="footer-brand">
          <div class="brand-icon-box small">
            <img :src="appFavicon" alt="Splito Logo" class="brand-logo" />
          </div>
          <span class="brand-font font-bold">Splito</span>
          <span class="footer-sep">|</span>
          <span class="text-muted text-xs">Automated Group Treasury &amp; Expense Settlement Protocol</span>
        </div>
        <div class="footer-meta">
          <span class="text-muted text-xs">&copy; 2026 Splito. All rights reserved.</span>
        </div>
        <div class="footer-links">
          <button @click="handleLogin" class="footer-link">Log In</button>
          <button @click="handleGetStarted" class="footer-link">Register</button>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.landing-page {
  min-height: 100vh;
  background-color: var(--bg, #eceff2);
  color: var(--text-main, #191c20);
  display: flex;
  flex-direction: column;
  font-family: inherit;
  transition: background-color 0.3s ease, color 0.3s ease;
}

/* Header */
.landing-header {
  height: 68px;
  background-color: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  position: sticky;
  top: 0;
  z-index: 40;
  display: flex;
  align-items: center;
}

.header-inner {
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand-cluster {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}

.brand-icon-box {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
}

.brand-icon-box.small {
  width: 26px;
  height: 26px;
  padding: 3px;
  border-radius: 8px;
}

.brand-logo {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 6px;
}

.brand-title {
  font-size: 21px;
  font-weight: 700;
  color: var(--text-main);
  letter-spacing: -0.02em;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.neu-icon-btn {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-main);
  font-size: 14px;
  transition: all 0.2s ease;
}

.neu-icon-btn:hover {
  transform: translateY(-1px);
}

.neu-icon-btn:active {
  transform: translateY(1px);
  box-shadow: var(--shadow-inset);
}

/* Landing Main */
.landing-main {
  flex: 1;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 40px 24px 60px 24px;
  display: flex;
  flex-direction: column;
  gap: 60px;
}

/* Hero Container */
.hero-container {
  display: grid;
  grid-template-columns: 1.15fr 0.85fr;
  align-items: center;
  gap: 48px;
  padding: 30px 0 10px 0;
}

.hero-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.hero-tag {
  font-weight: 600 !important;
  font-size: 11px !important;
  padding: 4px 10px !important;
}

.hero-heading {
  font-size: 46px;
  line-height: 1.15;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-main);
  margin: 0;
}

.gradient-text {
  background: linear-gradient(135deg, #0284c7 0%, #06b6d4 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.hero-subtext {
  font-size: 16.5px;
  line-height: 1.6;
  color: var(--text-muted);
  margin: 0;
}

.hero-cta-group {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 6px;
  flex-wrap: wrap;
}

.hero-primary-btn {
  font-weight: 700 !important;
  border-radius: 12px !important;
  padding: 12px 24px !important;
}

.hero-secondary-btn {
  font-weight: 600 !important;
  border-radius: 12px !important;
  padding: 12px 22px !important;
}

/* Trust Pill Row */
.trust-pill-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
  margin-top: 10px;
  flex-wrap: wrap;
}

.trust-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.trust-item i {
  color: var(--accent);
}

.trust-dot {
  opacity: 0.4;
}

/* Visual Widget: Boarding Pass */
.hero-visual {
  display: flex;
  justify-content: center;
}

.boarding-pass-card {
  width: 100%;
  max-width: 400px;
  background: var(--surface);
  border-radius: 24px;
  box-shadow: var(--shadow-flat);
  border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.06));
  overflow: hidden;
  display: flex;
  flex-direction: column;
  transition: transform 0.3s ease;
}

.boarding-pass-card:hover {
  transform: translateY(-4px);
}

.pass-top {
  padding: 22px 24px 18px 24px;
  border-bottom: 1px dashed var(--border-subtle, rgba(0, 0, 0, 0.1));
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pass-brand-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pass-badge {
  display: flex;
  align-items: center;
  gap: 6px;
}

.pass-icon {
  width: 18px;
  height: 18px;
  border-radius: 4px;
}

.pass-title {
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: var(--text-muted);
  text-transform: uppercase;
}

.pass-live-tag {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.05em;
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
  padding: 2px 7px;
  border-radius: 6px;
}

.pass-expedition-title {
  font-size: 17px;
  font-weight: 700;
  color: var(--text-main);
  margin-top: 2px;
}

.pass-status-pill {
  font-size: 11.5px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
}

.pass-route-row {
  padding: 18px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--bg);
}

.route-point {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.point-code {
  font-size: 22px;
  font-weight: 800;
  font-family: monospace;
  color: var(--text-main);
}

.point-name {
  font-size: 11px;
  color: var(--text-muted);
}

.route-line-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  color: var(--accent);
}

.route-dist {
  font-size: 9px;
  font-weight: 700;
  color: var(--text-muted);
}

.pass-stats-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  padding: 16px 24px;
}

.pass-stat-box {
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  border-radius: 12px;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-label {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.06em;
  color: var(--text-muted);
}

.stat-val {
  font-size: 17px;
  font-weight: 800;
  color: var(--text-main);
}

.stat-sub {
  font-size: 10px;
  color: var(--text-muted);
}

.pass-settlement-footer {
  padding: 14px 24px;
  border-top: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.05));
  background: rgba(37, 99, 235, 0.04);
}

.settle-notice {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.settle-text {
  display: flex;
  flex-direction: column;
  font-size: 12px;
  color: var(--text-main);
  line-height: 1.35;
}

/* Features Section */
.features-section {
  display: flex;
  flex-direction: column;
  gap: 36px;
  padding-top: 20px;
}

.features-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.section-badge {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.1em;
  color: var(--accent);
}

.features-heading {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-main);
  margin: 0;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.feature-card {
  background: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 28px 24px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.04));
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.feature-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.08);
}

.feature-icon-box {
  width: 50px;
  height: 50px;
  border-radius: 14px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent);
  font-size: 22px;
}

.feature-title {
  font-size: 16.5px;
  font-weight: 700;
  color: var(--text-main);
  margin: 0;
}

.feature-desc {
  font-size: 13.5px;
  line-height: 1.55;
  color: var(--text-muted);
  margin: 0;
}

/* Bottom CTA Banner */
.bottom-cta-banner {
  background: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 24px;
  border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.05));
  padding: 44px 32px;
  text-align: center;
}

.cta-inner {
  max-width: 540px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
}

.cta-title {
  font-size: 30px;
  font-weight: 800;
  margin: 0;
  color: var(--text-main);
}

.cta-desc {
  font-size: 15px;
  color: var(--text-muted);
  margin: 0 0 8px 0;
}

.cta-btn {
  border-radius: 12px !important;
  font-weight: 700 !important;
  padding: 12px 28px !important;
}

/* Footer */
.landing-footer {
  border-top: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
  padding: 24px 0;
  background: var(--surface);
}

.footer-inner {
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
}

.footer-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.footer-sep {
  color: var(--text-muted);
  opacity: 0.35;
  font-size: 13px;
}

.footer-meta {
  display: flex;
  align-items: center;
}

.footer-links {
  display: flex;
  align-items: center;
  gap: 18px;
}

.footer-link {
  background: none;
  border: none;
  font-size: 13px;
  color: var(--text-muted);
  font-weight: 600;
  cursor: pointer;
  transition: color 0.15s ease;
}

.footer-link:hover {
  color: var(--accent);
}

/* Responsive queries */
@media (max-width: 960px) {
  .hero-container {
    grid-template-columns: 1fr;
    gap: 36px;
    text-align: center;
  }

  .hero-content {
    align-items: center;
  }

  .hero-heading {
    font-size: 36px;
  }

  .hero-cta-group {
    justify-content: center;
  }

  .features-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 600px) {
  .hero-heading {
    font-size: 30px;
  }

  .trust-pill-row {
    flex-direction: column;
    gap: 6px;
  }

  .trust-dot {
    display: none;
  }

  .footer-inner {
    flex-direction: column;
    text-align: center;
    gap: 12px;
  }
}
</style>