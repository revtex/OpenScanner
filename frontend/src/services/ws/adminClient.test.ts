import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { configureStore } from "@reduxjs/toolkit";
import { adminWsClient } from "@/services/ws/adminClient";
import { authSlice } from "@/app/slices/shared/authSlice";

// ── Fake WebSocket stub ───────────────────────────────────────────────────

interface FakeSocket {
  url: string;
  readyState: number;
  onopen: ((ev: Event) => void) | null;
  onmessage: ((ev: MessageEvent) => void) | null;
  onclose: ((ev: CloseEvent) => void) | null;
  onerror: ((ev: Event) => void) | null;
  send: ReturnType<typeof vi.fn>;
  close: ReturnType<typeof vi.fn>;
  simulateOpen: () => void;
  simulateMessage: (data: string) => void;
}

const OPEN = 1;
const CLOSED = 3;

let constructed: FakeSocket[] = [];

function installFakeWebSocket(): void {
  class FakeWS implements FakeSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;
    readyState = 0;
    onopen: ((ev: Event) => void) | null = null;
    onmessage: ((ev: MessageEvent) => void) | null = null;
    onclose: ((ev: CloseEvent) => void) | null = null;
    onerror: ((ev: Event) => void) | null = null;
    send = vi.fn();
    close = vi.fn(() => {
      this.readyState = CLOSED;
    });
    constructor(public url: string) {
      constructed.push(this);
    }
    simulateOpen(): void {
      this.readyState = OPEN;
      this.onopen?.(new Event("open"));
    }
    simulateMessage(data: string): void {
      this.onmessage?.({ data } as MessageEvent);
    }
  }
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = FakeWS;
  (globalThis as unknown as { WebSocket: { OPEN: number } }).WebSocket.OPEN =
    OPEN;
}

function makeStore() {
  return configureStore({
    reducer: {
      auth: authSlice.reducer,
    },
  });
}

describe("adminWsClient", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    constructed = [];
    installFakeWebSocket();
    vi.spyOn(Math, "random").mockReturnValue(0);
    // crypto.randomUUID — deterministic for assertion stability.
    let n = 0;
    vi.spyOn(crypto, "randomUUID").mockImplementation(
      () => `req-${++n}` as `${string}-${string}-${string}-${string}-${string}`,
    );
  });

  afterEach(() => {
    adminWsClient.disconnect();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("connects to /api/v1/ws/admin", () => {
    const store = makeStore();
    adminWsClient.connect(store.dispatch, "admin-jwt");
    expect(constructed).toHaveLength(1);
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    expect(constructed[0].url).toBe(
      `${proto}//${window.location.host}/api/v1/ws/admin`,
    );
  });

  it("sends an admin.request JSON object frame on request()", async () => {
    const store = makeStore();
    adminWsClient.connect(store.dispatch, "admin-jwt");
    constructed[0].simulateOpen();
    // Drop the auth-token frame — only assert the next frame.
    constructed[0].send.mockClear();

    void adminWsClient.request("groups.list", { foo: "bar" }).catch(() => {
      /* disconnected during teardown — expected */
    });

    expect(constructed[0].send).toHaveBeenCalledWith(
      JSON.stringify({
        type: "admin.request",
        reqId: "req-1",
        op: "groups.list",
        params: { foo: "bar" },
      }),
    );
  });

  it("resolves the request promise on a matching admin.response with ok=true", async () => {
    const store = makeStore();
    adminWsClient.connect(store.dispatch, "admin-jwt");
    constructed[0].simulateOpen();

    const p = adminWsClient.request<{ count: number }>("activity.stats");
    constructed[0].simulateMessage(
      JSON.stringify({
        type: "admin.response",
        reqId: "req-1",
        ok: true,
        data: { count: 7 },
      }),
    );

    await expect(p).resolves.toEqual({ count: 7 });
  });

  it("rejects the request promise with code+message on ok=false", async () => {
    const store = makeStore();
    adminWsClient.connect(store.dispatch, "admin-jwt");
    constructed[0].simulateOpen();

    const p = adminWsClient.request("groups.update", { id: 99 });
    constructed[0].simulateMessage(
      JSON.stringify({
        type: "admin.response",
        reqId: "req-1",
        ok: false,
        error: {
          code: "validation_failed",
          message: "id must be positive",
          details: { field: "id" },
        },
      }),
    );

    await expect(p).rejects.toMatchObject({
      message: "id must be positive",
      code: "validation_failed",
      details: { field: "id" },
    });
  });

  it("dispatches admin.event payloads to topic listeners", () => {
    const store = makeStore();
    adminWsClient.connect(store.dispatch, "admin-jwt");
    constructed[0].simulateOpen();

    const cb = vi.fn();
    const off = adminWsClient.on("config.updated", cb);

    constructed[0].simulateMessage(
      JSON.stringify({
        type: "admin.event",
        topic: "config.updated",
        at: 1700000000,
        data: { key: "branding" },
      }),
    );

    expect(cb).toHaveBeenCalledWith(
      "config.updated",
      { key: "branding" },
      1700000000,
    );
    off();
  });
});
