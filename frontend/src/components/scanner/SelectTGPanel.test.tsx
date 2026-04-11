import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import SelectTGPanel from "@/components/scanner/SelectTGPanel";
import { scannerSlice } from "@/app/slices/scannerSlice";
import { authSlice } from "@/app/slices/authSlice";
import { callsSlice } from "@/app/slices/callsSlice";
import { api } from "@/app/api";
import type { RootState } from "@/app/store";
import type { ScannerConfig } from "@/types";

// --- Mocks ---

vi.mock("react-router-dom", () => ({
  useSearchParams: () => [new URLSearchParams()],
}));

vi.mock("@tanstack/react-virtual", () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    getVirtualItems: () =>
      Array.from({ length: count }, (_, i) => ({
        index: i,
        start: i * 52,
        size: 52,
        key: i,
      })),
    getTotalSize: () => count * 52,
    measureElement: () => {},
  }),
}));

// --- Helpers ---

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

const twoGroupConfig: ScannerConfig = {
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
};

function scannerState(
  overrides: Partial<RootState["scanner"]> = {},
): RootState["scanner"] {
  return {
    isLive: true,
    isPaused: false,
    heldSystem: null,
    heldTG: null,
    avoidList: [],
    callQueue: [],
    currentCall: null,
    history: [],
    listenerCount: 0,
    connectionStatus: "disconnected",
    config: twoGroupConfig,
    tgSelection: {},
    ...overrides,
  };
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

// --- Tests ---

describe("SelectTGPanel", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  describe("panel open / close", () => {
    it("is visible when isOpen is true", () => {
      renderPanel({ scanner: scannerState() }, true);
      expect(screen.getByText("Select Talkgroups")).toBeInTheDocument();
    });

    it("has translate-x-full class when isOpen is false", () => {
      const { container } = renderPanel({ scanner: scannerState() }, false);
      const panel = container.querySelector(".translate-x-full");
      expect(panel).toBeInTheDocument();
    });

    it("clicking backdrop calls onClose", () => {
      const onClose = vi.fn();
      renderPanel({ scanner: scannerState() }, true, onClose);
      // The backdrop is the first div with bg-black/50 class
      const backdrop = document.querySelector(".bg-black\\/50");
      expect(backdrop).toBeTruthy();
      fireEvent.click(backdrop!);
      expect(onClose).toHaveBeenCalledOnce();
    });

    it("clicking close button calls onClose", () => {
      const onClose = vi.fn();
      renderPanel({ scanner: scannerState() }, true, onClose);
      fireEvent.click(screen.getByRole("button", { name: "Close" }));
      expect(onClose).toHaveBeenCalledOnce();
    });
  });

  describe("group tri-state logic", () => {
    it("group shows ON (btn-primary) when all TGs in group are enabled", () => {
      // By default tgSelection is {}, which means all enabled (tgSelection[id] !== false)
      renderPanel({ scanner: scannerState() });
      const policeBtn = screen.getByRole("button", { name: /Police/ });
      expect(policeBtn.className).toContain("btn-primary");
      expect(policeBtn.className).not.toContain("btn-outline");
    });

    it("group shows OFF (btn-ghost) when all TGs in group are disabled", () => {
      // Police group has TG 10 and 20 — disable both
      renderPanel({
        scanner: scannerState({
          tgSelection: { 10: false, 11: false, 20: false },
        }),
      });
      const policeBtn = screen.getByRole("button", { name: /Police/ });
      expect(policeBtn.className).toContain("btn-ghost");
    });

    it("group shows PARTIAL (btn-outline btn-primary) when some TGs enabled", () => {
      // Police group has TG 10 (enabled by default) and TG 20 (disabled)
      renderPanel({
        scanner: scannerState({ tgSelection: { 20: false } }),
      });
      const policeBtn = screen.getByRole("button", { name: /Police/ });
      expect(policeBtn.className).toContain("btn-outline");
      expect(policeBtn.className).toContain("btn-primary");
    });

    it("clicking a PARTIAL group toggles all its TGs ON", () => {
      const { store } = renderPanel({
        scanner: scannerState({ tgSelection: { 20: false } }),
      });
      const policeBtn = screen.getByRole("button", { name: /Police/ });
      fireEvent.click(policeBtn);
      const state = store.getState();
      // TG 10 was already on (undefined → stays), TG 20 should now be toggled on
      expect(state.scanner.tgSelection[20]).toBe(true);
    });

    it("clicking an ON group toggles all its TGs OFF", () => {
      const { store } = renderPanel({
        scanner: scannerState(),
      });
      // Fire group has only TG 11, which is enabled by default
      const fireBtn = screen.getByRole("button", { name: /Fire/ });
      fireEvent.click(fireBtn);
      const state = store.getState();
      expect(state.scanner.tgSelection[11]).toBe(true);
    });
  });

  describe("system accordion", () => {
    it("shows system label and active/total count badge", () => {
      renderPanel({ scanner: scannerState() });
      expect(screen.getByText("System Alpha")).toBeInTheDocument();
      expect(screen.getByText("System Beta")).toBeInTheDocument();
      // Badge shows active/total — with no selection, all are enabled
      expect(screen.getByText("2/2")).toBeInTheDocument();
      expect(screen.getByText("1/1")).toBeInTheDocument();
    });

    it("expanding accordion shows talkgroup chips", () => {
      renderPanel({ scanner: scannerState() });
      // Click System Alpha to expand
      fireEvent.click(screen.getByText("System Alpha"));
      expect(screen.getByRole("button", { name: /TG-A1/ })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /TG-A2/ })).toBeInTheDocument();
    });

    it("per-system On button enables all TGs in that system", () => {
      const { store } = renderPanel({
        scanner: scannerState({ tgSelection: { 10: false, 11: false } }),
      });
      // Expand System Alpha
      fireEvent.click(screen.getByText("System Alpha"));
      // Find the system-level "On" button (within the expanded section)
      const onButtons = screen.getAllByRole("button", { name: "On" });
      fireEvent.click(onButtons[0]);
      const state = store.getState();
      expect(state.scanner.tgSelection[10]).toBe(true);
      expect(state.scanner.tgSelection[11]).toBe(true);
    });

    it("per-system Off button disables all TGs in that system", () => {
      const { store } = renderPanel({
        scanner: scannerState(),
      });
      // Expand System Alpha
      fireEvent.click(screen.getByText("System Alpha"));
      const offButtons = screen.getAllByRole("button", { name: "Off" });
      fireEvent.click(offButtons[0]);
      const state = store.getState();
      expect(state.scanner.tgSelection[10]).toBe(false);
      expect(state.scanner.tgSelection[11]).toBe(false);
    });
  });

  describe("TG chips", () => {
    it("shows TG label text", () => {
      renderPanel({ scanner: scannerState() });
      fireEvent.click(screen.getByText("System Alpha"));
      expect(screen.getByRole("button", { name: /TG-A1/ })).toBeInTheDocument();
    });

    it("has ledColor as left border style", () => {
      renderPanel({ scanner: scannerState() });
      fireEvent.click(screen.getByText("System Alpha"));
      const tgBtn = screen.getByRole("button", { name: /TG-A1/ });
      expect(tgBtn.style.borderLeft).toBe("6px solid rgb(0, 255, 0)");
    });

    it("clicking dispatches toggleTG action", () => {
      const { store } = renderPanel({ scanner: scannerState() });
      fireEvent.click(screen.getByText("System Alpha"));
      const tgBtn = screen.getByRole("button", { name: /TG-A1/ });
      fireEvent.click(tgBtn);
      // Initially undefined → toggleTG → true
      expect(store.getState().scanner.tgSelection[10]).toBe(true);
    });

    it("avoided TGs show pulse animation class", () => {
      renderPanel({
        scanner: scannerState({
          avoidList: [{ talkgroupId: 200, expiresAt: 0 }],
        }),
      });
      fireEvent.click(screen.getByText("System Alpha"));
      const tgBtn = screen.getByRole("button", { name: /TG-A1/ });
      expect(tgBtn.className).toContain("animate-pulse");
    });

    it("non-avoided TGs do not show pulse animation class", () => {
      renderPanel({ scanner: scannerState() });
      fireEvent.click(screen.getByText("System Alpha"));
      const tgBtn = screen.getByRole("button", { name: /TG-A1/ });
      expect(tgBtn.className).not.toContain("animate-pulse");
    });
  });

  describe("global All On / All Off", () => {
    it("All On dispatches setAllTGs(true)", () => {
      const { store } = renderPanel({
        scanner: scannerState({
          tgSelection: { 10: false, 11: false, 20: false },
        }),
      });
      fireEvent.click(screen.getByRole("button", { name: "All On" }));
      const state = store.getState();
      expect(state.scanner.tgSelection[10]).toBe(true);
      expect(state.scanner.tgSelection[11]).toBe(true);
      expect(state.scanner.tgSelection[20]).toBe(true);
    });

    it("All Off dispatches setAllTGs(false)", () => {
      const { store } = renderPanel({
        scanner: scannerState(),
      });
      fireEvent.click(screen.getByRole("button", { name: "All Off" }));
      const state = store.getState();
      expect(state.scanner.tgSelection[10]).toBe(false);
      expect(state.scanner.tgSelection[11]).toBe(false);
      expect(state.scanner.tgSelection[20]).toBe(false);
    });
  });
});
