import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  X,
  ChevronDown,
  ChevronRight,
  Search,
  Check,
  Minus,
} from "lucide-react";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  toggleTG,
  setAllTGs,
  setTGsBySystem,
  setTGsByGroup,
  setTGsByTag,
  restoreTGSelection,
} from "@/app/slices/scannerSlice";
import type { TalkgroupConfig } from "@/types";

interface SelectTGPanelProps {
  isOpen: boolean;
  onClose: () => void;
}

type TabId = "groups" | "tags" | "systems";

function storageKey(instanceId: string): string {
  return `openscanner-tg-selection-${instanceId}`;
}

// ---------- Section: a collapsible accordion row ----------

interface SectionProps {
  label: string;
  talkgroups: TalkgroupConfig[];
  tgSelection: Record<number, boolean>;
  expanded: boolean;
  onToggleExpand: () => void;
  onToggleAll: (enabled: boolean) => void;
  onToggleTG: (id: number) => void;
  secondaryLabels?: Record<number, string>;
}

function Section({
  label,
  talkgroups,
  tgSelection,
  expanded,
  onToggleExpand,
  onToggleAll,
  onToggleTG,
  secondaryLabels,
}: SectionProps) {
  const activeCount = talkgroups.filter(
    (tg) => tgSelection[tg.id] !== false,
  ).length;
  const total = talkgroups.length;
  const allOn = activeCount === total;
  const allOff = activeCount === 0;

  return (
    <div className="border-b border-base-300">
      {/* Header */}
      <button
        className="flex w-full items-center gap-2 px-4 py-3 text-left hover:bg-base-200 transition-colors"
        onClick={onToggleExpand}
      >
        {expanded ? (
          <ChevronDown size={16} className="shrink-0 text-base-content/50" />
        ) : (
          <ChevronRight size={16} className="shrink-0 text-base-content/50" />
        )}
        {/* Tri-state check icon */}
        <span className="shrink-0">
          {allOn ? (
            <Check size={16} className="text-primary" />
          ) : allOff ? (
            <span className="inline-block w-4 h-4 rounded border border-base-content/30" />
          ) : (
            <Minus size={16} className="text-primary" />
          )}
        </span>
        <span className="flex-1 font-medium text-sm truncate">{label}</span>
        <span className="badge badge-sm badge-ghost">
          {activeCount}/{total}
        </span>
      </button>

      {/* Expanded talkgroup list */}
      {expanded && (
        <div className="bg-base-200/40 px-4 py-2">
          <div className="flex gap-2 mb-2">
            <button
              className="btn btn-xs btn-outline"
              onClick={() => onToggleAll(true)}
            >
              All On
            </button>
            <button
              className="btn btn-xs btn-outline"
              onClick={() => onToggleAll(false)}
            >
              All Off
            </button>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-1">
            {talkgroups.map((tg) => {
              const enabled = tgSelection[tg.id] !== false;
              const secondary = secondaryLabels?.[tg.id];
              return (
                <label
                  key={tg.id}
                  className={`flex items-center gap-2 rounded px-2 py-1.5 cursor-pointer transition-colors ${
                    enabled
                      ? "bg-primary/10 hover:bg-primary/20"
                      : "hover:bg-base-300"
                  }`}
                >
                  <input
                    type="checkbox"
                    className="checkbox checkbox-xs checkbox-primary"
                    checked={enabled}
                    onChange={() => onToggleTG(tg.id)}
                  />
                  {tg.ledColor && (
                    <span
                      className="inline-block w-2 h-2 rounded-full shrink-0"
                      style={{ backgroundColor: tg.ledColor }}
                    />
                  )}
                  <span className="text-sm truncate flex-1">
                    {tg.label || tg.name}
                  </span>
                  {secondary && (
                    <span className="text-[11px] text-base-content/40 truncate max-w-24">
                      {secondary}
                    </span>
                  )}
                </label>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

// ---------- Main panel ----------

export default function SelectTGPanel({ isOpen, onClose }: SelectTGPanelProps) {
  const dispatch = useAppDispatch();
  const [searchParams] = useSearchParams();
  const instanceId = searchParams.get("id") ?? "default";

  const config = useAppSelector((s) => s.scanner.config);
  const tgSelection = useAppSelector((s) => s.scanner.tgSelection);

  const [activeTab, setActiveTab] = useState<TabId>("groups");
  const [search, setSearch] = useState("");
  const [expandedSections, setExpandedSections] = useState<
    Record<string, boolean>
  >({});

  const scrollRef = useRef<HTMLDivElement>(null);

  const systems = useMemo(() => config?.systems ?? [], [config]);
  const allTalkgroups = useMemo(
    () => systems.flatMap((s) => s.talkgroups ?? []),
    [systems],
  );

  // Build lookups for secondary labels
  const tgSystemLabel = useMemo(() => {
    const map: Record<number, string> = {};
    for (const sys of systems) {
      for (const tg of sys.talkgroups ?? []) {
        map[tg.id] = sys.label;
      }
    }
    return map;
  }, [systems]);

  const tgGroupLabel = useMemo(() => {
    const map: Record<number, string> = {};
    for (const tg of allTalkgroups) {
      map[tg.id] = tg.group;
    }
    return map;
  }, [allTalkgroups]);

  const tgTagLabel = useMemo(() => {
    const map: Record<number, string> = {};
    for (const tg of allTalkgroups) {
      map[tg.id] = tg.tag;
    }
    return map;
  }, [allTalkgroups]);

  // Derive grouped data
  const groupMap = useMemo(() => {
    const map = new Map<string, TalkgroupConfig[]>();
    for (const tg of allTalkgroups) {
      const key = tg.group || "(No Group)";
      const list = map.get(key);
      if (list) list.push(tg);
      else map.set(key, [tg]);
    }
    return new Map([...map.entries()].sort((a, b) => a[0].localeCompare(b[0])));
  }, [allTalkgroups]);

  const tagMap = useMemo(() => {
    const map = new Map<string, TalkgroupConfig[]>();
    for (const tg of allTalkgroups) {
      const key = tg.tag || "(No Tag)";
      const list = map.get(key);
      if (list) list.push(tg);
      else map.set(key, [tg]);
    }
    return new Map([...map.entries()].sort((a, b) => a[0].localeCompare(b[0])));
  }, [allTalkgroups]);

  // Search filtering
  const searchLower = search.toLowerCase();

  const matchesTG = useCallback(
    (tg: TalkgroupConfig) => {
      if (!searchLower) return true;
      return (
        tg.label.toLowerCase().includes(searchLower) ||
        tg.name.toLowerCase().includes(searchLower) ||
        tg.group.toLowerCase().includes(searchLower) ||
        tg.tag.toLowerCase().includes(searchLower) ||
        String(tg.talkgroupId).includes(searchLower)
      );
    },
    [searchLower],
  );

  const matchesSection = useCallback(
    (label: string, tgs: TalkgroupConfig[]) => {
      if (!searchLower) return true;
      if (label.toLowerCase().includes(searchLower)) return true;
      return tgs.some(matchesTG);
    },
    [searchLower, matchesTG],
  );

  const filterTGs = useCallback(
    (tgs: TalkgroupConfig[]) => {
      if (!searchLower) return tgs;
      return tgs.filter(matchesTG);
    },
    [searchLower, matchesTG],
  );

  // Restore tgSelection from localStorage on mount / config load
  useEffect(() => {
    if (!config) return;
    const raw = localStorage.getItem(storageKey(instanceId));
    if (!raw) return;
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
  }, [config, instanceId, dispatch]);

  // Persist tgSelection to localStorage on change
  useEffect(() => {
    if (!config) return;
    localStorage.setItem(storageKey(instanceId), JSON.stringify(tgSelection));
  }, [tgSelection, instanceId, config]);

  // Auto-expand sections matching search
  useEffect(() => {
    if (!searchLower) return;
    const expanded: Record<string, boolean> = {};
    const entries =
      activeTab === "groups" ? groupMap : activeTab === "tags" ? tagMap : null;
    if (entries) {
      for (const [label, tgs] of entries) {
        if (matchesSection(label, tgs)) {
          expanded[`${activeTab}-${label}`] = true;
        }
      }
    }
    if (activeTab === "systems") {
      for (const sys of systems) {
        if (matchesSection(sys.label, sys.talkgroups ?? [])) {
          expanded[`systems-${sys.id}`] = true;
        }
      }
    }
    setExpandedSections(expanded);
  }, [searchLower, activeTab, groupMap, tagMap, systems, matchesSection]);

  const toggleExpand = useCallback((key: string) => {
    setExpandedSections((prev) => ({ ...prev, [key]: !prev[key] }));
  }, []);

  // Stats
  const totalCount = allTalkgroups.length;
  const activeCount = allTalkgroups.filter(
    (tg) => tgSelection[tg.id] !== false,
  ).length;

  // Reset search when closing
  useEffect(() => {
    if (!isOpen) {
      setSearch("");
    }
  }, [isOpen]);

  // Scroll to top on tab change
  useEffect(() => {
    scrollRef.current?.scrollTo(0, 0);
  }, [activeTab]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-base-100">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-base-300 px-4 py-3">
        <h2 className="text-lg font-semibold">Select Talkgroups</h2>
        <div className="flex items-center gap-2">
          <button
            className="btn btn-xs btn-outline"
            onClick={() => dispatch(setAllTGs(true))}
          >
            All On
          </button>
          <button
            className="btn btn-xs btn-outline"
            onClick={() => dispatch(setAllTGs(false))}
          >
            All Off
          </button>
          <button
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            aria-label="Close"
          >
            <X size={18} />
          </button>
        </div>
      </div>

      {/* Search bar + stats */}
      <div className="flex items-center gap-3 border-b border-base-300 px-4 py-2">
        <div className="relative flex-1">
          <Search
            size={16}
            className="absolute left-2.5 top-1/2 -translate-y-1/2 text-base-content/40"
          />
          <input
            type="text"
            className="input input-sm w-full pl-8"
            placeholder="Search talkgroups..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            autoFocus
          />
        </div>
        <span className="text-sm text-base-content/60 whitespace-nowrap">
          {activeCount}/{totalCount} active
        </span>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-base-300">
        {(["groups", "tags", "systems"] as TabId[]).map((tab) => (
          <button
            key={tab}
            className={`flex-1 px-4 py-2 text-sm font-medium transition-colors ${
              activeTab === tab
                ? "border-b-2 border-primary text-primary"
                : "text-base-content/60 hover:text-base-content"
            }`}
            onClick={() => setActiveTab(tab)}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
          </button>
        ))}
      </div>

      {/* Content */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto">
        {activeTab === "groups" &&
          [...groupMap.entries()]
            .filter(([label, tgs]) => matchesSection(label, tgs))
            .map(([group, tgs]) => {
              const key = `groups-${group}`;
              return (
                <Section
                  key={key}
                  label={group}
                  talkgroups={filterTGs(tgs)}
                  tgSelection={tgSelection}
                  expanded={!!expandedSections[key]}
                  onToggleExpand={() => toggleExpand(key)}
                  onToggleAll={(enabled) =>
                    dispatch(setTGsByGroup({ group, enabled }))
                  }
                  onToggleTG={(id) => dispatch(toggleTG(id))}
                  secondaryLabels={tgSystemLabel}
                />
              );
            })}

        {activeTab === "tags" &&
          [...tagMap.entries()]
            .filter(([label, tgs]) => matchesSection(label, tgs))
            .map(([tag, tgs]) => {
              const key = `tags-${tag}`;
              return (
                <Section
                  key={key}
                  label={tag}
                  talkgroups={filterTGs(tgs)}
                  tgSelection={tgSelection}
                  expanded={!!expandedSections[key]}
                  onToggleExpand={() => toggleExpand(key)}
                  onToggleAll={(enabled) =>
                    dispatch(setTGsByTag({ tag, enabled }))
                  }
                  onToggleTG={(id) => dispatch(toggleTG(id))}
                  secondaryLabels={tgGroupLabel}
                />
              );
            })}

        {activeTab === "systems" &&
          systems
            .filter((sys) => matchesSection(sys.label, sys.talkgroups ?? []))
            .map((sys) => {
              const key = `systems-${sys.id}`;
              return (
                <Section
                  key={key}
                  label={sys.label}
                  talkgroups={filterTGs(sys.talkgroups ?? [])}
                  tgSelection={tgSelection}
                  expanded={!!expandedSections[key]}
                  onToggleExpand={() => toggleExpand(key)}
                  onToggleAll={(enabled) =>
                    dispatch(setTGsBySystem({ systemId: sys.id, enabled }))
                  }
                  onToggleTG={(id) => dispatch(toggleTG(id))}
                  secondaryLabels={tgTagLabel}
                />
              );
            })}
      </div>
    </div>
  );
}
