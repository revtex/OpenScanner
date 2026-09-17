import { useCallback, useEffect, useRef } from "react";
import { useSearchParams } from "react-router-dom";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  restoreTGSelection,
  restoreFromDisabledTGs,
  restoreAvoidList,
  resetTGSelection,
} from "../scannerSlice";
import {
  selectToken,
  useGetTGSelectionQuery,
  useUpdateTGSelectionMutation,
} from "@/features/auth";
import type { AvoidEntry } from "@/types";

function storageKey(instanceId: string): string {
  return `openscanner-tg-selection-${instanceId}`;
}

/** How many times a rejected write is re-sent with a refreshed version. */
const MAX_CONFLICT_RETRIES = 2;

interface PendingSave {
  disabledTGs: number[];
  avoidList: AvoidEntry[];
  snapshot: string;
}

/** Pulls `details.currentVersion` out of a 409 error envelope. */
function conflictVersion(err: unknown): string | undefined {
  const data = (err as { data?: unknown }).data;
  if (typeof data !== "object" || data === null) return undefined;
  const error = (data as { error?: unknown }).error;
  if (typeof error !== "object" || error === null) return undefined;
  const details = (error as { details?: unknown }).details;
  if (typeof details !== "object" || details === null) return undefined;
  const version = (details as { currentVersion?: unknown }).currentVersion;
  return typeof version === "string" ? version : undefined;
}

/**
 * Order-insensitive fingerprint of what is stored server-side, used to skip
 * no-op PUTs. Sorted so the server's stored order and our config iteration
 * order compare equal.
 */
function snapshotOf(disabledTGs: number[], avoidList: AvoidEntry[]): string {
  return JSON.stringify({
    d: [...disabledTGs].sort((a, b) => a - b),
    a: [...avoidList]
      .map((e) => [e.talkgroupId, e.expiresAt])
      .sort((x, y) => x[0] - y[0]),
  });
}

/**
 * Keeps tgSelection and avoidList in sync with the backend (authenticated)
 * or localStorage (anonymous). Must be mounted at the Scanner page level
 * so it runs regardless of whether SelectTGPanel is open.
 */
