import type { AppDispatch } from "@/app/store";
import {
  callReceived,
  setConfig,
  setBranding,
  setListenerCount,
  setConnectionStatus,
  transcriptReceived,
} from "@/app/slices/scannerSlice";
import { clearCredentials } from "@/app/slices/authSlice";
import type { Call, WsCommand } from "@/types";

const MAX_BACKOFF = 30_000;
const DEDUP_SIZE = 100;

type AudioReceivedCallback = (
  call: Call,
  audioUrl: string,
  audioData: ArrayBuffer,
) => void;
type CallFilter = (call: Call) => boolean;
type TokenExpiredCallback = () => Promise<string | null>;

interface WsAuth {
  token?: string;
  publicAccess?: boolean;
}

/** Decode a base64 string to an ArrayBuffer. */
function base64ToArrayBuffer(b64: string): ArrayBuffer {
  const bin = atob(b64);
  const buf = new ArrayBuffer(bin.length);
  const view = new Uint8Array(buf);
  for (let i = 0; i < bin.length; i++) {
    view[i] = bin.charCodeAt(i);
  }
  return buf;
}

class WsClient {
  private ws: WebSocket | null = null;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private backoff = 1000;
  private dispatch: AppDispatch | null = null;
  private auth: WsAuth = {};
  private audioCallback: AudioReceivedCallback | null = null;
  private callFilter: CallFilter | null = null;
  private tokenExpiredCallback: TokenExpiredCallback | null = null;
  private intentionalClose = false;
  private recentCallIds: number[] = [];

  connect(dispatch: AppDispatch, auth: WsAuth = {}): void {
    this.dispatch = dispatch;
    this.auth = auth;
    this.intentionalClose = false;
    this.doConnect();
  }

