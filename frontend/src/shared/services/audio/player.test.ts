import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Call } from "@/features/scanner";

// A stub media element that records play() calls and never fires `canplay`
// on its own — the state a hidden tab leaves the element in when the
// browser defers buffering.
class FakeAudio {
  static instances: FakeAudio[] = [];

  preload = "";
  src = "";
  volume = 1;
  currentTime = 0;
  paused = true;
  readyState = 0;
  playCalls = 0;
  loadCalls = 0;
  /** Set to an error name to make the next play() reject with it. */
  rejectPlayWith: string | null = null;
  /** Applied to the next instance created — the player owns construction. */
  static rejectFirstPlayWith: string | null = null;

  private listeners = new Map<string, Set<() => void>>();

  constructor() {
    FakeAudio.instances.push(this);
    if (FakeAudio.rejectFirstPlayWith) {
      this.rejectPlayWith = FakeAudio.rejectFirstPlayWith;
      FakeAudio.rejectFirstPlayWith = null;
    }
  }

  addEventListener(type: string, fn: () => void): void {
    const set = this.listeners.get(type) ?? new Set();
    set.add(fn);
    this.listeners.set(type, set);
  }

  removeEventListener(type: string, fn: () => void): void {
    this.listeners.get(type)?.delete(fn);
  }

  emit(type: string): void {
    for (const fn of this.listeners.get(type) ?? []) fn();
  }

  play(): Promise<void> {
    this.playCalls += 1;
    if (this.rejectPlayWith) {
      const err = new Error("blocked");
      err.name = this.rejectPlayWith;
      this.rejectPlayWith = null;
      return Promise.reject(err);
    }
    this.paused = false;
    return Promise.resolve();
  }

  pause(): void {
    this.paused = true;
  }

  load(): void {
    this.loadCalls += 1;
  }

  removeAttribute(): void {
    this.src = "";
  }
}

class FakeAudioContext {
  state = "running";
  onstatechange: (() => void) | null = null;
  createGain() {
    return { gain: { value: 1 }, connect: () => {} };
  }
  createMediaElementSource() {
    return { connect: () => {} };
  }
  resume() {
    this.state = "running";
    return Promise.resolve();
  }
  suspend() {
    this.state = "suspended";
    return Promise.resolve();
  }
  close() {
    return Promise.resolve();
  }
}

function makeCall(id: number): Call {
  return {
    id,
    audioName: `call-${id}.m4a`,
    audioType: "audio/mp4",
    dateTime: Math.floor(Date.now() / 1000),
    systemId: 100,
    system: 1,
    talkgroupId: 27501,
    talkgroup: 162,
    duration: 3000,
  };
}

/** Most recently constructed stub element. */
function lastElement(): FakeAudio {
  const el = FakeAudio.instances[FakeAudio.instances.length - 1];
  if (!el) throw new Error("no audio element was created");
  return el;
}

/** Fresh module instance per test — the player is a module singleton. */
async function loadPlayer() {
  vi.resetModules();
  const mod = await import("./player");
  return mod.audioPlayer;
}

function setVisibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => state,
  });
  document.dispatchEvent(new Event("visibilitychange"));
}

describe("audioPlayer", () => {
  beforeEach(() => {
    FakeAudio.instances = [];
    FakeAudio.rejectFirstPlayWith = null;
    vi.stubGlobal("Audio", FakeAudio);
    vi.stubGlobal("AudioContext", FakeAudioContext);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("plays without waiting for canplay", async () => {
    const player = await loadPlayer();
    player.enqueue(makeCall(1));
    await Promise.resolve();

    const el = lastElement();
    expect(el.src).toContain("/api/v1/calls/1/audio");
    // Regression: playback used to be gated on a `canplay` listener, which
    // a background tab may never fire — leaving the queue to pile up.
    expect(el.playCalls).toBe(1);
  });

  it("keeps the call queued when autoplay policy blocks play()", async () => {
    const player = await loadPlayer();
    FakeAudio.rejectFirstPlayWith = "NotAllowedError";

    player.enqueue(makeCall(1));
    player.enqueue(makeCall(2));
    await Promise.resolve();
    await Promise.resolve();

    // The blocked call must stay current rather than be skipped, or a
    // policy rejection would silently drain the whole queue.
    expect(player.getCurrentCall()?.id).toBe(1);
    expect(player.isPlaying()).toBe(false);

    // …and the foreground kick recovers it.
    const el = lastElement();
    el.paused = true;
    setVisibility("visible");
    await Promise.resolve();
    expect(el.playCalls).toBeGreaterThan(1);
  });

  it("resumes a stalled call when the page becomes visible", async () => {
    const player = await loadPlayer();
    player.enqueue(makeCall(1));
    await Promise.resolve();

    const el = lastElement();
    // Emulate a hidden-tab stall: play() was called, so the element is not
    // paused, but nothing ever buffered.
    el.paused = false;
    el.readyState = 0;
    const before = el.playCalls;

    setVisibility("visible");
    await Promise.resolve();

    expect(el.playCalls).toBeGreaterThan(before);
    expect(el.loadCalls).toBeGreaterThan(0);
    setVisibility("visible");
  });

  it("does not re-issue play() for healthy playback on visibility change", async () => {
    const player = await loadPlayer();
    player.enqueue(makeCall(1));
    await Promise.resolve();

    const el = lastElement();
    el.paused = false;
    el.readyState = 4;
    const before = el.playCalls;

    setVisibility("visible");
    await Promise.resolve();

    expect(el.playCalls).toBe(before);
  });

  it("advances the queue on ended", async () => {
    const player = await loadPlayer();
    player.enqueue(makeCall(1));
    player.enqueue(makeCall(2));
    await Promise.resolve();

    const el = lastElement();
    expect(player.getCurrentCall()?.id).toBe(1);
    el.emit("ended");
    await Promise.resolve();
    expect(player.getCurrentCall()?.id).toBe(2);
  });
});
