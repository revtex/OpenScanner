/**
 * Browser-storage access that survives the OpenScanner -> Squelch rename.
 *
 * Storage keys are not labels: they are how a returning visitor's state is
 * found again. Renaming `openscanner-tg-selection-<instance>` to
 * `squelch-…` without a fallback would present every existing user with a
 * blank slate — talkgroup selections, theme, paused state all apparently
 * reset — and the old values would still be sitting in the browser,
 * unreachable. So reads fall back to the old key and migrate the value
 * forward on first sight; writes only ever use the new key.
 *
 * Every accessor is wrapped: storage throws in private mode and in
 * sandboxed frames, and a thrown theme lookup should not blank the page.
 */

const PREFIX = "squelch-";
const LEGACY_PREFIX = "openscanner-";

/** The pre-rename spelling of a key, or null if it is not one of ours. */
function legacyKey(key: string): string | null {
  return key.startsWith(PREFIX)
    ? LEGACY_PREFIX + key.slice(PREFIX.length)
    : null;
}

/**
 * Read a key, falling back to its pre-rename name. When the fallback hits,
 * the value is rewritten under the new key and the old one dropped, so the
 * migration happens once per browser rather than on every read.
 */
export function readStored(storage: Storage, key: string): string | null {
  try {
    const current = storage.getItem(key);
    if (current !== null) return current;

    const legacy = legacyKey(key);
    if (legacy === null) return null;

    const value = storage.getItem(legacy);
    if (value === null) return null;

    try {
      storage.setItem(key, value);
      storage.removeItem(legacy);
    } catch {
      // Migration is best-effort; returning the value still works.
    }
    return value;
  } catch {
    return null;
  }
}

/** Write a key. Never writes the pre-rename name. */
export function writeStored(storage: Storage, key: string, value: string) {
  try {
    storage.setItem(key, value);
  } catch {
    // Private mode or blocked storage — the app works without persistence.
  }
}

/** Remove a key and its pre-rename counterpart. */
export function removeStored(storage: Storage, key: string) {
  try {
    storage.removeItem(key);
    const legacy = legacyKey(key);
    if (legacy !== null) storage.removeItem(legacy);
  } catch {
    // Nothing to do.
  }
}

/**
 * Map a persisted DaisyUI theme name forward. The theme names carry the
 * product name, so a stored "openscanner-dark" would otherwise resolve to
 * a theme that no longer exists and render an unstyled page.
 */
export function migrateThemeValue(value: string | null): string | null {
  if (value === null) return null;
  return value.startsWith(LEGACY_PREFIX)
    ? PREFIX + value.slice(LEGACY_PREFIX.length)
    : value;
}
