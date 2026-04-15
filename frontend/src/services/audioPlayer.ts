import type { Call } from "@/types";

interface QueueItem {
  call: Call;
  audioUrl: string;
}

class AudioPlayer {
  private audio: HTMLAudioElement;
  private preloadAudio: HTMLAudioElement;
  private audioContext: AudioContext | null = null;
  private gainNode: GainNode | null = null;
  private sourceNode: MediaElementAudioSourceNode | null = null;
  private volume = 1;
  private queue: QueueItem[] = [];
  private currentItem: QueueItem | null = null;
  private _paused = false;
  private callStartCb: ((call: Call) => void) | null = null;
  private callEndCb: (() => void) | null = null;
  private queueChangeCb: ((length: number) => void) | null = null;

  constructor() {
    this.audio = new Audio();
    this.preloadAudio = new Audio();
    this.preloadAudio.preload = "auto";

    this.audio.addEventListener("ended", () => {
      this.onEnded();
    });

    this.audio.addEventListener("error", () => {
      this.onEnded();
    });
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
      this.preloadNext();
    }
  }

  /**
   * Interrupt current playback to play a call immediately.
   * The interrupted call is pushed to the front of the queue
   * so it resumes after the new call finishes.
   */
  playNow(call: Call, audioUrl: string): void {
    const item: QueueItem = { call, audioUrl };
    if (!this.currentItem) {
      this.startPlayback(item);
      return;
    }
    // Push interrupted call to front of queue so it plays next
    this.audio.pause();
    this.queue.unshift(this.currentItem);
    this.currentItem = null;
    this.queueChangeCb?.(this.queue.length);
    this.startPlayback(item);
  }

  skip(): void {
    this.audio.pause();
    if (this.currentItem) {
      this.cleanup(this.currentItem.audioUrl);
    }
    this.playNext();
  }

  replay(): void {
    if (this.currentItem) {
      this.resumeAudioContext();
      this.audio.currentTime = 0;
      this.audio.play().catch(() => {});
    }
  }

  pause(): void {
    this._paused = true;
    this.audio.pause();
  }

  resume(): void {
    this._paused = false;
    if (this.currentItem) {
      this.resumeAudioContext();
      this.audio.play().catch(() => {});
    } else if (this.queue.length > 0) {
      this.playNext();
    }
  }

  setVolume(v: number): void {
    this.volume = Math.max(0, Math.min(1, v));
    if (this.gainNode) {
      this.gainNode.gain.value = this.volume;
    } else {
      this.audio.volume = this.volume;
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
    // audioName typically already includes the extension (e.g. "call.m4a")
    a.download = /\.\w+$/.test(name) ? name : `${name}.mp3`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  isPlaying(): boolean {
    return this.currentItem !== null && !this.audio.paused;
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
      this.audio.pause();
      this.cleanup(this.currentItem.audioUrl);
      this.currentItem = null;
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
    this.preloadNext();
  }

  getCurrentCall(): Call | null {
    return this.currentItem?.call ?? null;
  }

  private startPlayback(item: QueueItem): void {
    this.currentItem = item;
    this.audio.src = item.audioUrl;

    this.resumeAudioContext();
    this.audio.play().then(
      () => {
        this.callStartCb?.(item.call);
      },
      () => {
        // Autoplay blocked — keep the call so resume() can play it.
        // Subsequent play() calls will queue behind it normally.
        // Show the call in the UI so the user knows what's pending.
        this.callStartCb?.(item.call);
      },
    );
    this.preloadNext();
  }

  /** Create the Web Audio graph if needed and resume a suspended context. */
  private resumeAudioContext(): void {
    if (!this.audioContext) {
      try {
        this.audioContext = new AudioContext();
        this.sourceNode = this.audioContext.createMediaElementSource(
          this.audio,
        );
        this.gainNode = this.audioContext.createGain();
        this.gainNode.gain.value = this.volume;
        this.sourceNode.connect(this.gainNode);
        this.gainNode.connect(this.audioContext.destination);
      } catch {
        // Fallback to HTML volume if Web Audio not available
        this.audio.volume = this.volume;
        return;
      }
    }

    if (this.audioContext.state === "suspended") {
      this.audioContext.resume().catch(() => {});
    }
  }

  private onEnded(): void {
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
      this.callEndCb?.();
    }
  }

  private preloadNext(): void {
    if (this.queue.length > 0) {
      this.preloadAudio.src = this.queue[0].audioUrl;
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
