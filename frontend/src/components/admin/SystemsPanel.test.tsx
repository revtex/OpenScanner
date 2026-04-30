import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import SystemsPanel from "@/components/admin/SystemsPanel";
import { scannerSlice } from "@/app/slices/scanner/scannerSlice";
import { authSlice } from "@/features/auth";
import { callsSlice } from "@/app/slices/scanner/callsSlice";
import { api } from "@/app/api";
import type { AdminSystem } from "@/types";

// ── Mocks ────────────────────────────────────────────────────────────────

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
  {
    id: 11,
    systemId: 2,
    label: "Fire EMS",
    autoPopulateTalkgroups: 0,
    blacklistsJson: null,
    led: "red",
    order: 1,
  },
];

const createSystemUnwrap = vi.fn();
const updateSystemUnwrap = vi.fn();
const deleteSystemUnwrap = vi.fn();

const createSystemMutate = vi.fn((_arg: unknown) => ({
  unwrap: createSystemUnwrap,
}));
const updateSystemMutate = vi.fn((_arg: unknown) => ({
  unwrap: updateSystemUnwrap,
}));
const deleteSystemMutate = vi.fn((_arg: unknown) => ({
  unwrap: deleteSystemUnwrap,
}));

const noopMutate = vi.fn(() => ({
  unwrap: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("@/hooks/admin/useAdminWsOps", () => ({
  useListSystemsQuery: () => ({ data: mockSystems, isLoading: false }),
  useCreateSystemMutation: () => [createSystemMutate, {}],
  useUpdateSystemMutation: () => [updateSystemMutate, {}],
  useDeleteSystemMutation: () => [deleteSystemMutate, {}],
  useListTalkgroupsQuery: () => ({ data: [], isLoading: false }),
  useCreateTalkgroupMutation: () => [noopMutate, {}],
  useUpdateTalkgroupMutation: () => [noopMutate, {}],
  useDeleteTalkgroupMutation: () => [noopMutate, {}],
  useListUnitsQuery: () => ({ data: [], isLoading: false }),
  useCreateUnitMutation: () => [noopMutate, {}],
  useUpdateUnitMutation: () => [noopMutate, {}],
  useDeleteUnitMutation: () => [noopMutate, {}],
  useListGroupsQuery: () => ({ data: [], isLoading: false }),
  useListTagsQuery: () => ({ data: [], isLoading: false }),
  useGetConfigQuery: () => ({ data: { settings: [] }, isLoading: false }),
  useUpdateConfigMutation: () => [noopMutate, {}],
}));

function makeStore() {
  return configureStore({
    reducer: {
      scanner: scannerSlice.reducer,
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
        <SystemsPanel />
      </MemoryRouter>
    </Provider>,
  );
}

describe("SystemsPanel", () => {
  beforeEach(() => {
    createSystemMutate.mockClear();
    updateSystemMutate.mockClear();
    deleteSystemMutate.mockClear();
    createSystemUnwrap.mockReset();
    updateSystemUnwrap.mockReset();
    deleteSystemUnwrap.mockReset();
    createSystemUnwrap.mockResolvedValue(undefined);
    updateSystemUnwrap.mockResolvedValue(undefined);
    deleteSystemUnwrap.mockResolvedValue(undefined);
  });

  it("renders the list of systems", () => {
    renderPanel();
    expect(screen.getByText("County PD")).toBeInTheDocument();
    expect(screen.getByText("Fire EMS")).toBeInTheDocument();
  });

  it("opens the create system dialog", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: /add system/i }));
    expect(
      screen.getByRole("heading", { name: /create system/i, hidden: true }),
    ).toBeInTheDocument();
  });

  it("submits a create with systemId + label", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: /add system/i }));

    const dialog = document.querySelector(".modal-open") as HTMLElement;
    const scope = within(dialog);
    await user.type(scope.getByLabelText(/system id/i), "7");
    await user.type(scope.getByLabelText(/^label/i), "New Sys");

    await user.click(
      scope.getByRole("button", { name: /^create$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(createSystemMutate).toHaveBeenCalledTimes(1);
    });
    const arg = createSystemMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(arg.systemId).toBe(7);
    expect(arg.label).toBe("New Sys");
    expect(arg.autoPopulateTalkgroups).toBe(1);
  });

  it("submits an edit via updateSystem when saving", async () => {
    const user = userEvent.setup();
    renderPanel();

    const editButtons = screen.getAllByRole("button", { name: /edit system/i });
    await user.click(editButtons[0]);
    expect(
      screen.getByRole("heading", { name: /edit system/i, hidden: true }),
    ).toBeInTheDocument();

    const dialog = document.querySelector(".modal-open") as HTMLElement;
    const scope = within(dialog);
    const labelInput = scope.getByLabelText(/^label/i);
    await user.clear(labelInput);
    await user.type(labelInput, "County PD Renamed");

    await user.click(
      scope.getByRole("button", { name: /^save$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(updateSystemMutate).toHaveBeenCalledTimes(1);
    });
    const arg = updateSystemMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(arg.id).toBe(10);
    expect(arg.label).toBe("County PD Renamed");
  });

  it("prompts for confirm before deleting a system", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    renderPanel();

    const deleteButtons = screen.getAllByRole("button", {
      name: /delete system/i,
    });
    await user.click(deleteButtons[0]);

    expect(confirmSpy).toHaveBeenCalledWith(
      expect.stringContaining('Delete system "County PD"'),
    );
    await waitFor(() => {
      expect(deleteSystemMutate).toHaveBeenCalledWith(10);
    });
    confirmSpy.mockRestore();
  });

  it("does not delete when confirm is declined", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    renderPanel();

    const deleteButtons = screen.getAllByRole("button", {
      name: /delete system/i,
    });
    await user.click(deleteButtons[0]);
    expect(deleteSystemMutate).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });
});
