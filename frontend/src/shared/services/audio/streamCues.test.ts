import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { StreamCueScheduler } from "./streamCues";
import type { Call } from "@/features/scanner";

function makeCall(id: number): Call {
  return {
    id,
    audioName: "c.m4a",
    audioType: "audio/mp4",
    dateTime: 1_700_000_000,
    systemId: 100,
    system: 1,
    talkgroupId: 27000 + id,
    talkgroup: id,
  };
}

describe("StreamCueScheduler", () => {
  let clock: number | null;
  let published: number[];
  let sched: StreamCueScheduler;

  beforeEach(() => {
    vi.useFakeTimers();
    clock = 0;
    published = [];
    sched = new StreamCueScheduler();
    sched.configure(
      () => clock,
      (call) => published.push(call.id),
    );
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("holds a call until the stream reaches its cue", async () => {
    sched.hold(makeCall(1));
    sched.cue(1, 10);

    clock = 5;
    await vi.advanceTimersByTimeAsync(1000);
    // The audio is still five seconds down the buffer; labelling now is
    // the bug this exists to fix.
    expect(published).toEqual([]);

    clock = 10.1;
    await vi.advanceTimersByTimeAsync(300);
    expect(published).toEqual([1]);
  });

  it("publishes immediately when the cue is already behind playback", async () => {
    sched.hold(makeCall(7));
    clock = 30;
    sched.cue(7, 12);

    expect(published).toEqual([7]);
  });

  it("labels in stream order when several calls come due", async () => {
    sched.hold(makeCall(1));
    sched.hold(makeCall(2));
    sched.cue(1, 10);
    sched.cue(2, 12);

    clock = 20;
    await vi.advanceTimersByTimeAsync(300);
    expect(published).toEqual([1, 2]);
  });

  it("does not publish early while the stream clock is stalled", async () => {
    sched.hold(makeCall(1));
    sched.cue(1, 10);

    // Element paused or stalled: wall time passes, currentTime does not.
    // A one-shot timer computed at cue time would have fired here.
    clock = 2;
    await vi.advanceTimersByTimeAsync(9000);
    expect(published).toEqual([]);
  });

  it("does not label the next call while a long call is still playing", async () => {
    // Call 1 is long: cued at 10s, runs for a minute.
    sched.hold(makeCall(1));
    sched.cue(1, 10);
    clock = 10;
    await vi.advanceTimersByTimeAsync(300);
    expect(published).toEqual([1]);

    // Call 2 arrives while 1 is still playing. The server queues its audio
    // behind call 1, so its cue legitimately will not fire for another
    // minute.
    sched.hold(makeCall(2));
    clock = 40;
    await vi.advanceTimersByTimeAsync(30_000);

    // Regression: a 10s fallback published call 2 here, so the display and
    // lock screen jumped to the next call while call 1 was still audible.
    expect(published).toEqual([1]);

    // It appears when the stream actually reaches it.
    sched.cue(2, 70);
    clock = 70;
    await vi.advanceTimersByTimeAsync(300);
    expect(published).toEqual([1, 2]);
  });

  it("labels anyway when no cue ever arrives", async () => {
    sched.hold(makeCall(4));

    // A lost cue, a dropped WebSocket, or a server without this feature
    // must not leave the lock screen blank forever.
    await vi.advanceTimersByTimeAsync(11_000);
    expect(published).toEqual([4]);
  });

  it("drops everything pending when the stream restarts", async () => {
    sched.hold(makeCall(1));
    sched.cue(1, 10);

    // A reconnect restarts the server timeline at zero, so the offset no
    // longer refers to anything.
    sched.reset();
    clock = 50;
    await vi.advanceTimersByTimeAsync(1000);

    expect(published).toEqual([]);
    expect(sched.pendingCount()).toBe(0);
  });

  it("ignores a cue for a call it is not holding", () => {
    sched.cue(999, 5);
    expect(published).toEqual([]);
    expect(sched.pendingCount()).toBe(0);
  });

  it("does not grow without bound when the stream never reaches the calls", () => {
    clock = null;
    for (let i = 0; i < 100; i++) sched.hold(makeCall(i));
    expect(sched.pendingCount()).toBeLessThanOrEqual(32);
  });
});
