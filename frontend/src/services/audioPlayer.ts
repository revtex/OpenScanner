import type { Call, TranscriptionSegment } from "@/types";
import { bootstrapBeepContext } from "@/services/beepPlayer";

interface QueueItem {
  call: Call;
  audioData: ArrayBuffer;
  audioUrl: string; // blob URL kept for download only
}

/** Item waiting for its transcript before being queued for playback. */
interface PendingItem {
  call: Call;
  audioData: ArrayBuffer;
  audioUrl: string;
  timer: ReturnType<typeof setTimeout>;
  seq: number; // insertion order — used to preserve FIFO
}

/** Max time (ms) to wait for a transcript before playing anyway. */
const TRANSCRIPT_WAIT_MS = 20_000;

// Extend window for Safari's prefixed AudioContext
declare global {
  interface Window {
    webkitAudioContext?: typeof AudioContext;
  }
}

class AudioPlayer {
  private ctx: AudioContext | null = null;
  private gainNode: GainNode | null = null;
  private source: AudioBufferSourceNode | null = null;
  private fallbackAudio: HTMLAudioElement | null = null;
  private volume = 1;
  private queue: QueueItem[] = [];
  private currentItem: QueueItem | null = null;
  private _paused = false;
  private _playing = false;
  private _startedAt = 0; // AudioContext.currentTime when source.start() was called
  private callStartCb: ((call: Call) => void) | null = null;
  private callEndCb: (() => void) | null = null;
  private queueChangeCb: ((length: number) => void) | null = null;

  /** Calls waiting for their transcript before being released to the queue. */
  private pendingTranscript: Map<number, PendingItem> = new Map();
  private pendingSeq = 0;
  /** Whether to wait for transcripts before playing. */
  private _syncTranscripts = false;

  constructor() {
    this.bootstrapAudio();
  }

  /**
   * Attach gesture listeners so that the AudioContext is created and
   * resumed inside a user-interaction handler — required by mobile
   * browsers (Android Edge/Chrome, iOS Safari) that enforce strict
   * autoplay policies.
   */
  private bootstrapAudio(): void {
    const events: Array<keyof DocumentEventMap> = [
      "mousedown",
      "touchstart",
      "keydown",
    ];

    const handler = async () => {
      if (!this.ctx) {
        const Ctor = window.AudioContext || window.webkitAudioContext;
        if (!Ctor) {
          return;
        }
        this.ctx = new Ctor({ latencyHint: "playback" });
        this.gainNode = this.ctx.createGain();
        this.gainNode.gain.value = this.volume;
        this.gainNode.connect(this.ctx.destination);
      }

      // Await resume inside the gesture handler — required on Mobile Edge.
      if (this.ctx.state === "suspended") {
        try {
          await this.ctx.resume();
        } catch {
          // ignore
        }
      }

      // Re-resume if the browser suspends the context later.
      this.ctx.onstatechange = () => {
        if (this.ctx?.state === "suspended" && !this._paused) {
          this.ctx.resume().catch(() => {});
        }
      };

      await bootstrapBeepContext();

      // Only remove listeners once the context is confirmed running.
      if (this.ctx.state === "running") {
        for (const e of events) {
          document.body.removeEventListener(e, handler);
        }
      }
    };

    for (const e of events) {
      document.body.addEventListener(e, handler);
    }
  }

  play(call: Call, audioData: ArrayBuffer, audioUrl: string): void {
    // When transcript sync is enabled, hold the call in a pending buffer
    // until TRN arrives or the timeout fires.
    if (this._syncTranscripts) {
      const seq = this.pendingSeq++;
      const timer = setTimeout(() => {
        this.releasePending(call.id);
      }, TRANSCRIPT_WAIT_MS);
      this.pendingTranscript.set(call.id, {
        call,
        audioData,
        audioUrl,
        timer,
        seq,
      });
      this.notifyQueueChange();
      return;
    }

    const item: QueueItem = { call, audioData, audioUrl };
    if (this._paused) {
      this.queue.push(item);
      this.notifyQueueChange();
      return;
    }
    if (!this.currentItem) {
      this.startPlayback(item);
    } else {
      this.queue.push(item);
      this.notifyQueueChange();
    }
  }

  playNow(call: Call, audioData: ArrayBuffer, audioUrl: string): void {
    const item: QueueItem = { call, audioData, audioUrl };
    if (!this.currentItem) {
      this.startPlayback(item);
      return;
    }
    this.stopSource();
    this.queue.unshift(this.currentItem);
    this.currentItem = null;
    this.notifyQueueChange();
    this.startPlayback(item);
  }

  skip(): void {
    this.stopSource();
    if (this.currentItem) {
      this.cleanup(this.currentItem.audioUrl);
    }
    this.playNext();
  }

  replay(): void {
    if (this.currentItem) {
      this.stopSource();
      this.decodeAndPlay(this.currentItem);
    }
  }

