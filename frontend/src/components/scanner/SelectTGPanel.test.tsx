import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import SelectTGPanel from "@/components/scanner/SelectTGPanel";
import { scannerSlice } from "@/app/slices/scannerSlice";
import { authSlice } from "@/app/slices/authSlice";
import { callsSlice } from "@/app/slices/callsSlice";
import { api } from "@/app/api";
import type { RootState } from "@/app/store";
import type { ScannerConfig } from "@/types";

const testConfig: ScannerConfig = {
  systems: [
    {
      id: 1,
      systemId: 100,
      label: "System Alpha",
      talkgroups: [
        {
          id: 10,
          talkgroupId: 200,
          label: "TG-A1",
          name: "Alpha One",
          tag: "Law",
          group: "Police",
          ledColor: "#00ff00",
        },
        {
          id: 11,
          talkgroupId: 201,
          label: "TG-A2",
          name: "Alpha Two",
          tag: "Fire",
          group: "Fire",
          ledColor: "#ff0000",
        },
      ],
    },
    {
      id: 2,
      systemId: 101,
      label: "System Beta",
      talkgroups: [
        {
          id: 20,
          talkgroupId: 300,
          label: "TG-B1",
          name: "Beta One",
          tag: "Law",
          group: "Police",
          ledColor: "#0000ff",
        },
      ],
    },
  ],
  branding: "TEST",
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

function scannerState(
  overrides: Partial<RootState["scanner"]> = {},
): RootState["scanner"] {
  return {
    isLive: false,
    isPaused: false,
    isAudioActive: false,
    heldSystem: null,
    heldTG: null,
    avoidList: [],
    currentCall: null,
    history: [],
    listenerCount: 0,
    connectionStatus: "disconnected",
    config: testConfig,
    tgSelection: {},
    tgSelectionReady: true,
    ...overrides,
  };
}

function makeStore(preloadedState?: Partial<RootState>) {
  return configureStore({
    reducer: {
      scanner: scannerSlice.reducer,
      auth: authSlice.reducer,
      calls: callsSlice.reducer,
      [api.reducerPath]: api.reducer,
    },
    middleware: (gDM) => gDM().concat(api.middleware),
    preloadedState: preloadedState as RootState,
  });
}

function renderPanel(
  preloadedState?: Partial<RootState>,
  isOpen = true,
  onClose = vi.fn(),
) {
  const store = makeStore(preloadedState);
  const result = render(
    <Provider store={store}>
      <SelectTGPanel isOpen={isOpen} onClose={onClose} />
    </Provider>,
  );
  return { ...result, store, onClose };
}

function clickGroupToggle(
  label: string,
  actionLabel: "Turn all on" | "Turn all off",
) {
  const headerButton = screen.getByRole("button", {
    name: new RegExp(label, "i"),
  });
  const row = headerButton.closest("div");
  const toggleButton = row?.querySelector(
    `button[aria-label=\"${actionLabel}\"]`,
  ) as HTMLButtonElement | null;
  expect(toggleButton).toBeTruthy();
  fireEvent.click(toggleButton!);
}

describe("SelectTGPanel", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("is not rendered when isOpen is false", () => {
    const { container } = renderPanel({ scanner: scannerState() }, false);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders header when open", () => {
    renderPanel({ scanner: scannerState() }, true);
    expect(screen.getByText("Select Talkgroups")).toBeInTheDocument();
  });

  it("clicking close button calls onClose", () => {
    const onClose = vi.fn();
    renderPanel({ scanner: scannerState() }, true, onClose);
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("group toggle turns all talkgroups in group off", () => {
    const { store } = renderPanel({ scanner: scannerState() });
    clickGroupToggle("Police", "Turn all off");
    const state = store.getState().scanner;
    expect(state.tgSelection[10]).toBe(false);
    expect(state.tgSelection[20]).toBe(false);
  });

  it("group toggle turns all talkgroups in group on", () => {
    const { store } = renderPanel({
      scanner: scannerState({ tgSelection: { 10: false, 20: false } }),
    });

    clickGroupToggle("Police", "Turn all on");
    const state = store.getState().scanner;
    expect(state.tgSelection[10]).toBe(true);
    expect(state.tgSelection[20]).toBe(true);
  });

  it("system section expands and shows talkgroup names", () => {
    renderPanel({ scanner: scannerState() });

    fireEvent.click(screen.getByRole("button", { name: /systems/i }));
    fireEvent.click(screen.getByRole("button", { name: /System Alpha/i }));

    expect(screen.getByText("TG-A1 - Alpha One")).toBeInTheDocument();
    expect(screen.getByText("TG-A2 - Alpha Two")).toBeInTheDocument();
  });

  it("clicking talkgroup checkbox toggles selection", () => {
    const { store } = renderPanel({ scanner: scannerState() });

    fireEvent.click(screen.getByRole("button", { name: /systems/i }));
    fireEvent.click(screen.getByRole("button", { name: /System Alpha/i }));

    const tgLabel = screen.getByText("TG-A1 - Alpha One").closest("label");
    expect(tgLabel).toBeTruthy();
    const checkbox = within(tgLabel!).getByRole("checkbox");
    fireEvent.click(checkbox);
    expect(store.getState().scanner.tgSelection[10]).toBe(true);
  });

  it("avoided talkgroup shows avoid badge", () => {
    renderPanel({
      scanner: scannerState({ avoidList: [{ talkgroupId: 10, expiresAt: 0 }] }),
    });

    fireEvent.click(screen.getByRole("button", { name: /systems/i }));
    fireEvent.click(screen.getByRole("button", { name: /System Alpha/i }));

    expect(screen.getByText("AVOID")).toBeInTheDocument();
  });

  it("global toggle sets all talkgroups off", () => {
    const { store } = renderPanel({ scanner: scannerState() });

    const allRow = screen.getByText("All Talkgroups").closest("div");
    const globalToggle = allRow?.querySelector(
      'button[aria-label="Turn all off"]',
    ) as HTMLButtonElement | null;
    expect(globalToggle).toBeTruthy();
    fireEvent.click(globalToggle!);

    const state = store.getState().scanner;
    expect(state.tgSelection[10]).toBe(false);
    expect(state.tgSelection[11]).toBe(false);
    expect(state.tgSelection[20]).toBe(false);
  });
});
