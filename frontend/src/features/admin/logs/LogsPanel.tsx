import { useMemo, useState, useRef, useCallback, useEffect } from "react";
import {
  Clock3,
  RefreshCw,
  Search,
  ArrowDown,
  Filter,
  SlidersHorizontal,
  X,
} from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useUpdateConfigMutation } from "@/features/admin/_shell";
import { useAdminLogs, useAdminLogLevel } from "./useAdminLogs";
import type { AdminLog } from "@/types";

// ─── Constants ──────────────────────────────────────────────

const LEVELS = ["", "debug", "info", "warn", "error"] as const;
const LOG_LEVELS = ["debug", "info", "warn", "error"] as const;
const LIMIT_OPTIONS = [200, 500, 1000, 2500, 5000] as const;

const LEVEL_COLORS: Record<string, { badge: string; dot: string }> = {
  debug: {
    badge: "bg-base-300 text-base-content/70",
    dot: "bg-base-content/30",
  },
  info: {
    badge: "bg-info/15 text-info border border-info/20",
    dot: "bg-info",
  },
  warn: {
    badge: "bg-warning/15 text-warning border border-warning/20",
    dot: "bg-warning",
  },
  error: {
    badge: "bg-error/15 text-error border border-error/20",
    dot: "bg-error",
  },
};

const METHOD_COLORS: Record<string, string> = {
  GET: "text-info",
  POST: "text-success",
  PUT: "text-warning",
  PATCH: "text-warning",
  DELETE: "text-error",
};

// ─── Log parsing ────────────────────────────────────────────

type ChipTone = "default" | "error" | "muted";

interface LogChip {
  key: string;
  value: string;
  tone?: ChipTone;
}

interface ParsedLog {
  isRequest: boolean;
  method?: string;
  path?: string;
  status?: number;
  latencyMs?: number;
  summary: string;
  chips?: LogChip[];
}

const SHORT_KEYS: Record<string, string> = {
  call_id: "call",
  system_id: "sys",
  talkgroup_id: "tg",
  user_id: "user",
  downstream_id: "ds",
  duration_ms: "dur",
  segments: "seg",
  language: "lang",
  attempt: "try",
  workers: "workers",
  username: "user",
};

function shortKey(k: string): string {
  return SHORT_KEYS[k] ?? k;
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}

function chipFrom(
  attrs: Record<string, string>,
  attrKey: string,
  opts: { label?: string; tone?: ChipTone; max?: number } = {},
): LogChip | null {
  const raw = attrs[attrKey];
  if (raw === undefined || raw === "") return null;
  const value = opts.max ? truncate(raw, opts.max) : raw;
  return { key: opts.label ?? shortKey(attrKey), value, tone: opts.tone };
}

function compact(chips: Array<LogChip | null>): LogChip[] {
  return chips.filter((c): c is LogChip => c !== null);
}

