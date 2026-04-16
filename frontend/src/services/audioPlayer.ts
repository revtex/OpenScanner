import type { Call } from "@/types";

interface QueueItem {
  call: Call;
  audioUrl: string;
}

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
  private volume = 1;
  private queue: QueueItem[] = [];
  private currentItem: QueueItem | null = null;
  private _paused = false;
  private _playing = false;
  private callStartCb: ((call: Call) => void) | null = null;
  private callEndCb: (() => void) | null = null;
  private queueChangeCb: ((length: number) => void) | null = null;

  constructor() {
    // No bootstrap listeners needed — AudioContext is created when
    // the user clicks the Live button (a user gesture), which
    // satisfies the browser autoplay policy.
  }

  play(call: Call, audioUrl: string): void {
    const item: QueueItem = { call, audioUrl };
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

  playNow(call: Call, audioUrl: string): void {
    const item: QueueItem = { call, audioUrl };
    if (!this.currentItem) {
      this.startPlayback(item);
      return;
    }
    this.stopSource();
    this.queue.unshift(this.currentItem);
    this.currentItem = null;
    this.queueChangeCb?.(this.queue.length);
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
    this.queueChangeCb?.(this.queue.length);
  }

  getCurrentCall(): Call | null {
    return this.currentItem?.call ?? null;
  }

  // -- Private --

  private ensureContext(): void {
    if (!this.ctx) {
      const Ctor = window.AudioContext || window.webkitAudioContext;
      if (!Ctor) return;
      this.ctx = new Ctor({ latencyHint: "playback" });
      this.gainNode = this.ctx.createGain();
      this.gainNode.gain.value = this.volume;
      this.gainNode.connect(this.ctx.destination);
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
    }
  }

  private decodeAndPlay(item: QueueItem): void {
    if (!this.ctx || !this.gainNode) return;
    // Item may have changed by the time async work completes —
    // capture a reference to check against.
    const playingItem = item;

    fetch(item.audioUrl)
      .then((r) => r.arrayBuffer())
      .then((buf) => this.ctx!.decodeAudioData(buf))
      .then((audioBuffer) => {
        // Bail if item changed while we were decoding.
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
      })
      .catch(() => {
        // Decode or fetch failure — skip to next.
        if (this.currentItem === playingItem) {
          this.onEnded();
        }
      });
  }

  private stopSource(): void {
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
    this.queueChangeCb?.(this.queue.length);
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
}

export const audioPlayer = new AudioPlayer();
