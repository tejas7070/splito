import { openDB, type DBSchema, type IDBPDatabase } from 'idb';

export interface OfflineAction {
  id?: number;
  client_uuid: string;
  trip_id: number;
  action_type: 'claim' | 'expense' | 'contribution';
  endpoint: string;
  payload: Record<string, any>;
  receipt_blob?: Blob;
  receipt_filename?: string;
  created_at: string;
  sync_status: 'pending' | 'syncing' | 'failed' | 'synced';
  retry_count: number;
  error_message?: string;
}

interface TripLedgerDB extends DBSchema {
  offline_actions: {
    key: number;
    value: OfflineAction;
    indexes: {
      'by-status': string;
      'by-trip': number;
      'by-uuid': string;
    };
  };
}

const DB_NAME = 'tripledger_offline_db';
const DB_VERSION = 1;

let dbPromise: Promise<IDBPDatabase<TripLedgerDB>> | null = null;

function getDB() {
  if (!dbPromise) {
    dbPromise = openDB<TripLedgerDB>(DB_NAME, DB_VERSION, {
      upgrade(db) {
        if (!db.objectStoreNames.contains('offline_actions')) {
          const store = db.createObjectStore('offline_actions', {
            keyPath: 'id',
            autoIncrement: true,
          });
          store.createIndex('by-status', 'sync_status');
          store.createIndex('by-trip', 'trip_id');
          store.createIndex('by-uuid', 'client_uuid', { unique: true });
        }
      },
    });
  }
  return dbPromise;
}

// Generate secure random UUID v4
export function generateUUID(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export async function queueOfflineAction(action: Omit<OfflineAction, 'id' | 'sync_status' | 'retry_count'>): Promise<number> {
  const db = await getDB();
  const id = await db.add('offline_actions', {
    ...action,
    sync_status: 'pending',
    retry_count: 0,
  });
  return id as number;
}

export async function getPendingActions(): Promise<OfflineAction[]> {
  const db = await getDB();
  const all = await db.getAll('offline_actions');
  return all.filter((a) => a.sync_status === 'pending' || a.sync_status === 'failed');
}

export async function getAllOfflineActions(tripId?: number): Promise<OfflineAction[]> {
  const db = await getDB();
  const all = await db.getAll('offline_actions');
  if (tripId) {
    return all.filter((a) => a.trip_id === tripId);
  }
  return all;
}

export async function updateActionStatus(
  id: number,
  status: OfflineAction['sync_status'],
  errorMessage?: string
): Promise<void> {
  const db = await getDB();
  const item = await db.get('offline_actions', id);
  if (item) {
    item.sync_status = status;
    if (errorMessage !== undefined) {
      item.error_message = errorMessage;
      if (status === 'failed') {
        item.retry_count = (item.retry_count || 0) + 1;
      }
    }
    await db.put('offline_actions', item);
  }
}

export async function deleteOfflineAction(id: number): Promise<void> {
  const db = await getDB();
  await db.delete('offline_actions', id);
}

export async function clearSyncedActions(): Promise<void> {
  const db = await getDB();
  const all = await db.getAll('offline_actions');
  const tx = db.transaction('offline_actions', 'readwrite');
  for (const item of all) {
    if (item.sync_status === 'synced' && item.id) {
      await tx.store.delete(item.id);
    }
  }
  await tx.done;
}
