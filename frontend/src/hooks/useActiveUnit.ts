import { useState, useEffect, useMemo } from "react";
import { audioPlayer } from "@/services/audioPlayer";

interface SourceEntry {
  pos: number;
  src: number;
  tag?: string;
}

interface ActiveUnit {
  src: number;
  tag?: string;
}

/**
 * Parses the sources JSON from a call and returns the currently-active
 * transmitting unit based on the audio playback position. Updates at
 * ~10 Hz while audio is playing.
 */
export function useActiveUnit(
  sourcesJson: string | undefined,
): ActiveUnit | null {
  // Parse sources JSON during render — pure derivation from the prop.
  const entries = useMemo<SourceEntry[]>(() => {
    if (!sourcesJson) return [];
    try {
      const parsed: unknown = JSON.parse(sourcesJson);
      if (!Array.isArray(parsed)) return [];
      return (parsed as SourceEntry[])
        .filter((e) => typeof e.pos === "number" && typeof e.src === "number")
        .sort((a, b) => a.pos - b.pos);
    } catch {
      return [];
    }
  }, [sourcesJson]);

  // Track playback position via effect-driven ticks so render stays pure.
  const [playbackTime, setPlaybackTime] = useState(0);

  useEffect(() => {
    if (entries.length <= 1) return;
    const id = setInterval(() => {
      if (audioPlayer.isPlaying()) {
        setPlaybackTime(audioPlayer.getPlaybackTime());
      }
    }, 100);
    return () => clearInterval(id);
  }, [entries]);

  return useMemo<ActiveUnit | null>(() => {
    if (entries.length === 0) return null;
    let active = entries[0];
    for (let i = entries.length - 1; i >= 0; i--) {
      if (entries[i].pos <= playbackTime) {
        active = entries[i];
        break;
      }
    }
    return { src: active.src, tag: active.tag };
  }, [entries, playbackTime]);
}
