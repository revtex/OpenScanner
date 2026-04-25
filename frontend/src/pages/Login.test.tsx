import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import Login from "@/pages/Login";
import { scannerSlice } from "@/app/slices/scanner/scannerSlice";
import { authSlice } from "@/app/slices/shared/authSlice";
import { callsSlice } from "@/app/slices/scanner/callsSlice";
import { api } from "@/app/api";
import type { RootState } from "@/app/store";

// --- Mocks ---

const mockNavigate = vi.fn();
let mockLocationState: unknown = null;
vi.mock("react-router-dom", async () => {
  const actual =
    await vi.importActual<typeof import("react-router-dom")>(
      "react-router-dom",
    );
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({ state: mockLocationState }),
  };
});

const mockPostLogin = vi.fn();
const mockChangePassword = vi.fn();
const mockUseGetSetupStatusQuery = vi.fn();
vi.mock("@/app/api", async () => {
  const actual = await vi.importActual<typeof import("@/app/api")>("@/app/api");
  return {
    ...actual,
    useGetSetupStatusQuery: () => mockUseGetSetupStatusQuery(),
  };
});
vi.mock("@/app/slices/shared/authSlice", async () => {
  const actual = await vi.importActual<typeof import("@/app/slices/shared/authSlice")>(
    "@/app/slices/shared/authSlice",
  );
  return {
    ...actual,
    usePostLoginMutation: () => [mockPostLogin, { isLoading: false }],
    useChangePasswordMutation: () => [mockChangePassword, { isLoading: false }],
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

function renderLogin() {
  const store = makeStore();
  return render(
    <Provider store={store}>
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    </Provider>,
  );
}

// --- Tests ---

describe("Login", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLocationState = null;
    mockUseGetSetupStatusQuery.mockReturnValue({
      data: { needsSetup: false, publicAccess: false },
      isLoading: false,
    });
  });

  it("redirects to /setup while initial setup is active", () => {
    mockUseGetSetupStatusQuery.mockReturnValue({
      data: { needsSetup: true, publicAccess: false },
      isLoading: false,
    });

    renderLogin();
    expect(screen.queryByPlaceholderText("Username")).not.toBeInTheDocument();
  });

  it("renders login form", () => {
    renderLogin();
    expect(screen.getByPlaceholderText("Username")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Password")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign In" })).toBeInTheDocument();
  });

  it("shows error on login failure", async () => {
    mockPostLogin.mockReturnValue({
      unwrap: () => Promise.reject(new Error("bad credentials")),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "wrong" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(
        screen.getByText("Invalid username or password"),
      ).toBeInTheDocument();
    });
  });

  it("shows change password form when passwordNeedChange is true", async () => {
    mockPostLogin.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "t",
          user: { id: 1, username: "admin", role: "admin" },
          passwordNeedChange: true,
        }),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "password" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(screen.getByText("Change Password")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText("New password")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Confirm password")).toBeInTheDocument();
  });

  it("redirects to / after admin login", async () => {
    mockPostLogin.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "tok",
          user: { id: 1, username: "admin", role: "admin" },
          passwordNeedChange: false,
        }),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "pass1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", {
        replace: true,
      });
    });
  });

  it("redirects admin to requested admin route after login", async () => {
    mockLocationState = { from: "/admin/users" };
    mockPostLogin.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "tok",
          user: { id: 1, username: "admin", role: "admin" },
          passwordNeedChange: false,
        }),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "pass1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/admin/users", {
        replace: true,
      });
    });
  });

  it("redirects listener to / after login", async () => {
    mockPostLogin.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "tok",
          user: { id: 2, username: "user1", role: "listener" },
          passwordNeedChange: false,
        }),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "user1" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "pass1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });

  it("redirects listener to / when requested route is admin", async () => {
    mockLocationState = { from: "/admin/users" };
    mockPostLogin.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "tok",
          user: { id: 2, username: "user1", role: "listener" },
          passwordNeedChange: false,
        }),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "user1" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "pass1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });

  it("rejects protocol-relative redirect targets", async () => {
    mockLocationState = { from: "//evil.example/steal" };
    mockPostLogin.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "tok",
          user: { id: 1, username: "admin", role: "admin" },
          passwordNeedChange: false,
        }),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "pass1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });

  it("rejects redirect targets with query strings", async () => {
    mockLocationState = { from: "/admin/users?next=/admin" };
    mockPostLogin.mockReturnValue({
      unwrap: () =>
        Promise.resolve({
          token: "tok",
          user: { id: 1, username: "admin", role: "admin" },
          passwordNeedChange: false,
        }),
    });

    renderLogin();
    fireEvent.change(screen.getByPlaceholderText("Username"), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByPlaceholderText("Password"), {
      target: { value: "pass1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });
});
