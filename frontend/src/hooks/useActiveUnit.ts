import { useState, useEffect, useRef } from "react";
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
  const [activeUnit, setActiveUnit] = useState<ActiveUnit | null>(null);
  const entriesRef = useRef<SourceEntry[]>([]);

  // Parse sources JSON once when it changes.
  useEffect(() => {
    if (!sourcesJson) {
      entriesRef.current = [];
      setActiveUnit(null);
      return;
    }
    try {
      const parsed: SourceEntry[] = JSON.parse(sourcesJson);
      if (!Array.isArray(parsed) || parsed.length === 0) {
        entriesRef.current = [];
        setActiveUnit(null);
        return;
      }
      // Sort by position ascending.
      entriesRef.current = parsed
        .filter((e) => typeof e.pos === "number" && typeof e.src === "number")
        .sort((a, b) => a.pos - b.pos);
      // Show the first unit immediately.
      if (entriesRef.current.length > 0) {
        const first = entriesRef.current[0];
        setActiveUnit({ src: first.src, tag: first.tag });
      }
    } catch {
      entriesRef.current = [];
      setActiveUnit(null);
    }
  }, [sourcesJson]);

  // Poll playback position to update active unit.
  useEffect(() => {
    const entries = entriesRef.current;
    if (entries.length <= 1) return;

    const tick = () => {
      if (!audioPlayer.isPlaying()) return;
      const t = audioPlayer.getPlaybackTime();
      // Find the last source entry whose position <= current time.
      let active = entries[0];
      for (let i = entries.length - 1; i >= 0; i--) {
        if (entries[i].pos <= t) {
          active = entries[i];
          break;
        }
      }
      setActiveUnit({ src: active.src, tag: active.tag });
    };

    const id = setInterval(tick, 100);
    return () => clearInterval(id);
  }, [sourcesJson]); // re-run when sources change (new call)

  return activeUnit;
}
