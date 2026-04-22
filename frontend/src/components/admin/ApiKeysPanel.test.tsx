import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { configureStore } from "@reduxjs/toolkit";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";
import ApiKeysPanel from "@/components/admin/ApiKeysPanel";
import { scannerSlice } from "@/app/slices/scannerSlice";
import { authSlice } from "@/app/slices/authSlice";
import { callsSlice } from "@/app/slices/callsSlice";
import { api } from "@/app/api";
import type { AdminApiKey, AdminSystem } from "@/types";

// ── Mocks ────────────────────────────────────────────────────────────────

const mockKeys: AdminApiKey[] = [
  {
    id: 100,
    fingerprint: "ab12cd34",
    ident: "trunk-north",
    disabled: 0,
    systemsJson: null,
    callRateLimit: null,
    order: 0,
  },
  {
    id: 101,
    fingerprint: "ef56ab78",
    ident: null,
    disabled: 0,
    systemsJson: null,
    callRateLimit: 120,
    order: 1,
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

const createApiKeyUnwrap = vi.fn();
const updateApiKeyUnwrap = vi.fn();
const deleteApiKeyUnwrap = vi.fn();

const createApiKeyMutate = vi.fn((_arg: unknown) => ({ unwrap: createApiKeyUnwrap }));
const updateApiKeyMutate = vi.fn((_arg: unknown) => ({ unwrap: updateApiKeyUnwrap }));
const deleteApiKeyMutate = vi.fn((_arg: unknown) => ({ unwrap: deleteApiKeyUnwrap }));

vi.mock("@/hooks/useAdminWsOps", () => ({
  useListApiKeysQuery: () => ({ data: mockKeys, isLoading: false }),
  useListSystemsQuery: () => ({ data: mockSystems, isLoading: false }),
  useGetConfigQuery: () => ({ data: { settings: [] }, isLoading: false }),
  useCreateApiKeyMutation: () => [createApiKeyMutate, {}],
  useUpdateApiKeyMutation: () => [updateApiKeyMutate, {}],
  useDeleteApiKeyMutation: () => [deleteApiKeyMutate, {}],
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
        <ApiKeysPanel />
      </MemoryRouter>
    </Provider>,
  );
}

describe("ApiKeysPanel", () => {
  beforeEach(() => {
    createApiKeyMutate.mockClear();
    updateApiKeyMutate.mockClear();
    deleteApiKeyMutate.mockClear();
    createApiKeyUnwrap.mockReset();
    updateApiKeyUnwrap.mockReset();
    deleteApiKeyUnwrap.mockReset();
    createApiKeyUnwrap.mockResolvedValue({
      id: 999,
      fingerprint: "new123",
      ident: "newkey",
      disabled: 0,
      systemsJson: null,
      callRateLimit: null,
      order: 2,
      createdKey: "plaintext-secret-key-xyz",
    });
    updateApiKeyUnwrap.mockResolvedValue(undefined);
    deleteApiKeyUnwrap.mockResolvedValue(undefined);
  });

  it("lists keys showing only fingerprints (not plaintext)", () => {
    renderPanel();
    expect(screen.getByText("ab12cd34")).toBeInTheDocument();
    expect(screen.getByText("ef56ab78")).toBeInTheDocument();
    // No plaintext key should appear on the list page.
    expect(screen.queryByText(/plaintext-secret-key/i)).toBeNull();
  });

  it("reveals the created plaintext key once in the reveal-once dialog", async () => {
    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: /add api key/i }));
    await user.click(
      screen.getByRole("button", { name: /^create$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(screen.getByText("API Key Created")).toBeInTheDocument();
    });
    // Plaintext present in the readonly input
    const revealInput = screen.getByDisplayValue("plaintext-secret-key-xyz");
    expect(revealInput).toBeInTheDocument();
    expect(revealInput).toHaveAttribute("readonly");
  });

  it("hides the plaintext key after closing the reveal-once dialog", async () => {
    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: /add api key/i }));
    await user.click(
      screen.getByRole("button", { name: /^create$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(screen.getByText("API Key Created")).toBeInTheDocument();
    });
    await user.click(
      screen.getByRole("button", { name: /^done$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(screen.queryByDisplayValue("plaintext-secret-key-xyz")).toBeNull();
    });
  });

  it("submits create with the expected payload", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: /add api key/i }));

    await user.type(
      screen.getByPlaceholderText(/optional description/i),
      "my-key",
    );
    await user.click(
      screen.getByRole("button", { name: /^create$/i, hidden: true }),
    );

    await waitFor(() => {
      expect(createApiKeyMutate).toHaveBeenCalledTimes(1);
    });
    const arg = createApiKeyMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(arg.ident).toBe("my-key");
    expect(arg.disabled).toBe(0);
    expect(arg.systemsJson).toBeNull();
  });

  it("calls deleteApiKey with the key id on confirm", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    renderPanel();

    const deleteButtons = screen.getAllByRole("button", {
      name: /delete api key/i,
    });
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(deleteApiKeyMutate).toHaveBeenCalledWith(100);
    });
    confirmSpy.mockRestore();
  });

  it("toggles disabled via updateApiKey", async () => {
    const user = userEvent.setup();
    renderPanel();

    // Find the first disabled toggle in the table body (checkbox) and flip it.
    const toggles = screen.getAllByRole("checkbox");
    await user.click(toggles[0]);

    await waitFor(() => {
      expect(updateApiKeyMutate).toHaveBeenCalledTimes(1);
    });
    const arg = updateApiKeyMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(arg.id).toBe(100);
    expect(arg.disabled).toBe(1);
  });
});
