import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

class FakeAudio {
  static instances: FakeAudio[] = [];

  preload = "";
  src = "";
  paused = true;
  playCalls = 0;
  loadCalls = 0;
  private listeners = new Map<string, Set<() => void>>();

  constructor() {
    FakeAudio.instances.push(this);
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
    for (const fn of [...(this.listeners.get(type) ?? [])]) fn();
  }

  listenerCount(type: string): number {
    return this.listeners.get(type)?.size ?? 0;
  }

  /** Set to reject the next play(), as autoplay policy does. */
  static rejectNextPlay = false;

  play(): Promise<void> {
    this.playCalls += 1;
    if (FakeAudio.rejectNextPlay) {
      FakeAudio.rejectNextPlay = false;
      return Promise.reject(new Error("NotAllowedError"));
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

async function loadPlayer() {
  vi.resetModules();
  const mod = await import("./streamPlayer");
  return mod.streamPlayer;
}

function last(): FakeAudio {
  const el = FakeAudio.instances[FakeAudio.instances.length - 1];
  if (!el) throw new Error("no audio element was created");
  return el;
}

describe("streamPlayer", () => {
  beforeEach(() => {
    FakeAudio.instances = [];
    FakeAudio.rejectNextPlay = false;
    vi.stubGlobal("Audio", FakeAudio);
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("opens the stream endpoint and plays synchronously", async () => {
    const player = await loadPlayer();
    player.start();

    const el = last();
    expect(el.src).toContain("/api/v1/listener/stream");
    // start() runs inside a user gesture; iOS drops activation across an
    // await, so play() must already have been called by the time start()
    // returns.
    expect(el.playCalls).toBe(1);
    expect(player.isActive()).toBe(true);
  });

  it("is idempotent while already streaming", async () => {
    const player = await loadPlayer();
    player.start();
    player.start();
    expect(FakeAudio.instances).toHaveLength(1);
  });

  it("tears the connection down on stop", async () => {
    const player = await loadPlayer();
    player.start();
    const el = last();

    player.stop();

    expect(player.isActive()).toBe(false);
    expect(el.paused).toBe(true);
    // Clearing src is what actually closes the request server-side.
    expect(el.src).toBe("");
    expect(el.listenerCount("error")).toBe(0);
  });

  it("reconnects when the stream is cut", async () => {
    const player = await loadPlayer();
    player.start();

    // The stream is never supposed to end, so `ended` means something in
    // the path dropped it.
    last().emit("ended");
    await vi.advanceTimersByTimeAsync(2500);

    expect(FakeAudio.instances.length).toBeGreaterThan(1);
    expect(last().playCalls).toBe(1);
  });

  it("reports active only once audio is really playing", async () => {
    const player = await loadPlayer();
    const states: boolean[] = [];
    player.setOnActiveChange((a) => states.push(a));

    player.start();
    await vi.advanceTimersByTimeAsync(0);

    expect(states).toEqual([true]);
  });

  it("reports inactive when autoplay policy refuses the stream", async () => {
    const player = await loadPlayer();
    const states: boolean[] = [];
    player.setOnActiveChange((a) => states.push(a));

    // This is the state a reload lands in: the preference is on, but the
    // stream cannot open without a gesture. The UI must not claim to be
    // streaming, or the only way out is toggling off and on again.
    FakeAudio.rejectNextPlay = true;
    player.start();
    await vi.advanceTimersByTimeAsync(0);

    expect(states).toEqual([false]);
    // The setting stays on so the next gesture can retry.
    expect(player.isActive()).toBe(true);
  });

  it("reports inactive on stop", async () => {
    const player = await loadPlayer();
    player.start();
    await vi.advanceTimersByTimeAsync(0);

    const states: boolean[] = [];
    player.setOnActiveChange((a) => states.push(a));
    player.stop();

    expect(states).toContain(false);
  });

  it("gives each connection its own id", async () => {
    const player = await loadPlayer();
    player.start();
    const first = player.streamId();
    expect(first).not.toBe("");
    expect(last().src).toContain(`sid=${encodeURIComponent(first)}`);

    player.stop();
    player.start();
    // A new connection restarts the server-side timeline, so it must not
    // reuse the previous id or cues would be matched against the wrong one.
    expect(player.streamId()).not.toBe(first);
  });

  it("does not reconnect after stop", async () => {
    const player = await loadPlayer();
    player.start();
    const el = last();

    player.stop();
    el.emit("ended");
    await vi.advanceTimersByTimeAsync(5000);

    expect(FakeAudio.instances).toHaveLength(1);
  });
});
