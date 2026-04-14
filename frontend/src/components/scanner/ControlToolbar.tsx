import {
  Play,
  Pause,
  SkipForward,
  RotateCcw,
  Download,
  Star,
  Volume2,
  VolumeX,
  MoreHorizontal,
  Radio,
  Lock,
  Ban,
  List,
  Search,
  Keyboard,
  Maximize,
  Minimize,
} from "lucide-react";
import { useState, useCallback, useEffect } from "react";
import { playBeep } from "@/services/beepPlayer";
import type { AvoidEntry } from "@/types";

interface ControlToolbarProps {
  isPlaying: boolean;
  isPaused: boolean;
  isLive: boolean;
  volume: number;
  heldSystem: number | null;
  heldTG: number | null;
  avoidList: AvoidEntry[];
  currentCallTgId?: number;
  currentCallSystemId?: number;
  onTogglePause: () => void;
  onToggleLive: () => void;
  onSkip: () => void;
  onReplay: () => void;
  onDownload: () => void;
  onSetVolume: (v: number) => void;
  onHoldSystem: (id: number | null) => void;
  onHoldTG: (id: number | null) => void;
  onAddAvoid: (entry: AvoidEntry) => void;
  onClearAvoids: () => void;
  onToggleSelectTG: () => void;
  onToggleSearch: () => void;
  onToggleBookmarks?: () => void;
  keypadBeeps?: string;
}

