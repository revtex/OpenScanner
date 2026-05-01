import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, act } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { scannerSlice } from "@/features/scanner";
import { authSlice, setCredentials } from "./authSlice";
import { callsSlice } from "@/features/scanner";
import { api } from "@/app/api";
import { useTokenRefresh } from "./useTokenRefresh";
import { trMqttReducer } from "@/app/store";

// ── Mocks ────────────────────────────────────────────────────────────────

const mockPostRefresh = vi.fn();
vi.mock("./authSlice", async () => {
  const actual = await vi.importActual<typeof import("./authSlice")>(
    "./authSlice",
  );
  return {
    ...actual,
    usePostRefreshMutation: () => [mockPostRefresh, { isLoading: false }],
  };
});

// ── Helpers ──────────────────────────────────────────────────────────────

/** Build a fake JWT whose `exp` claim is N ms in the future. */
function fakeJwt(expiresInMs: number): string {
  const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const exp = Math.floor((Date.now() + expiresInMs) / 1000);
  const payload = btoa(JSON.stringify({ exp }));
  return `${header}.${payload}.sig`;
}

function makeStore() {
  return configureStore({
    reducer: {
      scanner: scannerSlice.reducer,
      trMqtt: trMqttReducer,
      auth: authSlice.reducer,
      calls: callsSlice.reducer,
      [api.reducerPath]: api.reducer,
    },
    middleware: (gDM) => gDM().concat(api.middleware),
  });
}

function TestHarness() {
  useTokenRefresh();
  return null;
}

function renderHook(store: ReturnType<typeof makeStore>) {
  return render(
    <Provider store={store}>
      <TestHarness />
    </Provider>,
  );
}

// ── Tests ────────────────────────────────────────────────────────────────

describe("useTokenRefresh", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockPostRefresh.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does not schedule a refresh when unauthenticated (no token)", () => {
    const store = makeStore();
    renderHook(store);

    vi.advanceTimersByTime(10 * 60_000);
    expect(mockPostRefresh).not.toHaveBeenCalled();
  });

  it("schedules a refresh 60s before token expiry", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: fakeJwt(5 * 60_000),
          user: { id: 1, username: "alice", role: "admin" },
        }),
    });

    const store = makeStore();
    // 2-minute token → refresh should fire at 60s.
    store.dispatch(
      setCredentials({
        token: fakeJwt(2 * 60_000),
        role: "admin",
        username: "alice",
        passwordNeedChange: false,
      }),
    );
    renderHook(store);

    // Before the scheduled time → no call.
    await act(async () => {
      vi.advanceTimersByTime(59_000);
    });
    expect(mockPostRefresh).not.toHaveBeenCalled();

    // At the scheduled time → call fires.
    await act(async () => {
      vi.advanceTimersByTime(2_000);
      await Promise.resolve();
    });
    expect(mockPostRefresh).toHaveBeenCalledTimes(1);
  });

  it("dispatches setCredentials on successful refresh", async () => {
    const newToken = fakeJwt(5 * 60_000);
    mockPostRefresh.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: newToken,
          user: { id: 1, username: "alice", role: "admin" },
        }),
    });

    const store = makeStore();
    store.dispatch(
      setCredentials({
        token: fakeJwt(90_000),
        role: "admin",
        username: "alice",
        passwordNeedChange: false,
      }),
    );
    renderHook(store);

    await act(async () => {
      vi.advanceTimersByTime(31_000);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(store.getState().auth.token).toBe(newToken);
  });

  it("dispatches clearCredentials on refresh failure", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () => Promise.reject(new Error("expired")),
    });

    const store = makeStore();
    store.dispatch(
      setCredentials({
        token: fakeJwt(90_000),
        role: "admin",
        username: "alice",
        passwordNeedChange: false,
      }),
    );
    renderHook(store);

    await act(async () => {
      vi.advanceTimersByTime(31_000);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(store.getState().auth.token).toBeNull();
    expect(store.getState().auth.role).toBeNull();
  });

  it("cancels the pending timer on unmount", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: fakeJwt(5 * 60_000),
          user: { id: 1, username: "alice", role: "admin" },
        }),
    });

    const store = makeStore();
    store.dispatch(
      setCredentials({
        token: fakeJwt(2 * 60_000),
        role: "admin",
        username: "alice",
        passwordNeedChange: false,
      }),
    );
    const { unmount } = renderHook(store);

    unmount();

    await act(async () => {
      vi.advanceTimersByTime(10 * 60_000);
    });

    expect(mockPostRefresh).not.toHaveBeenCalled();
  });

  it("fires immediately when the token is already within the 60s refresh window", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: fakeJwt(5 * 60_000),
          user: { id: 1, username: "alice", role: "admin" },
        }),
    });

    const store = makeStore();
    // 30s token → already past the 60s-before-expiry point → fires "now".
    store.dispatch(
      setCredentials({
        token: fakeJwt(30_000),
        role: "admin",
        username: "alice",
        passwordNeedChange: false,
      }),
    );
    renderHook(store);

    await act(async () => {
      vi.advanceTimersByTime(1);
      await Promise.resolve();
    });

    expect(mockPostRefresh).toHaveBeenCalledTimes(1);
  });
});
