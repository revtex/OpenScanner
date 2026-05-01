// Recent unit events feed — virtualised, last N entries from `tr.unit.*` topics.
import { useRef, useMemo, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTrUnitEvents } from "./useTrMqtt";
import type { TrInstance } from "./types";

function fmtTime(ms: number): string {
  return new Date(ms).toLocaleTimeString();
}

export default function UnitsView({ instance }: { instance: TrInstance }) {
  const events = useTrUnitEvents(instance.id);
  const [filterSys, setFilterSys] = useState("");
  const [filterTG, setFilterTG] = useState("");

  const filtered = useMemo(() => {
    return events
      .filter((e) => !filterSys || (e.shortname ?? "").includes(filterSys))
      .filter((e) => !filterTG || (e.talkgroupId ?? "").includes(filterTG))
      .slice()
      .reverse(); // newest first
  }, [events, filterSys, filterTG]);

  const parentRef = useRef<HTMLDivElement>(null);
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 28,
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
          placeholder="filter TG"
          value={filterTG}
          onChange={(e) => setFilterTG(e.target.value)}
        />
      </div>

      {filtered.length === 0 ? (
        <div className="text-xs text-base-content/50">No unit events yet.</div>
      ) : (
        <div
          ref={parentRef}
          className="border border-base-300 rounded-md max-h-[480px] overflow-auto"
        >
          <div
            style={{
              height: virtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {virtualizer.getVirtualItems().map((vi) => {
              const ev = filtered[vi.index];
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
                  }}
                  className="grid grid-cols-[80px_60px_120px_80px_80px_1fr] gap-2 px-2 py-1 text-xs border-b border-base-300/50 font-mono"
                >
                  <span className="text-base-content/60">{fmtTime(ev.at)}</span>
                  <span className="badge badge-ghost badge-xs">{ev.kind}</span>
                  <span>{ev.shortname ?? "—"}</span>
                  <span>{ev.unitId ?? "—"}</span>
                  <span>{ev.talkgroupId ?? "—"}</span>
                  <span className="truncate text-base-content/50">
                    {JSON.stringify(ev.raw)}
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
