// Trunking message feed — virtualised, last N `tr.message` frames.
// Surfaces opcode + opcode_type + decoded meta line from the plugin.
import { useRef, useMemo, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTrMessages } from "./useTrMqtt";
import type { TrInstance } from "./types";
import { fmtTime } from "./format";

export default function TrunkingMessagesView({
  instance,
}: {
  instance: TrInstance;
}) {
  const msgs = useTrMessages(instance.id);
  const [filterSys, setFilterSys] = useState("");
  const [filterType, setFilterType] = useState("");

  const ordered = useMemo(() => {
    return msgs
      .filter(
        (m) =>
          !filterSys ||
          (m.shortname ?? "")
            .toLowerCase()
            .includes(filterSys.toLowerCase()),
      )
      .filter(
        (m) =>
          !filterType ||
          (m.type ?? "").toLowerCase().includes(filterType.toLowerCase()) ||
          (m.opcodeType ?? "")
            .toLowerCase()
            .includes(filterType.toLowerCase()),
      )
      .slice()
      .reverse();
  }, [msgs, filterSys, filterType]);

  const parentRef = useRef<HTMLDivElement>(null);
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: ordered.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 32,
    overscan: 8,
  });

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 flex-wrap">
        <h3 className="text-lg font-semibold mr-auto">
          Trunking messages ({ordered.length})
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
          placeholder="filter type / opcode"
          value={filterType}
          onChange={(e) => setFilterType(e.target.value)}
        />
      </div>

      <div
        className="grid gap-2 px-2 py-1 text-[11px] uppercase tracking-wide text-base-content/60 font-semibold"
        style={{
          gridTemplateColumns: "80px 110px 110px 60px 130px 1fr",
        }}
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
          <div
            style={{
              height: virtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {virtualizer.getVirtualItems().map((vi) => {
              const m = ordered[vi.index];
              const meta =
                m.meta ??
                m.opcodeDesc ??
                m.trunkMsg ??
                JSON.stringify(m.raw).slice(0, 160);
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
                      "80px 110px 110px 60px 130px 1fr",
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
    </div>
  );
}
