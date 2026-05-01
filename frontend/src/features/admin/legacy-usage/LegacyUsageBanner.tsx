import { useState } from "react";
import { AlertTriangle, X } from "lucide-react";
import { useGetLegacyUsageQuery } from "@/app/api";
import type { LegacyUsageEntry } from "@/types";

const DISMISS_KEY = "os.legacyUsageBanner.dismissed";
const POLL_INTERVAL_MS = 60_000;

function formatRelative(iso: string, now: number = Date.now()): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return iso;
  const deltaSec = Math.max(0, Math.round((now - t) / 1000));
  if (deltaSec < 60) return `${deltaSec}s ago`;
  const deltaMin = Math.round(deltaSec / 60);
  if (deltaMin < 60)
    return `${deltaMin} minute${deltaMin === 1 ? "" : "s"} ago`;
  const deltaHr = Math.round(deltaMin / 60);
  if (deltaHr < 24) return `${deltaHr} hour${deltaHr === 1 ? "" : "s"} ago`;
  const deltaDay = Math.round(deltaHr / 24);
  return `${deltaDay} day${deltaDay === 1 ? "" : "s"} ago`;
}

function readDismissed(): boolean {
  try {
    return sessionStorage.getItem(DISMISS_KEY) === "1";
  } catch {
    return false;
  }
}

function writeDismissed(): void {
  try {
    sessionStorage.setItem(DISMISS_KEY, "1");
  } catch {
    // sessionStorage may be disabled — best-effort.
  }
}

export default function LegacyUsageBanner() {
  const [dismissed, setDismissed] = useState<boolean>(() => readDismissed());

  const { data, isLoading, isError } = useGetLegacyUsageQuery(undefined, {
    pollingInterval: POLL_INTERVAL_MS,
    refetchOnFocus: true,
    refetchOnMountOrArgChange: true,
  });

  if (dismissed) return null;
  if (isLoading || isError) return null;

  const entries: LegacyUsageEntry[] = data?.entries ?? [];
  if (entries.length === 0) return null;

  const totalRequests = entries.reduce((sum, e) => sum + e.count, 0);
  const distinctKeys = new Set(
    entries.map((e) => e.apiKeyIdent || "(unauthenticated)"),
  ).size;

  const handleDismiss = () => {
    writeDismissed();
    setDismissed(true);
  };

  return (
    <div role="alert" className="alert alert-warning mb-4 items-start">
      <AlertTriangle className="w-5 h-5 mt-0.5 shrink-0" />
      <div className="flex-1 min-w-0">
        <h3 className="font-bold">Legacy API in use</h3>
        <p className="text-sm">
          {totalRequests} request{totalRequests === 1 ? "" : "s"} across{" "}
          {distinctKeys} API key{distinctKeys === 1 ? "" : "s"} in the last 24h
          to deprecated <code className="font-mono">/api/*</code> endpoints.
          Migrate to <code className="font-mono">/api/v1/*</code>.
        </p>
        <details className="mt-2">
          <summary className="cursor-pointer text-sm font-semibold">
            Show details ({entries.length} endpoint
            {entries.length === 1 ? "" : "s"})
          </summary>
          <div className="overflow-x-auto mt-2">
            <table className="table table-xs">
              <thead>
                <tr>
                  <th>Method</th>
                  <th>Path</th>
                  <th>API key</th>
                  <th>Count</th>
                  <th>Last seen</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e, i) => (
                  <tr key={`${e.method}:${e.path}:${e.apiKeyIdent}:${i}`}>
                    <td className="font-mono">{e.method}</td>
                    <td className="font-mono">{e.path}</td>
                    <td className="font-mono">
                      {e.apiKeyIdent || "(unauthenticated)"}
                    </td>
                    <td>{e.count}</td>
                    <td title={e.lastSeen}>{formatRelative(e.lastSeen)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      </div>
      <button
        type="button"
        className="btn btn-ghost btn-sm btn-square"
        aria-label="Dismiss legacy API warning"
        onClick={handleDismiss}
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  );
}
