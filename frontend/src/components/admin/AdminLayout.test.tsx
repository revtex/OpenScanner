import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import AdminLayout from "@/components/admin/AdminLayout";
import { scannerSlice } from "@/app/slices/scannerSlice";
import { authSlice } from "@/app/slices/authSlice";
import { callsSlice } from "@/app/slices/callsSlice";
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
          <AdminLayout />
        </MemoryRouter>
      </Provider>,
    ),
  };
}

// --- Tests ---

describe("AdminLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("redirects to /login when no token", () => {
    renderAdmin();
    const navs = screen.getAllByTestId("navigate");
    const loginNav = navs.find((el) => el.getAttribute("data-to") === "/login");
    expect(loginNav).toBeDefined();
  });

  it("redirects to /login when role is listener", () => {
    renderAdmin({
      auth: {
        token: "test-token",
        role: "listener",
        username: "user",
        passwordNeedChange: false,
        setupStatus: null,
      },
    } as Partial<RootState>);
    const navs = screen.getAllByTestId("navigate");
    const loginNav = navs.find((el) => el.getAttribute("data-to") === "/login");
    expect(loginNav).toBeDefined();
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
      "Accesses",
      "Dir Watches",
      "Downstreams",
      "Options",
      "Logs",
      "Tools",
    ];

    for (const label of expectedLabels) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0);
    }
  });

  it("sign out button clears credentials", () => {
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

    expect(store.getState().auth.token).toBeNull();
    expect(mockNavigate).toHaveBeenCalledWith("/login", { replace: true });
  });
});
