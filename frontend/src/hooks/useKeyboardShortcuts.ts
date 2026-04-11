import { useEffect, useCallback, useRef } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import {
  togglePause,
  toggleLive,
  holdSystem,
  holdTG,
  addAvoid,
} from "@/app/slices/scannerSlice";

const AVOID_CYCLE = [30, 60, 120]; // minutes

export function useKeyboardShortcuts(callbacks: {
  onSkip: () => void;
  onReplay: () => void;
  onSetVolume: (v: number) => void;
  onToggleSelectTG: () => void;
  onToggleSearch: () => void;
  onToggleShortcutsModal: () => void;
  onCloseAllPanels: () => void;
  volume: number;
}) {
  const dispatch = useAppDispatch();
  const currentCall = useAppSelector((s) => s.scanner.currentCall);
  const heldTG = useAppSelector((s) => s.scanner.heldTG);
  const heldSystem = useAppSelector((s) => s.scanner.heldSystem);
  const avoidCycleRef = useRef(0);
  const volumeRef = useRef(callbacks.volume);

  useEffect(() => {
    volumeRef.current = callbacks.volume;
  }, [callbacks.volume]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      // Don't intercept when typing in inputs
      const tag = (e.target as HTMLElement).tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;

      switch (e.key) {
        case " ":
          e.preventDefault();
          dispatch(togglePause());
          break;
        case "s":
        case "S":
          callbacks.onSkip();
          break;
        case "r":
        case "R":
          callbacks.onReplay();
          break;
        case "h":
        case "H":
          dispatch(
            holdTG(heldTG !== null ? null : (currentCall?.talkgroupId ?? null)),
          );
          break;
        case "j":
        case "J":
          dispatch(
            holdSystem(
              heldSystem !== null ? null : (currentCall?.system ?? null),
            ),
          );
          break;
        case "a":
        case "A": {
          if (!currentCall?.talkgroupId) break;
          const minutes =
            AVOID_CYCLE[avoidCycleRef.current % AVOID_CYCLE.length];
          avoidCycleRef.current += 1;
          dispatch(
            addAvoid({
              talkgroupId: currentCall.talkgroupId,
              expiresAt: Date.now() + minutes * 60 * 1000,
            }),
          );
          break;
        }
        case "l":
        case "L":
          dispatch(toggleLive());
          break;
        case "f":
        case "F":
          if (document.fullscreenElement) {
            document.exitFullscreen();
          } else {
            document.documentElement.requestFullscreen();
          }
          break;
        case "ArrowLeft":
          e.preventDefault();
          callbacks.onSetVolume(Math.max(0, volumeRef.current - 0.05));
          break;
        case "ArrowRight":
          e.preventDefault();
          callbacks.onSetVolume(Math.min(1, volumeRef.current + 0.05));
          break;
        case "?":
          callbacks.onToggleShortcutsModal();
          break;
        case "Escape":
          callbacks.onCloseAllPanels();
          break;
        default:
          break;
      }
    },
    [dispatch, callbacks, currentCall, heldTG, heldSystem],
  );

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);
}
