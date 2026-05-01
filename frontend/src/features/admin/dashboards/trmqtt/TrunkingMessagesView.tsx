// Trunking message view — two sub-tabs:
//   Stats (default): aggregate counts per opcode/type, updated live. Easy to
//     read at a glance even during heavy control-channel traffic.
//   Live: scrolling feed with a pause toggle so individual frames can be read.
import { useRef, useMemo, useState, useEffect } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTrMessages } from "./useTrMqtt";
import type { TrInstance, MessageEntry } from "./types";
import { fmtTime, fmtNumber } from "./format";
import { Pause, Play } from "lucide-react";

// ── Stats sub-tab ────────────────────────────────────────────────────────────

interface StatRow {
  key: string;
  type: string;
  opcode: string;
  opcodeType: string;
  systems: string;
  count: number;
  lastSeen: number;
  lastMeta: string;
}

function buildStats(msgs: MessageEntry[]): StatRow[] {
  const map = new Map<string, StatRow>();
  for (const m of msgs) {
    const k = `${m.opcode ?? "?"}|${m.opcodeType ?? "?"}|${m.type ?? "?"}`;
    const existing = map.get(k);
    const sys = m.shortname ?? "?";
    if (existing) {
      existing.count++;
      if (m.at > existing.lastSeen) {
        existing.lastSeen = m.at;
        existing.lastMeta = m.meta ?? m.opcodeDesc ?? m.trunkMsg ?? "";
      }
      if (!existing.systems.split(", ").includes(sys)) {
        existing.systems = existing.systems ? existing.systems + ", " + sys : sys;
      }
    } else {
      map.set(k, {
        key: k,
        type: m.type ?? "—",
        opcode: m.opcode ?? "—",
        opcodeType: m.opcodeType ?? "—",
        systems: sys,
        count: 1,
        lastSeen: m.at,
        lastMeta: m.meta ?? m.opcodeDesc ?? m.trunkMsg ?? "",
      });
    }
  }
  return Array.from(map.values()).sort((a, b) => b.count - a.count);
}

