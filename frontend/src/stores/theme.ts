import { defineStore } from 'pinia';
import { ref } from 'vue';

export const useThemeStore = defineStore('theme', () => {
  const isDark = ref<boolean>(localStorage.getItem('splito-theme') === 'dark');

  const applyTheme = () => {
    if (isDark.value) {
      document.documentElement.classList.add('dark');
      localStorage.setItem('splito-theme', 'dark');
    } else {
      document.documentElement.classList.remove('dark');
      localStorage.setItem('splito-theme', 'light');
    }
  };

  const toggleTheme = () => {
    isDark.value = !isDark.value;
    applyTheme();
  };

  // Initialize
  applyTheme();

  return {
    isDark,
    toggleTheme,
  };
});