export function useTGSelectionSync() {
  const dispatch = useAppDispatch();
  const [searchParams] = useSearchParams();
  const instanceId = searchParams.get("id") ?? "default";

  const token = useAppSelector(selectToken);
  const isAuthenticated = !!token;
  const config = useAppSelector((s) => s.scanner.config);
  const tgSelection = useAppSelector((s) => s.scanner.tgSelection);
  const avoidList = useAppSelector((s) => s.scanner.avoidList);

  const { data: tgSelectionData, refetch } = useGetTGSelectionQuery(undefined, {
    skip: !isAuthenticated || !config,
  });
  const [saveTGSelection] = useUpdateTGSelectionMutation();
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const restoredRef = useRef(false);
  const configRef = useRef(config);
  // Track what was last saved/fetched so we skip no-op PUTs.
  const lastSavedRef = useRef<string>("");
  // Fingerprint of the server-side selection this tab last read. Sent with
  // every PUT so the server can reject a stale overwrite.
  const versionRef = useRef<string | undefined>(undefined);
  // Latest selection the user has locally, recomputed on every change.
  const pendingRef = useRef<PendingSave | null>(null);

  // Reset restored flag when auth state changes
  useEffect(() => {
    restoredRef.current = false;
    dispatch(resetTGSelection());
  }, [isAuthenticated, dispatch]);

  // Restore tgSelection from API (authenticated) or localStorage (anonymous).
  // Only runs once — subsequent config changes (auto-populate, backfills) do NOT
  // overwrite user selections. New talkgroups default to enabled (missing key = true).
  useEffect(() => {
    if (restoredRef.current) return;
    if (!config) return;

    if (isAuthenticated) {
      if (!tgSelectionData) return;
      // Seed the last-saved snapshot so the persist effect skips the initial no-op PUT.
      lastSavedRef.current = snapshotOf(
        tgSelectionData.disabledTGs,
        tgSelectionData.avoidList ?? [],
      );
      versionRef.current = tgSelectionData.version;
      dispatch(restoreFromDisabledTGs(tgSelectionData.disabledTGs));
      dispatch(restoreAvoidList(tgSelectionData.avoidList ?? []));
      restoredRef.current = true;
    } else {
      const raw = localStorage.getItem(storageKey(instanceId));
      const restored: Record<number, boolean> = {};
      for (const sys of config.systems) {
        for (const tg of sys.talkgroups ?? []) {
          restored[tg.id] = true;
        }
      }
      if (raw) {
        try {
          const saved = JSON.parse(raw) as Record<string, unknown>;
          for (const sys of config.systems) {
            for (const tg of sys.talkgroups ?? []) {
              const savedVal = saved[String(tg.id)];
              if (typeof savedVal === "boolean") {
                restored[tg.id] = savedVal;
              }
            }
          }
        } catch {
          // ignore malformed data
        }
      }
      dispatch(restoreTGSelection(restored));
      restoredRef.current = true;
    }
  }, [config, instanceId, dispatch, isAuthenticated, tgSelectionData]);

  // Keep configRef fresh without triggering the persist effect.
  useEffect(() => {
    configRef.current = config;
  }, [config]);

  // Flush the latest pending selection to the server.
  //
  // Reads `pendingRef` at call time rather than closing over a snapshot, so a
  // retry always sends what the user currently has. A 409 means someone else
  // saved first; which side is stale is decided by whether local state moved
  // on after the rejected write was issued.
  const flush = useCallback(async () => {
    // Retries loop rather than recurse: each pass re-reads `pendingRef`, so a
    // rejected write is re-sent with whatever the user has now.
    for (let attempt = 0; attempt <= MAX_CONFLICT_RETRIES; attempt++) {
      const pending = pendingRef.current;
      if (!pending || pending.snapshot === lastSavedRef.current) return;
      lastSavedRef.current = pending.snapshot;

      try {
        const res = await saveTGSelection({
          disabledTGs: pending.disabledTGs,
          avoidList: pending.avoidList,
          version: versionRef.current,
        }).unwrap();
        if (res.version) versionRef.current = res.version;
        return;
      } catch (err) {
        // Allow a retry on the next change.
        lastSavedRef.current = "";
        if ((err as { status?: number }).status !== 409) return;

        const serverVersion = conflictVersion(err);
        if (pendingRef.current?.snapshot !== pending.snapshot) {
          // The user kept editing while this write was in flight, so their
          // work is the newest thing there is: re-send it with the version
          // the server just handed us rather than throwing it away.
          if (serverVersion) versionRef.current = serverVersion;
          continue;
        }

        // Nothing changed here since the rejected write — this session is the
        // stale one (an old tab, another device). Adopt the newer selection
        // instead of overwriting whoever saved it.
        try {
          const fresh = await refetch().unwrap();
          versionRef.current = fresh.version;
          lastSavedRef.current = snapshotOf(
            fresh.disabledTGs,
            fresh.avoidList ?? [],
          );
          dispatch(restoreFromDisabledTGs(fresh.disabledTGs));
          dispatch(restoreAvoidList(fresh.avoidList ?? []));
        } catch {
          // Leave local state alone; the next change retries.
        }
        return;
      }
    }
  }, [saveTGSelection, refetch, dispatch]);

  // Persist tgSelection: API (authenticated) or localStorage (anonymous)
  useEffect(() => {
    if (!configRef.current || !restoredRef.current) return undefined;

    if (!isAuthenticated) {
      localStorage.setItem(storageKey(instanceId), JSON.stringify(tgSelection));
      return undefined;
    }

    const cfg = configRef.current;
    const disabledTGs: number[] = [];
    for (const sys of cfg.systems) {
      for (const tg of sys.talkgroups ?? []) {
        if (tgSelection[tg.id] === false) {
          disabledTGs.push(tg.id);
        }
      }
    }
    disabledTGs.sort((a, b) => a - b);
    const now = Date.now();
    const activeAvoids: AvoidEntry[] = avoidList.filter(
      (a) => a.expiresAt === 0 || a.expiresAt > now,
    );
    // Recomputed on every change so `flush` can tell "the user is still
    // editing" from "this tab is stale" when a write is rejected.
    pendingRef.current = {
      disabledTGs,
      avoidList: activeAvoids,
      snapshot: snapshotOf(disabledTGs, activeAvoids),
    };

    // Skip the PUT if nothing actually changed.
    if (pendingRef.current.snapshot === lastSavedRef.current) return undefined;

    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      void flush();
    }, 500);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [tgSelection, avoidList, instanceId, isAuthenticated, flush]);
}
