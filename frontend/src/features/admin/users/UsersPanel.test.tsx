import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import UsersPanel from "./UsersPanel";
import { scannerSlice } from "@/features/scanner";
import { authSlice } from "@/features/auth";
import { callsSlice } from "@/features/scanner";
import { api } from "@/app/api";
import type { AdminUser, AdminSystem } from "@/types";
import { trMqttReducer } from "@/app/store";

// ── Mocks ────────────────────────────────────────────────────────────────

const mockUsers: AdminUser[] = [
  {
    id: 1,
    username: "admin",
    role: "admin",
    disabled: 0,
    systemsJson: null,
    expiration: null,
    limit: null,
    createdAt: 0,
    updatedAt: 0,
  },
  {
    id: 2,
    username: "alice",
    role: "listener",
    disabled: 0,
    systemsJson: null,
    expiration: null,
    limit: null,
    createdAt: 0,
    updatedAt: 0,
  },
];

const mockSystems: AdminSystem[] = [
  {
    id: 10,
    systemId: 1,
    label: "County PD",
    autoPopulateTalkgroups: 1,
    blacklistsJson: null,
    led: null,
    order: 0,
  },
];

const createUserUnwrap = vi.fn();
const updateUserUnwrap = vi.fn();
const deleteUserUnwrap = vi.fn();

const createUserMutate = vi.fn((_arg: unknown) => ({
  unwrap: createUserUnwrap,
}));
const updateUserMutate = vi.fn((_arg: unknown) => ({
  unwrap: updateUserUnwrap,
}));
const deleteUserMutate = vi.fn((_arg: unknown) => ({
  unwrap: deleteUserUnwrap,
}));

vi.mock("@/features/admin/_shell", () => ({
  useListUsersQuery: () => ({ data: mockUsers, isLoading: false }),
  useListSystemsQuery: () => ({ data: mockSystems, isLoading: false }),
  useCreateUserMutation: () => [createUserMutate, {}],
  useUpdateUserMutation: () => [updateUserMutate, {}],
  useDeleteUserMutation: () => [deleteUserMutate, {}],
}));

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

function renderPanel() {
  return render(
    <Provider store={makeStore()}>
      <MemoryRouter>
        <UsersPanel />
      </MemoryRouter>
    </Provider>,
  );
}

describe("UsersPanel", () => {
  beforeEach(() => {
    createUserMutate.mockClear();
    updateUserMutate.mockClear();
    deleteUserMutate.mockClear();
    createUserUnwrap.mockReset();
    updateUserUnwrap.mockReset();
    deleteUserUnwrap.mockReset();
    createUserUnwrap.mockResolvedValue(undefined);
    updateUserUnwrap.mockResolvedValue(undefined);
    deleteUserUnwrap.mockResolvedValue(undefined);
  });

  it("renders the list of users", () => {
    renderPanel();
    // "admin" appears twice (username cell + role badge); "alice" is unique.
    expect(screen.getAllByText("admin").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("alice")).toBeInTheDocument();
  });

  it("opens the create dialog when Add User is clicked", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: /add user/i }));
    // Dialog is a <dialog> element — search in the hidden tree.
    expect(
      screen.getByRole("heading", { name: /create user/i, hidden: true }),
    ).toBeInTheDocument();
  });

  it("submits a create with the expected payload", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: /add user/i }));

    const usernameInput = screen.getByLabelText(/username/i);
    const passwordInput = screen.getByLabelText(/password/i);

    await user.type(usernameInput, "charlie");
    await user.type(passwordInput, "secret123");

    await user.click(
      screen.getByRole("button", { name: /^create$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(createUserMutate).toHaveBeenCalledTimes(1);
    });
    const arg = createUserMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(arg.username).toBe("charlie");
    expect(arg.password).toBe("secret123");
    expect(arg.role).toBe("listener");
    expect(arg.disabled).toBe(0);
  });

  it("prompts for confirmation before delete and calls deleteUser on confirm", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    renderPanel();

    // Primary admin (id=1) has no delete button — delete only the listener.
    const deleteButtons = screen.getAllByRole("button", {
      name: /delete user/i,
    });
    await user.click(deleteButtons[0]);

    expect(confirmSpy).toHaveBeenCalledWith('Delete user "alice"?');
    await waitFor(() => {
      expect(deleteUserMutate).toHaveBeenCalledWith(2);
    });
    confirmSpy.mockRestore();
  });

  it("does not call deleteUser when confirmation is declined", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    renderPanel();

    const deleteButtons = screen.getAllByRole("button", {
      name: /delete user/i,
    });
    await user.click(deleteButtons[0]);

    expect(deleteUserMutate).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it("submits an edit with updated fields when the edit dialog is saved", async () => {
    const user = userEvent.setup();
    renderPanel();

    const editButtons = screen.getAllByRole("button", { name: /edit user/i });
    await user.click(editButtons[1]); // edit "alice"
    expect(
      screen.getByRole("heading", { name: /edit user/i, hidden: true }),
    ).toBeInTheDocument();

    const usernameInput = screen.getByLabelText(/username/i);
    await user.clear(usernameInput);
    await user.type(usernameInput, "alice2");

    await user.click(
      screen.getByRole("button", { name: /^save$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(updateUserMutate).toHaveBeenCalledTimes(1);
    });
    const arg = updateUserMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(arg.id).toBe(2);
    expect(arg.username).toBe("alice2");
  });
});
