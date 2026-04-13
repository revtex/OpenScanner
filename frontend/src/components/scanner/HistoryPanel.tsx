import type { Call } from "@/types";

interface HistoryPanelProps {
  history: Call[];
  time12hFormat: boolean;
}

export function HistoryPanel({ history, time12hFormat }: HistoryPanelProps) {
  const formatTime = (ts: number) => {
    const d = new Date(ts * 1000);
    return d.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      hour12: time12hFormat,
    });
  };

  return (
    <div className="mt-2 border-t border-current/20 pt-1 min-h-[120px]">
      {/* Rows */}
      {history.slice(0, 5).map((call) => (
        <div
          key={call.id}
          className="px-1 py-0.5 border-b border-current/10 last:border-b-0 history-row"
        >
          {/* Line 1: talkgroup name + time */}
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-xs">
              {call.talkgroupName || call.talkgroupLabel || ""}
            </span>
            <span className="shrink-0 text-xs opacity-60">
              {formatTime(call.dateTime)}
            </span>
          </div>
          {/* Line 2: system · talkgroup label */}
          <div className="flex items-center gap-1 text-[10px] opacity-40">
            <span className="truncate">{call.systemLabel ?? ""}</span>
            {call.talkgroupLabel && call.talkgroupName && (
              <>
                <span>·</span>
                <span className="truncate">{call.talkgroupLabel}</span>
              </>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
