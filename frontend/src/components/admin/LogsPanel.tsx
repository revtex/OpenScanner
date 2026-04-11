import { useState, useRef, useCallback } from "react";
import { Search } from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useGetLogsQuery } from "@/app/slices/adminSlice";

const LEVELS = ["", "debug", "info", "warn", "error"] as const;

const levelBadge: Record<string, string> = {
  debug: "badge badge-ghost",
  info: "badge badge-info",
  warn: "badge badge-warning",
  error: "badge badge-error",
};

export default function LogsPanel() {
  const [level, setLevel] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [queryParams, setQueryParams] = useState<{
    from?: number;
    to?: number;
    level?: string;
  }>({});

  const { data: logs, isLoading } = useGetLogsQuery(queryParams);
  const [toast, setToast] = useState<string | null>(null);
  const parentRef = useRef<HTMLDivElement>(null);

  const showToast = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  }, []);

  // suppress unused-var lint — showToast is used in future error paths
  void showToast;

  const handleFilter = () => {
    const params: { from?: number; to?: number; level?: string } = {};
    if (fromDate) params.from = Math.floor(new Date(fromDate).getTime() / 1000);
    if (toDate) params.to = Math.floor(new Date(toDate).getTime() / 1000);
    if (level) params.level = level;
    setQueryParams(params);
  };

  const rows = logs ?? [];

  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 40,
    overscan: 20,
  });

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Logs</h1>
      <p className="text-sm text-base-content/70 mb-4">
        View server logs filtered by date range and severity level. Use this to
        diagnose issues with call ingestion, audio processing, transcription, or
        downstream forwarding.
      </p>
      <div className="card bg-base-200">
        <div className="card-body">
          {/* Filter bar */}
          <div className="flex flex-wrap items-end gap-3 mb-4">
            <label className="form-control">
              <div className="label">
                <span className="label-text">Level</span>
              </div>
              <select
                className="select select-bordered select-sm"
                value={level}
                onChange={(e) => setLevel(e.target.value)}
              >
                {LEVELS.map((l) => (
                  <option key={l} value={l}>
                    {l || "All"}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-control">
              <div className="label">
                <span className="label-text">From</span>
              </div>
              <input
                type="datetime-local"
                className="input input-bordered input-sm"
                value={fromDate}
                onChange={(e) => setFromDate(e.target.value)}
              />
            </label>
            <label className="form-control">
              <div className="label">
                <span className="label-text">To</span>
              </div>
              <input
                type="datetime-local"
                className="input input-bordered input-sm"
                value={toDate}
                onChange={(e) => setToDate(e.target.value)}
              />
            </label>
            <button className="btn btn-primary btn-sm" onClick={handleFilter}>
              <Search className="w-4 h-4" /> Filter
            </button>
          </div>

          {isLoading ? (
            <div className="loading loading-spinner loading-md" />
          ) : (
            <>
              {/* Table header */}
              <div className="overflow-x-auto">
                <table className="table table-zebra w-full">
                  <thead>
                    <tr>
                      <th className="w-48">Date/Time</th>
                      <th className="w-24">Level</th>
                      <th>Message</th>
                    </tr>
                  </thead>
                </table>
              </div>

              {/* Virtualized rows */}
              <div ref={parentRef} className="overflow-auto h-[600px]">
                <div
                  style={{
                    height: `${virtualizer.getTotalSize()}px`,
                    width: "100%",
                    position: "relative",
                  }}
                >
                  {virtualizer.getVirtualItems().map((virtualRow) => {
                    const log = rows[virtualRow.index];
                    return (
                      <div
                        key={log.id}
                        style={{
                          position: "absolute",
                          top: 0,
                          left: 0,
                          width: "100%",
                          height: `${virtualRow.size}px`,
                          transform: `translateY(${virtualRow.start}px)`,
                        }}
                        className="flex items-center border-b border-base-300 text-sm"
                      >
                        <span className="w-48 shrink-0 px-4">
                          {new Date(log.dateTime * 1000).toLocaleString()}
                        </span>
                        <span className="w-24 shrink-0 px-4">
                          <span
                            className={
                              levelBadge[log.level] ?? "badge badge-ghost"
                            }
                          >
                            {log.level}
                          </span>
                        </span>
                        <span className="flex-1 px-4 truncate">
                          {log.message}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>

              {rows.length === 0 && (
                <div className="text-center opacity-60 py-4">
                  No log entries found
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {toast && (
        <div className="toast toast-end">
          <div className="alert alert-error">
            <span>{toast}</span>
          </div>
        </div>
      )}
    </div>
  );
}
