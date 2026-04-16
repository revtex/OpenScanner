import { useEffect, useRef } from "react";
import { useSearchParams } from "react-router-dom";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  restoreTGSelection,
  restoreFromDisabledTGs,
  restoreAvoidList,
} from "@/app/slices/scannerSlice";
import {
  selectToken,
  useGetTGSelectionQuery,
  useUpdateTGSelectionMutation,
} from "@/app/slices/authSlice";
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

  // Reset restored flag when auth state changes
  useEffect(() => {
    restoredRef.current = false;
  }, [isAuthenticated]);

  // Restore tgSelection from API (authenticated) or localStorage (anonymous)
  useEffect(() => {
    if (!config) return;

    if (isAuthenticated) {
      if (!tgSelectionData) return;
      dispatch(restoreFromDisabledTGs(tgSelectionData.disabledTGs));
      dispatch(restoreAvoidList(tgSelectionData.avoidList ?? []));
      restoredRef.current = true;
    } else {
      const raw = localStorage.getItem(storageKey(instanceId));
      if (!raw) {
        restoredRef.current = true;
        return;
      }
      try {
        const saved = JSON.parse(raw) as Record<string, unknown>;
        const restored: Record<number, boolean> = {};
        for (const sys of config.systems) {
          for (const tg of sys.talkgroups ?? []) {
            const savedVal = saved[String(tg.id)];
            restored[tg.id] = typeof savedVal === "boolean" ? savedVal : true;
          }
        }
        dispatch(restoreTGSelection(restored));
      } catch {
        // ignore malformed data
      }
      restoredRef.current = true;
    }
  }, [config, instanceId, dispatch, isAuthenticated, tgSelectionData]);

  // Persist tgSelection: API (authenticated) or localStorage (anonymous)
  useEffect(() => {
    if (!config || !restoredRef.current) return;

    if (isAuthenticated) {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        const disabledTGs: number[] = [];
        for (const sys of config.systems) {
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
        saveTGSelection({ disabledTGs, avoidList: activeAvoids });
      }, 500);
      return () => {
        if (debounceRef.current) clearTimeout(debounceRef.current);
      };
    } else {
      localStorage.setItem(storageKey(instanceId), JSON.stringify(tgSelection));
    }
  }, [
    tgSelection,
    avoidList,
    instanceId,
    config,
    isAuthenticated,
    saveTGSelection,
  ]);
}
