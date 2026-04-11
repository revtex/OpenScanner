import { useEffect, useCallback, useState } from "react";
import { useAppDispatch } from "@/app/store";
import { audioPlayer } from "@/services/audioPlayer";
import { wsClient } from "@/services/wsClient";
import { setCurrentCall, clearCurrentCall } from "@/app/slices/scannerSlice";

export function useAudioPlayer() {
  const dispatch = useAppDispatch();
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
