// Trunking message feed — virtualised, last N `tr.message` frames.
import { useRef, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTrMessages } from "./useTrMqtt";
import type { TrInstance } from "./types";

function fmtTime(ms: number): string {
  return new Date(ms).toLocaleTimeString();
}

export default function TrunkingMessagesView({
  instance,
}: {
  instance: TrInstance;
}) {
  const msgs = useTrMessages(instance.id);
  const ordered = useMemo(() => msgs.slice().reverse(), [msgs]);

  const parentRef = useRef<HTMLDivElement>(null);
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: ordered.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 28,
    overscan: 8,
  });

  return (
    <div className="space-y-3">
      <h3 className="text-lg font-semibold">
        Trunking messages ({ordered.length})
      </h3>

      {ordered.length === 0 ? (
        <div className="text-xs text-base-content/50">No messages yet.</div>
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
              const m = ordered[vi.index];
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
                  className="grid grid-cols-[80px_120px_60px_1fr] gap-2 px-2 py-1 text-xs border-b border-base-300/50 font-mono"
                >
                  <span className="text-base-content/60">{fmtTime(m.at)}</span>
                  <span>{m.shortname ?? "—"}</span>
                  <span>{m.opcode ?? "—"}</span>
                  <span className="truncate">
                    {m.opcodeDesc ??
                      m.type ??
                      JSON.stringify(m.raw).slice(0, 120)}
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