function buildKnownChips(
  message: string,
  attrs: Record<string, string>,
): LogChip[] | null {
  switch (message) {
    case "dirmonitor: call ingested":
      // dirmonitor uses `id` for the call id (not `call_id`).
      return compact([
        chipFrom(attrs, "id", { label: "call" }),
        chipFrom(attrs, "system_id"),
        chipFrom(attrs, "talkgroup_id"),
        chipFrom(attrs, "duration_ms"),
      ]);
    case "call ingested":
      return compact([
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "system_id"),
        chipFrom(attrs, "talkgroup_id"),
        chipFrom(attrs, "duration_ms"),
      ]);
    case "call-upload: complete":
      return compact([
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "system_id"),
        chipFrom(attrs, "talkgroup_id"),
        chipFrom(attrs, "duration_ms"),
      ]);
    case "transcription stored":
      return compact([
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "language"),
        chipFrom(attrs, "segments"),
      ]);
    case "transcription failed":
      return compact([
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "error", { tone: "error", max: 60 }),
      ]);
    case "user logged in":
      return compact([
        chipFrom(attrs, "user_id"),
        chipFrom(attrs, "username"),
        chipFrom(attrs, "ip"),
      ]);
    case "downstream: call pushed successfully":
      return compact([
        chipFrom(attrs, "downstream_id"),
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "status"),
      ]);
    case "downstream: push failed":
      return compact([
        chipFrom(attrs, "downstream_id"),
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "attempt"),
        chipFrom(attrs, "error", { tone: "error", max: 60 }),
      ]);
    case "downstream: giving up after max retries":
      return compact([
        chipFrom(attrs, "downstream_id"),
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "attempt"),
      ]);
    case "dirmonitor: auto-populated system":
    case "auto-populated system":
      return compact([chipFrom(attrs, "system_id"), chipFrom(attrs, "label")]);
    case "dirmonitor: auto-populated talkgroup":
    case "auto-populated talkgroup":
      return compact([
        chipFrom(attrs, "system_id"),
        chipFrom(attrs, "talkgroup_id"),
        chipFrom(attrs, "label"),
      ]);
    case "dirmonitor: duplicate call rejected":
    case "duplicate call rejected":
      return compact([
        chipFrom(attrs, "call_id"),
        chipFrom(attrs, "system_id"),
        chipFrom(attrs, "talkgroup_id"),
      ]);
    case "transcriber pool started":
      return compact([
        chipFrom(attrs, "workers"),
        chipFrom(attrs, "model"),
        chipFrom(attrs, "base_url", { label: "url", max: 40 }),
      ]);
    case "transcription enabled":
      return compact([
        chipFrom(attrs, "model"),
        chipFrom(attrs, "url", { max: 40 }),
      ]);
    case "transcription disabled":
      return [];
    default:
      return null;
  }
}

const FALLBACK_PRIORITY = [
  "call_id",
  "user_id",
  "system_id",
  "talkgroup_id",
  "downstream_id",
  "username",
  "error",
];

function fallbackChips(attrs: Record<string, string>): LogChip[] {
  const picked = new Set<string>();
  const chips: LogChip[] = [];
  for (const k of FALLBACK_PRIORITY) {
    if (chips.length >= 3) break;
    const c = chipFrom(attrs, k, {
      tone: k === "error" ? "error" : "default",
      max: k === "error" ? 60 : undefined,
    });
    if (c) {
      chips.push(c);
      picked.add(k);
    }
  }
  if (chips.length === 0) {
    for (const [k, v] of Object.entries(attrs)) {
      if (chips.length >= 2) break;
      if (v === "" || v === undefined) continue;
      if (picked.has(k)) continue;
      chips.push({ key: shortKey(k), value: truncate(v, 40) });
    }
  }
  return chips;
}

function parseLog(log: AdminLog): ParsedLog {
  const attrs = log.attrs ?? {};

  if (log.message === "request" && attrs.method && attrs.path) {
    return {
      isRequest: true,
      method: attrs.method,
      path: attrs.path,
      status: attrs.status ? Number(attrs.status) : undefined,
      latencyMs: attrs.latency_ms ? Number(attrs.latency_ms) : undefined,
      summary: `${attrs.method} ${attrs.path}`,
    };
  }

  const known = buildKnownChips(log.message, attrs);
  const chipsRaw =
    known ?? (Object.keys(attrs).length > 0 ? fallbackChips(attrs) : undefined);
  const chips = chipsRaw && chipsRaw.length > 0 ? chipsRaw : undefined;

  return { isRequest: false, summary: log.message, chips };
}

function statusClass(status?: number): string {
  if (!status) return "";
  if (status >= 500) return "text-error font-semibold";
  if (status >= 400) return "text-warning font-semibold";
  if (status >= 300) return "text-base-content/60";
  return "text-success";
}

// ─── Component ──────────────────────────────────────────────

