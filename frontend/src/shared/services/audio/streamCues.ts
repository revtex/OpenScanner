/**
 * Schedules now-playing labels against the server audio stream's own clock.
 *
 * Why this exists: in background-audio mode the call arrives over the
 * WebSocket the instant it is ingested, but its audio is somewhere inside
 * the stream's buffer and will not be heard for several seconds (measured
 * at ~8.4s, and it varies by browser). Labelling on arrival therefore
 * changed the lock screen well before the matching audio played, which
 * reads as a bug.
 *
 * The server knows exactly where each call begins on each listener's
 * timeline and sends that position in a `stream.cue`. Since the stream is
 * paced in real time and the element's `currentTime` advances 1:1 with it
 * (verified: constant offset, no drift over several minutes), holding the
 * label until `currentTime` reaches the cue puts it in step with the audio
 * on any browser, whatever it chooses to buffer.
 */

import type { Call } from "@/features/scanner";

/**
 * How long a call waits for its cue before being labelled anyway. A cue is
 * emitted when the server starts *sending* the call, so it normally lands
 * within a second. Publishing late is much better than a lock screen that
 * stays blank because a cue was lost, the WebSocket dropped, or the server
 * predates this feature.
 */
const CUE_TIMEOUT_MS = 10_000;

/** How often the pending list is checked against the stream clock. */
const TICK_MS = 250;

/** Calls older than this are dropped unlabelled; the stream has moved on. */
const MAX_PENDING = 32;

interface Pending {
  call: Call;
  /** Stream position where this call starts, or null until its cue lands. */
  offset: number | null;
  /** When the call arrived, for the fallback. */
  at: number;
}

export class StreamCueScheduler {
  private pending = new Map<number, Pending>();
  private timer: ReturnType<typeof setInterval> | null = null;

  /** Reads the stream's current playback position, in seconds. */
  private clock: () => number | null = () => null;
  /** Publishes a call to the media session. */
  private publish: (call: Call) => void = () => {};

  configure(clock: () => number | null, publish: (call: Call) => void): void {
    this.clock = clock;
    this.publish = publish;
  }

  /**
   * Hold a call until the stream reaches it. Called instead of labelling
   * immediately while the server stream owns playback.
   */
  hold(call: Call): void {
    this.pending.set(call.id, { call, offset: null, at: Date.now() });
    // A burst of calls the stream never reaches (all filtered out, say)
    // must not grow this without bound.
    while (this.pending.size > MAX_PENDING) {
      const oldest = this.pending.keys().next().value;
      if (oldest === undefined) break;
      this.pending.delete(oldest);
    }
    this.ensureTimer();
  }

  /** Record where a call begins on the stream timeline. */
  cue(callId: number, offset: number): void {
    const entry = this.pending.get(callId);
    if (!entry) return;
    entry.offset = offset;
    this.ensureTimer();
    // The stream may already be past it (a cue for audio that was buffered
    // before this page started scheduling).
    this.flush();
  }

  /**
   * Drop everything pending. Used when the stream is closed or re-opened —
   * a reconnect restarts the server-side timeline at zero, so offsets from
   * the previous connection are meaningless.
   */
  reset(): void {
    this.pending.clear();
    this.stopTimer();
  }

  /** Visible for tests. */
  pendingCount(): number {
    return this.pending.size;
  }

  private ensureTimer(): void {
    if (this.timer !== null) return;
    this.timer = setInterval(() => {
      this.flush();
    }, TICK_MS);
  }

  private stopTimer(): void {
    if (this.timer === null) return;
    clearInterval(this.timer);
    this.timer = null;
  }

  /**
   * Publish every pending call the stream has reached. Driven off the
   * element's clock rather than a one-shot timer: `currentTime` stops while
   * the element is stalled or paused but the wall clock does not, so a
   * timer computed at cue time would fire early after any interruption.
   */
  private flush(): void {
    if (this.pending.size === 0) {
      this.stopTimer();
      return;
    }
    const now = this.clock();
    const wallNow = Date.now();

    // Oldest first, so a burst of calls is labelled in the order heard.
    const due: Array<[number, Pending]> = [];
    for (const [id, entry] of this.pending) {
      const reached = entry.offset !== null && now !== null && now >= entry.offset;
      const timedOut = wallNow - entry.at >= CUE_TIMEOUT_MS;
      if (reached || timedOut) due.push([id, entry]);
    }

    for (const [id, entry] of due) {
      this.pending.delete(id);
      this.publish(entry.call);
    }
    if (this.pending.size === 0) this.stopTimer();
  }
}

export const streamCues = new StreamCueScheduler();
