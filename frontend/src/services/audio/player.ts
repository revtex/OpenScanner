import type { Call } from "@/types";
import { bootstrapBeepContext } from "@/services/audio/beep";

interface QueueItem {
  call: Call;
  /** True for search/bookmark plays — discardable on a newer playNow. */
  onDemand?: boolean;
  /**
   * Set after we've already attempted a silent auth refresh + retry for
   * this item. Prevents an infinite refresh loop when the failure isn't
   * actually about auth.
   */
  recoveryTried?: boolean;
}

// Extend window for Safari's prefixed AudioContext.
declare global {
  interface Window {
    webkitAudioContext?: typeof AudioContext;
  }
}

/**
 * Build the on-demand audio URL for a call. The server authenticates the
 * request via the `os_session` cookie issued at login/refresh; no JS-side
 * header injection is required.
 */
function audioUrlFor(call: Call): string {
  return `/api/v1/calls/${call.id}/audio`;
}

/**
 * Choose a sensible filename for downloads triggered by the player.
 */
function downloadNameFor(call: Call): string {
  const name = call.audioName || `call-${call.id}`;
  return /\.\w+$/.test(name) ? name : `${name}.mp3`;
}

class AudioPlayer {
  private ctx: AudioContext | null = null;
  private gainNode: GainNode | null = null;
  private audio: HTMLAudioElement | null = null;
  private volume = 1;
  private queue: QueueItem[] = [];
  private currentItem: QueueItem | null = null;
  private _paused = false;
  private _playing = false;
  private callStartCb: ((call: Call) => void) | null = null;
  private callEndCb: (() => void) | null = null;
  private queueChangeCb: ((length: number) => void) | null = null;
  private authRecovery: (() => Promise<boolean>) | null = null;

  constructor() {
    this.bootstrapAudio();
  }

  /**
   * Attach gesture listeners so that the AudioContext is created/resumed
   * and the persistent <audio> element is unlocked inside a user-gesture
   * handler — required by mobile browsers (Android Edge/Chrome, iOS
   * Safari) that enforce strict autoplay policies. The unlock plays an
   * empty source then immediately pauses, leaving the element in a state
   * where subsequent programmatic play() calls succeed.
   */
  private bootstrapAudio(): void {
    const events: Array<keyof DocumentEventMap> = [
      "mousedown",
      "touchstart",
      "keydown",
    ];

    const handler = async () => {
      this.ensureContext();
      this.ensureAudioElement();

      if (this.ctx?.state === "suspended") {
        try {
          await this.ctx.resume();
        } catch {
          // ignore
        }
      }

      // Unlock the <audio> element on the same gesture so later
      // programmatic play() succeeds on Mobile Edge / Mobile Safari.
      if (this.audio) {
        try {
          await this.audio.play();
          this.audio.pause();
        } catch {
          // ignore — the element will still be considered
          // user-activated on most browsers once a gesture-scoped
          // play() has been attempted.
        }
      }

      await bootstrapBeepContext();

      if (this.ctx?.state === "running" && this.audio) {
        for (const e of events) {
          document.body.removeEventListener(e, handler);
        }
      }
    };

    for (const e of events) {
      document.body.addEventListener(e, handler);
    }
  }

  /** Enqueue a live (ingested) call for playback. */
  enqueue(call: Call): void {
    const item: QueueItem = { call };
    if (this._paused) {
      this.queue.push(item);
      this.queueChangeCb?.(this.queue.length);
      return;
    }
    if (!this.currentItem) {
      this.startPlayback(item);
    } else {
      this.queue.push(item);
      this.queueChangeCb?.(this.queue.length);
    }
  }

  /**
   * Play a call immediately from search/bookmarks.
   *
   * - If nothing is playing, just play.
   * - If an ingested (live) call is playing, push it back to the front
   *   of the queue so it resumes after this on-demand call finishes.
   * - If another on-demand call is playing, discard it (don't re-queue).
   * - Ingested calls in the queue are never touched.
   */
  playNow(call: Call): void {
    const item: QueueItem = { call, onDemand: true };
    if (!this.currentItem) {
      this.startPlayback(item);
      return;
    }

    if (!this.currentItem.onDemand) {
      // Currently playing an ingested call — push it back to front.
      this.queue.unshift(this.currentItem);
      this.queueChangeCb?.(this.queue.length);
    }

    this.currentItem = null;
    this._playing = false;
    this.stopAudio();
    this.startPlayback(item);
  }

  skip(): void {
    this.currentItem = null;
    this._playing = false;
    this.stopAudio();
    this.playNext();
  }

  replay(): void {
    if (!this.currentItem || !this.audio) return;
    try {
      this.audio.currentTime = 0;
    } catch {
      // ignore — element may not be ready
    }
    this.audio.play().catch(() => this.handleError());
  }

  pause(): void {
    this._paused = true;
    this.audio?.pause();
    this.ctx?.suspend().catch(() => {});
  }

  resume(): void {
    this._paused = false;
    this.ensureContext();
    this.ctx?.resume().catch(() => {});
    if (this.currentItem && this.audio && !this._playing) {
      this._playing = true;
      this.audio.play().catch(() => this.handleError());
    } else if (!this.currentItem && this.queue.length > 0) {
      this.playNext();
    }
  }

  setVolume(v: number): void {
    this.volume = Math.max(0, Math.min(1, v));
    if (this.gainNode) {
      this.gainNode.gain.value = this.volume;
    } else if (this.audio) {
      // GainNode unavailable — mirror to the element so the slider
      // still works in degraded environments.
      this.audio.volume = this.volume;
    }
  }

  getVolume(): number {
    return this.volume;
  }

