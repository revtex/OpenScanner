import type { AppDispatch } from "@/app/store";
import {
  callReceived,
  setConfig,
  setBranding,
  setListenerCount,
  setConnectionStatus,
  transcriptReceived,
} from "@/features/scanner";
import { clearCredentials } from "@/features/auth";
import type { SystemConfig } from "@/types";
import type { WsListenerInbound, WsListenerOutbound } from "@/shared/types/ws";

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

  send(message: WsListenerOutbound): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
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
    const url = `${proto}//${window.location.host}/api/v1/ws/listener`;
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
    let parsed: unknown;
    try {
      parsed = JSON.parse(data);
    } catch {
      return;
    }

    if (
      !parsed ||
      typeof parsed !== "object" ||
      Array.isArray(parsed) ||
      typeof (parsed as { type?: unknown }).type !== "string"
    ) {
      return;
    }

    const msg = parsed as WsListenerInbound;

    switch (msg.type) {
      case "call.new": {
        const call = msg.call;
        if (!call || typeof call.id !== "number") break;

        // Dedup: skip if this call ID was already processed recently.
        if (this.recentCallIds.includes(call.id)) {
          break;
        }
        this.recentCallIds.push(call.id);
        if (this.recentCallIds.length > DEDUP_SIZE) {
          this.recentCallIds.shift();
        }

        this.dispatch?.(callReceived(call));
        break;
      }
      case "scanner.config": {
        if (!this.dispatch) break;
        const cfg = msg.config;
        // scanner.config carries systems + display prefs. Branding/email/
        // version arrive separately via connection.welcome, so only
        // override if scanner.config explicitly provides them.
        this.dispatch(
          setConfig({
            systems: (cfg.systems ?? []) as SystemConfig[],
            branding: cfg.branding,
            email: cfg.email,
            version: cfg.version,
            time12hFormat:
              cfg.time12hFormat === true || cfg.time12hFormat === "true",
            showListenersCount:
              cfg.showListenersCount === true ||
              cfg.showListenersCount === "true",
            playbackGoesLive:
              cfg.playbackGoesLive === true || cfg.playbackGoesLive === "true",
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
        break;
      }
      case "connection.welcome":
        if (this.dispatch) {
          this.dispatch(
            setBranding({
              branding: msg.branding ?? "",
              email: msg.email ?? "",
              version: msg.version ?? "",
            }),
          );
        }
        break;
      case "listener.count":
        this.dispatch?.(setListenerCount(msg.count));
        break;
      case "session.expired":
        // Token expired — attempt silent refresh before disconnecting.
        console.warn("[WsClient] Session expired, attempting refresh");
        if (this.tokenExpiredCallback) {
          this.tokenExpiredCallback().then((newToken) => {
            if (newToken) {
              this.auth = { ...this.auth, token: newToken };
              this.disconnect();
              this.intentionalClose = false;
              this.doConnect();
            } else {
              this.dispatch?.(clearCredentials());
              this.disconnect();
            }
          });
        } else {
          this.dispatch?.(clearCredentials());
          this.disconnect();
        }
        break;
      case "connection.rejected":
        console.warn("[WsClient] Connection rejected:", msg.reason);
        break;
      case "call.transcript":
        this.dispatch?.(
          transcriptReceived({
            callId: msg.callId,
            text: msg.text,
            segments: msg.segments,
          }),
        );
        break;
      case "listener.feedMap.snapshot":
        // Server-echoed feedmap snapshot. No reducer wired today; logged
        // for parity with the legacy LFM behaviour.
        break;
      default: {
        // Exhaustiveness check — TS will error here if a message type is
        // added to WsListenerInbound without a matching case.
        const _exhaustive: never = msg;
        void _exhaustive;
        break;
      }
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
