import { useState, useEffect, useCallback } from 'react';
import { Share2 } from 'lucide-react';
import { BookmarkButton } from '@/components/scanner/BookmarkButton';
import { HistoryPanel } from '@/components/scanner/HistoryPanel';
import { TranscriptPanel } from '@/components/scanner/TranscriptPanel';
import type { Call } from '@/types';

interface DisplayPanelProps {
  currentCall: Call | null;
  history: Call[];
  listenerCount: number;
  queueCount: number;
  avoidList: { talkgroupId: number }[];
}

function useClock() {
  const [time, setTime] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(id);
  }, []);
  return time;
}

function formatClock(d: Date) {
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
}

function formatCallDateTime(ts: number) {
  const d = new Date(ts * 1000);
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  return `${month}/${day} ${time}`;
}

function formatFrequency(hz?: number) {
  if (!hz) return '';
  return `F: ${(hz / 1_000_000).toFixed(3)}`;
}

export function DisplayPanel({ currentCall, history, listenerCount, queueCount, avoidList }: DisplayPanelProps) {
  const clock = useClock();
  const [fullscreen, setFullscreen] = useState(false);
  const [bookmarked, setBookmarked] = useState<Set<number>>(new Set());

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

  const toggleBookmark = useCallback((callId: number) => {
    setBookmarked((prev) => {
      const next = new Set(prev);
      if (next.has(callId)) {
        next.delete(callId);
      } else {
        next.add(callId);
      }
      return next;
    });
  }, []);

  const isAvoided = currentCall
    ? avoidList.some((a) => a.talkgroupId === currentCall.talkgroupId)
    : false;

  const displayContent = (
    <div className="font-mono text-sm leading-5 p-3 min-h-[200px]">
      {/* Row 1: clock, listeners, queue */}
      <div className="flex justify-between">
        <span>{formatClock(clock)}</span>
        <span className="flex gap-4">
          <span>L: {listenerCount}</span>
          <span>Q: {queueCount}</span>
        </span>
      </div>

      {/* Row 2: spacer */}
      <div className="h-5" />

      {currentCall ? (
        <>
          {/* Row 3: system label, tag */}
          <div className="flex justify-between">
            <span className="truncate">{currentCall.systemLabel ?? ''}</span>
            <span className="opacity-60">{currentCall.talkgroupTag ?? ''}</span>
          </div>

          {/* Row 4: TG label, date+time */}
          <div className="flex justify-between">
            <span className="truncate">{currentCall.talkgroupLabel ?? ''}</span>
            <span className="opacity-60 text-xs">{formatCallDateTime(currentCall.dateTime)}</span>
          </div>

          {/* Row 5: TG name — large */}
          <div className="text-2xl font-bold text-center py-1 truncate">
            {currentCall.talkgroupName ?? ''}
          </div>

          {/* Row 6: frequency, TGID */}
          <div className="flex justify-between">
            <span>{formatFrequency(currentCall.frequency)}</span>
            <span>TG: {currentCall.talkgroupId}</span>
          </div>

          {/* Row 7: errors/spikes, unit ID */}
          <div className="flex justify-between">
            <span />
            <span>{currentCall.source ? `UID: ${currentCall.source}` : ''}</span>
          </div>

          {/* Row 8: bookmark, share, flags */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1">
              <BookmarkButton
                callId={currentCall.id}
                isBookmarked={bookmarked.has(currentCall.id)}
                onToggle={() => toggleBookmark(currentCall.id)}
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
                <span className="badge badge-xs bg-base-300 text-base-content">AVOID</span>
              )}
              {currentCall.patches && (
                <span className="badge badge-xs bg-base-300 text-base-content">PATCH</span>
              )}
            </div>
          </div>
        </>
      ) : (
        /* Idle state */
        <>
          <div className="h-5" />
          <div className="h-5" />
          <div className="text-2xl font-bold text-center py-1 opacity-30">
            OPENSCANNER
          </div>
          <div className="h-5" />
          <div className="h-5" />
          <div className="h-5" />
        </>
      )}

      {/* Transcript */}
      <TranscriptPanel call={currentCall} />

      {/* History */}
      <HistoryPanel history={history} currentCallId={currentCall?.id ?? null} />
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
          <div className="modal-box max-w-3xl bg-base-200" onClick={(e) => e.stopPropagation()}>
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
