import type { AppDispatch } from "@/app/store";
import { clearCredentials } from "@/app/slices/authSlice";

type TokenExpiredCallback = () => Promise<string | null>;
type EventCallback = (topic: string, data: unknown, at: number) => void;

interface PendingRequest {
  resolve: (data: unknown) => void;
  reject: (error: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

const REQUEST_TIMEOUT = 30_000;
const MAX_BACKOFF = 30_000;

class AdminWsClient {
  private ws: WebSocket | null = null;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private backoff = 1000;
  private dispatch: AppDispatch | null = null;
  private token: string | null = null;
  private intentionalClose = false;
  private pendingRequests = new Map<string, PendingRequest>();
  private eventListeners = new Map<string, Set<EventCallback>>();
  private tokenExpiredCallback: TokenExpiredCallback | null = null;
  private connected = false;

  connect(dispatch: AppDispatch, token: string): void {
    this.dispatch = dispatch;
    this.token = token;
    this.intentionalClose = false;
    this.doConnect();
  }

  disconnect(): void {
    this.intentionalClose = true;
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }
    // Reject all pending requests
    for (const [reqId, pending] of this.pendingRequests) {
      clearTimeout(pending.timer);
      pending.reject(new Error("WebSocket disconnected"));
      this.pendingRequests.delete(reqId);
    }
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.close();
      this.ws = null;
    }
    this.connected = false;
  }

  request<T = unknown>(
    op: string,
    params?: Record<string, unknown>,
    timeoutMs?: number,
  ): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new Error("Admin WebSocket is not connected"));
        return;
      }

      const reqId = crypto.randomUUID();
      const timer = setTimeout(() => {
        this.pendingRequests.delete(reqId);
        reject(new Error(`Admin WS request timed out: ${op}`));
      }, timeoutMs ?? REQUEST_TIMEOUT);

      this.pendingRequests.set(reqId, {
        resolve: resolve as (data: unknown) => void,
        reject,
        timer,
      });

      const msg: [
        string,
        { reqId: string; op: string; params?: Record<string, unknown> },
      ] = [
        "ADM_REQ",
        { reqId, op, ...(params !== undefined ? { params } : {}) },
      ];
      this.ws.send(JSON.stringify(msg));
    });
  }

  on(topic: string, callback: EventCallback): () => void {
    let listeners = this.eventListeners.get(topic);
    if (!listeners) {
      listeners = new Set();
      this.eventListeners.set(topic, listeners);
    }
    listeners.add(callback);
    return () => {
      listeners?.delete(callback);
      if (listeners?.size === 0) {
        this.eventListeners.delete(topic);
      }
    };
  }

  onAny(callback: EventCallback): () => void {
    return this.on("*", callback);
  }

  onTokenExpired(cb: TokenExpiredCallback): void {
    this.tokenExpiredCallback = cb;
  }

  isConnected(): boolean {
    return this.connected;
  }

  private doConnect(): void {
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.close();
      this.ws = null;
    }

    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${proto}//${location.host}/api/admin/ws`;

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      // Send JWT as first message for auth (token never appears in URL).
      if (this.token && this.ws) {
        this.ws.send(JSON.stringify([this.token]));
      }
      this.backoff = 1000;
      this.connected = true;
      // Emit connection event so hooks can re-fetch
      const connListeners = this.eventListeners.get("__connected__");
      if (connListeners) {
        for (const cb of connListeners) {
          cb("__connected__", null, Date.now());
        }
      }
    };

    this.ws.onmessage = (ev: MessageEvent) => {
      this.handleMessage(ev.data as string);
    };

    this.ws.onclose = () => {
      this.connected = false;
      if (!this.intentionalClose) {
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = () => {
      // onclose will fire after onerror, reconnect is handled there
    };
  }

  private handleMessage(raw: string): void {
    let parsed: unknown;
    try {
      parsed = JSON.parse(raw);
    } catch {
      console.warn("Admin WS: failed to parse message");
      return;
    }

    if (!Array.isArray(parsed) || parsed.length === 0) {
      return;
    }

    const command = parsed[0] as string;

    if (command === "ADM_RES") {
      this.handleResponse(parsed[1] as Record<string, unknown>);
    } else if (command === "ADM_EVT") {
      this.handleEvent(parsed[1] as Record<string, unknown>);
    } else if (command === "XPR") {
      this.handleTokenExpired();
    }
  }

  private handleResponse(payload: Record<string, unknown>): void {
    const reqId = payload.reqId as string;
    const pending = this.pendingRequests.get(reqId);
    if (!pending) return;

    clearTimeout(pending.timer);
    this.pendingRequests.delete(reqId);

    if (payload.ok) {
      pending.resolve(payload.data);
    } else {
      pending.reject(
        new Error((payload.error as string) ?? "Unknown admin WS error"),
      );
    }
  }

  private handleEvent(payload: Record<string, unknown>): void {
    const topic = payload.topic as string;
    const data = payload.data;
    const at = (payload.at as number) ?? 0;

    // Notify topic-specific listeners
    const topicListeners = this.eventListeners.get(topic);
    if (topicListeners) {
      for (const cb of topicListeners) {
        cb(topic, data, at);
      }
    }

    // Notify wildcard listeners
    const wildcardListeners = this.eventListeners.get("*");
    if (wildcardListeners) {
      for (const cb of wildcardListeners) {
        cb(topic, data, at);
      }
    }
  }

  private handleTokenExpired(): void {
    if (!this.tokenExpiredCallback) {
      this.disconnect();
      this.dispatch?.(clearCredentials());
      return;
    }

    this.tokenExpiredCallback()
      .then((newToken) => {
        if (newToken) {
          this.token = newToken;
          this.doConnect();
        } else {
          this.disconnect();
          this.dispatch?.(clearCredentials());
        }
      })
      .catch(() => {
        this.disconnect();
        this.dispatch?.(clearCredentials());
      });
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
    }
    // Exponential backoff with jitter
    const jitter = Math.random() * this.backoff * 0.3;
    const delay = Math.min(this.backoff + jitter, MAX_BACKOFF);
    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null;
      this.doConnect();
    }, delay);
    this.backoff = Math.min(this.backoff * 2, MAX_BACKOFF);
  }
}

export const adminWsClient = new AdminWsClient();
