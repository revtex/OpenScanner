import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import LegacyUsageBanner from "./LegacyUsageBanner";
import { api } from "@/app/api";
import { scannerSlice } from "@/features/scanner";
import { authSlice } from "@/features/auth";
import { callsSlice } from "@/features/scanner";
import type { LegacyUsageResponse } from "@/types";
import { trMqttReducer } from "@/app/store";

// ── Mocks ────────────────────────────────────────────────────────────────

type QueryResult = {
  data?: LegacyUsageResponse;
  isLoading: boolean;
  isError: boolean;
};

let mockResult: QueryResult = {
  data: undefined,
  isLoading: false,
  isError: false,
};

vi.mock("@/app/api", async () => {
  const actual = await vi.importActual<typeof import("@/app/api")>("@/app/api");
  return {
    ...actual,
    useGetLegacyUsageQuery: () => mockResult,
  };
});

// ── Harness ──────────────────────────────────────────────────────────────

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

function renderBanner() {
  return render(
    <Provider store={makeStore()}>
      <LegacyUsageBanner />
    </Provider>,
  );
}

const sampleResponse: LegacyUsageResponse = {
  windowSeconds: 86400,
  generatedAt: "2026-04-26T23:59:00Z",
  entries: [
    {
      path: "/api/call-upload",
      method: "POST",
      apiKeyIdent: "abc123",
      count: 47,
      lastSeen: new Date(Date.now() - 3 * 60_000).toISOString(),
    },
    {
      path: "/api/call",
      method: "GET",
      apiKeyIdent: "",
      count: 5,
      lastSeen: new Date(Date.now() - 30_000).toISOString(),
    },
  ],
};

describe("LegacyUsageBanner", () => {
  beforeEach(() => {
    sessionStorage.clear();
    mockResult = { data: undefined, isLoading: false, isError: false };
  });

  it("renders nothing when entries are empty", () => {
    mockResult = {
      data: {
        windowSeconds: 86400,
        generatedAt: "2026-04-26T23:59:00Z",
        entries: [],
      },
      isLoading: false,
      isError: false,
    };
    const { container } = renderBanner();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing while the query is loading", () => {
    mockResult = { data: undefined, isLoading: true, isError: false };
    const { container } = renderBanner();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the query errors (e.g. 401/403)", () => {
    mockResult = { data: undefined, isLoading: false, isError: true };
    const { container } = renderBanner();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders summary and entries when data is present", async () => {
    mockResult = {
      data: sampleResponse,
      isLoading: false,
      isError: false,
    };
    const user = userEvent.setup();
    renderBanner();

    expect(screen.getByText(/Legacy API in use/i)).toBeInTheDocument();
    // 52 total requests across 2 distinct keys ("abc123" + unauthenticated).
    expect(
      screen.getByText(/52 requests across 2 API keys/i),
    ).toBeInTheDocument();

    // Expand details.
    await user.click(screen.getByText(/Show details/i));

    expect(screen.getByText("/api/call-upload")).toBeInTheDocument();
    expect(screen.getByText("abc123")).toBeInTheDocument();
    expect(screen.getByText("(unauthenticated)")).toBeInTheDocument();
    expect(screen.getByText("47")).toBeInTheDocument();
  });

  it("hides the banner and persists dismissal in sessionStorage on dismiss", async () => {
    mockResult = {
      data: sampleResponse,
      isLoading: false,
      isError: false,
    };
    const user = userEvent.setup();
    renderBanner();

    await user.click(
      screen.getByRole("button", { name: /dismiss legacy api warning/i }),
    );

    expect(screen.queryByText(/Legacy API in use/i)).not.toBeInTheDocument();
    expect(sessionStorage.getItem("os.legacyUsageBanner.dismissed")).toBe("1");
  });

  it("stays hidden if sessionStorage already records dismissal", () => {
    sessionStorage.setItem("os.legacyUsageBanner.dismissed", "1");
    mockResult = {
      data: sampleResponse,
      isLoading: false,
      isError: false,
    };
    const { container } = renderBanner();
    expect(container).toBeEmptyDOMElement();
  });
});
