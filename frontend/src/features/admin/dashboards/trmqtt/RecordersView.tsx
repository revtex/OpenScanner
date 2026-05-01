// Live recorder pool — table view of every recorder reported by the
// `tr.recorders` frame. SDR source, current state, frequency, and call
// count are surfaced so an operator can spot stuck recorders quickly.
import { useMemo } from "react";
import { useTrRecorders } from "./useTrMqtt";
import type { TrInstance } from "./types";
import { asArray, asRecord, fmtFreqMHz, fmtNumber } from "./format";

interface RecorderRow {
  id: string;
  src_num?: string;
  rec_num?: string;
  type?: string;
  duration?: string;
  freq?: number;
  count?: string;
  rec_state_type?: string;
  squelched?: string;
}

function toRows(payload: unknown): RecorderRow[] {
  const items = asArray(asRecord(payload)?.recorders);
  return items.map((it) => {
    const rec = asRecord(it) ?? {};
    return {
      id: String(rec.id ?? rec.rec_num ?? Math.random()),
      src_num: rec.src_num != null ? String(rec.src_num) : undefined,
      rec_num: rec.rec_num != null ? String(rec.rec_num) : undefined,
      type: typeof rec.type === "string" ? rec.type : undefined,
      duration: rec.duration != null ? String(rec.duration) : undefined,
      freq: typeof rec.freq === "number" ? rec.freq : undefined,
      count: rec.count != null ? String(rec.count) : undefined,
      rec_state_type:
        typeof rec.rec_state_type === "string" ? rec.rec_state_type : undefined,
      squelched: rec.squelched != null ? String(rec.squelched) : undefined,
    };
  });
}

const STATE_BADGE: Record<string, string> = {
  IDLE: "badge-ghost",
  ACTIVE: "badge-success",
  STOPPED: "badge-warning",
  ERROR: "badge-error",
};

export default function RecordersView({ instance }: { instance: TrInstance }) {
  const data = useTrRecorders(instance.id);
  const rows = useMemo(() => toRows(data), [data]);

  return (
    <div className="space-y-3">
      <h3 className="text-lg font-semibold">
        Recorders ({rows.length})
      </h3>
      {rows.length === 0 ? (
        <div className="text-xs text-base-content/50">
          No `tr.recorders` frame received yet.
        </div>
      ) : (
        <div className="overflow-x-auto border border-base-300 rounded-md">
          <table className="table table-xs">
            <thead>
              <tr>
                <th>ID</th>
                <th>Src</th>
                <th>Rec #</th>
                <th>Type</th>
                <th>State</th>
                <th>Freq</th>
                <th className="text-right">Duration</th>
                <th className="text-right">Calls</th>
                <th>Squelched</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => {
                const stateCls =
                  STATE_BADGE[(r.rec_state_type ?? "").toUpperCase()] ??
                  "badge-ghost";
                return (
                  <tr key={r.id}>
                    <td className="font-mono">{r.id}</td>
                    <td>{fmtNumber(r.src_num)}</td>
                    <td>{fmtNumber(r.rec_num)}</td>
                    <td>{r.type ?? "—"}</td>
                    <td>
                      <span className={`badge badge-xs ${stateCls}`}>
                        {r.rec_state_type ?? "—"}
                      </span>
                    </td>
                    <td className="font-mono">{fmtFreqMHz(r.freq)}</td>
                    <td className="text-right">{fmtNumber(r.duration)}</td>
                    <td className="text-right">{fmtNumber(r.count)}</td>
                    <td>{r.squelched ?? "—"}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