export function ControlToolbar({
  isPlaying,
  isPaused,
  isLive,
  volume,
  heldSystem,
  heldTG,
  avoidList,
  currentCallTgId,
  currentCallSystemId,
  onTogglePause,
  onToggleLive,
  onSkip,
  onReplay,
  onDownload,
  onSetVolume,
  onHoldSystem,
  onHoldTG,
  onAddAvoid,
  onClearAvoids,
  onToggleSelectTG,
  onToggleSearch,
  onToggleBookmarks,
  keypadBeeps,
}: ControlToolbarProps) {
  const beep = useCallback(() => {
    if (keypadBeeps) playBeep(keypadBeeps);
  }, [keypadBeeps]);

  const isMuted = volume === 0;
  const isHolding = heldSystem !== null || heldTG !== null;
  const avoidCount = avoidList.length;
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);

  const handleAvoid = (minutes: number) => {
    if (!currentCallTgId) return;
    const expiresAt = minutes === 0 ? 0 : Date.now() + minutes * 60 * 1000;
    onAddAvoid({ talkgroupId: currentCallTgId, expiresAt });
  };

  const toggleFullscreen = useCallback(() => {
    if (document.fullscreenElement) {
      document.exitFullscreen();
    } else {
      document.documentElement.requestFullscreen();
    }
  }, []);

  useEffect(() => {
    const handler = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", handler);
    return () => document.removeEventListener("fullscreenchange", handler);
  }, []);

  return (
    <div className="fixed bottom-0 left-0 right-0 z-30 border-t border-base-300 bg-base-200/95 backdrop-blur-sm">
      <div className="max-w-2xl mx-auto space-y-1 px-4 py-2">
        {/* Row 1 — Playback + Quick Actions */}
        <div className="flex items-center justify-center gap-2 flex-wrap">
          {/* Play/Pause */}
          <div
            className="tooltip tooltip-top"
            data-tip={isPaused ? "Resume (Space)" : "Pause (Space)"}
          >
            <button
              className={`btn btn-circle w-10 h-10 ${isPaused ? "btn-warning" : "btn-primary"}`}
              onClick={() => {
                beep();
                onTogglePause();
              }}
              aria-label={isPaused ? "Resume" : "Pause"}
            >
              {isPaused ? (
                <Play className="w-5 h-5" />
              ) : (
                <Pause className="w-5 h-5" />
              )}
            </button>
          </div>

          {/* Skip */}
          <div className="tooltip tooltip-bottom" data-tip="Skip (S)">
            <button
              className="btn btn-circle btn-ghost w-9 h-9"
              onClick={() => {
                beep();
                onSkip();
              }}
              aria-label="Skip"
            >
              <SkipForward className="w-4 h-4" />
            </button>
          </div>

          {/* Replay */}
          <div className="tooltip tooltip-bottom" data-tip="Replay (R)">
            <button
              className="btn btn-circle btn-ghost w-9 h-9"
              onClick={() => {
                beep();
                onReplay();
              }}
              aria-label="Replay"
            >
              <RotateCcw className="w-4 h-4" />
            </button>
          </div>

          <div className="divider divider-horizontal mx-0" />

          {/* Volume — visible on md+ */}
          <div className="hidden md:flex items-center gap-1">
            <button
              className="btn btn-circle btn-ghost w-9 h-9"
              onClick={() => onSetVolume(isMuted ? 0.8 : 0)}
              aria-label={isMuted ? "Unmute" : "Mute"}
            >
              {isMuted ? (
                <VolumeX className="w-4 h-4" />
              ) : (
                <Volume2 className="w-4 h-4" />
              )}
            </button>
            <input
              type="range"
              min={0}
              max={1}
              step={0.05}
              value={volume}
              onChange={(e) => onSetVolume(Number(e.target.value))}
              className="range range-xs range-primary w-28"
            />
          </div>

          {/* Volume — sm only (icon with dropdown) */}
          <div className="md:hidden dropdown dropdown-top">
            <div
              tabIndex={0}
              role="button"
              className="btn btn-circle btn-ghost w-9 h-9"
            >
              {isMuted ? (
                <VolumeX className="w-4 h-4" />
              ) : (
                <Volume2 className="w-4 h-4" />
              )}
            </div>
            <div
              tabIndex={0}
              className="dropdown-content p-3 shadow bg-base-200 rounded-box"
            >
              <input
                type="range"
                min={0}
                max={1}
                step={0.05}
                value={volume}
                onChange={(e) => onSetVolume(Number(e.target.value))}
                className="range range-xs range-primary w-28"
              />
            </div>
          </div>

          <div className="divider divider-horizontal mx-0" />

          {/* Download */}
          <div className="tooltip tooltip-bottom" data-tip="Download (D)">
            <button
              className="btn btn-circle btn-ghost w-9 h-9"
              onClick={() => {
                beep();
                onDownload();
              }}
              aria-label="Download"
            >
              <Download className="w-4 h-4" />
            </button>
          </div>

          {/* Bookmarks panel toggle */}
          {onToggleBookmarks && (
            <div className="tooltip tooltip-bottom" data-tip="Bookmarks (B)">
              <button
                className="btn btn-circle btn-ghost w-9 h-9"
                onClick={() => {
                  beep();
                  onToggleBookmarks();
                }}
                aria-label="Bookmarks"
              >
                <Star className="w-4 h-4" />
              </button>
            </div>
          )}
        </div>

        {/* Row 2 — Mode Toggles */}
        <div className="flex items-center justify-center gap-2 flex-wrap">
          {/* LIVE */}
          <div className="tooltip tooltip-bottom" data-tip="Live Mode (L)">
            <button
              className={`btn btn-sm gap-1 ${isLive ? "btn-primary" : "btn-ghost"}`}
              onClick={() => {
                beep();
                onToggleLive();
              }}
            >
              <Radio className="w-3.5 h-3.5" />
              LIVE
              {isLive && isPlaying && (
                <span className="w-2 h-2 rounded-full bg-success animate-pulse" />
              )}
            </button>
          </div>

          {/* HOLD ▾ */}
          <div className="dropdown dropdown-bottom">
            <div
              tabIndex={0}
              role="button"
              className={`btn btn-sm ${isHolding ? "btn-secondary" : "btn-ghost"}`}
            >
              <Lock className="w-3.5 h-3.5" />
              HOLD▾
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu p-2 shadow bg-base-200 rounded-box w-48 z-50"
            >
              <li>
                <button
                  onClick={() =>
                    onHoldSystem(
                      heldSystem !== null
                        ? null
                        : (currentCallSystemId ?? null),
                    )
                  }
                >
                  {heldSystem !== null ? "Release System" : "Hold System"}
                </button>
              </li>
              <li>
                <button
                  onClick={() =>
                    onHoldTG(heldTG !== null ? null : (currentCallTgId ?? null))
                  }
                >
                  {heldTG !== null ? "Release Talkgroup" : "Hold Talkgroup"}
                </button>
              </li>
            </ul>
          </div>

          {/* AVOID ▾ */}
          <div className="dropdown dropdown-bottom">
            <div
              tabIndex={0}
              role="button"
              className={`btn btn-sm gap-1 ${avoidCount > 0 ? "btn-warning" : "btn-ghost"}`}
            >
              <Ban className="w-3.5 h-3.5" />
              AVOID▾
              {avoidCount > 0 && (
                <span className="badge badge-xs">{avoidCount}</span>
              )}
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu p-2 shadow bg-base-200 rounded-box w-44 z-50"
            >
              <li>
                <button onClick={() => handleAvoid(30)}>30 minutes</button>
              </li>
              <li>
                <button onClick={() => handleAvoid(60)}>60 minutes</button>
              </li>
              <li>
                <button onClick={() => handleAvoid(120)}>120 minutes</button>
              </li>
              <li>
                <button onClick={() => handleAvoid(0)}>Permanent</button>
              </li>
              {avoidCount > 0 && (
                <>
                  <li className="menu-title">
                    <span>—</span>
                  </li>
                  <li>
                    <button onClick={onClearAvoids}>Clear All</button>
                  </li>
                </>
              )}
            </ul>
          </div>

          {/* SELECT */}
          <div className="tooltip tooltip-bottom" data-tip="Select Talkgroups">
            <button
              className="btn btn-sm btn-ghost"
              onClick={() => {
                beep();
                onToggleSelectTG();
              }}
            >
              <List className="w-3.5 h-3.5" />
              SELECT▾
            </button>
          </div>

          {/* SEARCH */}
          <div className="tooltip tooltip-bottom" data-tip="Search Calls">
            <button
              className="btn btn-sm btn-ghost"
              onClick={() => {
                beep();
                onToggleSearch();
              }}
            >
              <Search className="w-3.5 h-3.5" />
              SEARCH
            </button>
          </div>

          {/* Overflow ⋯ */}
          <div className="dropdown dropdown-bottom dropdown-end">
            <div tabIndex={0} role="button" className="btn btn-sm btn-ghost">
              <MoreHorizontal className="w-4 h-4" />
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu p-2 shadow bg-base-200 rounded-box w-52 z-50"
            >
              <li>
                <button onClick={toggleFullscreen}>
                  {isFullscreen ? (
                    <Minimize className="w-4 h-4" />
                  ) : (
                    <Maximize className="w-4 h-4" />
                  )}{" "}
                  {isFullscreen ? "Exit Fullscreen" : "Fullscreen"}
                </button>
              </li>
              <li>
                <button onClick={() => setShortcutsOpen(true)}>
                  <Keyboard className="w-4 h-4" /> Keyboard Shortcuts
                </button>
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* Keyboard Shortcuts Modal */}
      {shortcutsOpen && (
        <dialog
          className="modal modal-open"
          onClick={() => setShortcutsOpen(false)}
        >
          <div
            className="modal-box max-w-md"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="font-semibold text-lg mb-4">Keyboard Shortcuts</h3>
            <div className="space-y-2 text-sm">
              {[
                ["Space", "Pause / Resume"],
                ["S", "Skip next"],
                ["R", "Replay last"],
                ["H", "Hold current TG"],
                ["J", "Hold current system"],
                ["A", "Avoid (cycle 30/60/120 min)"],
                ["F", "Toggle fullscreen"],
                ["← / →", "Volume down / up"],
                ["?", "Show shortcuts"],
                ["B", "Bookmark current call"],
                ["Esc", "Close any open panel"],
              ].map(([key, desc]) => (
                <div key={key} className="flex items-center justify-between">
                  <span>{desc}</span>
                  <kbd className="kbd kbd-sm">{key}</kbd>
                </div>
              ))}
            </div>
            <div className="modal-action">
              <button
                className="btn btn-sm"
                onClick={() => setShortcutsOpen(false)}
              >
                Close
              </button>
            </div>
          </div>
        </dialog>
      )}
    </div>
  );
}
