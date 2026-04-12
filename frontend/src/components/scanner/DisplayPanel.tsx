import { useState, useEffect, useCallback } from "react";
import { Share2 } from "lucide-react";
import { BookmarkButton } from "@/components/scanner/BookmarkButton";
import { useGetBookmarkIDsQuery, useToggleBookmarkMutation } from "@/app/api";
import { HistoryPanel } from "@/components/scanner/HistoryPanel";
import { TranscriptPanel } from "@/components/scanner/TranscriptPanel";
import type { Call } from "@/types";

interface DisplayPanelProps {
  currentCall: Call | null;
  history: Call[];
  listenerCount: number;
  queueCount: number;
  avoidList: { talkgroupId: number }[];
  time12hFormat: boolean;
  showListenersCount: boolean;
  isAuthenticated: boolean;
}

function useClock() {
  const [time, setTime] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(id);
  }, []);
  return time;
}

function formatClock(d: Date, hour12: boolean) {
  return d.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12,
  });
}

function formatCallTime(ts: number, hour12: boolean) {
  const d = new Date(ts * 1000);
  return d.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    hour12,
  });
}

function formatFrequency(hz?: number) {
  if (!hz) return "";
  const str = hz.toString();
  const spaced = str.replace(/\B(?=(\d{3})+(?!\d))/g, " ");
  return `F: ${spaced} Hz`;
}

export function DisplayPanel({
  currentCall,
  history,
  listenerCount,
  queueCount,
  avoidList,
  time12hFormat,
  showListenersCount,
  isAuthenticated,
}: DisplayPanelProps) {
  const clock = useClock();
  const [fullscreen, setFullscreen] = useState(false);

  const { data: bookmarkData } = useGetBookmarkIDsQuery(undefined, {
    skip: !isAuthenticated,
  });
  const [toggleBookmark] = useToggleBookmarkMutation();
  const bookmarkedCallIds = bookmarkData?.callIds ?? [];

  const handleDoubleClick = useCallback(() => {
    setFullscreen((prev) => !prev);
  }, []);

  const handleShare = useCallback(() => {
    if (!currentCall) return;
    const url = `${window.location.origin}/call/${currentCall.id}`;
    void navigator.clipboard.writeText(url).catch(() => {
      // Clipboard API may not be available in insecure contexts
    });
  }, [currentCall]);

  const handleToggleBookmark = useCallback(
    (callId: number) => {
      if (!isAuthenticated) return;
      void toggleBookmark(callId);
    },
    [isAuthenticated, toggleBookmark],
  );

  const isAvoided = currentCall
    ? avoidList.some((a) => a.talkgroupId === currentCall.talkgroupId)
    : false;

  const displayContent = (
    <div className="font-mono text-sm leading-5 p-3 min-h-[200px]">
      {/* Row 1: clock, listeners, queue */}
      <div className="flex justify-between">
        <span>{formatClock(clock, time12hFormat)}</span>
        {showListenersCount && <span>L: {listenerCount}</span>}
        <span>Q: {queueCount}</span>
      </div>

      {currentCall ? (
        <>
          {/* Row 3: system label, tag */}
          <div className="flex justify-between">
            <span className="truncate">{currentCall.systemLabel ?? ""}</span>
            <span className="opacity-60">{currentCall.talkgroupTag ?? ""}</span>
          </div>

          {/* Row 4: TG group/label, call time */}
          <div className="flex justify-between">
            <span className="truncate">
              {[currentCall.talkgroupGroup, currentCall.talkgroupLabel]
                .filter(Boolean)
                .join(" · ")}
            </span>
            <span className="opacity-60">
              {formatCallTime(currentCall.dateTime, time12hFormat)}
            </span>
          </div>

          {/* Row 5: TG name — large */}
          <div className="text-2xl font-bold text-center py-1 truncate">
            {currentCall.talkgroupName ?? ""}
          </div>

          {/* Row 6: frequency, TGID */}
          <div className="flex justify-between">
            <span>{formatFrequency(currentCall.frequency)}</span>
            <span>TGID: {currentCall.talkgroupId}</span>
          </div>

          {/* Row 7: site/decoder, unit ID */}
          <div className="flex justify-between">
            <span className="truncate opacity-60">
              {[currentCall.site, currentCall.decoder]
                .filter(Boolean)
                .join(" · ")}
            </span>
            <span>
              {currentCall.source ? `UID: ${currentCall.source}` : ""}
            </span>
          </div>

          {/* Row 8: bookmark, share, flags */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1">
              <BookmarkButton
                isBookmarked={bookmarkedCallIds.includes(currentCall.id)}
                onToggle={() => handleToggleBookmark(currentCall.id)}
              />
              <button
                className="btn btn-ghost btn-xs btn-circle opacity-50 hover:opacity-100"
                onClick={handleShare}
                aria-label="Share call"
              >
                <Share2 className="w-4 h-4" />
              </button>
            </div>
            <div className="flex gap-1">
              {isAvoided && (
                <span className="badge badge-xs bg-base-300 text-base-content">
                  AVOID
                </span>
              )}
              {currentCall.patches && (
                <span className="badge badge-xs bg-base-300 text-base-content">
                  PATCH
                </span>
              )}
            </div>
          </div>
        </>
      ) : (
        /* Idle state */
        <>
          <div className="text-2xl font-bold text-center py-4 opacity-30">
            OPENSCANNER
          </div>
        </>
      )}

      {/* Transcript */}
      <TranscriptPanel call={currentCall} />

      {/* History */}
      <HistoryPanel
        history={history}
        currentCallId={currentCall?.id ?? null}
        time12hFormat={time12hFormat}
      />
    </div>
  );

  return (
    <>
      <div
        className="bg-base-200 rounded-lg shadow-inner"
        onDoubleClick={handleDoubleClick}
      >
        {displayContent}
      </div>

      {/* Fullscreen modal */}
      {fullscreen && (
        <dialog className="modal modal-open" onClick={handleDoubleClick}>
          <div
            className="modal-box max-w-3xl bg-base-200"
            onClick={(e) => e.stopPropagation()}
          >
            {displayContent}
          </div>
          <form method="dialog" className="modal-backdrop">
            <button onClick={handleDoubleClick}>close</button>
          </form>
        </dialog>
      )}
    </>
  );
}