  disconnect(): void {
    this.intentionalClose = true;
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }
    if (this.ws) {
      // Detach handlers before closing to avoid browser console noise
      // when the socket is still in CONNECTING state.
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.close();
      this.ws = null;
    }
    this.recentCallIds = [];
    this.dispatch?.(setConnectionStatus("disconnected"));
  }

  send(command: WsCommand, payload?: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      const msg = payload !== undefined ? [command, payload] : [command];
      this.ws.send(JSON.stringify(msg));
    }
  }

  onAudioReceived(cb: AudioReceivedCallback): void {
    this.audioCallback = cb;
  }

  setCallFilter(filter: CallFilter): void {
    this.callFilter = filter;
  }

  onTokenExpired(cb: TokenExpiredCallback): void {
    this.tokenExpiredCallback = cb;
  }

  private doConnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }

    this.dispatch?.(setConnectionStatus("connecting"));

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${proto}//${window.location.host}/ws`;
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      this.backoff = 1000;
      this.dispatch?.(setConnectionStatus("connected"));

      // Authenticate based on mode
      if (this.auth.token) {
        if (this.ws?.readyState === WebSocket.OPEN) {
          this.ws.send(JSON.stringify([this.auth.token]));
        }
      }
    };

    this.ws.onmessage = (event: MessageEvent) => {
      if (typeof event.data === "string") {
        this.handleTextMessage(event.data);
      }
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.dispatch?.(setConnectionStatus("disconnected"));
      if (!this.intentionalClose) {
        this.reconnect();
      }
    };

    this.ws.onerror = () => {
      // Let onclose handle reconnect
    };
  }

  private handleTextMessage(data: string): void {
    let parsed: unknown[];
    try {
      parsed = JSON.parse(data) as unknown[];
    } catch {
      return;
    }

    if (!Array.isArray(parsed) || parsed.length === 0) return;

    const command = parsed[0] as WsCommand;
    const payload = parsed[1];

    switch (command) {
      case "CAL":
        if (
          payload &&
          typeof payload === "object" &&
          "id" in payload &&
          "dateTime" in payload
        ) {
          const raw = payload as Record<string, unknown>;
          const audioB64 = raw.audio as string | undefined;
          // Strip the audio field before dispatching metadata to Redux.
          delete raw.audio;
          const call = raw as unknown as Call;

          if (this.callFilter && !this.callFilter(call)) {
            break;
          }

          // Dedup: skip if this call ID was already processed recently.
          if (this.recentCallIds.includes(call.id)) {
            break;
          }
          this.recentCallIds.push(call.id);
          if (this.recentCallIds.length > DEDUP_SIZE) {
            this.recentCallIds.shift();
          }

          this.dispatch?.(callReceived(call));

          if (audioB64) {
            const audioData = base64ToArrayBuffer(audioB64);
            const mimeType = call.audioType || "audio/mpeg";
            const blob = new Blob([audioData], { type: mimeType });
            const audioUrl = URL.createObjectURL(blob);
            this.audioCallback?.(call, audioUrl, audioData);
          }
        }
        break;
      case "CFG":
        if (this.dispatch) {
          const cfg = payload as {
            systems?: unknown;
            branding?: string;
            email?: string;
            version?: string;
            time12hFormat?: boolean | string;
            showListenersCount?: boolean | string;
            playbackGoesLive?: boolean | string;
            shareableLinks?: boolean | string;
            keypadBeeps?: string;
          };
          // CFG carries systems + display prefs. Branding/email/version
          // arrive separately via VER, so only override if CFG explicitly
          // provides them — otherwise keep current values.
          this.dispatch(
            setConfig({
              systems: (cfg.systems ?? []) as import("@/types").SystemConfig[],
              branding: cfg.branding,
              email: cfg.email,
              version: cfg.version,
              time12hFormat:
                cfg.time12hFormat === true || cfg.time12hFormat === "true",
              showListenersCount:
                cfg.showListenersCount === true ||
                cfg.showListenersCount === "true",
              playbackGoesLive:
                cfg.playbackGoesLive === true ||
                cfg.playbackGoesLive === "true",
              shareableLinks:
                cfg.shareableLinks === true || cfg.shareableLinks === "true",
              keypadBeeps: cfg.keypadBeeps ?? "",
            }),
          );
        }
        break;
      case "VER":
        if (this.dispatch && payload && typeof payload === "object") {
          const ver = payload as {
            version?: string;
            branding?: string;
            email?: string;
          };
          // VER updates branding/version/email — merge with existing config
          this.dispatch(
            setBranding({
              branding: ver.branding ?? "",
              email: ver.email ?? "",
              version: ver.version ?? "",
            }),
          );
        }
        break;
      case "LSC":
        if (typeof payload === "number") {
          this.dispatch?.(setListenerCount(payload));
        }
        break;
      case "XPR":
        // Token expired — attempt silent refresh before disconnecting.
        console.warn("[WsClient] Token expired, attempting refresh");
        if (this.tokenExpiredCallback) {
          this.tokenExpiredCallback().then((newToken) => {
            if (newToken) {
              // Update auth and reconnect with new token.
              this.auth = { ...this.auth, token: newToken };
              this.disconnect();
              this.intentionalClose = false;
              this.doConnect();
            } else {
              // Refresh failed — clear credentials.
              this.dispatch?.(clearCredentials());
              this.disconnect();
            }
          });
        } else {
          this.dispatch?.(clearCredentials());
          this.disconnect();
        }
        break;
      case "MAX":
        console.warn("[WsClient] Server max clients reached");
        break;
      case "TRN":
        if (
          payload &&
          typeof payload === "object" &&
          "callId" in payload &&
          "text" in payload
        ) {
          this.dispatch?.(
            transcriptReceived(payload as { callId: number; text: string }),
          );
        }
        break;
      default:
        break;
    }
  }

  private reconnect(): void {
    if (this.reconnectTimeout) return;

    // Exponential backoff with jitter
    const jitter = Math.random() * 500;
    const delay = Math.min(this.backoff + jitter, MAX_BACKOFF);
    this.backoff = Math.min(this.backoff * 2, MAX_BACKOFF);

    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null;
      this.doConnect();
    }, delay);
  }
}

export const wsClient = new WsClient();
