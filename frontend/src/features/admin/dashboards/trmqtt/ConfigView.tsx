// Trunk Recorder config view — renders the static `tr.config` frame
// (capture/upload settings + SDR sources). The config frame is published
// once at plugin start, so this view stays static unless the plugin
// reconnects.
import { useMemo } from "react";
import { useTrConfig } from "./useTrMqtt";
import type { TrInstance } from "./types";
import { asArray, asRecord, fmtFreqMHz, fmtNumber } from "./format";

interface SourceRow {
  key: string;
  driver?: string;
  device?: string;
  rate?: number;
  center?: number;
  ppm?: string;
  gain?: string;
  digital_recorders?: string;
  analog_recorders?: string;
  signal_detector?: string;
  silence_threshold?: string;
}

/** The TR plugin wraps everything under a `config` key: `{type, config:{sources, systems, ...}}` */
function unwrapConfig(payload: unknown): Record<string, unknown> {
  const top = asRecord(payload);
  return asRecord(top?.config) ?? top ?? {};
}

function toSourceRows(payload: unknown): SourceRow[] {
  const cfg = unwrapConfig(payload);
  const items = asArray(cfg.sources);
  return items.map((it, idx) => {
    const r = asRecord(it) ?? {};
    return {
      key: String(r.device ?? idx),
      driver: typeof r.driver === "string" ? r.driver : undefined,
      device: typeof r.device === "string" ? r.device : undefined,
      rate: typeof r.rate === "number" ? r.rate : undefined,
      center: typeof r.center === "number" ? r.center : undefined,
      ppm: r.ppm != null ? String(r.ppm) : undefined,
      gain: r.gain != null ? String(r.gain) : undefined,
      digital_recorders:
        r.digital_recorders != null ? String(r.digital_recorders) : undefined,
      analog_recorders:
        r.analog_recorders != null ? String(r.analog_recorders) : undefined,
      signal_detector:
        r.signal_detector != null ? String(r.signal_detector) : undefined,
      silence_threshold:
        r.silence_threshold != null ? String(r.silence_threshold) : undefined,
    };
  });
}

interface SettingItem {
  label: string;
  value: string;
}

const SETTING_KEYS: { key: string; label: string }[] = [
  { key: "instance_id", label: "Instance ID" },
  { key: "default_mode", label: "Default mode" },
  { key: "capture_dir", label: "Capture dir" },
  { key: "upload_server", label: "Upload server" },
  { key: "broadcast_signals", label: "Broadcast signals" },
  { key: "control_message_warn_rate", label: "CC warn rate" },
  { key: "control_retune_limit", label: "CC retune limit" },
  { key: "call_timeout", label: "Call timeout" },
  { key: "log_file", label: "Log file" },
  { key: "log_dir", label: "Log dir" },
  { key: "frequency_format", label: "Freq format" },
  { key: "audio_archive", label: "Audio archive" },
  { key: "transmission_archive", label: "Tx archive" },
  { key: "call_log", label: "Call log" },
  { key: "compress_wav", label: "Compress WAV" },
];

function toSettingItems(payload: unknown): SettingItem[] {
  const rec = unwrapConfig(payload);
  if (!rec) return [];
  const items: SettingItem[] = [];
  for (const { key, label } of SETTING_KEYS) {
    const v = rec[key];
    if (v == null || v === "") continue;
    items.push({ label, value: String(v) });
  }
  return items;
}

export default function ConfigView({ instance }: { instance: TrInstance }) {
  const payload = useTrConfig(instance.id);
  const sources = useMemo(() => toSourceRows(payload), [payload]);
  const settings = useMemo(() => toSettingItems(payload), [payload]);

  if (!payload) {
    return (
      <div className="text-xs text-base-content/50">
        No `tr.config` frame received yet. The plugin publishes this once on
        connect.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="card bg-base-200">
        <div className="card-body p-4">
          <h4 className="card-title text-sm">Recorder settings</h4>
          {settings.length === 0 ? (
            <div className="text-xs text-base-content/50">
              No known settings present in the config frame.
            </div>
          ) : (
            <dl className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-1 text-xs">
              {settings.map((s) => (
                <div key={s.label} className="flex gap-2">
                  <dt className="text-base-content/60 min-w-35">{s.label}</dt>
                  <dd className="font-mono break-all">{s.value}</dd>
                </div>
              ))}
            </dl>
          )}
        </div>
      </div>

      <div className="card bg-base-200">
        <div className="card-body p-4">
          <h4 className="card-title text-sm">SDR sources ({sources.length})</h4>
          {sources.length === 0 ? (
            <div className="text-xs text-base-content/50">
              No SDR sources reported.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="table table-xs">
                <thead>
                  <tr>
                    <th>Driver</th>
                    <th>Device</th>
                    <th>Center</th>
                    <th className="text-right">Rate</th>
                    <th>Gain</th>
                    <th>PPM</th>
                    <th className="text-right">Digital</th>
                    <th className="text-right">Analog</th>
                    <th>Signal det.</th>
                  </tr>
                </thead>
                <tbody>
                  {sources.map((s) => (
                    <tr key={s.key}>
                      <td>{s.driver ?? "—"}</td>
                      <td className="font-mono break-all">{s.device ?? "—"}</td>
                      <td className="font-mono">{fmtFreqMHz(s.center)}</td>
                      <td className="text-right font-mono">
                        {s.rate != null
                          ? `${(s.rate / 1e6).toFixed(3)} Msps`
                          : "—"}
                      </td>
                      <td>{s.gain ?? "—"}</td>
                      <td>{s.ppm ?? "—"}</td>
                      <td className="text-right">
                        {fmtNumber(s.digital_recorders)}
                      </td>
                      <td className="text-right">
                        {fmtNumber(s.analog_recorders)}
                      </td>
                      <td>{s.signal_detector ?? "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      <details className="bg-base-200 rounded-md">
        <summary className="cursor-pointer p-3 text-sm font-semibold">
          Raw config payload
        </summary>
        <pre className="text-[10px] p-3 overflow-x-auto">
          {JSON.stringify(payload, null, 2)}
        </pre>
      </details>
    </div>
  );
}
