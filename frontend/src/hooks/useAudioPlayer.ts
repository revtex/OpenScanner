import { useEffect, useCallback, useState, useRef } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { store } from "@/app/store";
import { audioPlayer } from "@/services/audioPlayer";
import { wsClient } from "@/services/wsClient";
import {
  setCurrentCall,
  clearCurrentCall,
  setAudioActive,
} from "@/app/slices/scannerSlice";

export function useAudioPlayer() {
  const dispatch = useAppDispatch();
  const isLive = useAppSelector((s) => s.scanner.isLive);
  const heldTG = useAppSelector((s) => s.scanner.heldTG);
  const heldSystem = useAppSelector((s) => s.scanner.heldSystem);
  const transcriptionEnabled = useAppSelector(
    (s) => s.scanner.config?.transcriptionEnabled ?? false,
  );
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

    wsClient.onAudioReceived((call, audioUrl, audioData) => {
      // LIVE mode gates only streaming WS audio. Manual playback
      // (Search/Bookmarks) uses audioPlayer.playNow and remains available.
      if (!store.getState().scanner.isLive) {
        try {
          URL.revokeObjectURL(audioUrl);
        } catch {
          // ignore
        }
        return;
      }
      audioPlayer.play(call, audioData, audioUrl);
    });

    // Forward transcript events to audioPlayer so buffered calls get
    // their transcript attached before playback starts.
    wsClient.onTranscriptReceived((callId, text, segments) => {
      audioPlayer.resolveTranscript(callId, text, segments);
    });

    // If restored as paused (e.g. after refresh), tell audioPlayer so
    // incoming calls queue instead of trying to auto-play.
    if (store.getState().scanner.isPaused) {
      audioPlayer.pause();
    }

    // Client-side filter — checks hold, avoid, and tgSelection each
    // time a CAL arrives so changes take effect immediately.
    wsClient.setCallFilter((call) => {
      const { heldTG, heldSystem, avoidList, tgSelection } =
        store.getState().scanner;

      // Hold: if a talkgroup is held, only that TG plays
      if (heldTG !== null) return call.talkgroup === heldTG;

      // Hold: if a system is held, only calls from that system play
      if (heldSystem !== null) {
        if (call.system !== heldSystem) return false;
      }

      // Avoid: block avoided talkgroups
      const now = Date.now();
      for (const entry of avoidList) {
        if (entry.talkgroupId === call.talkgroup) {
          if (entry.expiresAt === 0 || entry.expiresAt > now) return false;
        }
      }

      // tgSelection: undefined = enabled; only explicit false rejects
      return tgSelection[call.talkgroup] !== false;
    });
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

  // Only buffer calls for transcript sync when transcription is enabled.
  // When disabled, calls play immediately without waiting for TRN.
  useEffect(() => {
    audioPlayer.setSyncTranscripts(transcriptionEnabled);
  }, [transcriptionEnabled]);

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