function StatsTab({ msgs }: { msgs: MessageEntry[] }) {
  const rows = useMemo(() => buildStats(msgs), [msgs]);
  if (rows.length === 0)
    return <div className="text-xs text-base-content/50">No messages yet.</div>;
  return (
    <div className="overflow-x-auto border border-base-300 rounded-md">
      <table className="table table-xs">
        <thead>
          <tr>
            <th className="text-right w-14">Count</th>
            <th>Type</th>
            <th>Opcode</th>
            <th>Opcode Type</th>
            <th>Systems</th>
            <th>Last seen</th>
            <th>Description</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.key}>
              <td className="text-right font-mono font-semibold">{fmtNumber(r.count)}</td>
              <td>{r.type}</td>
              <td className="font-mono">{r.opcode}</td>
              <td>{r.opcodeType}</td>
              <td className="text-base-content/60">{r.systems}</td>
              <td className="text-base-content/60">{fmtTime(r.lastSeen)}</td>
              <td className="max-w-xs truncate">{r.lastMeta || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ── Live sub-tab ─────────────────────────────────────────────────────────────

export default function TrunkingMessagesView({
  instance,
}: {
  instance: TrInstance;
}) {
  const msgs = useTrMessages(instance.id);
  const [tab, setTab] = useState<"stats" | "live">("stats");
  const [paused, setPaused] = useState(false);
  const [frozenMsgs, setFrozenMsgs] = useState<typeof msgs>([]);
  const [filterSys, setFilterSys] = useState("");
  const [filterType, setFilterType] = useState("");

  // Release frozen snapshot when resuming.
  useEffect(() => {
    if (!paused) setFrozenMsgs([]);
  }, [paused]);

  const liveSource = paused ? frozenMsgs : msgs;

  const ordered = useMemo(() => {
    return liveSource
      .filter(
        (m) =>
          !filterSys ||
          (m.shortname ?? "").toLowerCase().includes(filterSys.toLowerCase()),
      )
      .filter(
        (m) =>
          !filterType ||
          (m.type ?? "").toLowerCase().includes(filterType.toLowerCase()) ||
          (m.opcodeType ?? "").toLowerCase().includes(filterType.toLowerCase()),
      )
      .slice()
      .reverse();
  }, [liveSource, filterSys, filterType]);

  const parentRef = useRef<HTMLDivElement>(null);
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: ordered.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 32,
    overscan: 8,
  });

  function handlePauseToggle() {
    if (!paused) setFrozenMsgs(msgs);
    setPaused((p) => !p);
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 flex-wrap">
        <h3 className="text-lg font-semibold mr-auto">
          Trunking messages ({msgs.length})
        </h3>
        <div role="tablist" className="tabs tabs-boxed bg-base-200">
          <button
            role="tab"
            className={`tab ${tab === "stats" ? "tab-active" : ""}`}
            onClick={() => setTab("stats")}
          >
            Stats
          </button>
          <button
            role="tab"
            className={`tab ${tab === "live" ? "tab-active" : ""}`}
            onClick={() => setTab("live")}
          >
            Live
          </button>
        </div>
      </div>

      {tab === "stats" && <StatsTab msgs={msgs} />}

      {tab === "live" && (
        <>
          <div className="flex items-center gap-2 flex-wrap">
            <button
              className={`btn btn-sm gap-1 ${paused ? "btn-warning" : "btn-ghost"}`}
              onClick={handlePauseToggle}
            >
              {paused ? (
                <>
                  <Play className="w-3 h-3" /> Resume
                </>
              ) : (
                <>
                  <Pause className="w-3 h-3" /> Pause
                </>
              )}
            </button>
            {paused && (
              <span className="text-xs text-warning">
                Feed paused — showing {frozenMsgs.length} captured frames
              </span>
            )}
            <input
              type="text"
              className="input input-bordered input-sm"
              placeholder="filter system"
              value={filterSys}
              onChange={(e) => setFilterSys(e.target.value)}
            />
            <input
              type="text"
              className="input input-bordered input-sm"
              placeholder="filter type / opcode"
              value={filterType}
              onChange={(e) => setFilterType(e.target.value)}
            />
          </div>

          <div
            className="grid gap-2 px-2 py-1 text-[11px] uppercase tracking-wide text-base-content/60 font-semibold"
            style={{ gridTemplateColumns: "80px 110px 110px 60px 130px 1fr" }}
          >
            <span>Time</span>
            <span>System</span>
            <span>Type</span>
            <span>Opcode</span>
            <span>Opcode Type</span>
            <span>Meta / Description</span>
          </div>

          {ordered.length === 0 ? (
            <div className="text-xs text-base-content/50">No messages yet.</div>
          ) : (
            <div
              ref={parentRef}
              className="border border-base-300 rounded-md max-h-130 overflow-auto"
            >
              <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
                {virtualizer.getVirtualItems().map((vi) => {
                  const m = ordered[vi.index];
                  const meta =
                    m.meta ?? m.opcodeDesc ?? m.trunkMsg ?? JSON.stringify(m.raw).slice(0, 160);
                  return (
                    <div
                      key={vi.key}
                      data-index={vi.index}
                      ref={virtualizer.measureElement}
                      style={{
                        position: "absolute",
                        top: 0,
                        left: 0,
                        right: 0,
                        transform: `translateY(${vi.start}px)`,
                        gridTemplateColumns: "80px 110px 110px 60px 130px 1fr",
                      }}
                      className="grid gap-2 px-2 py-1 text-xs border-b border-base-300/50 font-mono"
                      title={m.trunkMsg}
                    >
                      <span className="text-base-content/60">{fmtTime(m.at)}</span>
                      <span className="truncate">{m.shortname ?? "—"}</span>
                      <span className="truncate">{m.type ?? "—"}</span>
                      <span>{m.opcode ?? "—"}</span>
                      <span className="truncate">{m.opcodeType ?? "—"}</span>
                      <span className="truncate">{meta}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
