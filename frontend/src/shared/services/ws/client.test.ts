import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { wsClient } from "@/shared/services/ws/client";
import { configureStore } from "@reduxjs/toolkit";
import { scannerSlice } from "@/features/scanner";
import { authSlice } from "@/features/auth";
import { callsSlice } from "@/features/scanner";
import { api } from "@/app/api";

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
  simulateClose: () => void;
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
    simulateClose(): void {
      this.readyState = CLOSED;
      this.onclose?.(new Event("close") as CloseEvent);
    }
  }
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = FakeWS;
  (globalThis as unknown as { WebSocket: { OPEN: number } }).WebSocket.OPEN =
    OPEN;
}

function makeStore() {
  return configureStore({
    reducer: {
      scanner: scannerSlice.reducer,
      auth: authSlice.reducer,
      calls: callsSlice.reducer,
      [api.reducerPath]: api.reducer,
    },
    middleware: (gDM) => gDM().concat(api.middleware),
  });
}

/**
 * Reset the wsClient's internal backoff state (which leaks across tests
 * because wsClient is a module-level singleton). A connect → open →
 * disconnect cycle forces onopen to reset backoff to 1000ms.
 */
function resetWsClientBackoff(): void {
  const tmpStore = makeStore();
  wsClient.connect(tmpStore.dispatch);
  const idx = constructed.length - 1;
  constructed[idx].simulateOpen();
  wsClient.disconnect();
  constructed = [];
}

