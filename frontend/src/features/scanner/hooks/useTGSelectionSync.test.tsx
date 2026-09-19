import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, act } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import { authSlice, setCredentials } from "@/features/auth";
import { api } from "@/app/api";
import { trMqttReducer } from "@/app/store";
import {
  scannerSlice,
  setBranding,
  setConfig,
  toggleTG,
} from "../scannerSlice";
import { callsSlice } from "../callsSlice";
import { useTGSelectionSync } from "./useTGSelectionSync";
import type { ScannerConfig } from "@/types";

// ── Mocks ────────────────────────────────────────────────────────────────

const mockSave = vi.fn();
const mockRefetch = vi.fn();
let selectionData: {
  disabledTGs: number[];
  avoidList?: { talkgroupId: number; expiresAt: number }[];
  version?: string;
} = { disabledTGs: [], avoidList: [], version: "v0" };

vi.mock("@/features/auth", async () => {
  const actual =
    await vi.importActual<typeof import("@/features/auth")>("@/features/auth");
  return {
    ...actual,
    useGetTGSelectionQuery: () => ({
      data: selectionData,
      refetch: mockRefetch,
    }),
    useUpdateTGSelectionMutation: () => [mockSave, { isLoading: false }],
  };
});

// ── Helpers ──────────────────────────────────────────────────────────────

const testConfig: ScannerConfig = {
  systems: [
    {
      id: 1,
      systemId: 100,
      label: "System Alpha",
      ledColor: "",
      talkgroups: [
        {
          id: 1,
          talkgroupId: 200,
          label: "One",
          name: "One",
          ledColor: "",
        },
        {
          id: 2,
          talkgroupId: 201,
          label: "Two",
          name: "Two",
          ledColor: "",
        },
        {
          id: 5,
          talkgroupId: 205,
          label: "Five",
          name: "Five",
          ledColor: "",
        },
      ],
    },
  ],
  branding: "TEST",
  email: "",
  version: "1.0",
  time12hFormat: false,
  showListenersCount: false,
  keypadBeeps: "",
  shareableLinks: false,
  transcriptionEnabled: false,
  liveTranscriptDisplay: false,
};

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

function Harness() {
  useTGSelectionSync();
  return null;
}

/**
 * Mounts the hook with an authenticated user. `deliverConfig: false` stops
 * before the scanner.config frame, leaving only the talkgroup-less
 * placeholder that connection.welcome creates.
 */
function mount(deliverConfig = true) {
  const store = makeStore();
  store.dispatch(
    setCredentials({
      token: "token",
      role: "admin",
      username: "alice",
      passwordNeedChange: false,
    }),
  );
  if (deliverConfig) {
    store.dispatch(setConfig(testConfig));
  } else {
    // connection.welcome lands first and fabricates config.systems = [].
    store.dispatch(
      setBranding({ branding: "TEST", email: "", version: "1.0" }),
    );
  }
  render(
    <MemoryRouter>
      <Provider store={store}>
        <Harness />
      </Provider>
    </MemoryRouter>,
  );
  return store;
}

/** Runs the 500 ms save debounce. */
async function flushDebounce() {
  await act(async () => {
    vi.advanceTimersByTime(600);
    await Promise.resolve();
  });
}

function conflict(currentVersion: string) {
  return {
    status: 409,
    data: { error: { code: "conflict", details: { currentVersion } } },
  };
}

// ── Tests ────────────────────────────────────────────────────────────────

describe("useTGSelectionSync", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockSave.mockReset();
    mockRefetch.mockReset();
    mockSave.mockReturnValue({ unwrap: () => Promise.resolve({ ok: true }) });
    selectionData = { disabledTGs: [], avoidList: [], version: "v0" };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does not save anything just because it restored", async () => {
    selectionData = {
      disabledTGs: [1],
      // An active timed avoid must not be written into disabledTGs — that
      // turned a temporary mute into a permanent disable.
      avoidList: [{ talkgroupId: 2, expiresAt: Date.now() + 60_000 }],
      version: "v0",
    };
    const store = mount();
    await flushDebounce();

    expect(mockSave).not.toHaveBeenCalled();
    expect(store.getState().scanner.tgSelection[2]).not.toBe(false);
  });

  it("does not save before the real config arrives", async () => {
    // Regression: connection.welcome fabricates a config with no talkgroups.
    // Restoring against it mapped every saved id onto nothing and then
    // persisted that back as "nothing disabled", wiping the selection.
    selectionData = { disabledTGs: [1, 5], avoidList: [], version: "v0" };
    const store = mount(false);
    await flushDebounce();

    expect(mockSave).not.toHaveBeenCalled();
    expect(store.getState().scanner.tgSelectionReady).toBe(false);

    // The real config lands: now the saved selection is applied, still no PUT.
    await act(async () => {
      store.dispatch(setConfig(testConfig));
    });
    await flushDebounce();

    const sel = store.getState().scanner.tgSelection;
    expect(sel[1]).toBe(false);
    expect(sel[5]).toBe(false);
    expect(sel[2]).toBe(true);
    expect(store.getState().scanner.tgSelectionReady).toBe(true);
    expect(mockSave).not.toHaveBeenCalled();
  });

  it("saves a change with the version it last read", async () => {
    const store = mount();
    await act(async () => {
      store.dispatch(toggleTG(5));
    });
    await flushDebounce();

    expect(mockSave).toHaveBeenCalledWith({
      disabledTGs: [5],
      avoidList: [],
      version: "v0",
    });
  });

  it("adopts the server selection when a stale write is rejected", async () => {
    const store = mount();
    mockSave.mockReturnValue({
      unwrap: () => Promise.reject(conflict("v9")),
    });
    mockRefetch.mockReturnValue({
      unwrap: () =>
        Promise.resolve({ disabledTGs: [1, 2], avoidList: [], version: "v9" }),
    });

    await act(async () => {
      store.dispatch(toggleTG(5));
    });
    await flushDebounce();

    // The other session's selection wins; this tab's stale write is dropped.
    const sel = store.getState().scanner.tgSelection;
    expect(sel[1]).toBe(false);
    expect(sel[2]).toBe(false);
    expect(sel[5]).toBe(true);
  });

  it("keeps the user's in-flight edits when a write is rejected", async () => {
    const store = mount();
    let rejectSave: ((err: unknown) => void) | undefined;
    mockSave.mockReturnValueOnce({
      unwrap: () =>
        new Promise((_resolve, reject) => {
          rejectSave = reject;
        }),
    });

    await act(async () => {
      store.dispatch(toggleTG(5));
    });
    await flushDebounce();
    expect(mockSave).toHaveBeenCalledTimes(1);

    // User keeps clicking while the first PUT is still in flight, then it
    // comes back 409: their newer selection must be re-sent, not discarded.
    mockSave.mockReturnValue({ unwrap: () => Promise.resolve({ ok: true }) });
    await act(async () => {
      store.dispatch(toggleTG(1));
    });
    await act(async () => {
      rejectSave?.(conflict("v9"));
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(mockRefetch).not.toHaveBeenCalled();
    expect(mockSave).toHaveBeenLastCalledWith({
      disabledTGs: [1, 5],
      avoidList: [],
      version: "v9",
    });
    const sel = store.getState().scanner.tgSelection;
    expect(sel[1]).toBe(false);
    expect(sel[5]).toBe(false);
  });
});
