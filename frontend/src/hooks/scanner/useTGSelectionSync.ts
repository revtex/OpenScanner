import { useEffect, useRef } from "react";
import { useSearchParams } from "react-router-dom";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  restoreTGSelection,
  restoreFromDisabledTGs,
  restoreAvoidList,
  resetTGSelection,
} from "@/app/slices/scanner/scannerSlice";
import {
  selectToken,
  useGetTGSelectionQuery,
  useUpdateTGSelectionMutation,
} from "@/features/auth";
import type { AvoidEntry } from "@/types";

function storageKey(instanceId: string): string {
  return `openscanner-tg-selection-${instanceId}`;
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

  const { data: tgSelectionData } = useGetTGSelectionQuery(undefined, {
    skip: !isAuthenticated || !config,
  });
  const [saveTGSelection] = useUpdateTGSelectionMutation();
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const restoredRef = useRef(false);
  const configRef = useRef(config);
  // Track what was last saved/fetched so we skip no-op PUTs.
  const lastSavedRef = useRef<string>("");

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
      lastSavedRef.current = JSON.stringify({
        d: tgSelectionData.disabledTGs,
        a: tgSelectionData.avoidList ?? [],
      });
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

  // Persist tgSelection: API (authenticated) or localStorage (anonymous)
  useEffect(() => {
    if (!configRef.current || !restoredRef.current) return undefined;

    if (isAuthenticated) {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        const cfg = configRef.current;
        if (!cfg) return;
        const disabledTGs: number[] = [];
        for (const sys of cfg.systems) {
          for (const tg of sys.talkgroups ?? []) {
            if (tgSelection[tg.id] === false) {
              disabledTGs.push(tg.id);
            }
          }
        }
        const now = Date.now();
        const activeAvoids: AvoidEntry[] = avoidList.filter(
          (a) => a.expiresAt === 0 || a.expiresAt > now,
        );
        // Skip PUT if nothing actually changed.
        const snapshot = JSON.stringify({ d: disabledTGs, a: activeAvoids });
        if (snapshot === lastSavedRef.current) return;
        lastSavedRef.current = snapshot;
        saveTGSelection({ disabledTGs, avoidList: activeAvoids });
      }, 500);
      return () => {
        if (debounceRef.current) clearTimeout(debounceRef.current);
      };
    } else {
      localStorage.setItem(storageKey(instanceId), JSON.stringify(tgSelection));
      return undefined;
    }
  }, [tgSelection, avoidList, instanceId, isAuthenticated, saveTGSelection]);
}
