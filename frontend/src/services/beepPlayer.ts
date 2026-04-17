// beepPlayer.ts — Generates scanner-style keypad beeps via Web Audio API.
// Two styles are supported: "uniden" and "whistler".

type BeepStyle = "uniden" | "whistler";

interface ToneSpec {
  frequency: number;
  duration: number; // seconds
  type: OscillatorType;
  gain: number;
}

// Uniden-style: short high-pitched beep
const unidenTones: ToneSpec[] = [
  { frequency: 1200, duration: 0.08, type: "square", gain: 0.25 },
];

// Whistler-style: two-tone chirp
const whistlerTones: ToneSpec[] = [
  { frequency: 800, duration: 0.06, type: "sine", gain: 0.3 },
  { frequency: 1400, duration: 0.06, type: "sine", gain: 0.3 },
];

const styleTones: Record<BeepStyle, ToneSpec[]> = {
  uniden: unidenTones,
  whistler: whistlerTones,
};

let audioCtx: AudioContext | null = null;

/**
 * Create and resume the beep AudioContext inside a user-gesture handler.
 * Called from audioPlayer's bootstrapAudio() to satisfy mobile autoplay policy.
 * Returns a promise so the caller can await the resume.
 */
export async function bootstrapBeepContext(): Promise<void> {
  if (audioCtx) return;
  audioCtx = new AudioContext();
  if (audioCtx.state === "suspended") {
    try {
      await audioCtx.resume();
    } catch {
      // ignore
    }
  }
  audioCtx.onstatechange = () => {
    if (audioCtx?.state === "suspended") {
      audioCtx.resume().catch(() => {});
    }
  };
}

function getContext(): AudioContext {
  if (!audioCtx) {
    // Fallback if bootstrap hasn't fired yet.
    audioCtx = new AudioContext();
  }
  if (audioCtx.state === "suspended") {
    audioCtx.resume().catch(() => {});
  }
  return audioCtx;
}

export function playBeep(style: string): void {
  if (!style || style === "disabled" || !(style in styleTones)) return;

  const ctx = getContext();

  // AudioContext may be suspended until a user gesture resumes it.
  // We must wait for it to be running before scheduling oscillators.
  const schedule = () => {
    const tones = styleTones[style as BeepStyle];
    let offset = 0;

    for (const tone of tones) {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();

      osc.type = tone.type;
      osc.frequency.value = tone.frequency;
      gain.gain.value = tone.gain;

      osc.connect(gain);
      gain.connect(ctx.destination);

      const start = ctx.currentTime + offset;
      osc.start(start);
      osc.stop(start + tone.duration);

      offset += tone.duration;
    }
  };

  if (ctx.state === "suspended") {
    ctx
      .resume()
      .then(schedule)
      .catch(() => {});
  } else {
    schedule();
  }
}