export default function LogsPanel() {
  const [level, setLevel] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [textQuery, setTextQuery] = useState("");
  const [limit, setLimit] = useState<number>(500);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [showFilters, setShowFilters] = useState(false);
  const [showRuntimeLevel, setShowRuntimeLevel] = useState(false);
  const [selectedLog, setSelectedLog] = useState<AdminLog | null>(null);
  const [autoScroll, setAutoScroll] = useState(false);
  const [savingLevel, setSavingLevel] = useState(false);
  const [levelSaveError, setLevelSaveError] = useState<string | null>(null);
  const [levelToast, setLevelToast] = useState<string | null>(null);

  const [queryParams, setQueryParams] = useState<{
    from?: number;
    to?: number;
    level?: string;
    q?: string;
    limit?: number;
  }>({ limit: 500 });

  const {
    logs: logsData,
    isLoading,
    isFetching,
    refetch,
  } = useAdminLogs(queryParams, autoRefresh);
  const logs = logsData;

  const { level: runtimeLevelFromWs, refetch: refetchLevel } =
    useAdminLogLevel();
  const [updateConfig] = useUpdateConfigMutation();

  const parentRef = useRef<HTMLDivElement>(null);
  const prevCountRef = useRef(0);

  useEffect(() => {
    if (!autoScroll || !logs) return;
    if (logs.length > prevCountRef.current && parentRef.current) {
      parentRef.current.scrollTop = parentRef.current.scrollHeight;
    }
    prevCountRef.current = logs.length;
  }, [logs, autoScroll]);

  const handleFilter = useCallback(() => {
    const params: typeof queryParams = { limit };
    if (fromDate) params.from = Math.floor(new Date(fromDate).getTime() / 1000);
    if (toDate) params.to = Math.floor(new Date(toDate).getTime() / 1000);
    if (level) params.level = level;
    const q = textQuery.trim();
    if (q) params.q = q;
    setQueryParams(params);
  }, [fromDate, toDate, level, textQuery, limit]);

  const clearFilters = useCallback(() => {
    setLevel("");
    setFromDate("");
    setToDate("");
    setTextQuery("");
    setLimit(500);
    setShowFilters(false);
    setQueryParams({ limit: 500 });
  }, []);

  const setQuickRange = useCallback((hours: number) => {
    const now = new Date();
    const from = new Date(now.getTime() - hours * 60 * 60 * 1000);
    const fmt = (dt: Date) => {
      const off = dt.getTimezoneOffset();
      const local = new Date(dt.getTime() - off * 60 * 1000);
      return local.toISOString().slice(0, 16);
    };
    setFromDate(fmt(from));
    setToDate(fmt(now));
  }, []);

  const rows = useMemo(() => logs ?? [], [logs]);

  const counts = useMemo(() => {
    const m = { debug: 0, info: 0, warn: 0, error: 0 };
    for (const r of rows) {
      const k = r.level?.toLowerCase() as keyof typeof m;
      if (k in m) m[k] += 1;
    }
    return m;
  }, [rows]);

  const hasActiveFilters = !!(level || fromDate || toDate || textQuery);
  const runtimeLevel = runtimeLevelFromWs ?? "info";
  const saveRuntimeLevel = useCallback(
    async (nextLevel: string) => {
      if (nextLevel === runtimeLevel) return;
      setSavingLevel(true);
      setLevelSaveError(null);
      try {
        await updateConfig([{ key: "logLevel", value: nextLevel }]).unwrap();
        setLevelToast("Log level updated");
        setTimeout(() => setLevelToast(null), 2500);
        void refetchLevel();
        setShowRuntimeLevel(false);
      } catch {
        setLevelSaveError("Failed to update log level");
      } finally {
        setSavingLevel(false);
      }
    },
    [runtimeLevel, updateConfig, refetchLevel],
  );

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 44,
    overscan: 30,
  });

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
        <div>
          <h1 className="text-xl font-semibold">Logs</h1>
          <p className="text-sm text-base-content/60 mt-0.5">
            Live server logs &mdash; {rows.length} entries in buffer
          </p>
        </div>
        <div className="flex items-center gap-1.5 flex-wrap">
          <div className="flex items-center gap-1 text-xs text-base-content/60 mr-2">
            <span>Level:</span>
            <span
              className={`badge badge-xs ${LEVEL_COLORS[runtimeLevel]?.badge ?? ""}`}
            >
              {runtimeLevel.toUpperCase()}
            </span>
          </div>
          <button
            className={`btn btn-sm btn-ghost ${showRuntimeLevel ? "text-primary" : ""}`}
            onClick={() => setShowRuntimeLevel((p) => !p)}
            title="Runtime log level"
          >
            <SlidersHorizontal className="w-4 h-4" />
          </button>
          <button
            className={`btn btn-sm btn-ghost ${hasActiveFilters ? "text-primary" : ""}`}
            onClick={() => setShowFilters((p) => !p)}
            title="Toggle filters"
          >
            <Filter className="w-4 h-4" />
            {hasActiveFilters && (
              <span className="badge badge-xs badge-primary">!</span>
            )}
          </button>
          <button
            className="btn btn-sm btn-ghost"
            onClick={() => void refetch()}
            title="Refresh now"
          >
            <RefreshCw
              className={`w-4 h-4 ${isFetching ? "animate-spin" : ""}`}
            />
          </button>
          <button
            className={`btn btn-sm btn-ghost ${autoScroll ? "text-primary" : ""}`}
            onClick={() => setAutoScroll((p) => !p)}
            title="Auto-scroll to latest"
          >
            <ArrowDown className="w-4 h-4" />
          </button>
          <label className="flex items-center gap-1.5 text-sm cursor-pointer">
            <input
              type="checkbox"
              className="toggle toggle-xs toggle-primary"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            <span className="text-xs">Live</span>
          </label>
        </div>
      </div>

      {/* Runtime log level popdown */}
      {showRuntimeLevel && (
        <div className="rounded-lg border border-base-300 bg-base-200/50 p-3 mb-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-medium text-base-content/70 mr-1">
              Runtime log level
            </span>
            {LOG_LEVELS.map((l) => (
              <button
                key={l}
                className={`badge gap-1 cursor-pointer transition-opacity ${
                  LEVEL_COLORS[l].badge
                } ${runtimeLevel === l ? "ring-1 ring-primary" : "opacity-80"}`}
                disabled={savingLevel || runtimeLevel === l}
                onClick={() => void saveRuntimeLevel(l)}
              >
                <span
                  className={`w-1.5 h-1.5 rounded-full ${LEVEL_COLORS[l].dot}`}
                />
                {l.toUpperCase()}
              </button>
            ))}
            <div className="flex-1" />
            <span className="text-xs text-base-content/50">
              Higher verbosity increases log volume.
            </span>
          </div>
          {savingLevel && (
            <p className="text-xs text-base-content/60 mt-2">
              Updating log level...
            </p>
          )}
          {levelSaveError && (
            <p className="text-xs text-error mt-2">{levelSaveError}</p>
          )}
          {levelToast && (
            <p className="text-xs text-success mt-2">{levelToast}</p>
          )}
        </div>
      )}

      {/* Level summary bar */}
      <div className="flex flex-wrap items-center gap-2 mb-3">
        {(["debug", "info", "warn", "error"] as const).map((l) => (
          <button
            key={l}
            className={`badge gap-1 cursor-pointer transition-opacity ${
              LEVEL_COLORS[l].badge
            } ${level && level !== l ? "opacity-40" : ""}`}
            onClick={() => {
              const next = level === l ? "" : l;
              setLevel(next);
              const params: typeof queryParams = { limit };
              if (fromDate)
                params.from = Math.floor(new Date(fromDate).getTime() / 1000);
              if (toDate)
                params.to = Math.floor(new Date(toDate).getTime() / 1000);
              if (next) params.level = next;
              const q = textQuery.trim();
              if (q) params.q = q;
              setQueryParams(params);
            }}
          >
            <span
              className={`w-1.5 h-1.5 rounded-full ${LEVEL_COLORS[l].dot}`}
            />
            {l.toUpperCase()} {counts[l]}
          </button>
        ))}
        <div className="flex-1" />
        <div className="flex gap-1">
          {[1, 24, 168].map((h) => (
            <button
              key={h}
              className="btn btn-ghost btn-xs"
              onClick={() => {
                setQuickRange(h);
                setShowFilters(true);
              }}
            >
              <Clock3 className="w-3 h-3" />
              {h < 24 ? `${h}h` : `${h / 24}d`}
            </button>
          ))}
        </div>
      </div>

      {/* Expanded filter panel */}
      {showFilters && (
        <div className="rounded-lg border border-base-300 bg-base-200/50 p-3 mb-3">
          <div className="flex flex-wrap items-end gap-3">
            <label className="flex flex-col text-sm">
              <span className="mb-0.5 text-xs font-medium text-base-content/70">
                Level
              </span>
              <select
                className="select select-sm select-bordered"
                value={level}
                onChange={(e) => setLevel(e.target.value)}
              >
                {LEVELS.map((l) => (
                  <option key={l} value={l}>
                    {l || "All levels"}
                  </option>
                ))}
              </select>
            </label>

            <label className="flex flex-col text-sm flex-1 min-w-0">
              <span className="mb-0.5 text-xs font-medium text-base-content/70">
                Search
              </span>
              <input
                type="text"
                className="input input-sm input-bordered"
                value={textQuery}
                placeholder="Filter by message or attributes..."
                onChange={(e) => setTextQuery(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleFilter()}
              />
            </label>

            <label className="flex flex-col text-sm">
              <span className="mb-0.5 text-xs font-medium text-base-content/70">
                From
              </span>
              <input
                type="datetime-local"
                className="input input-sm input-bordered"
                value={fromDate}
                onChange={(e) => setFromDate(e.target.value)}
              />
            </label>

            <label className="flex flex-col text-sm">
              <span className="mb-0.5 text-xs font-medium text-base-content/70">
                To
              </span>
              <input
                type="datetime-local"
                className="input input-sm input-bordered"
                value={toDate}
                onChange={(e) => setToDate(e.target.value)}
              />
            </label>

            <label className="flex flex-col text-sm">
              <span className="mb-0.5 text-xs font-medium text-base-content/70">
                Limit
              </span>
              <select
                className="select select-sm select-bordered"
                value={String(limit)}
                onChange={(e) => setLimit(Number(e.target.value))}
              >
                {LIMIT_OPTIONS.map((o) => (
                  <option key={o} value={o}>
                    {o}
                  </option>
                ))}
              </select>
            </label>

            <button className="btn btn-primary btn-sm" onClick={handleFilter}>
              <Search className="w-4 h-4" /> Apply
            </button>
            {hasActiveFilters && (
              <button className="btn btn-ghost btn-sm" onClick={clearFilters}>
                <X className="w-4 h-4" /> Clear
              </button>
            )}
          </div>
        </div>
      )}

      {/* Log entries */}
      {isLoading ? (
        <div className="flex justify-center py-12">
          <span className="loading loading-spinner loading-lg" />
        </div>
      ) : rows.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-base-content/50">
          <Search className="w-10 h-10 mb-2 opacity-30" />
          <p className="text-sm">No log entries found</p>
          {hasActiveFilters && (
            <button
              className="btn btn-ghost btn-sm mt-2"
              onClick={clearFilters}
            >
              Clear filters
            </button>
          )}
        </div>
      ) : (
        <div
          ref={parentRef}
          className="flex-1 overflow-auto rounded-lg border border-base-300 bg-base-100 min-h-0"
          style={{ maxHeight: "calc(100vh - 340px)" }}
        >
          <div
            style={{
              height: `${virtualizer.getTotalSize()}px`,
              width: "100%",
              position: "relative",
            }}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const log = rows[virtualRow.index];
              const parsed = parseLog(log);
              const levelStyle = LEVEL_COLORS[log.level] ?? LEVEL_COLORS.debug;

              return (
                <div
                  key={virtualRow.index}
                  data-index={virtualRow.index}
                  ref={virtualizer.measureElement}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                  className={`border-b border-base-200 hover:bg-base-200/50 transition-colors ${
                    log.level === "error" ? "bg-error/5" : ""
                  } ${log.level === "warn" ? "bg-warning/5" : ""}`}
                >
                  {/* Main row */}
                  <div
                    className="flex items-center gap-2 px-3 py-2 cursor-pointer select-none"
                    onClick={() => setSelectedLog(log)}
                  >
                    <span className="w-4 shrink-0" />

                    <span
                      className={`w-2 h-2 rounded-full shrink-0 ${levelStyle.dot}`}
                      title={log.level}
                    />

                    <span className="text-xs text-base-content/50 font-mono w-20 shrink-0">
                      {new Date(log.dateTime * 1000).toLocaleTimeString()}
                    </span>

                    <span
                      className={`text-[10px] font-semibold uppercase w-12 text-center rounded px-1 py-0.5 shrink-0 ${levelStyle.badge}`}
                    >
                      {log.level}
                    </span>

                    <div className="flex-1 min-w-0 flex items-center gap-2">
                      {parsed.isRequest && parsed.method && (
                        <span
                          className={`text-xs font-bold shrink-0 ${
                            METHOD_COLORS[parsed.method] ??
                            "text-base-content/60"
                          }`}
                        >
                          {parsed.method}
                        </span>
                      )}
                      <span className="text-sm truncate">
                        {parsed.isRequest ? parsed.path : parsed.summary}
                      </span>
                      {!parsed.isRequest &&
                        parsed.chips &&
                        parsed.chips.length > 0 && (
                          <div className="flex items-center gap-1 shrink min-w-0 overflow-hidden">
                            {parsed.chips.map((chip) => (
                              <span
                                key={chip.key}
                                className={`font-mono text-[10px] leading-4 px-1.5 py-0.5 rounded bg-base-200/60 border border-base-300/60 whitespace-nowrap ${
                                  chip.tone === "error"
                                    ? "text-error"
                                    : "text-base-content/60"
                                }`}
                                title={`${chip.key}=${chip.value}`}
                              >
                                <span className="opacity-60">{chip.key}=</span>
                                <span className="text-base-content/80">
                                  {chip.value}
                                </span>
                              </span>
                            ))}
                          </div>
                        )}
                      {parsed.isRequest && parsed.status != null && (
                        <span
                          className={`text-xs font-mono shrink-0 ${statusClass(parsed.status)}`}
                        >
                          {parsed.status}
                        </span>
                      )}
                      {parsed.isRequest && parsed.latencyMs != null && (
                        <span className="text-xs text-base-content/40 shrink-0">
                          {parsed.latencyMs}ms
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Log details popup */}
      {selectedLog && (
        <div
          className="fixed inset-0 z-50 bg-black/50 flex items-center justify-center p-4"
          onClick={() => setSelectedLog(null)}
        >
          <div
            className="w-full max-w-3xl max-h-[80vh] overflow-auto rounded-lg border border-base-300 bg-base-100 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-4 py-3 border-b border-base-300">
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold">Log Details</span>
                <span
                  className={`text-[10px] font-semibold uppercase rounded px-1.5 py-0.5 ${
                    LEVEL_COLORS[selectedLog.level]?.badge ??
                    LEVEL_COLORS.debug.badge
                  }`}
                >
                  {selectedLog.level}
                </span>
                <span className="text-xs text-base-content/50 font-mono">
                  {new Date(selectedLog.dateTime * 1000).toLocaleString()}
                </span>
              </div>
              <button
                className="btn btn-xs btn-ghost"
                onClick={() => setSelectedLog(null)}
                title="Close"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="p-4 space-y-3">
              <div>
                <p className="text-xs font-medium text-base-content/60 mb-1">
                  Message
                </p>
                <p className="text-sm wrap-break-word">{selectedLog.message}</p>
              </div>

              <div>
                <p className="text-xs font-medium text-base-content/60 mb-1">
                  Attributes
                </p>
                {selectedLog.attrs &&
                Object.keys(selectedLog.attrs).length > 0 ? (
                  <div className="rounded bg-base-200 p-3 text-xs font-mono grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
                    {Object.entries(selectedLog.attrs).map(([k, v]) => (
                      <div key={k} className="contents">
                        <span className="text-base-content/50">{k}</span>
                        <span className="text-base-content/80 break-all">
                          {v}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-base-content/50">
                    No attributes for this entry.
                  </p>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
