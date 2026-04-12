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
import { audioPlayer } from "@/services/audioPlayer";

const MAX_BACKOFF = 30_000;

type AudioReceivedCallback = (call: Call, audioUrl: string) => void;

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
  private pendingAudioForCall: Call | null = null;
  private audioCallback: AudioReceivedCallback | null = null;
  private intentionalClose = false;

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
      this.ws.close();
      this.ws = null;
    }
    this.pendingAudioForCall = null;
    audioPlayer.clearQueue();
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

  private doConnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }

    this.dispatch?.(setConnectionStatus("connecting"));

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${proto}//${window.location.host}/ws`;
    this.ws = new WebSocket(url);
    this.ws.binaryType = "arraybuffer";

    this.ws.onopen = () => {
      this.backoff = 1000;
      this.dispatch?.(setConnectionStatus("connected"));

      // Authenticate based on mode
      if (this.auth.token) {
        // Send JWT as first message (raw string in array)
        if (this.ws?.readyState === WebSocket.OPEN) {
          this.ws.send(JSON.stringify([this.auth.token]));
        }
      }
      // publicAccess: no auth message needed
    };

    this.ws.onmessage = (event: MessageEvent) => {
      if (event.data instanceof ArrayBuffer) {
        this.handleBinaryMessage(event.data);
      } else if (typeof event.data === "string") {
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
          const call = payload as Call;
          this.pendingAudioForCall = call;
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
          };
          // CFG may be partial — merge with existing config concept
          this.dispatch(
            setConfig({
              systems: (cfg.systems ?? []) as import("@/types").SystemConfig[],
              branding: cfg.branding ?? "",
              email: cfg.email ?? "",
              version: cfg.version ?? "",
              time12hFormat:
                cfg.time12hFormat === true || cfg.time12hFormat === "true",
              showListenersCount:
                cfg.showListenersCount === true ||
                cfg.showListenersCount === "true",
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
        // Token expired on listener WS — clear credentials so the UI
        // redirects to the login page.
        console.warn("[WsClient] Token expired");
        this.dispatch?.(clearCredentials());
        this.disconnect();
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

  private handleBinaryMessage(data: ArrayBuffer): void {
    if (!this.pendingAudioForCall) return;

    const call = this.pendingAudioForCall;
    this.pendingAudioForCall = null;

    const mimeType = call.audioType || "audio/mpeg";
    const blob = new Blob([data], { type: mimeType });
    const audioUrl = URL.createObjectURL(blob);

    this.audioCallback?.(call, audioUrl);
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
