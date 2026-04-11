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
} from "lucide-react";
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
}: ControlToolbarProps) {
  const isMuted = volume === 0;
  const isHolding = heldSystem !== null || heldTG !== null;
  const avoidCount = avoidList.length;

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
          data-tip={isPaused ? "Resume (Space)" : "Pause (Space)"}
        >
          <button
            className="btn btn-circle btn-primary w-11 h-11"
            onClick={onTogglePause}
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
            onClick={onSkip}
            aria-label="Skip"
          >
            <SkipForward className="w-4 h-4" />
          </button>
        </div>

        {/* Replay */}
        <div className="tooltip tooltip-bottom" data-tip="Replay (R)">
          <button
            className="btn btn-circle btn-ghost w-9 h-9"
            onClick={onReplay}
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
            onClick={onDownload}
            aria-label="Download"
          >
            <Download className="w-4 h-4" />
          </button>
        </div>

        {/* Bookmark placeholder — uses Star icon, toggle not wired to backend yet */}
        <div className="tooltip tooltip-bottom" data-tip="Bookmark (B)">
          <button
            className="btn btn-circle btn-ghost w-9 h-9"
            aria-label="Bookmark"
          >
            <Star className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Row 2 — Mode Toggles */}
      <div className="flex items-center justify-center gap-2 flex-wrap">
        {/* LIVE */}
        <div className="tooltip tooltip-bottom" data-tip="Live Mode (L)">
          <button
            className={`btn btn-sm gap-1 ${isLive ? "btn-primary" : "btn-ghost"}`}
            onClick={onToggleLive}
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
          <button className="btn btn-sm btn-ghost" onClick={onToggleSelectTG}>
            <List className="w-3.5 h-3.5" />
            SELECT▾
          </button>
        </div>

        {/* SEARCH */}
        <div className="tooltip tooltip-bottom" data-tip="Search Calls">
          <button className="btn btn-sm btn-ghost" onClick={onToggleSearch}>
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
              <button>
                <Star className="w-4 h-4" /> Saved Calls
              </button>
            </li>
            <li>
              <button>
                <Maximize className="w-4 h-4" /> Fullscreen
              </button>
            </li>
            <li>
              <button>
                <Keyboard className="w-4 h-4" /> Keyboard Shortcuts
              </button>
            </li>
          </ul>
        </div>
      </div>
    </div>
  );
}
