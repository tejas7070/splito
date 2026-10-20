import { createApp } from 'vue';
import { createPinia } from 'pinia';
import PrimeVue from 'primevue/config';
import Aura from '@primevue/themes/aura';
import { definePreset } from '@primevue/themes';
import 'primeicons/primeicons.css';

import App from './App.vue';
import router from './router';
import './assets/neumorphic.css';

// Custom Cyan Blue Theme Preset for PrimeVue
const CyanPreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: '#ecfeff',
      100: '#cffafe',
      200: '#a5f3fc',
      300: '#67e8f9',
      400: '#22d3ee',
      500: '#06b6d4',
      600: '#0891b2',
      700: '#0e7490',
      800: '#155e75',
      900: '#164e63',
      950: '#083344',
    },
  },
});

// Purge any residual sensitive data from localStorage
localStorage.removeItem('token');
localStorage.removeItem('user');

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(PrimeVue, {
  theme: {
    preset: CyanPreset,
    options: {
      darkModeSelector: '.dark',
    },
  },
});

app.mount('#app');
