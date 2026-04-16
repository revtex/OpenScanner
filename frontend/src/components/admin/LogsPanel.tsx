import { useMemo, useState, useRef, useCallback, useEffect } from "react";
import {
  Clock3,
  RefreshCw,
  Search,
  ChevronDown,
  ChevronRight,
  ArrowDown,
  Filter,
  X,
} from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useGetLogsQuery, useGetLogLevelQuery } from "@/app/slices/adminSlice";
import type { AdminLog } from "@/types";

// ─── Constants ──────────────────────────────────────────────

const LEVELS = ["", "debug", "info", "warn", "error"] as const;
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

interface ParsedLog {
  isRequest: boolean;
  method?: string;
  path?: string;
  status?: number;
  latencyMs?: number;
  summary: string;
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

  return { isRequest: false, summary: log.message };
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
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const [autoScroll, setAutoScroll] = useState(false);

  const [queryParams, setQueryParams] = useState<{
    from?: number;
    to?: number;
    level?: string;
    q?: string;
    limit?: number;
  }>({ limit: 500 });

  const {
    data: logs,
    isLoading,
    isFetching,
    refetch,
  } = useGetLogsQuery(queryParams, {
    pollingInterval: autoRefresh ? 5000 : 0,
    refetchOnFocus: true,
  });

  const { data: logLevelData } = useGetLogLevelQuery(undefined, {
    pollingInterval: 30000,
  });

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

  const toggleRow = useCallback((idx: number) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  }, []);

  const rows = logs ?? [];

  const counts = useMemo(() => {
    const m = { debug: 0, info: 0, warn: 0, error: 0 };
    for (const r of rows) {
      const k = r.level?.toLowerCase() as keyof typeof m;
      if (k in m) m[k] += 1;
    }
    return m;
  }, [rows]);

  const hasActiveFilters = !!(level || fromDate || toDate || textQuery);
  const runtimeLevel = logLevelData?.level ?? "info";

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (idx) => (expandedRows.has(idx) ? 120 : 44),
    overscan: 30,
  });

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div>
          <h1 className="text-xl font-semibold">Logs</h1>
          <p className="text-sm text-base-content/60 mt-0.5">
            Live server logs &mdash; {rows.length} entries in buffer
          </p>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="flex items-center gap-1 text-xs text-base-content/60 mr-2">
            <span>Level:</span>
            <span
              className={`badge badge-xs ${LEVEL_COLORS[runtimeLevel]?.badge ?? ""}`}
            >
              {runtimeLevel.toUpperCase()}
            </span>
          </div>
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

            <label className="flex flex-col text-sm flex-1 min-w-48">
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
              const expanded = expandedRows.has(virtualRow.index);
              const levelStyle = LEVEL_COLORS[log.level] ?? LEVEL_COLORS.debug;
              const hasAttrs = log.attrs && Object.keys(log.attrs).length > 0;

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
                    onClick={() => hasAttrs && toggleRow(virtualRow.index)}
                  >
                    <span className="w-4 shrink-0">
                      {hasAttrs &&
                        (expanded ? (
                          <ChevronDown className="w-3.5 h-3.5 text-base-content/40" />
                        ) : (
                          <ChevronRight className="w-3.5 h-3.5 text-base-content/40" />
                        ))}
                    </span>

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

                  {/* Expanded details */}
                  {expanded && log.attrs && (
                    <div className="px-10 pb-2">
                      <div className="rounded bg-base-200 p-2 text-xs font-mono grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5">
                        {Object.entries(log.attrs).map(([k, v]) => (
                          <div key={k} className="contents">
                            <span className="text-base-content/50">{k}</span>
                            <span className="text-base-content/80 break-all">
                              {v}
                            </span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
