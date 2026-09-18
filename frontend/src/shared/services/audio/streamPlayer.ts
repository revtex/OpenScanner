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

/** Delay before re-opening a stream that ended or errored. */
const RECONNECT_MS = 2000;

class StreamPlayer {
  private audio: HTMLAudioElement | null = null;
  private active = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

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

  stop(): void {
    this.active = false;
    this.clearTimer();
    this.teardown();
  }

  isActive(): boolean {
    return this.active;
  }

  private open(): void {
    this.teardown();

    const el = new Audio();
    el.preload = "auto";
    // A cache-buster keeps a reconnect from being served a dead response
    // out of the HTTP cache.
    el.src = `${STREAM_PATH}?t=${Date.now()}`;
    el.addEventListener("error", this.handleDrop);
    el.addEventListener("ended", this.handleDrop);
    this.audio = el;

    void el.play().catch(() => {
      // Autoplay policy refused it, or the element was replaced. Leave the
      // toggle on so the next gesture or reconnect can retry.
    });
  }

  /**
   * The stream should never end on its own, so `ended` means the response
   * was cut — by a proxy, a network change, or the server restarting.
   */
  private handleDrop = (): void => {
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
