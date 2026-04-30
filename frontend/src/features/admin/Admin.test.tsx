import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import Admin from "./Admin";
import { scannerSlice } from "@/features/scanner";
import { authSlice } from "@/features/auth";
import { callsSlice } from "@/features/scanner";
import { api } from "@/app/api";
import type { RootState } from "@/app/store";

// --- Mocks ---

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual =
    await vi.importActual<typeof import("react-router-dom")>(
      "react-router-dom",
    );
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    Navigate: ({ to }: { to: string }) => (
      <div data-testid="navigate" data-to={to} />
    ),
  };
});

// LegacyUsageBanner mounts on Admin; stub the data hook so it stays empty
// and we don't trigger a real fetch in jsdom.
vi.mock("@/components/admin/LegacyUsageBanner", () => ({
  default: () => null,
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

function renderAdmin(preloadedState?: Partial<RootState>) {
  const store = makeStore(preloadedState);
  return {
    store,
    ...render(
      <Provider store={store}>
        <MemoryRouter initialEntries={["/admin/users"]}>
          <Admin />
        </MemoryRouter>
      </Provider>,
    ),
  };
}

// --- Tests ---

describe("Admin", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("redirects to /login when no token", () => {
    renderAdmin();
    const navs = screen.getAllByTestId("navigate");
    const loginNav = navs.find((el) => el.getAttribute("data-to") === "/login");
    expect(loginNav).toBeDefined();
  });

  it("shows access denied when role is listener", () => {
    renderAdmin({
      auth: {
        token: "test-token",
        role: "listener",
        username: "user",
        passwordNeedChange: false,
        setupStatus: null,
      },
    } as Partial<RootState>);
    expect(screen.getByText("Access Denied")).toBeInTheDocument();
  });

  it("renders sidebar nav items when authenticated", () => {
    renderAdmin({
      auth: {
        token: "test-token",
        role: "admin",
        username: "admin",
        passwordNeedChange: false,
        setupStatus: null,
      },
    } as Partial<RootState>);

    const expectedLabels = [
      "Users",
      "Systems",
      "Groups & Tags",
      "API Keys",
      "Monitors",
      "Downstreams",
      "Options",
      "Logs",
      "Tools",
    ];

    for (const label of expectedLabels) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0);
    }
  });

  it("sign out button clears credentials", async () => {
    // Suppress RTK Query unhandled-error log (Node's Request can't resolve relative URLs in jsdom)
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});

    const { store } = renderAdmin({
      auth: {
        token: "test-token",
        role: "admin",
        username: "admin",
        passwordNeedChange: false,
        setupStatus: null,
      },
    } as Partial<RootState>);

    // Multiple sign out buttons may exist (mobile + desktop sidebars)
    const signOutButtons = screen.getAllByText("Sign Out");
    fireEvent.click(signOutButtons[0]);

    await waitFor(() => {
      expect(store.getState().auth.token).toBeNull();
    });
    expect(mockNavigate).toHaveBeenCalledWith("/login", { replace: true });

    spy.mockRestore();
  });
});
