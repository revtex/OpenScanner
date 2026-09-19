// Live dashboard overview — connection status, decode-rate sparkline, and
// per-system rate breakdown. Tables for recorders / calls / systems live in
// their own views to keep this pane scannable at a glance.
import { useMemo } from "react";
import {
  useTrMqttState,
  useTrInstanceConnection,
  useTrLagWarning,
  useTrSystemRates,
  useTrPluginStatus,
} from "./useTrMqtt";
import { useGetTrSnapshotQuery } from "./trMqttApi";
import type { TrInstance, RateSample } from "./types";
import { AlertTriangle, Plug, Radio, Wifi } from "lucide-react";
import { asArray, asRecord, fmtDateTime, fmtFreqMHz } from "./format";

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
        <span>Aggregate decode rate (msgs/sec)</span>
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

function StatCard({
  label,
  value,
  icon,
}: {
  label: string;
  value: string | number;
  icon: React.ReactNode;
}) {
  return (
    <div className="card bg-base-200 shadow-sm">
      <div className="card-body p-3">
        <div className="flex items-center gap-2 text-base-content/60 text-xs uppercase tracking-wide">
          {icon}
          {label}
        </div>
        <div className="text-2xl font-semibold mt-1">{value}</div>
      </div>
    </div>
  );
}

export default function DashboardView({ instance }: { instance: TrInstance }) {
  // Hydrate on mount; live WS events keep it fresh.
  useGetTrSnapshotQuery(instance.id);

  const conn = useTrInstanceConnection(instance.id);
  const lag = useTrLagWarning(instance.id);
  const state = useTrMqttState();
  const rates = state.rates[instance.id] ?? [];
  const systemRates = useTrSystemRates(instance.id);
  const pluginStatus = useTrPluginStatus(instance.id);
  const recorders = state.recorders[instance.id];
  const callsActive = state.callsActive[instance.id];
  const systems = state.systems[instance.id];

  const counts = useMemo(() => {
    const recRec = asRecord(recorders);
    const callRec = asRecord(callsActive);
    const sysRec = asRecord(systems);
    const recItems = asArray(recRec?.recorders);
    // Plugin recorder states: AVAILABLE / IDLE / ACTIVE / RECORDING /
    // STOPPED / IGNORE. Treat anything actively recording (or any
    // post-IDLE state) as "active" to mirror trunk-recorder's own UI.
    const activeRecorders = recItems.filter((it) => {
      const s = asRecord(it)?.rec_state_type;
      if (typeof s !== "string") return false;
      const up = s.toUpperCase();
      return up === "ACTIVE" || up === "RECORDING";
    }).length;
    return {
      recordersTotal: recItems.length,
      recordersActive: activeRecorders,
      activeCalls: asArray(callRec?.calls ?? callRec?.callsActive).length,
      systems: asArray(sysRec?.systems).length,
    };
  }, [recorders, callsActive, systems]);

  const sysRateRows = useMemo(
    () =>
      Object.values(systemRates).sort((a, b) =>
        a.sysName.localeCompare(b.sysName),
      ),
    [systemRates],
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <h3 className="text-lg font-semibold">{instance.label}</h3>
        <span
          className={`badge ${
            conn.connected ? "badge-success" : "badge-warning"
          } badge-sm`}
        >
          {conn.connected ? "broker connected" : "broker disconnected"}
        </span>
        {pluginStatus && (
          <span
            className={`badge badge-sm ${
              pluginStatus.status === "connected"
                ? "badge-success"
                : "badge-error"
            }`}
            title={
              pluginStatus.clientId
                ? `client_id: ${pluginStatus.clientId}`
                : undefined
            }
          >
            plugin: {pluginStatus.status}
          </span>
        )}
        {conn.lastError && (
          <span className="text-xs text-error">{conn.lastError}</span>
        )}
        {conn.lastSeenAt && (
          <span className="text-xs text-base-content/50">
            last frame: {fmtDateTime(conn.lastSeenAt)}
          </span>
        )}
        {lag && (
          <span className="badge badge-warning gap-1">
            <AlertTriangle className="w-3 h-3" /> processing lag
          </span>
        )}
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
        <StatCard
          label="Systems"
          value={counts.systems}
          icon={<Radio className="w-3 h-3" />}
        />
        <StatCard
          label="Recorders (active / total)"
          value={`${counts.recordersActive} / ${counts.recordersTotal}`}
          icon={<Plug className="w-3 h-3" />}
        />
        <StatCard
          label="Active calls"
          value={counts.activeCalls}
          icon={<Wifi className="w-3 h-3" />}
        />
        <StatCard
          label="Decode rate (msgs/s)"
          value={
            rates.length > 0 ? rates[rates.length - 1].rate.toFixed(1) : "—"
          }
          icon={<Radio className="w-3 h-3" />}
        />
      </div>

      <div className="card bg-base-200">
        <div className="card-body p-4">
          <Sparkline samples={rates} />
        </div>
      </div>

      <div className="card bg-base-200">
        <div className="card-body p-4">
          <h4 className="card-title text-sm">Per-system decode rates</h4>
          {sysRateRows.length === 0 ? (
            <div className="text-xs text-base-content/50">
              No rates frame yet.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="table table-xs">
                <thead>
                  <tr>
                    <th>System</th>
                    <th className="text-right">Rate (msgs/s)</th>
                    <th className="text-right">Interval</th>
                    <th>Control channel</th>
                  </tr>
                </thead>
                <tbody>
                  {sysRateRows.map((r) => (
                    <tr key={r.sysName}>
                      <td className="font-mono">{r.sysName}</td>
                      <td className="text-right font-mono">
                        {r.decoderate.toFixed(2)}
                      </td>
                      <td className="text-right text-base-content/60">
                        {r.decoderateInterval ?? "—"}s
                      </td>
                      <td className="font-mono text-xs">
                        {fmtFreqMHz(r.controlChannel)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
