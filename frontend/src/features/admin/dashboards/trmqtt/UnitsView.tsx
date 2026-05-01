// Recent unit events feed — virtualised, last N entries from `tr.unit.*`
// topics. Surfaces every plugin field the backend forwards: alpha tags,
// talkgroup metadata, frequency, call number, and encryption flag.
import { useRef, useMemo, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTrUnitEvents } from "./useTrMqtt";
import type { TrInstance } from "./types";
import { fmtFreqMHz, fmtTime } from "./format";

const KIND_BADGE: Record<string, string> = {
  on: "badge-success",
  off: "badge-ghost",
  call: "badge-info",
  end: "badge-ghost",
  data: "badge-secondary",
  join: "badge-primary",
  ackresp: "badge-accent",
  location: "badge-warning",
};

export default function UnitsView({ instance }: { instance: TrInstance }) {
  const events = useTrUnitEvents(instance.id);
  const [filterSys, setFilterSys] = useState("");
  const [filterTG, setFilterTG] = useState("");
  const [filterUnit, setFilterUnit] = useState("");

  const filtered = useMemo(() => {
    return events
      .filter(
        (e) =>
          !filterSys ||
          (e.shortname ?? "").toLowerCase().includes(filterSys.toLowerCase()),
      )
      .filter(
        (e) =>
          !filterTG ||
          (e.talkgroupId ?? "").includes(filterTG) ||
          (e.talkgroupAlpha ?? "")
            .toLowerCase()
            .includes(filterTG.toLowerCase()),
      )
      .filter(
        (e) =>
          !filterUnit ||
          (e.unitId ?? "").includes(filterUnit) ||
          (e.unitAlpha ?? "")
            .toLowerCase()
            .includes(filterUnit.toLowerCase()),
      )
      .slice()
      .reverse();
  }, [events, filterSys, filterTG, filterUnit]);

  const parentRef = useRef<HTMLDivElement>(null);
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 32,
    overscan: 8,
  });

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 flex-wrap">
        <h3 className="text-lg font-semibold mr-auto">
          Unit events ({filtered.length})
        </h3>
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
          placeholder="filter talkgroup"
          value={filterTG}
          onChange={(e) => setFilterTG(e.target.value)}
        />
        <input
          type="text"
          className="input input-bordered input-sm"
          placeholder="filter unit"
          value={filterUnit}
          onChange={(e) => setFilterUnit(e.target.value)}
        />
      </div>

      <div
        className="grid gap-2 px-2 py-1 text-[11px] uppercase tracking-wide text-base-content/60 font-semibold"
        style={{
          gridTemplateColumns:
            "80px 70px 110px 90px 110px 130px 110px 90px 80px 1fr",
        }}
      >
        <span>Time</span>
        <span>Kind</span>
        <span>System</span>
        <span>Unit</span>
        <span>Unit Alpha</span>
        <span>TG Alpha</span>
        <span>TG Group</span>
        <span>TG ID</span>
        <span>Freq</span>
        <span>Notes</span>
      </div>

      {filtered.length === 0 ? (
        <div className="text-xs text-base-content/50">No unit events yet.</div>
      ) : (
        <div
          ref={parentRef}
          className="border border-base-300 rounded-md max-h-130 overflow-auto"
        >
          <div
            style={{
              height: virtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {virtualizer.getVirtualItems().map((vi) => {
              const ev = filtered[vi.index];
              const badgeCls = KIND_BADGE[ev.kind] ?? "badge-ghost";
              const notes: string[] = [];
              if (ev.encrypted) notes.push("ENC");
              if (ev.callNum) notes.push(`call#${ev.callNum}`);
              if (ev.talkgroupTag) notes.push(ev.talkgroupTag);
              if (ev.talkgroupPatches) notes.push(`patches:${ev.talkgroupPatches}`);
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
                    gridTemplateColumns:
                      "80px 70px 110px 90px 110px 130px 110px 90px 80px 1fr",
                  }}
                  className="grid gap-2 px-2 py-1 text-xs border-b border-base-300/50 font-mono"
                  title={ev.talkgroupDescription}
                >
                  <span className="text-base-content/60">{fmtTime(ev.at)}</span>
                  <span>
                    <span className={`badge badge-xs ${badgeCls}`}>
                      {ev.kind}
                    </span>
                  </span>
                  <span className="truncate">{ev.shortname ?? "—"}</span>
                  <span className="truncate">{ev.unitId ?? "—"}</span>
                  <span className="truncate">{ev.unitAlpha ?? "—"}</span>
                  <span className="truncate">{ev.talkgroupAlpha ?? "—"}</span>
                  <span className="truncate">{ev.talkgroupGroup ?? "—"}</span>
                  <span className="truncate">{ev.talkgroupId ?? "—"}</span>
                  <span className="truncate">{fmtFreqMHz(ev.freq)}</span>
                  <span className="truncate text-base-content/60">
                    {notes.join(" · ")}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
