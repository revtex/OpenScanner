import { useCallback } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { useWebSocket } from "@/shared/hooks/useWebSocket";
import { useAudioPlayer } from "../hooks/useAudioPlayer";
import {
  togglePause,
  toggleLive,
  holdSystem,
  holdTG,
  addAvoid,
  removeAvoid,
  clearAvoids,
  toggleTG,
  setAllTGs,
  setBackgroundAudio,
} from "../scannerSlice";
import { streamPlayer } from "@/shared/services/audio/streamPlayer";
import { audioPlayer } from "@/shared/services/audio/player";
import type { AvoidEntry } from "@/types";

export function useScanner() {
  const dispatch = useAppDispatch();
  const { connectionStatus } = useWebSocket();
  const audio = useAudioPlayer();

  const currentCall = useAppSelector((s) => s.scanner.currentCall);
  const history = useAppSelector((s) => s.scanner.history);
  const isLive = useAppSelector((s) => s.scanner.isLive);
  const isPaused = useAppSelector((s) => s.scanner.isPaused);
  const heldSystem = useAppSelector((s) => s.scanner.heldSystem);
  const heldTG = useAppSelector((s) => s.scanner.heldTG);
  const avoidList = useAppSelector((s) => s.scanner.avoidList);
  const listenerCount = useAppSelector((s) => s.scanner.listenerCount);
  const backgroundAudio = useAppSelector((s) => s.scanner.backgroundAudio);
  const config = useAppSelector((s) => s.scanner.config);
  const tgSelection = useAppSelector((s) => s.scanner.tgSelection);

  const doTogglePause = useCallback(() => {
    dispatch(togglePause());
    if (isPaused) {
      audio.resume();
    } else {
      audio.pause();
    }
  }, [dispatch, isPaused, audio]);
  const doToggleLive = useCallback(() => dispatch(toggleLive()), [dispatch]);
  const doHoldSystem = useCallback(
    (id: number | null) => dispatch(holdSystem(id)),
    [dispatch],
  );
  const doHoldTG = useCallback(
    (id: number | null) => dispatch(holdTG(id)),
    [dispatch],
  );
  const doAddAvoid = useCallback(
    (entry: AvoidEntry) => dispatch(addAvoid(entry)),
    [dispatch],
  );
  const doRemoveAvoid = useCallback(
    (tgId: number) => dispatch(removeAvoid(tgId)),
    [dispatch],
  );
  const doClearAvoids = useCallback(() => dispatch(clearAvoids()), [dispatch]);
  const doToggleTG = useCallback(
    (id: number) => dispatch(toggleTG(id)),
    [dispatch],
  );
  const doSetAllTGs = useCallback(
    (enabled: boolean) => dispatch(setAllTGs(enabled)),
    [dispatch],
  );

  // Runs straight off the button press: opening the stream is subject to
  // autoplay policy, and on iOS the user activation does not survive an
  // await, so start() has to happen inside the gesture.
  const doToggleBackgroundAudio = useCallback(() => {
    const next = !backgroundAudio;
    if (next) {
      // Drop anything the local player has queued — the stream is about to
      // deliver the same traffic, and both playing at once would double it.
      audioPlayer.clearQueue();
      streamPlayer.start();
    } else {
      streamPlayer.stop();
    }
    dispatch(setBackgroundAudio(next));
  }, [backgroundAudio, dispatch]);

  return {
    // Connection
    connectionStatus,

    // Scanner state
    currentCall,
    history,
    isLive,
    isPaused,
    heldSystem,
    heldTG,
    avoidList,
    listenerCount,
    backgroundAudio,
    config,
    tgSelection,

    // Scanner actions
    togglePause: doTogglePause,
    toggleLive: doToggleLive,
    holdSystem: doHoldSystem,
    holdTG: doHoldTG,
    addAvoid: doAddAvoid,
    removeAvoid: doRemoveAvoid,
    clearAvoids: doClearAvoids,
    toggleTG: doToggleTG,
    setAllTGs: doSetAllTGs,
    toggleBackgroundAudio: doToggleBackgroundAudio,

    // Audio controls
    ...audio,
  };
}
