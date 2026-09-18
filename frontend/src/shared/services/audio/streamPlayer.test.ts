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

  play(): Promise<void> {
    this.playCalls += 1;
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
