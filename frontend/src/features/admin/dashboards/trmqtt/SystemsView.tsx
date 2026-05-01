// Systems list — table view from `tr.systems` and one-shot `tr.system`
// frames. Surfaces the trunked system identity (sys_num, sysid, wacn,
// nac, rfss, site_id, type) so an operator can verify the plugin is
// reporting the correct affiliation.
import { useMemo } from "react";
import { useTrMqttState } from "./useTrMqtt";
import type { TrInstance } from "./types";
import { asArray, asRecord, fmtNumber } from "./format";

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
}

function toRows(payload: unknown): SystemRow[] {
  const items = asArray(asRecord(payload)?.systems);
  return items.map((it, idx) => {
    const rec = asRecord(it) ?? {};
    return {
      key: String(rec.sys_num ?? rec.sys_name ?? idx),
      sys_num: rec.sys_num != null ? String(rec.sys_num) : undefined,
      sys_name: typeof rec.sys_name === "string" ? rec.sys_name : undefined,
      type: typeof rec.type === "string" ? rec.type : undefined,
      sysid: rec.sysid != null ? String(rec.sysid) : undefined,
      wacn: rec.wacn != null ? String(rec.wacn) : undefined,
      nac: rec.nac != null ? String(rec.nac) : undefined,
      rfss: rec.rfss != null ? String(rec.rfss) : undefined,
      site_id: rec.site_id != null ? String(rec.site_id) : undefined,
    };
  });
}

export default function SystemsView({ instance }: { instance: TrInstance }) {
  const state = useTrMqttState();
  const payload = state.systems[instance.id];
  const rows = useMemo(() => toRows(payload), [payload]);

  return (
    <div className="space-y-3">
      <h3 className="text-lg font-semibold">Systems ({rows.length})</h3>
      {rows.length === 0 ? (
        <div className="text-xs text-base-content/50">
          No `tr.systems` frame received yet.
        </div>
      ) : (
        <div className="overflow-x-auto border border-base-300 rounded-md">
          <table className="table table-xs">
            <thead>
              <tr>
                <th>#</th>
                <th>Name</th>
                <th>Type</th>
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
