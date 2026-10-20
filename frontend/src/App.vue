<script setup lang="ts">
import { onMounted } from 'vue';
import { RouterView } from 'vue-router';
import { useToast } from './stores/toast';
import { useOfflineSync } from './services/offlineSync';
import InstallPwaPrompt from './components/InstallPwaPrompt.vue';

const toast = useToast();
const offlineSync = useOfflineSync();

onMounted(() => {
  offlineSync.initListeners();
});
</script>

<template>
  <!-- Global Toast Notifications Stack -->
  <div class="toast-stack" v-if="toast.toasts.value.length > 0">
    <div 
      v-for="t in toast.toasts.value" 
      :key="t.id" 
      :class="['neu-toast', t.type]"
      @click="toast.remove(t.id)"
    >
      <span class="toast-icon">
        {{ t.type === 'success' ? '✓' : t.type === 'error' ? '✕' : t.type === 'warn' ? '⚠️' : 'ℹ' }}
      </span>
      <span class="toast-text">{{ t.message }}</span>
    </div>
  </div>

  <RouterView />

  <!-- PWA Install Prompt for Android and iPhone -->
  <InstallPwaPrompt />
</template>

<style>
body {
  margin: 0;
  padding: 0;
  background-color: var(--bg, #eceff2);
  color: var(--text-main, #191c20);
  touch-action: manipulation;
  -webkit-tap-highlight-color: transparent;
  padding-top: env(safe-area-inset-top);
  padding-bottom: env(safe-area-inset-bottom);
}
</style>
