import { useEffect, useCallback, useState } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
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
  const callQueue = useAppSelector((s) => s.scanner.callQueue);
  const playbackGoesLive = useAppSelector(
    (s) => s.scanner.config?.playbackGoesLive ?? false,
  );
  const [volume, setVolumeState] = useState(() => audioPlayer.getVolume());
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    audioPlayer.setOnCallStart((call) => {
      dispatch(setCurrentCall(call));
      setPlaying(true);
    });

    audioPlayer.setOnCallEnd(() => {
      dispatch(clearCurrentCall());
      setPlaying(false);
    });

    wsClient.onAudioReceived((call, audioUrl) => {
      audioPlayer.play(call, audioUrl);
    });
  }, [dispatch]);

  // When playback ends and queue is empty, auto-switch to live if configured.
  useEffect(() => {
    if (!playing && callQueue.length === 0 && !isLive && playbackGoesLive) {
      dispatch(toggleLive());
    }
  }, [playing, callQueue.length, isLive, playbackGoesLive, dispatch]);

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
    download,
  };
}
