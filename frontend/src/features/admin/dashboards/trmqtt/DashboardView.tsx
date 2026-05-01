// Live dashboard view — sources/recorders + decode-rate sparkline + active calls.
import { useMemo } from "react";
import {
  useTrMqttState,
  useTrInstanceConnection,
  useTrLagWarning,
} from "./useTrMqtt";
import { useGetTrSnapshotQuery } from "./trMqttApi";
import type { TrInstance, RateSample } from "./types";
import { AlertTriangle } from "lucide-react";

function Sparkline({ samples }: { samples: RateSample[] }) {
  const W = 600;
  const H = 80;
  const PAD = 4;

  if (samples.length === 0) {
    return (
      <div className="text-xs text-base-content/50 h-20 flex items-center">
        No rate samples yet.
      </div>
    );
  }

  const maxRate = Math.max(...samples.map((s) => s.rate), 1);
  const n = samples.length;
  const xOf = (i: number) =>
    PAD + (n > 1 ? (i / (n - 1)) * (W - 2 * PAD) : (W - 2 * PAD) / 2);
  const yOf = (v: number) => PAD + (1 - v / maxRate) * (H - 2 * PAD);

  const points = samples.map((s, i) => `${xOf(i)},${yOf(s.rate)}`).join(" ");
  const last = samples[samples.length - 1];

  return (
    <div>
      <div className="flex justify-between text-xs text-base-content/60 mb-1">
        <span>Decode rate</span>
        <span>
          last: <strong>{last.rate.toFixed(1)}</strong> · max:{" "}
          <strong>{maxRate.toFixed(1)}</strong>
        </span>
      </div>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="w-full h-20"
        preserveAspectRatio="none"
        aria-label="Decode rate sparkline"
      >
        <polyline
          fill="none"
          stroke="currentColor"
          strokeWidth={1.5}
          points={points}
          className="text-primary"
        />
      </svg>
    </div>
  );
}

function asArray(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" ? (v as Record<string, unknown>) : null;
}

export default function DashboardView({ instance }: { instance: TrInstance }) {
  // Hydrate on mount; live WS events keep it fresh.
  useGetTrSnapshotQuery(instance.id);

  const conn = useTrInstanceConnection(instance.id);
  const lag = useTrLagWarning(instance.id);
  const state = useTrMqttState();
  const rates = state.rates[instance.id] ?? [];
  const recorders = state.recorders[instance.id];
  const callsActive = state.callsActive[instance.id];
  const systems = state.systems[instance.id];

  const recorderRows = useMemo(() => {
    const rec = asRecord(recorders);
    return asArray(rec?.recorders);
  }, [recorders]);

  const callRows = useMemo(() => {
    const rec = asRecord(callsActive);
    return asArray(rec?.calls ?? rec?.callsActive ?? callsActive);
  }, [callsActive]);

  const systemRows = useMemo(() => {
    const rec = asRecord(systems);
    return asArray(rec?.systems ?? systems);
  }, [systems]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <h3 className="text-lg font-semibold">{instance.label}</h3>
        <span
          className={`badge ${
            conn.connected ? "badge-success" : "badge-warning"
          } badge-sm`}
        >
          {conn.connected ? "connected" : "disconnected"}
        </span>
        {conn.lastError && (
          <span className="text-xs text-error">{conn.lastError}</span>
        )}
        {lag && (
          <span className="badge badge-warning gap-1">
            <AlertTriangle className="w-3 h-3" /> processing lag
          </span>
        )}
      </div>

      <div className="card bg-base-200">
        <div className="card-body p-4">
          <Sparkline samples={rates} />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="card bg-base-200">
          <div className="card-body p-4">
            <h4 className="card-title text-sm">Recorders</h4>
            {recorderRows.length === 0 ? (
              <div className="text-xs text-base-content/50">No data.</div>
            ) : (
              <div className="overflow-x-auto max-h-64">
                <table className="table table-xs">
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>State</th>
                      <th>Type</th>
                    </tr>
                  </thead>
                  <tbody>
                    {recorderRows.map((r, i) => {
                      const rec = asRecord(r);
                      return (
                        <tr key={i}>
                          <td className="font-mono text-xs">
                            {String(rec?.id ?? rec?.rec_num ?? i)}
                          </td>
                          <td>
                            {String(rec?.rec_state_type ?? rec?.state ?? "")}
                          </td>
                          <td className="text-xs">
                            {String(rec?.type ?? rec?.recorder_type ?? "")}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>

        <div className="card bg-base-200">
          <div className="card-body p-4">
            <h4 className="card-title text-sm">Active calls</h4>
            {callRows.length === 0 ? (
              <div className="text-xs text-base-content/50">
                No active calls.
              </div>
            ) : (
              <div className="overflow-x-auto max-h-64">
                <table className="table table-xs">
                  <thead>
                    <tr>
                      <th>System</th>
                      <th>TG</th>
                      <th>Freq</th>
                      <th>Unit</th>
                    </tr>
                  </thead>
                  <tbody>
                    {callRows.map((c, i) => {
                      const rec = asRecord(c);
                      return (
                        <tr key={i}>
                          <td>
                            {String(
                              rec?.shortname ??
                                rec?.short_name ??
                                rec?.sys_name ??
                                rec?.system ??
                                rec?.sys_num ??
                                "",
                            )}
                          </td>
                          <td>{String(rec?.talkgroup ?? "")}</td>
                          <td className="font-mono text-xs">
                            {String(rec?.freq ?? rec?.frequency ?? "")}
                          </td>
                          <td>{String(rec?.unit ?? rec?.src ?? "")}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="card bg-base-200">
        <div className="card-body p-4">
          <h4 className="card-title text-sm">Systems</h4>
          {systemRows.length === 0 ? (
            <div className="text-xs text-base-content/50">
              No systems frame yet.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="table table-xs">
                <thead>
                  <tr>
                    <th>Shortname</th>
                    <th>System type</th>
                    <th>Talkgroups</th>
                  </tr>
                </thead>
                <tbody>
                  {systemRows.map((s, i) => {
                    const rec = asRecord(s);
                    const tgs = asArray(rec?.talkgroups);
                    return (
                      <tr key={i}>
                        <td>
                          {String(rec?.shortname ?? rec?.short_name ?? "")}
                        </td>
                        <td>{String(rec?.system_type ?? rec?.type ?? "")}</td>
                        <td>{tgs.length}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
