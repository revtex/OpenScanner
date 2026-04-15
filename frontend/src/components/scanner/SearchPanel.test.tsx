import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import SearchPanel from "@/components/scanner/SearchPanel";
import { scannerSlice } from "@/app/slices/scannerSlice";
import { authSlice } from "@/app/slices/authSlice";
import { callsSlice } from "@/app/slices/callsSlice";
import { api } from "@/app/api";
import type { RootState } from "@/app/store";
import type { ScannerConfig } from "@/types";

// --- Mocks ---

const mockSearchCallsQuery = vi.fn();

vi.mock("@/app/slices/callsSlice", async () => {
  const actual = await vi.importActual<
    typeof import("@/app/slices/callsSlice")
  >("@/app/slices/callsSlice");
  return {
    ...actual,
    useSearchCallsQuery: (...args: unknown[]) => mockSearchCallsQuery(...args),
  };
});

vi.mock("@tanstack/react-virtual", () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    getVirtualItems: () =>
      Array.from({ length: count }, (_, i) => ({
        index: i,
        start: i * 44,
        size: 44,
        key: i,
      })),
    getTotalSize: () => count * 44,
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
          group: "Fire Dept",
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
          tag: "EMS",
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
    currentCall: null,
    history: [],
    listenerCount: 0,
    connectionStatus: "disconnected",
    config: testConfig,
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
      <SearchPanel isOpen={isOpen} onClose={onClose} />
    </Provider>,
  );
  return { ...result, store, onClose };
}

// --- Tests ---

describe("SearchPanel", () => {
  beforeEach(() => {
    mockSearchCallsQuery.mockReturnValue({
      data: { calls: [], total: 0 },
      isFetching: false,
    });
  });

  describe("panel visibility", () => {
    it("is visible when isOpen is true", () => {
      renderPanel({ scanner: scannerState() }, true);
      expect(screen.getByText("Search Calls")).toBeInTheDocument();
    });

    it("has -translate-x-full class when isOpen is false", () => {
      const { container } = renderPanel({ scanner: scannerState() }, false);
      const panel = container.querySelector(".-translate-x-full");
      expect(panel).toBeInTheDocument();
    });

    it("clicking close button calls onClose", () => {
      const onClose = vi.fn();
      renderPanel({ scanner: scannerState() }, true, onClose);
      fireEvent.click(screen.getByRole("button", { name: "Close" }));
      expect(onClose).toHaveBeenCalledOnce();
    });

    it("clicking backdrop calls onClose", () => {
      const onClose = vi.fn();
      renderPanel({ scanner: scannerState() }, true, onClose);
      const backdrop = document.querySelector(".bg-black\\/50");
      expect(backdrop).toBeTruthy();
      fireEvent.click(backdrop!);
      expect(onClose).toHaveBeenCalledOnce();
    });
  });

  describe("filter controls", () => {
    it("system checkbox filters are populated from config", () => {
      renderPanel({ scanner: scannerState() });
      expect(screen.getByText("System Alpha")).toBeInTheDocument();
      expect(screen.getByText("System Beta")).toBeInTheDocument();
    });

    it("sort dropdown has 'Newest first' and 'Oldest first' options", () => {
      renderPanel({ scanner: scannerState() });
      const sortSelect = screen.getByDisplayValue("Newest first");
      expect(sortSelect).toBeInTheDocument();
      const options = sortSelect.querySelectorAll("option");
      expect(options).toHaveLength(2);
      expect(options[0].textContent).toBe("Newest first");
      expect(options[1].textContent).toBe("Oldest first");
    });

    it("reset filters button clears all filters", () => {
      const { store } = renderPanel({ scanner: scannerState() });
      // First set a filter
      act(() => {
        store.dispatch(callsSlice.actions.setSort("asc"));
      });
      // Click reset
      fireEvent.click(screen.getByText("Reset filters"));
      const state = store.getState();
      expect(state.calls.sort).toBe("desc");
      expect(state.calls.systemIds).toEqual([]);
      expect(state.calls.page).toBe(1);
    });
  });

  describe("active filter count", () => {
    it("badge shows correct count of active filters", () => {
      renderPanel({
        scanner: scannerState(),
        calls: {
          systemIds: [1],
          talkgroupIds: [],
          groupFilters: [],
          tagFilters: [],
          sort: "desc",
          page: 1,
          limit: 25,
          bookmarkedOnly: true,
          downloadMode: false,
        },
      });
      // systemIds + bookmarkedOnly = 2 active filters
      expect(screen.getByText("2")).toBeInTheDocument();
    });
  });

  describe("pagination", () => {
    it("shows 'Page X of Y' text", () => {
      mockSearchCallsQuery.mockReturnValue({
        data: { calls: [], total: 50 },
        isFetching: false,
      });
      renderPanel({ scanner: scannerState() });
      expect(screen.getByText("Page 1 of 2")).toBeInTheDocument();
    });

    it("Next and Prev buttons exist", () => {
      renderPanel({ scanner: scannerState() });
      expect(screen.getByText("Prev")).toBeInTheDocument();
      expect(screen.getByText("Next")).toBeInTheDocument();
    });

    it("clicking Next dispatches setPage", () => {
      mockSearchCallsQuery.mockReturnValue({
        data: { calls: [], total: 50 },
        isFetching: false,
      });
      const { store } = renderPanel({ scanner: scannerState() });
      fireEvent.click(screen.getByText("Next"));
      expect(store.getState().calls.page).toBe(2);
    });
  });

  describe("search results", () => {
    it("shows 'No results' when no calls returned", () => {
      mockSearchCallsQuery.mockReturnValue({
        data: { calls: [], total: 0 },
        isFetching: false,
      });
      renderPanel({ scanner: scannerState() });
      expect(screen.getByText("No results")).toBeInTheDocument();
    });

    it("renders call rows from query results", () => {
      mockSearchCallsQuery.mockReturnValue({
        data: {
          calls: [
            {
              id: 1,
              dateTime: 1700000000,
              frequency: 0,
              duration: 5000,
              source: 0,
              systemId: 100,
              talkgroupId: 200,
              systemLabel: "Sys1",
              talkgroupLabel: "TG1",
              talkgroupName: "Talkgroup One",
              talkgroupTag: "Law",
              talkgroupGroup: "Police",
              talkgroupLed: "#00ff00",
              transcript: "",
              bookmarked: false,
            },
          ],
          total: 1,
        },
        isFetching: false,
      });
      renderPanel({ scanner: scannerState() });
      expect(screen.getByText("Talkgroup One")).toBeInTheDocument();
      expect(screen.getByText("Sys1")).toBeInTheDocument();
    });
  });
});
