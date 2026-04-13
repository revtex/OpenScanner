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
      {/* Header */}
      <div className="grid grid-cols-[10%_25%_25%_40%] text-xs opacity-40 px-1 gap-x-2">
        <span>Time</span>
        <span>System</span>
        <span>Talkgroup</span>
        <span>Name</span>
      </div>
      {/* Rows */}
      {history.slice(0, 5).map((call) => (
        <div
          key={call.id}
          className="grid grid-cols-[10%_25%_25%_40%] px-1 gap-x-2 border-b border-current/20 last:border-b-0 history-row"
        >
          <span className="truncate">{formatTime(call.dateTime)}</span>
          <span className="truncate">{call.systemLabel ?? ""}</span>
          <span className="truncate">{call.talkgroupName ?? ""}</span>
          <span className="truncate">{call.talkgroupLabel ?? ""}</span>
        </div>
      ))}
    </div>
  );
}
