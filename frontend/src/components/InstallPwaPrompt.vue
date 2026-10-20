<script setup lang="ts">
import { ref, onMounted } from 'vue';

const showPrompt = ref(false);
const isIOS = ref(false);
const deferredPrompt = ref<any>(null);

onMounted(() => {
  // Check if running in standalone mode (already installed)
  const isStandalone =
    window.matchMedia('(display-mode: standalone)').matches ||
    (window.navigator as any).standalone === true;

  if (isStandalone) {
    showPrompt.value = false;
    return;
  }

  // Detect iOS Safari
  const userAgent = window.navigator.userAgent.toLowerCase();
  isIOS.value = /iphone|ipad|ipod/.test(userAgent) && !(window as any).MSStream;

  // Listen for beforeinstallprompt event (Android / Chromium)
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault();
    deferredPrompt.value = e;
    showPrompt.value = true;
  });

  // If iOS and not standalone, show install helper after 3 seconds
  if (isIOS.value) {
    const dismissed = localStorage.getItem('pwa_ios_dismissed');
    if (!dismissed) {
      setTimeout(() => {
        showPrompt.value = true;
      }, 3000);
    }
  }
});

async function handleInstallClick() {
  if (deferredPrompt.value) {
    deferredPrompt.value.prompt();
    const { outcome } = await deferredPrompt.value.userChoice;
    if (outcome === 'accepted') {
      showPrompt.value = false;
    }
    deferredPrompt.value = null;
  }
}

function dismissPrompt() {
  showPrompt.value = false;
  if (isIOS.value) {
    localStorage.setItem('pwa_ios_dismissed', 'true');
  }
}
</script>

<template>
  <transition name="pwa-slide">
    <div v-if="showPrompt" class="pwa-banner-card">
      <div class="pwa-main-group">
        <div class="pwa-app-icon">
          <i class="pi pi-compass text-lg"></i>
        </div>
        <div class="pwa-text-box">
          <div class="pwa-card-title">Install Splito App</div>
          <div v-if="!isIOS" class="pwa-card-sub">
            Save on your phone for full offline access during expeditions.
          </div>
          <div v-else class="pwa-card-sub">
            Tap <strong class="font-semibold text-[var(--accent)]">Share</strong> <i class="pi pi-external-link text-xs"></i> below, then select <strong class="font-semibold text-[var(--accent)]">'Add to Home Screen'</strong>.
          </div>
        </div>
      </div>

      <div class="pwa-btn-group">
        <button
          v-if="!isIOS"
          @click="handleInstallClick"
          class="pwa-action-install"
        >
          <i class="pi pi-download"></i>
          <span>Install</span>
        </button>
        <button
          @click="dismissPrompt"
          class="pwa-action-close"
          aria-label="Dismiss banner"
          title="Dismiss"
        >
          <i class="pi pi-times"></i>
        </button>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.pwa-banner-card {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  width: calc(100% - 32px);
  max-width: 460px;
  background: var(--surface) !important;
  border: 1px solid var(--border, rgba(125, 133, 146, 0.2));
  border-radius: 16px;
  padding: 12px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  box-shadow: var(--shadow-flat-lg);
  z-index: 99999;
}

.pwa-main-group {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  flex: 1;
}

.pwa-app-icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.pwa-text-box {
  min-width: 0;
  flex: 1;
}

.pwa-card-title {
  color: var(--text-main) !important;
  font-size: 14px !important;
  font-weight: 700 !important;
  line-height: 1.3 !important;
  font-family: 'Space Grotesk', sans-serif;
  letter-spacing: -0.01em;
}

.pwa-card-sub {
  color: var(--text-muted) !important;
  font-size: 12px !important;
  line-height: 1.35 !important;
  margin-top: 2px;
}

.pwa-btn-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.pwa-action-install {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--accent);
  color: #ffffff !important;
  font-size: 12px;
  font-weight: 600;
  padding: 8px 16px;
  border-radius: 10px;
  border: none;
  cursor: pointer;
  box-shadow: var(--shadow-flat-sm);
  transition: all 0.2s ease;
  font-family: inherit;
}

.pwa-action-install:hover {
  transform: translateY(-1px);
  filter: brightness(1.1);
}

.pwa-action-install:active {
  transform: translateY(1px);
  box-shadow: var(--shadow-inset);
}

.pwa-action-close {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border: 1px solid transparent;
  color: var(--text-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
  font-size: 12px;
}

.pwa-action-close:hover {
  color: var(--text-main);
  box-shadow: var(--shadow-inset);
}

.pwa-slide-enter-active,
.pwa-slide-leave-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.pwa-slide-enter-from,
.pwa-slide-leave-to {
  transform: translate(-50%, 50px);
  opacity: 0;
}
</style>
