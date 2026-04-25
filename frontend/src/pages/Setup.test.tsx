import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import Setup from "@/pages/Setup";
import { scannerSlice } from "@/app/slices/scanner/scannerSlice";
import { authSlice } from "@/app/slices/shared/authSlice";
import { callsSlice } from "@/app/slices/scanner/callsSlice";
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
    Navigate: ({ to }: { to: string }) => {
      mockNavigate(to, { replace: true });
      return null;
    },
  };
});

const mockPostSetup = vi.fn();
const mockSetupStatus = { data: { needsSetup: true }, isLoading: false };
vi.mock("@/app/api", async () => {
  const actual = await vi.importActual<typeof import("@/app/api")>("@/app/api");
  return {
    ...actual,
    usePostSetupMutation: () => [mockPostSetup, { isLoading: false }],
    useGetSetupStatusQuery: () => mockSetupStatus,
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

function renderSetup() {
  const store = makeStore();
  return render(
    <Provider store={store}>
      <Setup />
    </Provider>,
  );
}

// --- Tests ---

describe("Setup", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders setup form", () => {
    renderSetup();
    expect(screen.getByPlaceholderText("Admin username")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Password")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Confirm password")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create Admin" }),
    ).toBeInTheDocument();
  });

  it("shows error when passwords don't match", async () => {
    renderSetup();
    fireEvent.change(screen.getByPlaceholderText("Admin username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "password123" },
    });
    fireEvent.change(screen.getByPlaceholderText("Confirm password"), {
      target: { value: "different123" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create Admin" }));

    await waitFor(() => {
      expect(screen.getByText("Passwords do not match")).toBeInTheDocument();
    });
  });

  it("shows error when password too short", async () => {
    renderSetup();
    fireEvent.change(screen.getByPlaceholderText("Admin username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "short" },
    });
    fireEvent.change(screen.getByPlaceholderText("Confirm password"), {
      target: { value: "short" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create Admin" }));

    await waitFor(() => {
      expect(
        screen.getByText("Password must be at least 8 characters"),
      ).toBeInTheDocument();
    });
  });

  it("submits setup and redirects to /login", async () => {
    mockPostSetup.mockReturnValue({
      unwrap: () => Promise.resolve(),
    });

    renderSetup();
    fireEvent.change(screen.getByPlaceholderText("Admin username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "password123" },
    });
    fireEvent.change(screen.getByPlaceholderText("Confirm password"), {
      target: { value: "password123" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create Admin" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/login", { replace: true });
    });
  });
});