  download(): void {
    if (!this.currentItem) return;
    const a = document.createElement("a");
    a.href = audioUrlFor(this.currentItem.call);
    a.download = downloadNameFor(this.currentItem.call);
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  isPlaying(): boolean {
    return this._playing;
  }

  /** Current playback position in seconds, or 0 if not playing. */
  getPlaybackTime(): number {
    if (!this._playing || !this.audio) return 0;
    return this.audio.currentTime || 0;
  }

  setOnCallStart(cb: (call: Call) => void): void {
    this.callStartCb = cb;
  }

  setOnCallEnd(cb: () => void): void {
    this.callEndCb = cb;
  }

  setOnQueueChange(cb: (length: number) => void): void {
    this.queueChangeCb = cb;
  }

  /**
   * Register a callback invoked when an audio fetch fails. The callback
   * should attempt a silent auth refresh (e.g. POST /api/v1/auth/refresh)
   * and resolve to `true` if it succeeded — in which case the player
   * retries the current call with the new session cookie. Required
   * because <audio src=…> bypasses the RTK Query 401 retry path, so a
   * server-side token revocation (e.g. a sibling device exhausting the
   * concurrent-token cap) would otherwise leave playback broken until
   * the next scheduled refresh fires.
   */
  setAuthRecovery(fn: () => Promise<boolean>): void {
    this.authRecovery = fn;
  }

  clearQueue(): void {
    this.queue = [];
    this.queueChangeCb?.(0);
    if (this.currentItem) {
      this.currentItem = null;
      this._playing = false;
      this.stopAudio();
    }
    this.callEndCb?.();
  }

  filterQueue(predicate: (call: Call) => boolean): void {
    this.queue = this.queue.filter((item) => predicate(item.call));
    this.queueChangeCb?.(this.queue.length);
  }

  getCurrentCall(): Call | null {
    return this.currentItem?.call ?? null;
  }

  // -- Private --

  private ensureContext(): void {
    if (this.ctx) return;
    const Ctor = window.AudioContext || window.webkitAudioContext;
    if (!Ctor) return;
    this.ctx = new Ctor({ latencyHint: "playback" });
    this.gainNode = this.ctx.createGain();
    this.gainNode.gain.value = this.volume;
    this.gainNode.connect(this.ctx.destination);
    this.ctx.onstatechange = () => {
      if (this.ctx?.state === "suspended" && !this._paused) {
        this.ctx.resume().catch(() => {});
      }
    };
  }

  /**
   * Lazily create the persistent HTMLAudioElement and wire it through
   * MediaElementAudioSourceNode → GainNode → destination. The element
   * and the source node are created exactly once for the lifetime of
   * the player; subsequent calls only reset the element's `src`.
   */
  private ensureAudioElement(): void {
    if (this.audio) return;

    const audio = new Audio();
    audio.preload = "auto";
    audio.addEventListener("ended", this.handleEnded);
    audio.addEventListener("error", this.handleError);

    if (this.ctx && this.gainNode) {
      try {
        const node = this.ctx.createMediaElementSource(audio);
        node.connect(this.gainNode);
      } catch {
        // Element already attached to a source, or feature unavailable —
        // fall back to direct element output.
        audio.volume = this.volume;
      }
    } else {
      audio.volume = this.volume;
    }

    this.audio = audio;
  }

  private startPlayback(item: QueueItem): void {
    this.currentItem = item;
    this.callStartCb?.(item.call);
    this.ensureContext();
    this.ensureAudioElement();
    if (!this.audio) return;

    if (this.ctx?.state === "suspended") {
      this.ctx.resume().catch(() => {});
    }

    const audio = this.audio;
    const onCanPlay = () => {
      audio.removeEventListener("canplay", onCanPlay);
      if (this.currentItem !== item) return;
      this._playing = true;
      audio.play().catch(() => this.handleError());
    };
    audio.addEventListener("canplay", onCanPlay);
    audio.src = audioUrlFor(item.call);
    // preload="auto" + setting src starts the network fetch.
    try {
      audio.load();
    } catch {
      // ignore
    }
  }

  private stopAudio(): void {
    if (!this.audio) return;
    try {
      this.audio.pause();
    } catch {
      // ignore
    }
    this.audio.removeAttribute("src");
    try {
      this.audio.load();
    } catch {
      // ignore
    }
  }

  private handleEnded = (): void => {
    if (!this.currentItem) return;
    this.currentItem = null;
    this._playing = false;
    this.playNext();
  };

  private handleError = (): void => {
    if (!this.currentItem) return;
    const item = this.currentItem;

    // First failure for this item: try a silent auth refresh in case the
    // session cookie's JWT was revoked server-side (concurrent-token cap,
    // explicit logout-elsewhere, etc.). On success, reload the same src
    // — the new Set-Cookie will be picked up automatically.
    if (!item.recoveryTried && this.authRecovery && this.audio) {
      item.recoveryTried = true;
      const recovery = this.authRecovery;
      const audio = this.audio;
      void recovery().then((ok) => {
        if (!ok || this.currentItem !== item || this.audio !== audio) {
          this.skipCurrent(item);
          return;
        }
        try {
          audio.load();
        } catch {
          // ignore
        }
        audio.play().catch(() => this.skipCurrent(item));
      });
      return;
    }

    this.skipCurrent(item);
  };

  private skipCurrent(item: QueueItem): void {
    if (this.currentItem !== item) return;
    console.warn("[audioPlayer] failed to play call", item.call.id);
    this.currentItem = null;
    this._playing = false;
    this.playNext();
  }

  private playNext(): void {
    const next = this.queue.shift();
    this.queueChangeCb?.(this.queue.length);
    if (next) {
      this.startPlayback(next);
    } else {
      this.currentItem = null;
      this._playing = false;
      this.callEndCb?.();
    }
  }
}

export const audioPlayer = new AudioPlayer();
