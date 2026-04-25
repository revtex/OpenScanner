import {
  useState,
  useEffect,
  useLayoutEffect,
  useCallback,
  useMemo,
  useRef,
} from "react";
import { Share2, Sun, Copy, X, ExternalLink } from "lucide-react";
import { BookmarkButton } from "@/components/scanner/BookmarkButton";
import { useGetBookmarkIDsQuery, useToggleBookmarkMutation } from "@/app/api";
import { useShareCallMutation } from "@/app/slices/scanner/shareSlice";
import { HistoryPanel } from "@/components/scanner/HistoryPanel";
import { TranscriptPanel } from "@/components/scanner/TranscriptPanel";
import { useActiveUnit } from "@/hooks/scanner/useActiveUnit";
import { useAppSelector } from "@/app/store";
import type { AvoidEntry, Call } from "@/types";

interface DisplayPanelProps {
  currentCall: Call | null;
  history: Call[];
  heldSystem: number | null;
  heldTG: number | null;
  listenerCount: number;
  queueCount: number;
  avoidList: AvoidEntry[];
  time12hFormat: boolean;
  showListenersCount: boolean;
  shareableLinks: boolean;
  isAuthenticated: boolean;
  isLive: boolean;
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

function AutoSizeText({
  text,
  className,
}: {
  text: string;
  className?: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const textRef = useRef<HTMLSpanElement>(null);

  useLayoutEffect(() => {
    const container = containerRef.current;
    const textEl = textRef.current;
    if (!container || !textEl) return;
    // Reset to natural size before measuring.
    textEl.style.transform = "";
    textEl.style.display = "inline-block";
    textEl.style.transformOrigin = "left center";
    const textWidth = textEl.offsetWidth;
    const containerWidth = container.clientWidth;
    if (textWidth > containerWidth) {
      const scale = containerWidth / textWidth;
      textEl.style.transform = `scaleX(${scale})`;
    }
  }, [text]);

  return (
    <div
      ref={containerRef}
      className={`overflow-hidden whitespace-nowrap ${className ?? ""}`}
    >
      <span ref={textRef}>{text}</span>
    </div>
  );
}

export function DisplayPanel({
  currentCall,
  history,
  heldSystem,
  heldTG,
  listenerCount,
  queueCount,
  avoidList,
  time12hFormat,
  showListenersCount,
  shareableLinks,
  isAuthenticated,
  isLive,
}: DisplayPanelProps) {
  const clock = useClock();
  const liveTranscriptDisplay = useAppSelector(
    (s) => s.scanner.config?.liveTranscriptDisplay ?? false,
  );
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
  const [shareUrl, setShareUrl] = useState<string | null>(null);

  // Tick once per second while any timed avoid is active so `isAvoided`
  // expires on its own without waiting for an external state change.
  const [nowTick, setNowTick] = useState(() => Date.now());
  const hasTimedAvoids = useMemo(
    () => avoidList.some((a) => a.expiresAt > 0),
    [avoidList],
  );
  useEffect(() => {
    if (!hasTimedAvoids) return;
    const id = setInterval(() => setNowTick(Date.now()), 1000);
    return () => clearInterval(id);
  }, [hasTimedAvoids]);

  const copyToClipboard = useCallback(
    async (text: string): Promise<boolean> => {
      if (navigator.clipboard?.writeText) {
        try {
          await navigator.clipboard.writeText(text);
          return true;
        } catch {
          // Fall through to legacy copy for non-secure contexts or blocked permissions.
        }
      }

      try {
        const textarea = document.createElement("textarea");
        textarea.value = text;
        textarea.setAttribute("readonly", "");
        textarea.style.position = "fixed";
        textarea.style.opacity = "0";
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        const copied = document.execCommand("copy");
        document.body.removeChild(textarea);
        return copied;
      } catch {
        return false;
      }
    },
    [],
  );

  const handleShare = useCallback(async () => {
    if (!currentCall) return;
    try {
      const result = await shareCall(currentCall.id).unwrap();
      const url = `${window.location.origin}${result.url}`;
      setShareUrl(url);
    } catch {
      setToastMessage("Failed to share call");
      setTimeout(() => setToastMessage(null), 3000);
    }
  }, [currentCall, shareCall]);

  const handleCopyShareUrl = useCallback(async () => {
    if (!shareUrl) return;
    const copied = await copyToClipboard(shareUrl);
    if (copied) {
      setToastMessage("Link copied to clipboard");
    } else {
      setToastMessage("Copy failed - long-press URL to copy");
    }
    setTimeout(() => setToastMessage(null), 3000);
  }, [copyToClipboard, shareUrl]);

  const handleToggleBookmark = useCallback(
    (callId: number) => {
      if (!isAuthenticated) return;
      void toggleBookmark(callId);
    },
    [isAuthenticated, toggleBookmark],
  );

  const isAvoided = useMemo(() => {
    if (!currentCall) return false;
    const nowMs = nowTick;
    return avoidList.some(
      (a) =>
        a.talkgroupId === currentCall.talkgroup &&
        (a.expiresAt === 0 || a.expiresAt > nowMs),
    );
  }, [currentCall, avoidList, nowTick]);

  const isHeld = currentCall
    ? heldTG === currentCall.talkgroup || heldSystem === currentCall.system
    : false;

  const activeUnit = useActiveUnit(currentCall?.sources);

  const displayContent = (
    <div className="font-mono text-sm leading-5 p-3 h-105 flex flex-col">
      {/* Row 1: clock, listeners, queue */}
      <div className="flex justify-between">
        <span>{formatClock(clock, time12hFormat)}</span>
        <div className="flex items-center gap-4">
          {showListenersCount && <span>L: {listenerCount}</span>}
          <span>Q: {queueCount}</span>
        </div>
      </div>

      {currentCall ? (
        <>
          {/* Row 3: system label, tag */}
          <div className="flex items-center justify-between gap-2">
            <span className="min-w-0 flex-1 truncate">
              {currentCall.systemLabel ?? ""}
            </span>
            <span className="shrink-0 whitespace-nowrap opacity-60">
              {currentCall.talkgroupTag ?? ""}
            </span>
          </div>

          {/* Row 4: TG group/label, call time */}
          <div className="flex justify-between">
            <span className="truncate">
              {[currentCall.talkgroupGroup, currentCall.talkgroupLabel]
                .filter(Boolean)
                .join(" · ")}
            </span>
            <span className="shrink-0 whitespace-nowrap opacity-60">
              {formatCallTime(currentCall.dateTime, time12hFormat)}
            </span>
          </div>

          {/* Row 5: TG name — large, auto-sized to fit */}
          <AutoSizeText
            text={
              currentCall.talkgroupName?.trim() ||
              currentCall.talkgroupLabel?.trim() ||
              `TGID: ${currentCall.talkgroupId}`
            }
            className="text-2xl font-bold text-center py-1"
          />

          {/* Row 6: frequency, TGID */}
          <div className="flex justify-between">
            <span>{formatFrequency(currentCall.frequency)}</span>
            <span>TGID: {currentCall.talkgroupId}</span>
          </div>

          {/* Row 7: site/decoder, unit ID / talker alias */}
          <div className="flex justify-between">
            <span className="truncate opacity-60">
              {[currentCall.site, currentCall.decoder]
                .filter(Boolean)
                .join(" · ")}
            </span>
            <span className="truncate text-right">
              {(() => {
                const uid = activeUnit?.src ?? currentCall.source;
                const alias = activeUnit?.tag || currentCall.talkerAlias;
                return [uid ? `UID: ${uid}` : "", alias]
                  .filter(Boolean)
                  .join(" · ");
              })()}
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
              {isAuthenticated && (
                <BookmarkButton
                  isBookmarked={bookmarkedCallIds.includes(currentCall.id)}
                  onToggle={() => handleToggleBookmark(currentCall.id)}
                />
              )}
              {isAuthenticated && shareableLinks && (
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
              {isHeld && (
                <span className="badge badge-xs bg-base-300 text-base-content">
                  HOLD
                </span>
              )}
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

          {/* Hint to enable LIVE when offline */}
          {!isLive && (
            <div className="text-center text-sm opacity-40 py-1">
              Tap LIVE to start listening
            </div>
          )}

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

      {/* Transcript (when enabled in admin) */}
      {liveTranscriptDisplay && <TranscriptPanel call={currentCall} />}

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

      {shareUrl && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-xl rounded-lg border border-base-300 bg-base-100 p-4 shadow-xl">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-sm font-semibold uppercase tracking-wide text-base-content/70">
                Share Link
              </h3>
              <button
                className="btn btn-ghost btn-xs btn-circle"
                onClick={() => setShareUrl(null)}
                aria-label="Close share popup"
              >
                <X size={14} />
              </button>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="text"
                readOnly
                value={shareUrl}
                className="input input-sm w-full"
                onFocus={(e) => e.currentTarget.select()}
                aria-label="Share URL"
              />
              <button
                className="btn btn-primary btn-sm btn-square"
                onClick={handleCopyShareUrl}
                aria-label="Copy share URL"
                title="Copy"
              >
                <Copy size={16} />
              </button>
              <button
                className="btn btn-ghost btn-sm btn-square"
                onClick={() => {
                  if (!shareUrl) return;
                  window.open(shareUrl, "_blank", "noopener,noreferrer");
                }}
                aria-label="Open share URL"
                title="Open"
              >
                <ExternalLink size={16} />
              </button>
            </div>
          </div>
        </div>
      )}

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
