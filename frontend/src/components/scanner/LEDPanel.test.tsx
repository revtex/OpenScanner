import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import { LEDPanel } from "@/components/scanner/LEDPanel";
import { scannerSlice } from "@/app/slices/scanner/scannerSlice";
import { authSlice } from "@/app/slices/shared/authSlice";
import { callsSlice } from "@/app/slices/scanner/callsSlice";
import { api } from "@/app/api";
import type { RootState } from "@/app/store";
import type { Call, ScannerConfig } from "@/types";

// Mock useTheme since it reads localStorage / sets DOM attributes
const mockToggle = vi.fn();
let mockIsDark = true;

vi.mock("@/hooks/shared/useTheme", () => ({
  useTheme: () => ({
    isDark: mockIsDark,
    toggle: mockToggle,
    theme: mockIsDark ? "openscanner-dark" : "openscanner-light",
  }),
}));

function makeStore(preloadedState?: Partial<RootState>) {
  return configureStore({
    reducer: {
      scanner: scannerSlice.reducer,
      auth: authSlice.reducer,
      calls: callsSlice.reducer,
      [api.reducerPath]: api.reducer,
    },
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware().concat(api.middleware),
    preloadedState: preloadedState as RootState,
  });
}

function renderLED(preloadedState?: Partial<RootState>) {
  const store = makeStore(preloadedState);
  return {
    ...render(
      <MemoryRouter>
        <Provider store={store}>
          <LEDPanel />
        </Provider>
      </MemoryRouter>,
    ),
    store,
  };
}

function makeCall(overrides: Partial<Call> = {}): Call {
  return {
    id: 1,
    audioName: "test.wav",
    audioType: "audio/wav",
    dateTime: Date.now(),
    systemId: 100,
    system: 1,
    talkgroupId: 200,
    talkgroup: 2,
    ...overrides,
  };
}

describe("LEDPanel", () => {
  beforeEach(() => {
    mockToggle.mockClear();
    mockIsDark = true;
  });

  it('renders default branding text "OPENSCANNER"', () => {
    renderLED();
    expect(screen.getByText("OPENSCANNER")).toBeInTheDocument();
  });

  it("renders custom branding from config", () => {
    const config: ScannerConfig = {
      systems: [],
      branding: "MY SCANNER",
      email: "",
      version: "1.0",
      time12hFormat: false,
      showListenersCount: false,
      playbackGoesLive: false,
      keypadBeeps: "uniden",
      shareableLinks: false,
      transcriptionEnabled: false,
      liveTranscriptDisplay: false,
    };
    renderLED({
      scanner: {
        isLive: true,
        isPaused: false,
        isAudioActive: false,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: null,
        history: [],
        listenerCount: 0,
        connectionStatus: "disconnected",
        config,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    expect(screen.getByText("MY SCANNER")).toBeInTheDocument();
  });

  it('falls back to "OPENSCANNER" when branding is blank', () => {
    const config: ScannerConfig = {
      systems: [],
      branding: "   ",
      email: "",
      version: "1.0",
      time12hFormat: false,
      showListenersCount: false,
      playbackGoesLive: false,
      keypadBeeps: "uniden",
      shareableLinks: false,
      transcriptionEnabled: false,
      liveTranscriptDisplay: false,
    };
    renderLED({
      scanner: {
        isLive: true,
        isPaused: false,
        isAudioActive: false,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: null,
        history: [],
        listenerCount: 0,
        connectionStatus: "disconnected",
        config,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    expect(screen.getByText("OPENSCANNER")).toBeInTheDocument();
  });

  it("shows theme toggle button", () => {
    renderLED();
    const btn = screen.getByRole("button", { name: /toggle theme/i });
    expect(btn).toBeInTheDocument();
  });

  it("calls toggle when theme button clicked", () => {
    renderLED();
    fireEvent.click(screen.getByRole("button", { name: /toggle theme/i }));
    expect(mockToggle).toHaveBeenCalledOnce();
  });

  it("shows green LED when live and playing", () => {
    renderLED({
      scanner: {
        isLive: true,
        isPaused: false,
        isAudioActive: false,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: makeCall(),
        history: [],
        listenerCount: 0,
        connectionStatus: "connected",
        config: null,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    const led = document.querySelector(
      '[style*="background-color"]',
    ) as HTMLElement;
    expect(led).toBeTruthy();
    expect(led.style.backgroundColor).toBe("rgb(0, 230, 118)"); // #00e676
  });

  it("shows orange LED when paused", () => {
    renderLED({
      scanner: {
        isLive: true,
        isPaused: true,
        isAudioActive: false,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: null,
        history: [],
        listenerCount: 0,
        connectionStatus: "connected",
        config: null,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    const led = document.querySelector(
      '[style*="background-color"]',
    ) as HTMLElement;
    expect(led).toBeTruthy();
    expect(led.style.backgroundColor).toBe("rgb(255, 145, 0)"); // #ff9100 orange
  });

  it("shows blink animation when paused", () => {
    renderLED({
      scanner: {
        isLive: true,
        isPaused: true,
        isAudioActive: false,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: null,
        history: [],
        listenerCount: 0,
        connectionStatus: "connected",
        config: null,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    const led = document.querySelector(".animate-pulse") as HTMLElement;
    expect(led).toBeTruthy();
  });

  it("does not blink when not paused", () => {
    renderLED({
      scanner: {
        isLive: true,
        isPaused: false,
        isAudioActive: false,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: makeCall(),
        history: [],
        listenerCount: 0,
        connectionStatus: "connected",
        config: null,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    const led = document.querySelector(".animate-pulse");
    expect(led).toBeNull();
  });

  it("uses talkgroupLedColor from current call when available", () => {
    renderLED({
      scanner: {
        isLive: true,
        isPaused: false,
        isAudioActive: true,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: makeCall({ talkgroupLedColor: "#ff00ff" }),
        history: [],
        listenerCount: 0,
        connectionStatus: "connected",
        config: null,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    const led = document.querySelector(
      '[style*="background-color"]',
    ) as HTMLElement;
    expect(led).toBeTruthy();
    expect(led.style.backgroundColor).toBe("rgb(255, 0, 255)"); // #ff00ff
  });

  it("shows gray LED when not live and not playing", () => {
    renderLED({
      scanner: {
        isLive: false,
        isPaused: false,
        isAudioActive: false,
        heldSystem: null,
        heldTG: null,
        avoidList: [],
        currentCall: null,
        history: [],
        listenerCount: 0,
        connectionStatus: "connected",
        config: null,
        tgSelection: {},
        tgSelectionReady: true,
        pendingTranscripts: {},
      },
    });
    const led = document.querySelector(
      '[style*="background-color"]',
    ) as HTMLElement;
    expect(led).toBeTruthy();
    expect(led.style.backgroundColor).toBe("rgb(80, 80, 80)"); // #505050
  });
});
