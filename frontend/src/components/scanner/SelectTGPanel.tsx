import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useVirtualizer } from "@tanstack/react-virtual";
import { X, ChevronDown, ChevronRight } from "lucide-react";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  toggleTG,
  setAllTGs,
  setTGsBySystem,
  restoreTGSelection,
} from "@/app/slices/scannerSlice";
import type { TalkgroupConfig, SystemConfig } from "@/types";

interface SelectTGPanelProps {
  isOpen: boolean;
  onClose: () => void;
}

function storageKey(instanceId: string): string {
  return `openscanner-tg-selection-${instanceId}`;
}

export default function SelectTGPanel({ isOpen, onClose }: SelectTGPanelProps) {
  const dispatch = useAppDispatch();
  const [searchParams] = useSearchParams();
  const instanceId = searchParams.get("id") ?? "default";

  const config = useAppSelector((s) => s.scanner.config);
  const tgSelection = useAppSelector((s) => s.scanner.tgSelection);
  const avoidList = useAppSelector((s) => s.scanner.avoidList);

  const [expandedSystems, setExpandedSystems] = useState<
    Record<number, boolean>
  >({});

  const avoidSet = useMemo(
    () => new Set(avoidList.map((a) => a.talkgroupId)),
    [avoidList],
  );

  const systems = useMemo(() => config?.systems ?? [], [config]);

  // Derive unique groups across all talkgroups
  const groups = useMemo(() => {
    const groupMap = new Map<string, TalkgroupConfig[]>();
    for (const sys of systems) {
      for (const tg of sys.talkgroups ?? []) {
        const list = groupMap.get(tg.group);
        if (list) {
          list.push(tg);
        } else {
          groupMap.set(tg.group, [tg]);
        }
      }
    }
    return groupMap;
  }, [systems]);

  // Restore tgSelection from localStorage on mount / config load
  useEffect(() => {
    if (!config) return;
    const raw = localStorage.getItem(storageKey(instanceId));
    if (!raw) return;
    try {
      const saved = JSON.parse(raw) as Record<string, unknown>;
      // Build a validated selection map from saved data
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
  }, [config, instanceId, dispatch]);

  // Persist tgSelection to localStorage on change
  useEffect(() => {
    if (!config) return;
    localStorage.setItem(storageKey(instanceId), JSON.stringify(tgSelection));
  }, [tgSelection, instanceId, config]);

  // Group chip state
  const groupState = useCallback(
    (tgs: TalkgroupConfig[]): "on" | "off" | "partial" => {
      let onCount = 0;
      for (const tg of tgs) {
        if (tgSelection[tg.id] !== false) onCount++;
      }
      if (onCount === tgs.length) return "on";
      if (onCount === 0) return "off";
      return "partial";
    },
    [tgSelection],
  );

  const handleGroupToggle = useCallback(
    (tgs: TalkgroupConfig[]) => {
      const state = groupState(tgs);
      // ON → OFF, OFF → ON, PARTIAL → ON
      const enable = state !== "on";
      for (const tg of tgs) {
        const current = tgSelection[tg.id] !== false;
        if (current !== enable) {
          dispatch(toggleTG(tg.id));
        }
      }
    },
    [groupState, tgSelection, dispatch],
  );

  const toggleSystem = useCallback((sysId: number) => {
    setExpandedSystems((prev) => ({ ...prev, [sysId]: !prev[sysId] }));
  }, []);

  // Virtual scrolling for systems list
  const parentRef = useRef<HTMLDivElement>(null);
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: systems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 52, // collapsed height estimate
    overscan: 3,
  });

  const activeCount = useCallback(
    (sys: SystemConfig) => {
      let count = 0;
      for (const tg of sys.talkgroups ?? []) {
        if (tgSelection[tg.id] !== false) count++;
      }
      return count;
    },
    [tgSelection],
  );

  return (
    <>
      {/* Backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50"
          onClick={onClose}
          aria-hidden
        />
      )}

      {/* Panel */}
      <div
        className={`fixed top-0 right-0 z-50 flex h-full w-full flex-col bg-base-100 transition-transform duration-300 ease-in-out sm:w-100 ${
          isOpen ? "translate-x-0" : "translate-x-full"
        }`}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-3">
          <h2 className="text-lg font-semibold">Select Talkgroups</h2>
          <button
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            aria-label="Close"
          >
            <X size={18} />
          </button>
        </div>

        {/* Group chips + global actions */}
        <div className="border-b border-base-300 px-4 py-3">
          <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-base-content/60">
            Groups
          </div>
          <div className="flex flex-wrap gap-2">
            {[...groups.entries()].map(([name, tgs]) => {
              const state = groupState(tgs);
              const cls =
                state === "on"
                  ? "btn-primary"
                  : state === "partial"
                    ? "btn-outline btn-primary"
                    : "btn-ghost";
              return (
                <button
                  key={`group-${name}`}
                  className={`btn btn-xs ${cls}`}
                  onClick={() => handleGroupToggle(tgs)}
                >
                  {name}
                  {state === "on" && " ✔"}
                </button>
              );
            })}
          </div>
          <div className="mt-2 flex gap-2">
            <button
              className="btn btn-xs btn-outline"
              onClick={() => dispatch(setAllTGs(false))}
            >
              All Off
            </button>
            <button
              className="btn btn-xs btn-outline"
              onClick={() => dispatch(setAllTGs(true))}
            >
              All On
            </button>
          </div>
        </div>

        {/* Systems list (virtualized) */}
        <div ref={parentRef} className="flex-1 overflow-y-auto">
          <div
            className="relative w-full"
            style={{ height: virtualizer.getTotalSize() }}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const sys = systems[virtualRow.index];
              const expanded = !!expandedSystems[sys.id];
              const active = activeCount(sys);
              const total = (sys.talkgroups ?? []).length;

              return (
                <div
                  key={sys.id}
                  data-index={virtualRow.index}
                  ref={virtualizer.measureElement}
                  className="absolute left-0 w-full"
                  style={{ top: virtualRow.start }}
                >
                  {/* System header */}
                  <button
                    className="flex w-full items-center gap-2 bg-base-200 px-4 py-3 text-left hover:bg-base-300"
                    onClick={() => toggleSystem(sys.id)}
                  >
                    {expanded ? (
                      <ChevronDown size={16} />
                    ) : (
                      <ChevronRight size={16} />
                    )}
                    <span className="flex-1 font-medium">{sys.label}</span>
                    <span className="badge badge-sm">
                      {active}/{total}
                    </span>
                  </button>

                  {/* Expanded TG list */}
                  {expanded && (
                    <div className="bg-base-100 px-4 py-2">
                      <div className="flex flex-wrap gap-2">
                        {(sys.talkgroups ?? []).map((tg) => {
                          const enabled = tgSelection[tg.id] !== false;
                          const avoided = avoidSet.has(tg.talkgroupId);
                          return (
                            <button
                              key={`${sys.id}-${tg.id}-${tg.talkgroupId}`}
                              className={`btn btn-xs ${enabled ? "btn-primary" : "btn-ghost"} ${avoided ? "animate-pulse" : ""}`}
                              style={{
                                borderLeft: `6px solid ${tg.ledColor || "transparent"}`,
                              }}
                              onClick={() => dispatch(toggleTG(tg.id))}
                            >
                              {tg.label}
                              {enabled && " ✔"}
                            </button>
                          );
                        })}
                      </div>
                      <div className="mt-2 flex gap-2">
                        <button
                          className="btn btn-xs btn-outline"
                          onClick={() =>
                            dispatch(
                              setTGsBySystem({
                                systemId: sys.id,
                                enabled: false,
                              }),
                            )
                          }
                        >
                          Off
                        </button>
                        <button
                          className="btn btn-xs btn-outline"
                          onClick={() =>
                            dispatch(
                              setTGsBySystem({
                                systemId: sys.id,
                                enabled: true,
                              }),
                            )
                          }
                        >
                          On
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </>
  );
}
