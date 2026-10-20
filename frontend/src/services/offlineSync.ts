import { ref, computed } from 'vue';
import api from '../api/client';
import {
  getPendingActions,
  getAllOfflineActions,
  updateActionStatus,
  queueOfflineAction,
  clearSyncedActions,
  generateUUID,
  type OfflineAction,
} from './offlineStorage';
import { useToast } from '../stores/toast';

const isOnline = ref(typeof navigator !== 'undefined' ? navigator.onLine : true);
const isSyncing = ref(false);
const pendingCount = ref(0);
const offlineActions = ref<OfflineAction[]>([]);

export function useOfflineSync() {
  const toast = useToast();

  async function refreshPendingList(tripId?: number) {
    try {
      const actions = await getAllOfflineActions(tripId);
      offlineActions.value = actions;
      pendingCount.value = actions.filter((a) => a.sync_status === 'pending' || a.sync_status === 'failed').length;
    } catch (e) {
      console.warn('Failed to refresh pending actions:', e);
    }
  }

  // Record an action offline
  async function recordOffline(
    action: Omit<OfflineAction, 'id' | 'sync_status' | 'retry_count' | 'created_at' | 'client_uuid'>,
    receiptFile?: File
  ): Promise<number> {
    let receiptBlob: Blob | undefined;
    let receiptFilename: string | undefined;

    if (receiptFile) {
      receiptBlob = receiptFile;
      receiptFilename = receiptFile.name;
    }

    const client_uuid = generateUUID();
    const id = await queueOfflineAction({
      ...action,
      client_uuid,
      created_at: new Date().toISOString(),
      receipt_blob: receiptBlob,
      receipt_filename: receiptFilename,
    });

    await refreshPendingList(action.trip_id);

    toast.info(`💾 Saved locally (${action.action_type}). Will sync when connection is available.`);

    // If online right now, attempt immediate background flush
    if (isOnline.value) {
      syncPendingQueue();
    }

    return id;
  }

  // Upload receipt blob if present
  async function uploadBlobReceipt(blob: Blob, filename = 'offline_receipt.jpg'): Promise<string> {
    const formData = new FormData();
    formData.append('receipt', blob, filename);
    const res = await api.post('/uploads/receipt', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return res.data.file_path || res.data.path || res.data.url;
  }

  // Sync entire pending queue
  async function syncPendingQueue(tripId?: number) {
    if (isSyncing.value || !isOnline.value) return;

    isSyncing.value = true;
    try {
      const pending = await getPendingActions();
      if (pending.length === 0) {
        isSyncing.value = false;
        return;
      }

      toast.info(`🔄 Syncing ${pending.length} offline record(s)...`);
      let successCount = 0;

      for (const item of pending) {
        if (!item.id) continue;

        try {
          await updateActionStatus(item.id, 'syncing');

          const payload: Record<string, any> = { ...item.payload, client_uuid: item.client_uuid };

          // If receipt was captured offline and not yet uploaded
          if (item.receipt_blob && !payload.receipt_path) {
            try {
              const uploadedPath = await uploadBlobReceipt(item.receipt_blob, item.receipt_filename);
              payload.receipt_path = uploadedPath;
            } catch (err: any) {
              console.warn('Receipt upload failed, continuing or pausing:', err);
              // If receipt upload fails, mark item failed and continue
              await updateActionStatus(item.id, 'failed', err?.response?.data?.error || 'Receipt upload failed');
              continue;
            }
          }

          // Submit payload to endpoint
          await api.post(item.endpoint, payload);

          await updateActionStatus(item.id, 'synced');
          successCount++;
        } catch (err: any) {
          const errMsg = err?.response?.data?.error || err?.message || 'Sync failed';
          console.error(`Failed to sync action #${item.id}:`, errMsg);
          await updateActionStatus(item.id, 'failed', errMsg);
        }
      }

      await refreshPendingList(tripId);

      if (successCount > 0) {
        toast.success(`✅ Successfully synced ${successCount} offline expense(s)!`);
        // Clean up synced items after brief delay
        setTimeout(() => {
          clearSyncedActions().then(() => refreshPendingList(tripId));
        }, 3000);
      }
    } finally {
      isSyncing.value = false;
    }
  }

  function initListeners() {
    if (typeof window === 'undefined') return;

    window.addEventListener('online', () => {
      isOnline.value = true;
      toast.info('🌐 Connection restored! Syncing pending records...');
      syncPendingQueue();
    });

    window.addEventListener('offline', () => {
      isOnline.value = false;
      toast.warn('⚠️ Offline mode: Expenses and claims will be saved locally on this device.', 6000);
    });

    refreshPendingList();
  }

  return {
    isOnline: computed(() => isOnline.value),
    isSyncing: computed(() => isSyncing.value),
    pendingCount: computed(() => pendingCount.value),
    offlineActions: computed(() => offlineActions.value),
    recordOffline,
    syncPendingQueue,
    refreshPendingList,
    initListeners,
  };
}
