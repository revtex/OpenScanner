import { useEffect, useCallback, useState, useRef } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { store } from "@/app/store";
import { audioPlayer } from "@/shared/services/audio/player";
import {
  setCurrentCall,
  clearCurrentCall,
  setAudioActive,
} from "../scannerSlice";

export function useAudioPlayer() {
  const dispatch = useAppDispatch();
  const isLive = useAppSelector((s) => s.scanner.isLive);
  const heldTG = useAppSelector((s) => s.scanner.heldTG);
  const heldSystem = useAppSelector((s) => s.scanner.heldSystem);
  const [volume, setVolumeState] = useState(() => audioPlayer.getVolume());
  const [playing, setPlaying] = useState(false);
  const [pendingCount, setPendingCount] = useState(0);

  // Track previous hold values to detect activation
  const prevHeldTG = useRef(heldTG);
  const prevHeldSystem = useRef(heldSystem);

  useEffect(() => {
    audioPlayer.setOnCallStart((call) => {
      dispatch(setCurrentCall(call));
      dispatch(setAudioActive(true));
      setPlaying(true);
    });

    audioPlayer.setOnCallEnd(() => {
      dispatch(clearCurrentCall());
      dispatch(setAudioActive(false));
      setPlaying(false);
    });

    audioPlayer.setOnQueueChange((length) => {
      setPendingCount(length);
    });

    // If restored as paused (e.g. after refresh), tell audioPlayer so
    // incoming calls queue instead of trying to auto-play.
    if (store.getState().scanner.isPaused) {
      audioPlayer.pause();
    }

    // Stop all audio when the hook unmounts (e.g. navigating away).
    return () => {
      audioPlayer.clearQueue();
    };
  }, [dispatch]);

  // Flush queue + skip current when hold activates
  useEffect(() => {
    const tgActivated = prevHeldTG.current === null && heldTG !== null;
    const sysActivated = prevHeldSystem.current === null && heldSystem !== null;

    prevHeldTG.current = heldTG;
    prevHeldSystem.current = heldSystem;

    if (tgActivated && heldTG !== null) {
      // Keep only the held TG in queue
      audioPlayer.filterQueue((call) => call.talkgroup === heldTG);
      // Skip current if it doesn't match
      const current = audioPlayer.getCurrentCall();
      if (current && current.talkgroup !== heldTG) {
        audioPlayer.skip();
      }
    } else if (sysActivated && heldSystem !== null) {
      // Keep only calls from the held system
      audioPlayer.filterQueue((call) => call.system === heldSystem);
      const current = audioPlayer.getCurrentCall();
      if (current && current.system !== heldSystem) {
        audioPlayer.skip();
      }
    }
  }, [heldTG, heldSystem]);

  // LIVE off behaves like radio power off for stream playback:
  // stop current audio immediately and clear any queued stream calls.
  useEffect(() => {
    if (!isLive) {
      audioPlayer.clearQueue();
      dispatch(setAudioActive(false));
    }
  }, [dispatch, isLive]);

  const skip = useCallback(() => {
    audioPlayer.skip();
  }, []);

  const replay = useCallback(() => {
    audioPlayer.replay();
  }, []);

  const pause = useCallback(() => {
    audioPlayer.pause();
    setPlaying(false);
  }, []);

  const resume = useCallback(() => {
    audioPlayer.resume();
    setPlaying(true);
  }, []);

  const setVolume = useCallback((v: number) => {
    audioPlayer.setVolume(v);
    setVolumeState(v);
  }, []);

  const download = useCallback(() => {
    audioPlayer.download();
  }, []);

  return {
    skip,
    replay,
    pause,
    resume,
    setVolume,
    volume,
    isPlaying: playing,
    pendingCount,
    download,
  };
}
