/**
 * Browser-storage access.
 *
 * Every accessor is wrapped: storage throws in private mode and in
 * sandboxed frames, and a thrown theme lookup should not blank the page.
 * Callers get null rather than an exception, and the app works — without
 * persistence — when storage is unavailable.
 */

const PREFIX = "squelch-";

/** Read a key, or null when absent or storage is unavailable. */
export function readStored(storage: Storage, key: string): string | null {
  try {
    return storage.getItem(key);
  } catch {
    return null;
  }
}

/** Write a key. */
export function writeStored(storage: Storage, key: string, value: string) {
  try {
    storage.setItem(key, value);
  } catch {
    // Private mode or blocked storage — the app works without persistence.
  }
}

/** Remove a key. */
export function removeStored(storage: Storage, key: string) {
  try {
    storage.removeItem(key);
  } catch {
    // Nothing to do.
  }
}

/** The prefix every Squelch storage key carries. */
export { PREFIX as STORAGE_PREFIX };
