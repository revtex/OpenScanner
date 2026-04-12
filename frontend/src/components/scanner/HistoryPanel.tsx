import type { Call } from "@/types";

interface HistoryPanelProps {
  history: Call[];
  currentCallId: number | null;
  time12hFormat: boolean;
}

export function HistoryPanel({
  history,
  currentCallId,
  time12hFormat,
}: HistoryPanelProps) {
  const formatTime = (ts: number) => {
    const d = new Date(ts * 1000);
    return d.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      hour12: time12hFormat,
    });
  };

  return (
    <div className="mt-2 border-t border-base-content/20 pt-1">
      {/* Header */}
      <div className="grid grid-cols-[10%_25%_25%_40%] text-xs opacity-40 px-1">
        <span>Time</span>
        <span>System</span>
        <span>Talkgroup</span>
        <span>Name</span>
      </div>
      {/* Rows */}
      {history.map((call) => (
        <div
          key={call.id}
          className={`grid grid-cols-[10%_25%_25%_40%] px-1 border-b border-base-content/20 last:border-b-0 history-row ${
            call.id === currentCallId ? "font-bold" : ""
          }`}
        >
          <span className="truncate">{formatTime(call.dateTime)}</span>
          <span className="truncate">{call.systemLabel ?? ""}</span>
          <span className="truncate">{call.talkgroupLabel ?? ""}</span>
          <span className="truncate">{call.talkgroupName ?? ""}</span>
        </div>
      ))}
    </div>
  );
}
