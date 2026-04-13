import { useState, useEffect, useCallback } from "react";
import { Share2, Sun } from "lucide-react";
import { BookmarkButton } from "@/components/scanner/BookmarkButton";
import { useGetBookmarkIDsQuery, useToggleBookmarkMutation } from "@/app/api";
import { useShareCallMutation } from "@/app/slices/shareSlice";
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
  shareableLinks: boolean;
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
  shareableLinks,
  isAuthenticated,
}: DisplayPanelProps) {
  const clock = useClock();
  const [brightness, setBrightness] = useState(() => {
    const saved = localStorage.getItem("lcd-brightness");
    return saved ? Number(saved) : 50;
  });
  const [showBrightness, setShowBrightness] = useState(false);

  const handleBrightness = useCallback((val: number) => {
    setBrightness(val);
    localStorage.setItem("lcd-brightness", String(val));
  }, []);

  const { data: bookmarkData } = useGetBookmarkIDsQuery(undefined, {
    skip: !isAuthenticated,
  });
  const [toggleBookmark] = useToggleBookmarkMutation();
  const [shareCall] = useShareCallMutation();
  const bookmarkedCallIds = bookmarkData?.callIds ?? [];
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const handleShare = useCallback(async () => {
    if (!currentCall) return;
    try {
      const result = await shareCall(currentCall.id).unwrap();
      const url = `${window.location.origin}${result.url}`;
      await navigator.clipboard.writeText(url);
      setToastMessage("Link copied to clipboard");
      setTimeout(() => setToastMessage(null), 3000);
    } catch {
      setToastMessage("Failed to share call");
      setTimeout(() => setToastMessage(null), 3000);
    }
  }, [currentCall, shareCall]);

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
    <div className="font-mono text-sm leading-5 p-3 h-[420px] flex flex-col">
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
              <div className="relative flex items-center">
                <button
                  className="btn btn-ghost btn-xs btn-circle opacity-50 hover:opacity-50"
                  onClick={() => setShowBrightness((p) => !p)}
                  aria-label="Adjust brightness"
                >
                  <Sun className="w-4 h-4" />
                </button>
                {showBrightness && (
                  <input
                    type="range"
                    min={20}
                    max={120}
                    value={brightness}
                    onChange={(e) => handleBrightness(Number(e.target.value))}
                    className="brightness-slider ml-1"
                    aria-label="Display brightness"
                  />
                )}
              </div>
              <BookmarkButton
                isBookmarked={bookmarkedCallIds.includes(currentCall.id)}
                onToggle={() => handleToggleBookmark(currentCall.id)}
              />
              {shareableLinks && (
                <button
                  className="btn btn-ghost btn-xs btn-circle opacity-50 hover:opacity-50"
                  onClick={handleShare}
                  aria-label="Share call"
                >
                  <Share2 className="w-4 h-4" />
                </button>
              )}
            </div>
            <div className="flex items-center gap-1">
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
              <span className="opacity-50 text-xs">
                {currentCall.errorCount != null
                  ? `E: ${currentCall.errorCount}`
                  : ""}
                {currentCall.errorCount != null &&
                currentCall.spikeCount != null
                  ? " "
                  : ""}
                {currentCall.spikeCount != null
                  ? `S: ${currentCall.spikeCount}`
                  : ""}
              </span>
            </div>
          </div>
        </>
      ) : (
        /* Idle state — same row structure to keep constant height */
        <>
          {/* Row 3: system label, tag */}
          <div className="flex justify-between invisible">
            <span>&nbsp;</span>
          </div>

          {/* Row 4: TG group/label, call time */}
          <div className="flex justify-between invisible">
            <span>&nbsp;</span>
          </div>

          {/* Row 5: TG name — large */}
          <div className="text-2xl font-bold text-center py-1 opacity-30">
            OPENSCANNER
          </div>

          {/* Row 6: frequency, TGID */}
          <div className="flex justify-between invisible">
            <span>&nbsp;</span>
          </div>

          {/* Row 7: site/decoder, unit ID */}
          <div className="flex justify-between invisible">
            <span>&nbsp;</span>
          </div>

          {/* Row 8: bookmark, share, flags */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1">
              <div className="relative flex items-center">
                <button
                  className="btn btn-ghost btn-xs btn-circle opacity-50 hover:opacity-50"
                  onClick={() => setShowBrightness((p) => !p)}
                  aria-label="Adjust brightness"
                >
                  <Sun className="w-4 h-4" />
                </button>
                {showBrightness && (
                  <input
                    type="range"
                    min={20}
                    max={120}
                    value={brightness}
                    onChange={(e) => handleBrightness(Number(e.target.value))}
                    className="brightness-slider ml-1"
                    aria-label="Display brightness"
                  />
                )}
              </div>
            </div>
            <span>&nbsp;</span>
          </div>
        </>
      )}

      {/* Transcript */}
      <TranscriptPanel call={currentCall} />

      {/* History */}
      <HistoryPanel history={history} time12hFormat={time12hFormat} />
    </div>
  );

  return (
    <>
      <div
        className="lcd-display rounded-lg"
        style={{ filter: `brightness(${brightness / 100})` }}
      >
        {displayContent}
      </div>

      {/* Toast notification */}
      {toastMessage && (
        <div className="toast toast-end toast-bottom z-50">
          <div className="alert alert-info">
            <span>{toastMessage}</span>
          </div>
        </div>
      )}
    </>
  );
}
