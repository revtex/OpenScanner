/**
 * Plays the server's continuous listener stream.
 *
 * Why this exists alongside the per-call player: iOS only keeps a
 * backgrounded page alive while audio is actively playing, and a scanner is
 * silent between calls. The per-call player therefore stops dead once the
 * screen locks and never gets to play the next call. The server stream is a
 * single never-ending response — silence padded with calls as they arrive —
 * so the audio session never closes.
 *
 * Deliberately not routed through the Web Audio graph, for the same reason
 * the per-call player avoids it on iOS: iOS suspends an AudioContext when
 * the page is backgrounded and interrupts it on screen lock, which is
 * exactly the situation this player exists to survive.
 */

const STREAM_PATH = "/api/v1/listener/stream";

/**
 * Identifies this connection to the server so its `stream.cue` events can
 * be told apart from those of the same account's other tabs, each of which
 * has its own stream timeline.
 */
function newSid(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `s-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

/** Delay before re-opening a stream that ended or errored. */
const RECONNECT_MS = 2000;

class StreamPlayer {
  private audio: HTMLAudioElement | null = null;
  private active = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  /** Detaches a pending startOnGesture listener, if one is armed. */
  private pendingGesture: (() => void) | null = null;
  private sid = "";
  /** Called whenever the timeline restarts, so stale cues can be dropped. */
  private onReset: (() => void) | null = null;
  /** Reports whether audio is actually flowing, not merely requested. */
  private onActiveChange: ((active: boolean) => void) | null = null;

  /**
   * Open the stream. Must be called from inside a user gesture — the
   * element's first play() is subject to autoplay policy just like any
   * other, and on iOS the activation does not survive an await.
   */
  start(): void {
    if (this.active) return;
    this.active = true;
    this.open();
  }

  /**
   * Start on the next user gesture. Used when the page loads with the
   * toggle already on: autoplay policy will refuse a stream opened without
   * one, so we wait for the first interaction rather than failing silently.
   */
  startOnGesture(): void {
    if (this.active || this.pendingGesture) return;
    const events = ["mousedown", "touchstart", "keydown"] as const;
    const once = () => {
      this.pendingGesture = null;
      for (const e of events) document.removeEventListener(e, once);
      this.start();
    };
    this.pendingGesture = () => {
      for (const e of events) document.removeEventListener(e, once);
    };
    for (const e of events) document.addEventListener(e, once);
  }

  stop(): void {
    this.pendingGesture?.();
    this.pendingGesture = null;
    this.active = false;
    this.onReset?.();
    this.onActiveChange?.(false);
    this.clearTimer();
    this.teardown();
  }

  /** Pause playback without closing the request (lock-screen pause). */
  pause(): void {
    try {
      this.audio?.pause();
    } catch {
      // Element already disposed.
    }
  }

  /**
   * Resume after a pause by re-opening the stream rather than continuing
   * from the buffered position — a paused live stream would otherwise
   * resume minutes behind the actual traffic.
   */
  resume(): void {
    if (!this.active) return;
    this.open();
  }

  isActive(): boolean {
    return this.active;
  }

  /** This connection's id, as sent to the server. */
  streamId(): string {
    return this.sid;
  }

  /**
   * Playback position on the stream's own timeline, or null when nothing
   * is streaming. Scheduling is driven off this rather than the wall clock.
   */
  currentTime(): number | null {
    return this.audio ? this.audio.currentTime : null;
  }

  /** Register a callback fired each time the stream timeline restarts. */
  setOnReset(fn: (() => void) | null): void {
    this.onReset = fn;
  }

  /**
   * Register a callback for whether the stream is really playing. The
   * setting being on is not the same thing: after a reload autoplay policy
   * refuses a stream opened without a gesture, so the player sits armed
   * and silent until the next interaction. The UI has to show that state
   * rather than claim to be streaming.
   */
  setOnActiveChange(fn: ((active: boolean) => void) | null): void {
    this.onActiveChange = fn;
  }

  private open(): void {
    this.teardown();

    // A new connection restarts the server-side timeline at zero, so any
    // cue held from the previous one no longer means anything.
    this.sid = newSid();
    this.onReset?.();

    const el = new Audio();
    el.preload = "auto";
    // A cache-buster keeps a reconnect from being served a dead response
    // out of the HTTP cache.
    el.src = `${STREAM_PATH}?t=${Date.now()}&sid=${encodeURIComponent(this.sid)}`;
    el.addEventListener("error", this.handleDrop);
    el.addEventListener("ended", this.handleDrop);
    this.audio = el;

    void el.play().then(
      () => {
        if (this.audio === el) this.onActiveChange?.(true);
      },
      () => {
        // Autoplay policy refused it, or the element was replaced. Leave
        // the setting on so the next gesture or reconnect can retry, but
        // do not pretend audio is playing.
        if (this.audio === el) this.onActiveChange?.(false);
      },
    );
  }

  /**
   * The stream should never end on its own, so `ended` means the response
   * was cut — by a proxy, a network change, or the server restarting.
   */
  private handleDrop = (): void => {
    this.onActiveChange?.(false);
    if (!this.active || this.reconnectTimer) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (this.active) this.open();
    }, RECONNECT_MS);
  };

  private clearTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private teardown(): void {
    const el = this.audio;
    if (!el) return;
    el.removeEventListener("error", this.handleDrop);
    el.removeEventListener("ended", this.handleDrop);
    try {
      el.pause();
      // Drop the source so the browser tears the connection down instead of
      // leaving it open server-side.
      el.removeAttribute("src");
      el.load();
    } catch {
      // Element already disposed.
    }
    this.audio = null;
  }
}

export const streamPlayer = new StreamPlayer();
