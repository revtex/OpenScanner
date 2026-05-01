// Systems list — table view from `tr.systems` / `tr.system` frames,
// falling back to the systems array embedded in the `tr.config` frame
// when no dedicated systems topic is published.
import { useMemo } from "react";
import { useTrMqttState } from "./useTrMqtt";
import type { TrInstance } from "./types";
import { asArray, asRecord, fmtFreqMHz, fmtNumber } from "./format";

interface SystemRow {
  key: string;
  sys_num?: string;
  sys_name?: string;
  type?: string;
  sysid?: string;
  wacn?: string;
  nac?: string;
  rfss?: string;
  site_id?: string;
  control_channel?: number;
}

function extractSystems(payload: unknown): SystemRow[] {
  if (!payload) return [];
  const rec = asRecord(payload);
  // tr.systems frame: {systems: [...]}
  // tr.config frame: {config: {systems: [...]}}
  const items =
    asArray(rec?.systems).length > 0
      ? asArray(rec?.systems)
      : asArray(asRecord(rec?.config)?.systems);
  return items.map((it, idx) => {
    const r = asRecord(it) ?? {};
    return {
      key: String(r.sys_num ?? r.sys_name ?? idx),
      sys_num: r.sys_num != null ? String(r.sys_num) : undefined,
      sys_name: typeof r.sys_name === "string" ? r.sys_name : undefined,
      // tr.systems uses `type`; tr.config uses `system_type`
      type:
        typeof r.system_type === "string"
          ? r.system_type
          : typeof r.type === "string"
            ? r.type
            : undefined,
      sysid: r.sysid != null ? String(r.sysid) : undefined,
      wacn: r.wacn != null ? String(r.wacn) : undefined,
      nac: r.nac != null ? String(r.nac) : undefined,
      rfss: r.rfss != null ? String(r.rfss) : undefined,
      site_id: r.site_id != null ? String(r.site_id) : undefined,
      control_channel:
        typeof r.control_channel === "number" ? r.control_channel : undefined,
    };
  });
}

export default function SystemsView({ instance }: { instance: TrInstance }) {
  const state = useTrMqttState();
  // Prefer a dedicated tr.systems frame; fall back to the systems list
  // embedded in the tr.config frame (plugin may not publish /systems).
  const systemsPayload = state.systems[instance.id];
  const configPayload = state.config[instance.id];
  const source = systemsPayload ?? configPayload;
  const rows = useMemo(() => extractSystems(source), [source]);

  return (
    <div className="space-y-3">
      <h3 className="text-lg font-semibold">Systems ({rows.length})</h3>
      {rows.length === 0 ? (
        <div className="text-xs text-base-content/50">
          No systems data yet (waiting for tr.systems or tr.config frame).
        </div>
      ) : (
        <div className="overflow-x-auto border border-base-300 rounded-md">
          <table className="table table-xs">
            <thead>
              <tr>
                <th>#</th>
                <th>Name</th>
                <th>Type</th>
                <th>Control channel</th>
                <th>SYSID</th>
                <th>WACN</th>
                <th>NAC</th>
                <th>RFSS</th>
                <th>Site</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.key}>
                  <td>{fmtNumber(r.sys_num)}</td>
                  <td className="font-mono">{r.sys_name ?? "—"}</td>
                  <td>{r.type ?? "—"}</td>
                  <td className="font-mono">{fmtFreqMHz(r.control_channel)}</td>
                  <td className="font-mono">{r.sysid ?? "—"}</td>
                  <td className="font-mono">{r.wacn ?? "—"}</td>
                  <td className="font-mono">{r.nac ?? "—"}</td>
                  <td>{fmtNumber(r.rfss)}</td>
                  <td>{fmtNumber(r.site_id)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
