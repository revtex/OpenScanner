import {
  Play,
  Pause,
  SkipForward,
  RotateCcw,
  Star,
  Volume2,
  VolumeX,
  Radio,
  Lock,
  Ban,
  List,
  Search,
} from "lucide-react";
import { useCallback } from "react";
import { playBeep } from "@/services/beepPlayer";
import type { AvoidEntry } from "@/types";

interface ControlToolbarProps {
  isPaused: boolean;
  isLive: boolean;
  volume: number;
  heldSystem: number | null;
  heldTG: number | null;
  currentCallTgId?: number;
  currentCallSystemId?: number;
  onTogglePause: () => void;
  onToggleLive: () => void;
  onSkip: () => void;
  onReplay: () => void;
  onSetVolume: (v: number) => void;
  onHoldSystem: (id: number | null) => void;
  onHoldTG: (id: number | null) => void;
  onAddAvoid: (entry: AvoidEntry) => void;
  onToggleSelectTG: () => void;
  onToggleSearch: () => void;
  onToggleBookmarks?: () => void;
  keypadBeeps?: string;
}

export function ControlToolbar({
  isPaused,
  isLive,
  volume,
  heldSystem,
  heldTG,
  currentCallTgId,
  currentCallSystemId,
  onTogglePause,
  onToggleLive,
  onSkip,
  onReplay,
  onSetVolume,
  onHoldSystem,
  onHoldTG,
  onAddAvoid,
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

  const handleAvoid = (minutes: number) => {
    if (!currentCallTgId) return;
    const expiresAt = minutes === 0 ? 0 : Date.now() + minutes * 60 * 1000;
    onAddAvoid({ talkgroupId: currentCallTgId, expiresAt });
  };

  return (
    <div className="mt-4 space-y-2">
      {/* Row 1 — Playback + Quick Actions */}
      <div className="flex items-center justify-center gap-2 flex-wrap">
        {/* Play/Pause */}
        <div
          className="tooltip tooltip-bottom"
          data-tip={isPaused ? "Resume" : "Pause"}
        >
          <button
            className="btn btn-circle btn-ghost w-11 h-11"
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
        <div className="tooltip tooltip-bottom" data-tip="Skip">
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
        <div className="tooltip tooltip-bottom" data-tip="Replay">
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

        {/* Bookmarks panel toggle */}
        {onToggleBookmarks && (
          <div className="tooltip tooltip-bottom" data-tip="Bookmarks">
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
      <div className="grid grid-cols-5 gap-1 sm:gap-2 w-full items-center">
        {/* LIVE */}
        <div className="tooltip tooltip-bottom" data-tip="Live Mode">
          <button
            className={`btn btn-xs sm:btn-sm w-full min-w-0 px-1 sm:px-2 gap-1 ${
              isLive ? "btn-success" : "btn-ghost text-base-content"
            }`}
            onClick={() => {
              beep();
              onToggleLive();
            }}
          >
            <Radio className="hidden sm:inline w-3.5 h-3.5" />
            LIVE
          </button>
        </div>

        {/* HOLD */}
        <div className="dropdown dropdown-top w-full">
          <div
            tabIndex={0}
            role="button"
            className={`btn btn-xs sm:btn-sm w-full min-w-0 px-1 sm:px-2 gap-1 ${
              isHolding ? "btn-secondary" : "btn-ghost"
            }`}
          >
            <Lock className="hidden sm:inline w-3.5 h-3.5" />
            HOLD
          </div>
          <ul
            tabIndex={0}
            className="dropdown-content menu p-2 shadow bg-base-200 rounded-box w-48 z-50"
          >
            <li>
              <button
                onClick={() =>
                  onHoldSystem(
                    heldSystem !== null ? null : (currentCallSystemId ?? null),
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

        {/* AVOID */}
        <div className="dropdown dropdown-top w-full">
          <div
            tabIndex={0}
            role="button"
            className="btn btn-xs sm:btn-sm w-full min-w-0 px-1 sm:px-2 gap-1 btn-ghost"
          >
            <Ban className="hidden sm:inline w-3.5 h-3.5" />
            AVOID
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
          </ul>
        </div>

        {/* SELECT */}
        <div className="tooltip tooltip-bottom" data-tip="Select Talkgroups">
          <button
            className="btn btn-xs sm:btn-sm w-full min-w-0 px-1 sm:px-2 gap-1 btn-ghost"
            onClick={() => {
              beep();
              onToggleSelectTG();
            }}
          >
            <List className="hidden sm:inline w-3.5 h-3.5" />
            SELECT
          </button>
        </div>

        {/* SEARCH */}
        <div className="tooltip tooltip-bottom" data-tip="Search Calls">
          <button
            className="btn btn-xs sm:btn-sm w-full min-w-0 px-1 sm:px-2 gap-1 btn-ghost"
            onClick={() => {
              beep();
              onToggleSearch();
            }}
          >
            <Search className="hidden sm:inline w-3.5 h-3.5" />
            SEARCH
          </button>
        </div>
      </div>
    </div>
  );
}
