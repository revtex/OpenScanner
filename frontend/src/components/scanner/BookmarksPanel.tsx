import { useGetBookmarkCallsQuery, useToggleBookmarkMutation } from "@/app/api";
import { useAppSelector } from "@/app/store";
import { selectToken } from "@/app/slices/authSlice";
import { audioPlayer } from "@/services/audioPlayer";
import { X, Play, Download, Star } from "lucide-react";
import type { Call } from "@/types";

interface BookmarksPanelProps {
  isOpen: boolean;
  onClose: () => void;
}

function formatDuration(secs: number): string {
  const minutes = Math.floor(secs / 60);
  const seconds = secs % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
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
    systemLabel: bc.systemLabel,
    talkgroupLabel: bc.talkgroupLabel,
    talkgroupName: bc.talkgroupName,
    talkgroupLedColor: bc.talkgroupLed,
    audioUrl: `/api/calls/${bc.id}/audio`,
  };
}

export default function BookmarksPanel({
  isOpen,
  onClose,
}: BookmarksPanelProps) {
  const token = useAppSelector(selectToken);

  const { data: bookmarkData, isLoading } = useGetBookmarkCallsQuery(
    undefined,
    { skip: !token },
  );

  const [toggleBookmark] = useToggleBookmarkMutation();

  const bookmarkedCalls = bookmarkData?.calls ?? [];

  const handlePlay = (bc: BookmarkCall) => {
    const call = bookmarkCallToCall(bc);
    const audioUrl = `/api/calls/${bc.id}/audio`;
    audioPlayer.play(call, audioUrl);
  };

  const handleUnbookmark = (callId: number) => {
    toggleBookmark(callId);
  };

  return (
    <div
      className={`fixed inset-y-0 right-0 z-50 w-80 max-w-full bg-base-200 shadow-xl transform transition-transform duration-300 ${
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
            className="flex items-center gap-2 px-4 py-3 border-b border-base-300 hover:bg-base-300/50"
          >
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium truncate">
                {call.talkgroupLabel}
              </div>
              <div className="text-xs text-base-content/60 truncate">
                {call.systemLabel}
              </div>
              <div className="text-xs text-base-content/40">
                {formatDate(call.dateTime)}
                {call.duration != null &&
                  call.duration > 0 &&
                  ` · ${formatDuration(call.duration)}`}
              </div>
            </div>
            <div className="flex items-center gap-1">
              <button
                onClick={() => handlePlay(call)}
                className="btn btn-ghost btn-xs btn-square"
                aria-label="Play call"
              >
                <Play className="w-3.5 h-3.5" />
              </button>
              <a
                href={`/api/calls/${call.id}/audio`}
                download
                className="btn btn-ghost btn-xs btn-square"
                aria-label="Download call"
              >
                <Download className="w-3.5 h-3.5" />
              </a>
              <button
                onClick={() => handleUnbookmark(call.id)}
                className="btn btn-ghost btn-xs btn-square text-warning"
                aria-label="Remove bookmark"
              >
                <Star className="w-3.5 h-3.5 fill-current" />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