describe("wsClient", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    constructed = [];
    installFakeWebSocket();
    // Deterministic jitter
    vi.spyOn(Math, "random").mockReturnValue(0);
    resetWsClientBackoff();
  });

  afterEach(() => {
    wsClient.disconnect();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("connects on first call to /api/v1/ws/listener derived from window.location", () => {
    const store = makeStore();
    wsClient.connect(store.dispatch);
    expect(constructed).toHaveLength(1);
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    expect(constructed[0].url).toBe(
      `${proto}//${window.location.host}/api/v1/ws/listener`,
    );
  });

  it("sends the auth token as a JSON array after onopen", () => {
    const store = makeStore();
    wsClient.connect(store.dispatch, { token: "jwt-test-token" });
    constructed[0].simulateOpen();
    expect(constructed[0].send).toHaveBeenCalledWith(
      JSON.stringify(["jwt-test-token"]),
    );
  });

  it("schedules a reconnect after unexpected close with exponential backoff", () => {
    const store = makeStore();
    wsClient.connect(store.dispatch);

    // First attempt. Unexpected close → next reconnect in ~1000ms.
    constructed[0].simulateClose();
    expect(constructed).toHaveLength(1);
    vi.advanceTimersByTime(999);
    expect(constructed).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(constructed).toHaveLength(2);

    // Second close without an open (so backoff does not reset) → ~2000ms.
    constructed[1].simulateClose();
    vi.advanceTimersByTime(1999);
    expect(constructed).toHaveLength(2);
    vi.advanceTimersByTime(1);
    expect(constructed).toHaveLength(3);

    // Third close → ~4000ms.
    constructed[2].simulateClose();
    vi.advanceTimersByTime(3999);
    expect(constructed).toHaveLength(3);
    vi.advanceTimersByTime(1);
    expect(constructed).toHaveLength(4);
  });

  it("does not reconnect when disconnect() was called intentionally", () => {
    const store = makeStore();
    wsClient.connect(store.dispatch);
    wsClient.disconnect();
    // Advance well beyond any possible backoff window.
    vi.advanceTimersByTime(60_000);
    expect(constructed).toHaveLength(1);
  });

  it("resets backoff after a successful open", () => {
    const store = makeStore();
    wsClient.connect(store.dispatch);

    // Cycle through a couple of failures to push backoff up to 4s.
    constructed[0].simulateClose();
    vi.advanceTimersByTime(1000);
    constructed[1].simulateClose();
    vi.advanceTimersByTime(2000);
    expect(constructed).toHaveLength(3);

    // Successful open → backoff reset to 1000.
    constructed[2].simulateOpen();
    constructed[2].simulateClose();
    vi.advanceTimersByTime(999);
    expect(constructed).toHaveLength(3);
    vi.advanceTimersByTime(1);
    expect(constructed).toHaveLength(4);
  });

  it("deduplicates call.new messages by call id", () => {
    const store = makeStore();
    const dispatchSpy = vi.spyOn(store, "dispatch");
    wsClient.connect(store.dispatch);
    constructed[0].simulateOpen();
    dispatchSpy.mockClear();

    const call = {
      id: 42,
      dateTime: "2026-01-01T00:00:00Z",
      systemId: 1,
      talkgroupId: 100,
    };
    const msg = JSON.stringify({ type: "call.new", call });

    constructed[0].simulateMessage(msg);
    constructed[0].simulateMessage(msg);
    constructed[0].simulateMessage(msg);

    const callReceivedDispatches = dispatchSpy.mock.calls.filter(
      (c) =>
        typeof c[0] === "object" &&
        c[0] !== null &&
        "type" in (c[0] as object) &&
        (c[0] as { type: string }).type === "scanner/callReceived",
    );
    expect(callReceivedDispatches).toHaveLength(1);
  });

  it("dispatches again for a distinct call id (not deduped)", () => {
    const store = makeStore();
    const dispatchSpy = vi.spyOn(store, "dispatch");
    wsClient.connect(store.dispatch);
    constructed[0].simulateOpen();
    dispatchSpy.mockClear();

    constructed[0].simulateMessage(
      JSON.stringify({
        type: "call.new",
        call: {
          id: 1,
          dateTime: "2026-01-01T00:00:00Z",
          systemId: 1,
          talkgroupId: 100,
        },
      }),
    );
    constructed[0].simulateMessage(
      JSON.stringify({
        type: "call.new",
        call: {
          id: 2,
          dateTime: "2026-01-01T00:00:01Z",
          systemId: 1,
          talkgroupId: 100,
        },
      }),
    );

    const callReceivedDispatches = dispatchSpy.mock.calls.filter(
      (c) =>
        typeof c[0] === "object" &&
        c[0] !== null &&
        "type" in (c[0] as object) &&
        (c[0] as { type: string }).type === "scanner/callReceived",
    );
    expect(callReceivedDispatches).toHaveLength(2);
  });

  it("ignores non-JSON text frames without throwing", () => {
    const store = makeStore();
    wsClient.connect(store.dispatch);
    constructed[0].simulateOpen();
    expect(() => constructed[0].simulateMessage("not-json")).not.toThrow();
    expect(() => constructed[0].simulateMessage("[")).not.toThrow();
  });

  it("evicts the oldest dedup id once the window fills", () => {
    const store = makeStore();
    const dispatchSpy = vi.spyOn(store, "dispatch");
    wsClient.connect(store.dispatch);
    constructed[0].simulateOpen();
    dispatchSpy.mockClear();

    // DEDUP_SIZE is 100 — send id 1, then 100 distinct fresh ids to push
    // it out, then id 1 again — it should be accepted as new.
    const send = (id: number) =>
      constructed[0].simulateMessage(
        JSON.stringify({
          type: "call.new",
          call: {
            id,
            dateTime: "2026-01-01T00:00:00Z",
            systemId: 1,
            talkgroupId: 100,
          },
        }),
      );

    send(1);
    for (let i = 1000; i < 1100; i++) send(i);
    send(1);

    const callReceivedCount = dispatchSpy.mock.calls.filter(
      (c) =>
        typeof c[0] === "object" &&
        c[0] !== null &&
        "type" in (c[0] as object) &&
        (c[0] as { type: string }).type === "scanner/callReceived",
    ).length;

    // 1 (initial) + 100 (fills) + 1 (re-accept after eviction) = 102
    expect(callReceivedCount).toBe(102);
  });

  it("dispatches setBranding when connection.welcome arrives", () => {
    const store = makeStore();
    const dispatchSpy = vi.spyOn(store, "dispatch");
    wsClient.connect(store.dispatch);
    constructed[0].simulateOpen();
    dispatchSpy.mockClear();

    constructed[0].simulateMessage(
      JSON.stringify({
        type: "connection.welcome",
        version: "1.2.3",
        branding: "OpenScanner",
        email: "ops@example.com",
      }),
    );

    expect(dispatchSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "scanner/setBranding",
        payload: {
          version: "1.2.3",
          branding: "OpenScanner",
          email: "ops@example.com",
        },
      }),
    );
  });

  it("serialises outbound listener.feedMap.update as a JSON object frame", () => {
    const store = makeStore();
    wsClient.connect(store.dispatch);
    constructed[0].simulateOpen();
    constructed[0].send.mockClear();

    wsClient.send({
      type: "listener.feedMap.update",
      feedMap: { "1": { "100": true } },
    });

    expect(constructed[0].send).toHaveBeenCalledWith(
      JSON.stringify({
        type: "listener.feedMap.update",
        feedMap: { "1": { "100": true } },
      }),
    );
  });
});
