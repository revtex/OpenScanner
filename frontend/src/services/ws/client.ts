import type { AppDispatch } from "@/app/store";
import {
  callReceived,
  setConfig,
  setBranding,
  setListenerCount,
  setConnectionStatus,
  transcriptReceived,
} from "@/app/slices/scanner/scannerSlice";
import { clearCredentials } from "@/app/slices/shared/authSlice";
import type { Call, WsCommand, TranscriptionSegment } from "@/types";

const MAX_BACKOFF = 30_000;
const DEDUP_SIZE = 100;

type TokenExpiredCallback = () => Promise<string | null>;

interface WsAuth {
  token?: string;
  publicAccess?: boolean;
}

class WsClient {
  private ws: WebSocket | null = null;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private backoff = 1000;
  private dispatch: AppDispatch | null = null;
  private auth: WsAuth = {};
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
      const old = this.ws;
      old.onopen = null;
      old.onmessage = null;
      old.onclose = null;
      old.onerror = null;
      if (old.readyState === WebSocket.CONNECTING) {
        old.onopen = () => old.close();
      } else if (old.readyState === WebSocket.OPEN) {
        old.close();
      }
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

  onTokenExpired(cb: TokenExpiredCallback): void {
    this.tokenExpiredCallback = cb;
  }

  private doConnect(): void {
    if (this.ws) {
      const old = this.ws;
      old.onopen = null;
      old.onmessage = null;
      old.onclose = null;
      old.onerror = null;
      if (old.readyState === WebSocket.CONNECTING) {
        old.addEventListener("open", () => old.close(), { once: true });
      } else if (old.readyState === WebSocket.OPEN) {
        old.close();
      }
      this.ws = null;
    }

    this.dispatch?.(setConnectionStatus("connecting"));

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${proto}//${window.location.host}/api/ws`;
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
          // Older servers embedded base64 audio bytes here; the field is
          // ignored now that audio is fetched on demand from
          // /api/calls/:id/audio.
          if ("audio" in raw) {
            delete raw.audio;
          }
          const call = raw as unknown as Call;

          // Dedup: skip if this call ID was already processed recently.
          if (this.recentCallIds.includes(call.id)) {
            break;
          }
          this.recentCallIds.push(call.id);
          if (this.recentCallIds.length > DEDUP_SIZE) {
            this.recentCallIds.shift();
          }

          this.dispatch?.(callReceived(call));
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
            transcriptionEnabled?: boolean | string;
            liveTranscriptDisplay?: boolean | string;
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
              transcriptionEnabled:
                cfg.transcriptionEnabled === true ||
                cfg.transcriptionEnabled === "true",
              liveTranscriptDisplay:
                cfg.liveTranscriptDisplay === true ||
                cfg.liveTranscriptDisplay === "true",
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
          const trn = payload as {
            callId: number;
            text: string;
            segments?: TranscriptionSegment[];
          };
          this.dispatch?.(transcriptReceived(trn));
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
