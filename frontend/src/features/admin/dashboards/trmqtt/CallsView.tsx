// Calls view — combines the live "active calls" snapshot from
// `tr.callsActive` with a rolling history of `tr.callStart` /
// `tr.callEnd` events captured by the slice. Internal sub-tabs keep the
// two presentations distinct so an operator can quickly compare what is
// currently transmitting vs what just finished.
import { useMemo, useState } from "react";
import { useTrCallsActive, useTrRecentCalls } from "./useTrMqtt";
import type { TrInstance, RecentCallEntry } from "./types";
import {
  asArray,
  asRecord,
  fmtDuration,
  fmtFreqMHz,
  fmtTime,
} from "./format";

interface ActiveCallRow {
  key: string;
  call_num?: string;
  sys_name?: string;
  freq?: number;
  unit?: string;
  unit_alpha_tag?: string;
  talkgroup?: string;
  talkgroup_alpha_tag?: string;
  talkgroup_group?: string;
  talkgroup_tag?: string;
  encrypted?: boolean;
  emergency?: boolean;
  call_state_type?: string;
  rec_state_type?: string;
  start_time?: number;
}

function toActiveRows(payload: unknown): ActiveCallRow[] {
  const rec = asRecord(payload);
  const items = asArray(rec?.calls ?? rec?.callsActive);
  return items.map((it, idx) => {
    const c = asRecord(it) ?? {};
    return {
      key: String(c.call_num ?? c.id ?? idx),
      call_num: c.call_num != null ? String(c.call_num) : undefined,
      sys_name: typeof c.sys_name === "string" ? c.sys_name : undefined,
      freq: typeof c.freq === "number" ? c.freq : undefined,
      unit: c.unit != null ? String(c.unit) : undefined,
      unit_alpha_tag:
        typeof c.unit_alpha_tag === "string" ? c.unit_alpha_tag : undefined,
      talkgroup: c.talkgroup != null ? String(c.talkgroup) : undefined,
      talkgroup_alpha_tag:
        typeof c.talkgroup_alpha_tag === "string"
          ? c.talkgroup_alpha_tag
          : undefined,
      talkgroup_group:
        typeof c.talkgroup_group === "string" ? c.talkgroup_group : undefined,
      talkgroup_tag:
        typeof c.talkgroup_tag === "string" ? c.talkgroup_tag : undefined,
      encrypted: typeof c.encrypted === "boolean" ? c.encrypted : undefined,
      emergency: typeof c.emergency === "boolean" ? c.emergency : undefined,
      call_state_type:
        typeof c.call_state_type === "string" ? c.call_state_type : undefined,
      rec_state_type:
        typeof c.rec_state_type === "string" ? c.rec_state_type : undefined,
      start_time:
        typeof c.start_time === "number" ? c.start_time : undefined,
    };
  });
}

function ActiveTable({ rows }: { rows: ActiveCallRow[] }) {
  if (rows.length === 0) {
    return (
      <div className="text-xs text-base-content/50">
        No active calls currently.
      </div>
    );
  }
  return (
    <div className="overflow-x-auto border border-base-300 rounded-md">
      <table className="table table-xs">
        <thead>
          <tr>
            <th>Call #</th>
            <th>System</th>
            <th>Freq</th>
            <th>TG Alpha</th>
            <th>TG Group</th>
            <th>Unit</th>
            <th>Unit Alpha</th>
            <th>Call State</th>
            <th>Rec State</th>
            <th>Flags</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.key}>
              <td className="font-mono">{r.call_num ?? "—"}</td>
              <td className="font-mono">{r.sys_name ?? "—"}</td>
              <td className="font-mono">{fmtFreqMHz(r.freq)}</td>
              <td>{r.talkgroup_alpha_tag ?? r.talkgroup ?? "—"}</td>
              <td>{r.talkgroup_group ?? "—"}</td>
              <td className="font-mono">{r.unit ?? "—"}</td>
              <td>{r.unit_alpha_tag ?? "—"}</td>
              <td>
                <span className="badge badge-xs badge-ghost">
                  {r.call_state_type ?? "—"}
                </span>
              </td>
              <td>
                <span className="badge badge-xs badge-ghost">
                  {r.rec_state_type ?? "—"}
                </span>
              </td>
              <td className="text-xs space-x-1">
                {r.encrypted && (
                  <span className="badge badge-xs badge-warning">ENC</span>
                )}
                {r.emergency && (
                  <span className="badge badge-xs badge-error">EMRG</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RecentTable({ rows }: { rows: RecentCallEntry[] }) {
  if (rows.length === 0) {
    return (
      <div className="text-xs text-base-content/50">
        No call_start / call_end events yet.
      </div>
    );
  }
  return (
    <div className="overflow-x-auto border border-base-300 rounded-md">
      <table className="table table-xs">
        <thead>
          <tr>
            <th>Time</th>
            <th>Kind</th>
            <th>Call #</th>
            <th>System</th>
            <th>Freq</th>
            <th>TG Alpha</th>
            <th>TG Group</th>
            <th>Unit</th>
            <th className="text-right">Length</th>
            <th>Flags</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={`${r.callId ?? r.callNum ?? i}-${r.kind}-${r.at}`}>
              <td className="font-mono text-base-content/60">{fmtTime(r.at)}</td>
              <td>
                <span
                  className={`badge badge-xs ${
                    r.kind === "start" ? "badge-success" : "badge-ghost"
                  }`}
                >
                  {r.kind}
                </span>
              </td>
              <td className="font-mono">{r.callNum ?? "—"}</td>
              <td className="font-mono">{r.sysName ?? "—"}</td>
              <td className="font-mono">{fmtFreqMHz(r.freq)}</td>
              <td>{r.talkgroupAlpha ?? r.talkgroup ?? "—"}</td>
              <td>{r.talkgroupGroup ?? "—"}</td>
              <td className="font-mono">
                {r.unitAlpha ?? r.unit ?? "—"}
              </td>
              <td className="text-right">{fmtDuration(r.length)}</td>
              <td className="text-xs space-x-1">
                {r.encrypted && (
                  <span className="badge badge-xs badge-warning">ENC</span>
                )}
                {r.emergency && (
                  <span className="badge badge-xs badge-error">EMRG</span>
                )}
                {r.conventional && (
                  <span className="badge badge-xs badge-info">CONV</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function CallsView({ instance }: { instance: TrInstance }) {
  const activePayload = useTrCallsActive(instance.id);
  const recentRaw = useTrRecentCalls(instance.id);
  const [tab, setTab] = useState<"active" | "recent">("active");

  const activeRows = useMemo(() => toActiveRows(activePayload), [activePayload]);
  const recent = useMemo(() => recentRaw.slice().reverse(), [recentRaw]);

  return (
    <div className="space-y-3">
      <div role="tablist" className="tabs tabs-boxed bg-base-200 w-fit">
        <button
          role="tab"
          className={`tab ${tab === "active" ? "tab-active" : ""}`}
          onClick={() => setTab("active")}
        >
          Active ({activeRows.length})
        </button>
        <button
          role="tab"
          className={`tab ${tab === "recent" ? "tab-active" : ""}`}
          onClick={() => setTab("recent")}
        >
          Recent ({recent.length})
        </button>
      </div>
      {tab === "active" ? (
        <ActiveTable rows={activeRows} />
      ) : (
        <RecentTable rows={recent} />
      )}
    </div>
  );
}
