import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, waitFor, act } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import { scannerSlice } from "@/app/slices/scanner/scannerSlice";
import { authSlice } from "@/app/slices/shared/authSlice";
import { callsSlice } from "@/app/slices/scanner/callsSlice";
import { api } from "@/app/api";
import type { RootState } from "@/app/store";
import { useAuthInit } from "@/hooks/shared/useAuthInit";

// ── Mock the refresh mutation ─────────────────────────────────────────────

const mockPostRefresh = vi.fn();
vi.mock("@/app/slices/shared/authSlice", async () => {
  const actual = await vi.importActual<typeof import("@/app/slices/shared/authSlice")>(
    "@/app/slices/shared/authSlice",
  );
  return {
    ...actual,
    usePostRefreshMutation: () => [mockPostRefresh, { isLoading: false }],
  };
});

// ── Harness ───────────────────────────────────────────────────────────────

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

function TestHarness() {
  useAuthInit();
  return null;
}

function renderHook(store: ReturnType<typeof makeStore>) {
  return render(
    <Provider store={store}>
      <MemoryRouter>
        <TestHarness />
      </MemoryRouter>
    </Provider>,
  );
}

describe("useAuthInit", () => {
  beforeEach(() => {
    mockPostRefresh.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("dispatches setCredentials and setAuthReady on successful refresh", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "jwt-new",
          user: { id: 1, username: "alice", role: "admin" },
        }),
    });

    const store = makeStore();
    renderHook(store);

    await waitFor(() => {
      expect(store.getState().auth.token).toBe("jwt-new");
    });
    const state = store.getState() as RootState;
    expect(state.auth.role).toBe("admin");
    expect(state.auth.username).toBe("alice");
    expect(state.auth.authReady).toBe(true);
  });

  it("dispatches setAuthReady but not setCredentials on refresh failure", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () => Promise.reject(new Error("no refresh cookie")),
    });

    const store = makeStore();
    renderHook(store);

    await waitFor(() => {
      expect(store.getState().auth.authReady).toBe(true);
    });
    expect(store.getState().auth.token).toBeNull();
  });

  it("only calls postRefresh once on mount", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () => Promise.reject(new Error("no refresh cookie")),
    });

    const store = makeStore();
    const { rerender } = renderHook(store);

    // Rerender same tree — should not fire again.
    rerender(
      <Provider store={store}>
        <MemoryRouter>
          <TestHarness />
        </MemoryRouter>
      </Provider>,
    );

    await waitFor(() => {
      expect(store.getState().auth.authReady).toBe(true);
    });
    expect(mockPostRefresh).toHaveBeenCalledTimes(1);
  });

  it("sets authReady=true after either success or failure", async () => {
    mockPostRefresh.mockReturnValue({
      unwrap: () => Promise.reject(new Error("x")),
    });

    const store = makeStore();
    renderHook(store);

    await act(async () => {
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(store.getState().auth.authReady).toBe(true);
    });
  });
});
