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
    if (!this.currentItem) {
      this.startPlayback(item);
    } else {
      this.queue.push(item);
      this.queueChangeCb?.(this.queue.length);
      this.preloadNext();
    }
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
      this.audio.currentTime = 0;
      this.audio.play().catch(() => {});
    }
  }

  pause(): void {
    this.audio.pause();
  }

  resume(): void {
    if (this.currentItem) {
      this.audio.play().catch(() => {});
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
    const ext = this.currentItem.call.audioType?.includes("wav")
      ? "wav"
      : "mp3";
    a.download = `${this.currentItem.call.audioName || "call"}.${ext}`;
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

  private startPlayback(item: QueueItem): void {
    this.currentItem = item;
    this.audio.src = item.audioUrl;

    this.ensureAudioContext();
    this.audio.play().catch(() => {});
    this.callStartCb?.(item.call);
    this.preloadNext();
  }

  private ensureAudioContext(): void {
    if (this.audioContext) return;

    try {
      this.audioContext = new AudioContext();
      this.sourceNode = this.audioContext.createMediaElementSource(this.audio);
      this.gainNode = this.audioContext.createGain();
      this.gainNode.gain.value = this.volume;
      this.sourceNode.connect(this.gainNode);
      this.gainNode.connect(this.audioContext.destination);
    } catch {
      // Fallback to HTML volume if Web Audio not available
      this.audio.volume = this.volume;
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
