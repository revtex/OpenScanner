import { useEffect, useCallback, useState } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { store } from "@/app/store";
import { audioPlayer } from "@/services/audioPlayer";
import { wsClient } from "@/services/wsClient";
import {
  setCurrentCall,
  clearCurrentCall,
  toggleLive,
} from "@/app/slices/scannerSlice";

export function useAudioPlayer() {
  const dispatch = useAppDispatch();
  const isLive = useAppSelector((s) => s.scanner.isLive);
  const playbackGoesLive = useAppSelector(
    (s) => s.scanner.config?.playbackGoesLive ?? false,
  );
  const [volume, setVolumeState] = useState(() => audioPlayer.getVolume());
  const [playing, setPlaying] = useState(false);
  const [pendingCount, setPendingCount] = useState(0);

  useEffect(() => {
    audioPlayer.setOnCallStart((call) => {
      dispatch(setCurrentCall(call));
      setPlaying(true);
    });

    audioPlayer.setOnCallEnd(() => {
      dispatch(clearCurrentCall());
      setPlaying(false);
    });

    audioPlayer.setOnQueueChange((length) => {
      setPendingCount(length);
    });

    wsClient.onAudioReceived((call, audioUrl) => {
      audioPlayer.play(call, audioUrl);
    });

    // Client-side talkgroup selection filter — reads live Redux state
    // each time a CAL arrives so toggling takes effect immediately.
    wsClient.setCallFilter((call) => {
      const { tgSelection } = store.getState().scanner;
      // tgSelection uses the DB talkgroup row ID (call.talkgroup).
      // undefined = enabled (default on); only explicit false rejects.
      return tgSelection[call.talkgroup] !== false;
    });
  }, [dispatch]);

  // When playback ends and queue is empty, auto-switch to live if configured.
  useEffect(() => {
    if (!playing && pendingCount === 0 && !isLive && playbackGoesLive) {
      dispatch(toggleLive());
    }
  }, [playing, pendingCount, isLive, playbackGoesLive, dispatch]);

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
