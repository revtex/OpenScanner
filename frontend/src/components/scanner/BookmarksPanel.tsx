import { useState } from "react";
import { useGetBookmarkCallsQuery, useToggleBookmarkMutation } from "@/app/api";
import { useAppSelector } from "@/app/store";
import { selectToken } from "@/app/slices/shared/authSlice";
import { audioPlayer } from "@/shared/services/audio/player";
import { sanitizeDownloadFilename } from "@/shared/services/download/filename";
import { ShareCallButton } from "@/components/scanner/ShareCallButton";
import { X, Play, Download, Star, ChevronDown } from "lucide-react";
import type { Call } from "@/types";

interface BookmarksPanelProps {
  isOpen: boolean;
  onClose: () => void;
}

function formatDate(unix: number): string {
  return new Date(unix * 1000).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

interface BookmarkCall {
  id: number;
  audioName: string;
  audioType: string;
  dateTime: number;
  systemId: number;
  talkgroupId: number;
  systemLabel: string;
  talkgroupLabel: string;
  talkgroupName: string;
  talkgroupLed: string;
  frequency?: number;
  duration?: number;
  source?: number;
  site?: string;
  channel?: string;
  decoder?: string;
  errorCount?: number;
  spikeCount?: number;
  transcript?: string;
  bookmarked: boolean;
}

function bookmarkCallToCall(bc: BookmarkCall): Call {
  return {
    id: bc.id,
    audioName: bc.audioName,
    audioType: bc.audioType,
    dateTime: bc.dateTime,
    systemId: bc.systemId,
    system: 0,
    talkgroupId: bc.talkgroupId,
    talkgroup: 0,
    frequency: bc.frequency,
    duration: bc.duration,
    source: bc.source,
    site: bc.site,
    channel: bc.channel,
    decoder: bc.decoder,
    errorCount: bc.errorCount,
    spikeCount: bc.spikeCount,
    systemLabel: bc.systemLabel,
    talkgroupLabel: bc.talkgroupLabel,
    talkgroupName: bc.talkgroupName,
    talkgroupLedColor: bc.talkgroupLed,
    audioUrl: `/api/v1/calls/${bc.id}/audio`,
  };
}

export default function BookmarksPanel({
  isOpen,
  onClose,
}: BookmarksPanelProps) {
  const token = useAppSelector(selectToken);
  const shareableLinks = useAppSelector(
    (s) => s.scanner.config?.shareableLinks ?? false,
  );

  const { data: bookmarkData, isLoading } = useGetBookmarkCallsQuery(
    undefined,
    { skip: !token },
  );

  const [toggleBookmark] = useToggleBookmarkMutation();
  const [expandedTranscripts, setExpandedTranscripts] = useState(
    () => new Set<number>(),
  );

  const bookmarkedCalls = bookmarkData?.calls ?? [];

  const handlePlay = (bc: BookmarkCall) => {
    const call = bookmarkCallToCall(bc);
    audioPlayer.playNow(call);
  };

  const handleUnbookmark = (callId: number) => {
    toggleBookmark(callId);
  };

  const handleDownload = (bc: BookmarkCall) => {
    const a = document.createElement("a");
    a.href = `/api/v1/calls/${bc.id}/audio`;
    a.download = sanitizeDownloadFilename(bc.audioName, `call-${bc.id}.mp3`);
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  return (
    <>
      {/* Backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50"
          onClick={onClose}
          aria-hidden
        />
      )}

      <div
        className={`fixed inset-y-0 right-0 z-50 w-full sm:w-125 max-w-full bg-base-100 shadow-xl transform transition-transform duration-300 ${
          isOpen ? "translate-x-0" : "translate-x-full"
        }`}
      >
        <div className="flex items-center justify-between p-4 border-b border-base-300">
          <h2 className="text-lg font-bold">Bookmarks</h2>
          <button
            onClick={onClose}
            className="btn btn-ghost btn-sm btn-square"
            aria-label="Close bookmarks"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="overflow-y-auto h-[calc(100%-4rem)]">
          {isLoading && (
            <div className="flex justify-center p-8">
              <span className="loading loading-spinner loading-md" />
            </div>
          )}

          {!isLoading && bookmarkedCalls.length === 0 && (
            <div className="flex flex-col items-center justify-center p-8 text-base-content/50">
              <Star className="w-10 h-10 mb-2" />
              <p className="text-sm">No bookmarked calls</p>
            </div>
          )}

          {bookmarkedCalls.map((call) => (
            <div
              key={call.id}
              className="flex items-start gap-2 px-3 py-2 border-b border-base-300 hover:bg-base-200"
            >
              <div className="flex-1 min-w-0">
                {/* Row 1: talkgroup name */}
                <div className="text-xs font-medium truncate">
                  {call.talkgroupName || call.talkgroupLabel}
                </div>
                {/* Row 2: system */}
                <div className="text-[11px] text-base-content/60 truncate">
                  {call.systemLabel}
                </div>
                {/* Row 3: freq, UID, TGID */}
                <div className="flex items-center gap-2 text-[11px] text-base-content/40">
                  {call.frequency != null && call.frequency > 0 && (
                    <span>{(call.frequency / 1e6).toFixed(4)} MHz</span>
                  )}
                  {call.source != null && call.source > 0 && (
                    <span>UID: {call.source}</span>
                  )}
                  <span className="shrink-0">TGID: {call.talkgroupId}</span>
                  {call.errorCount != null && call.errorCount > 0 && (
                    <span>E:{call.errorCount}</span>
                  )}
                  {call.spikeCount != null && call.spikeCount > 0 && (
                    <span>S:{call.spikeCount}</span>
                  )}
                </div>
                {/* Row 4: transcript fold-down */}
                {call.transcript && (
                  <button
                    type="button"
                    className="flex items-center gap-1 text-[10px] text-base-content/50 hover:text-base-content/70 mt-0.5"
                    onClick={(e) => {
                      e.stopPropagation();
                      setExpandedTranscripts((prev) => {
                        const next = new Set(prev);
                        if (next.has(call.id)) next.delete(call.id);
                        else next.add(call.id);
                        return next;
                      });
                    }}
                  >
                    <ChevronDown
                      size={12}
                      className={`transition-transform ${
                        expandedTranscripts.has(call.id) ? "rotate-180" : ""
                      }`}
                    />
                    Transcription
                  </button>
                )}
                {call.transcript && expandedTranscripts.has(call.id) && (
                  <div className="text-[11px] italic text-base-content/60 whitespace-pre-wrap mt-0.5">
                    {call.transcript}
                  </div>
                )}
              </div>
              {/* Date/time + action buttons */}
              <div className="flex shrink-0 flex-col items-end gap-0.5">
                <span className="text-[11px] text-base-content/60">
                  {formatDate(call.dateTime)}
                </span>
                <div className="flex items-center gap-0.5">
                  <button
                    onClick={() => handlePlay(call)}
                    className="btn btn-ghost btn-xs btn-square"
                    aria-label="Play call"
                  >
                    <Play className="w-3 h-3" />
                  </button>
                  <button
                    onClick={() => handleDownload(call)}
                    className="btn btn-ghost btn-xs btn-square"
                    aria-label="Download call"
                  >
                    <Download className="w-3 h-3" />
                  </button>
                  <button
                    onClick={() => handleUnbookmark(call.id)}
                    className="btn btn-ghost btn-xs btn-square text-warning"
                    aria-label="Remove bookmark"
                  >
                    <Star className="w-3 h-3 fill-current" />
                  </button>
                  {shareableLinks && <ShareCallButton callId={call.id} />}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