  pause(): void {
    this._paused = true;
    this.ctx?.suspend();
  }

  resume(): void {
    this._paused = false;
    this.ensureContext();
    this.ctx?.resume().then(() => {
      if (this.currentItem && !this._playing) {
        this.decodeAndPlay(this.currentItem);
      } else if (!this.currentItem && this.queue.length > 0) {
        this.playNext();
      }
    });
  }

  setVolume(v: number): void {
    this.volume = Math.max(0, Math.min(1, v));
    if (this.gainNode) {
      this.gainNode.gain.value = this.volume;
    }
    if (this.fallbackAudio) {
      this.fallbackAudio.volume = this.volume;
    }
  }

  getVolume(): number {
    return this.volume;
  }

  download(): void {
    if (!this.currentItem) return;
    const a = document.createElement("a");
    a.href = this.currentItem.audioUrl;
    const name = this.currentItem.call.audioName || "call";
    a.download = /\.\w+$/.test(name) ? name : `${name}.mp3`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  isPlaying(): boolean {
    return this._playing;
  }

  /** Current playback position in seconds, or 0 if not playing. */
  getPlaybackTime(): number {
    if (!this._playing) return 0;
    if (this.fallbackAudio) {
      return this.fallbackAudio.currentTime;
    }
    if (this.ctx && this._startedAt > 0) {
      return this.ctx.currentTime - this._startedAt;
    }
    return 0;
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

  clearQueue(): void {
    for (const item of this.queue) {
      this.cleanup(item.audioUrl);
    }
    this.queue = [];
    // Also clear pending transcript buffer.
    for (const p of this.pendingTranscript.values()) {
      clearTimeout(p.timer);
      this.cleanup(p.audioUrl);
    }
    this.pendingTranscript.clear();
    this.queueChangeCb?.(0);
    if (this.currentItem) {
      this.stopSource();
      this.cleanup(this.currentItem.audioUrl);
      this.currentItem = null;
      this._playing = false;
    }
    this.callEndCb?.();
  }

  filterQueue(predicate: (call: Call) => boolean): void {
    const kept: QueueItem[] = [];
    for (const item of this.queue) {
      if (predicate(item.call)) {
        kept.push(item);
      } else {
        this.cleanup(item.audioUrl);
      }
    }
    this.queue = kept;
    // Also filter pending transcript buffer.
    for (const [id, pending] of this.pendingTranscript) {
      if (!predicate(pending.call)) {
        clearTimeout(pending.timer);
        this.cleanup(pending.audioUrl);
        this.pendingTranscript.delete(id);
      }
    }
    this.notifyQueueChange();
  }

  /**
   * Enable/disable waiting for transcripts before playing.
   * When enabled, calls are buffered until resolveTranscript() is called
   * or TRANSCRIPT_WAIT_MS elapses.
   */
  setSyncTranscripts(enabled: boolean): void {
    this._syncTranscripts = enabled;
    // If disabling, flush all pending items into the queue immediately.
    if (!enabled) {
      this.flushAllPending();
    }
  }

  /**
   * Called when a TRN message arrives. Attaches the transcript to the
   * pending call and releases it (in order) to the playback queue.
   */
  resolveTranscript(
    callId: number,
    text: string,
    segments?: TranscriptionSegment[],
  ): void {
    const pending = this.pendingTranscript.get(callId);
    if (pending) {
      clearTimeout(pending.timer);
      pending.call.transcript = text;
      pending.call.transcriptSegments = segments;
      this.pendingTranscript.delete(callId);
      this.flushReady();
    }
    // Also update the current playing call or queued items.
    if (this.currentItem?.call.id === callId) {
      this.currentItem.call.transcript = text;
      this.currentItem.call.transcriptSegments = segments;
      // Re-fire callStartCb so Redux gets the updated call.
      this.callStartCb?.(this.currentItem.call);
    }
    for (const item of this.queue) {
      if (item.call.id === callId) {
        item.call.transcript = text;
        item.call.transcriptSegments = segments;
      }
    }
  }

  getCurrentCall(): Call | null {
    return this.currentItem?.call ?? null;
  }

  // -- Private --

  /** Fallback in case bootstrapAudio hasn't fired yet. */
  private ensureContext(): void {
    if (!this.ctx) {
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
    if (this.ctx.state === "suspended") {
      this.ctx.resume().catch(() => {});
    }
  }

  private startPlayback(item: QueueItem): void {
    this.currentItem = item;
    this.callStartCb?.(item.call);
    this.ensureContext();
    if (this.ctx?.state === "running") {
      this.decodeAndPlay(item);
    } else if (this.ctx) {
      // Context not yet running — wait for the bootstrap resume.
      const onReady = () => {
        if (this.ctx?.state === "running") {
          this.ctx.removeEventListener("statechange", onReady);
          if (this.currentItem === item) {
            this.decodeAndPlay(item);
          }
        }
      };
      this.ctx.addEventListener("statechange", onReady);
    }
  }

  private decodeAndPlay(item: QueueItem): void {
    if (!this.ctx || !this.gainNode) {
      return;
    }
    const playingItem = item;
    const p = this.ctx.decodeAudioData(
      item.audioData.slice(0),
      (audioBuffer) => {
        if (this.currentItem !== playingItem) return;

        this.stopSource();
        const src = this.ctx!.createBufferSource();
        src.buffer = audioBuffer;
        src.connect(this.gainNode!);
        src.onended = () => {
          if (this.currentItem === playingItem) {
            this.onEnded();
          }
        };
        src.start();
        this.source = src;
        this._playing = true;
        this._startedAt = this.ctx!.currentTime;
      },
      () => {
        // WebAudio decode failed — fall back to HTMLAudioElement which
        // has broader codec support on mobile browsers.
        if (this.currentItem !== playingItem) return;
        this.playViaAudioElement(item);
      },
    );
    // Suppress "Uncaught (in promise) EncodingError" on Mobile Edge.
    if (p && typeof p.catch === "function") {
      p.catch(() => {});
    }
  }

  /**
   * Fallback playback via HTMLAudioElement. Used when decodeAudioData
   * fails (e.g. unsupported codec in WebAudio but supported by the
   * platform media decoder).
   */
  private playViaAudioElement(item: QueueItem): void {
    const playingItem = item;
    this.stopSource();

    const audio = new Audio(item.audioUrl);
    audio.volume = this.volume;
    audio.onended = () => {
      if (this.currentItem === playingItem) {
        this.onEnded();
      }
    };
    audio.onerror = () => {
      if (this.currentItem === playingItem) {
        this.onEnded();
      }
    };
    audio.play().catch(() => {
      if (this.currentItem === playingItem) {
        this.onEnded();
      }
    });
    this.fallbackAudio = audio;
    this._playing = true;
  }

  private stopSource(): void {
    if (this.fallbackAudio) {
      this.fallbackAudio.onended = null;
      this.fallbackAudio.onerror = null;
      this.fallbackAudio.pause();
      this.fallbackAudio.src = "";
      this.fallbackAudio = null;
    }
    if (this.source) {
      this.source.onended = null;
      try {
        this.source.stop();
      } catch {
        // already stopped
      }
      this.source.disconnect();
      this.source = null;
    }
    this._playing = false;
  }

  private onEnded(): void {
    this.stopSource();
    if (this.currentItem) {
      this.cleanup(this.currentItem.audioUrl);
      this.currentItem = null;
    }
    this.playNext();
  }

  private playNext(): void {
    const next = this.queue.shift();
    this.notifyQueueChange();
    if (next) {
      this.startPlayback(next);
    } else {
      this.currentItem = null;
      this._playing = false;
      this.callEndCb?.();
    }
  }

  private cleanup(url: string): void {
    try {
      URL.revokeObjectURL(url);
    } catch {
      // ignore
    }
  }

  /**
   * Release a pending item (by callId) into the playback queue.
   * Called on timeout or when transcript arrives.
   */
  private releasePending(callId: number): void {
    const pending = this.pendingTranscript.get(callId);
    if (!pending) return;
    clearTimeout(pending.timer);
    this.pendingTranscript.delete(callId);
    this.enqueueItem({
      call: pending.call,
      audioData: pending.audioData,
      audioUrl: pending.audioUrl,
    });
  }

  /**
   * Flush pending items that are ready. When an item gets its transcript
   * or times out, it is released. We release contiguous items from the
   * front (lowest seq) to preserve call order.
   */
  private flushReady(): void {
    // Find the lowest seq still pending.
    let minPendingSeq = Infinity;
    for (const p of this.pendingTranscript.values()) {
      if (p.seq < minPendingSeq) minPendingSeq = p.seq;
    }
    // Nothing else to flush — releasePending already enqueued the item.
    this.notifyQueueChange();
  }

  /** Move all pending items into the playback queue, ordered by seq. */
  private flushAllPending(): void {
    const items = [...this.pendingTranscript.values()].sort(
      (a, b) => a.seq - b.seq,
    );
    for (const p of items) {
      clearTimeout(p.timer);
    }
    this.pendingTranscript.clear();
    for (const p of items) {
      this.enqueueItem({ call: p.call, audioData: p.audioData, audioUrl: p.audioUrl });
    }
  }

  /** Total items waiting (pending transcript + queued). */
  private notifyQueueChange(): void {
    this.queueChangeCb?.(this.queue.length + this.pendingTranscript.size);
  }

  /** Push an item to the queue or start playback if idle. */
  private enqueueItem(item: QueueItem): void {
    if (this._paused) {
      this.queue.push(item);
      this.notifyQueueChange();
      return;
    }
    if (!this.currentItem) {
      this.startPlayback(item);
    } else {
      this.queue.push(item);
      this.notifyQueueChange();
    }
  }
}

export const audioPlayer = new AudioPlayer();
