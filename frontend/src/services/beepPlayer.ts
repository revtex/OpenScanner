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
  { frequency: 1200, duration: 0.05, type: "square", gain: 0.15 },
];

// Whistler-style: two-tone chirp
const whistlerTones: ToneSpec[] = [
  { frequency: 800, duration: 0.04, type: "sine", gain: 0.18 },
  { frequency: 1400, duration: 0.04, type: "sine", gain: 0.18 },
];

const styleTones: Record<BeepStyle, ToneSpec[]> = {
  uniden: unidenTones,
  whistler: whistlerTones,
};

let audioCtx: AudioContext | null = null;

function getContext(): AudioContext {
  if (!audioCtx) {
    audioCtx = new AudioContext();
  }
  if (audioCtx.state === "suspended") {
    audioCtx.resume().catch(() => {});
  }
  return audioCtx;
}

export function playBeep(style: string): void {
  if (!style || !(style in styleTones)) return;

  const ctx = getContext();
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
}
