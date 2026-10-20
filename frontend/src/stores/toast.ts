import { ref } from 'vue';

export interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'info' | 'warn';
}

const toasts = ref<Toast[]>([]);
let toastId = 0;

export function useToast() {
  const show = (message: string, type: 'success' | 'error' | 'info' | 'warn' = 'info', duration = 4000) => {
    const id = ++toastId;
    toasts.value.push({ id, message, type });
    setTimeout(() => {
      remove(id);
    }, duration);
  };

  const success = (message: string, duration = 4000) => show(message, 'success', duration);
  const error = (message: string, duration = 5000) => show(message, 'error', duration);
  const info = (message: string, duration = 4000) => show(message, 'info', duration);
  const warn = (message: string, duration = 6500) => show(message, 'warn', duration);

  const remove = (id: number) => {
    toasts.value = toasts.value.filter(t => t.id !== id);
  };

  return {
    toasts,
    show,
    success,
    error,
    info,
    warn,
    remove,
  };
}
