import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus, Copy, Check } from "lucide-react";
import {
  useListApiKeysQuery,
  useCreateApiKeyMutation,
  useUpdateApiKeyMutation,
  useDeleteApiKeyMutation,
  useListSystemsQuery,
  useGetConfigQuery,
} from "@/hooks/useAdminWsOps";
import type { AdminApiKey } from "@/types";

// ─── Form state ───

interface ApiKeyFormState {
  ident: string;
  disabled: number;
  systemsJson: string;
  callRateLimit: string;
}

const emptyForm: ApiKeyFormState = {
  ident: "",
  disabled: 0,
  systemsJson: "",
  callRateLimit: "",
};

// ─── Copy button ───

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      className="btn btn-ghost btn-xs"
      onClick={handleCopy}
      aria-label="Copy key"
    >
      {copied ? (
        <Check className="w-3 h-3 text-success" />
      ) : (
        <Copy className="w-3 h-3" />
      )}
    </button>
  );
}

// ─── Main panel ───

export default function ApiKeysPanel() {
  const { data: apiKeys, isLoading } = useListApiKeysQuery();
  const { data: systems } = useListSystemsQuery();
  const { data: config } = useGetConfigQuery();
  const [createApiKey] = useCreateApiKeyMutation();
  const [updateApiKey] = useUpdateApiKeyMutation();
  const [deleteApiKey] = useDeleteApiKeyMutation();

  const globalRateLimit = useMemo(() => {
    const val = config?.settings?.find(
      (s) => s.key === "apiKeyCallRate",
    )?.value;
    if (val) {
      const n = Number(val);
      if (n > 0) return n;
    }
    return 60; // hardcoded server default
  }, [config]);

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<ApiKeyFormState>(emptyForm);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  const showError = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  }, []);

  const sortedKeys = useMemo(
    () => (apiKeys ? [...apiKeys].sort((a, b) => a.order - b.order) : []),
    [apiKeys],
  );

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setModalOpen(true);
  };

  const openEdit = (ak: AdminApiKey) => {
    setEditingId(ak.id);
    setForm({
      ident: ak.ident ?? "",
      disabled: ak.disabled,
      systemsJson: ak.systemsJson ?? "",
      callRateLimit:
        ak.callRateLimit != null && ak.callRateLimit > 0
          ? String(ak.callRateLimit)
          : "",
    });
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingId != null) {
        await updateApiKey({
          id: editingId,
          ident: form.ident || null,
          disabled: form.disabled,
          systemsJson: form.systemsJson || null,
          callRateLimit: form.callRateLimit ? Number(form.callRateLimit) : null,
          order: sortedKeys.find((k) => k.id === editingId)?.order ?? 0,
        }).unwrap();
      } else {
        const created = await createApiKey({
          ident: form.ident || null,
          disabled: form.disabled,
          systemsJson: form.systemsJson || null,
          callRateLimit: form.callRateLimit ? Number(form.callRateLimit) : null,
          order: sortedKeys.length,
        }).unwrap();
        setCreatedKey(created.createdKey);
      }
      setModalOpen(false);
    } catch {
      showError(
        editingId ? "Failed to update API key" : "Failed to create API key",
      );
    }
  };

  const handleDelete = async (ak: AdminApiKey) => {
    if (!window.confirm(`Delete API key "${ak.ident || ak.fingerprint}"?`))
      return;
    try {
      await deleteApiKey(ak.id).unwrap();
    } catch {
      showError("Failed to delete API key");
    }
  };

  const handleToggleDisabled = async (ak: AdminApiKey) => {
    try {
      await updateApiKey({
        id: ak.id,
        ident: ak.ident,
        disabled: ak.disabled ? 0 : 1,
        systemsJson: ak.systemsJson,
        callRateLimit: ak.callRateLimit,
        order: ak.order,
      }).unwrap();
    } catch {
      showError("Failed to update API key");
    }
  };

  const updateField = <K extends keyof ApiKeyFormState>(
    key: K,
    value: ApiKeyFormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  // Parse selected systems for checkbox UI
  const selectedSystems: number[] = form.systemsJson
    ? (() => {
        try {
          return JSON.parse(form.systemsJson) as number[];
        } catch {
          return [];
        }
      })()
    : [];

  const toggleSystem = (systemId: number) => {
    const updated = selectedSystems.includes(systemId)
      ? selectedSystems.filter((id) => id !== systemId)
      : [...selectedSystems, systemId];
    updateField(
      "systemsJson",
      updated.length > 0 ? JSON.stringify(updated) : "",
    );
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner loading-lg" />
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">API Keys</h1>
      <p className="text-sm text-base-content/70 mb-4">
        API keys authenticate external sources (e.g. trunk-recorder) that upload
        calls. Each key can be restricted to specific systems and optionally
        rate-limited. Provide the key in the X-API-Key header.
      </p>
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="overflow-x-auto">
            <table className="table table-zebra w-full">
              <thead>
                <tr>
                  <th>Fingerprint</th>
                  <th>Ident</th>
                  <th>Rate Limit</th>
                  <th>Disabled</th>
                  <th>Systems</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sortedKeys.map((ak) => {
                  const systemsList = ak.systemsJson
                    ? (() => {
                        try {
                          return (JSON.parse(ak.systemsJson) as number[]).join(
                            ", ",
                          );
                        } catch {
                          return ak.systemsJson;
                        }
                      })()
                    : "All";
                  return (
                    <tr key={ak.id}>
                      <td className="font-mono text-sm">{ak.fingerprint}</td>
                      <td>{ak.ident ?? "—"}</td>
                      <td>
                        {ak.callRateLimit != null
                          ? `${ak.callRateLimit}/min`
                          : "Default"}
                      </td>
                      <td>
                        <input
                          type="checkbox"
                          className="toggle toggle-primary toggle-sm"
                          checked={ak.disabled === 1}
                          onChange={() => handleToggleDisabled(ak)}
                        />
                      </td>
                      <td>{systemsList}</td>
                      <td className="flex gap-1">
                        <button
                          className="btn btn-ghost btn-xs"
                          onClick={() => openEdit(ak)}
                          aria-label="Edit API key"
                        >
                          <Pencil className="w-4 h-4" />
                        </button>
                        <button
                          className="btn btn-ghost btn-xs"
                          onClick={() => handleDelete(ak)}
                          aria-label="Delete API key"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </td>
                    </tr>
                  );
                })}
                {sortedKeys.length === 0 && (
                  <tr>
                    <td colSpan={6} className="text-center opacity-60">
                      No API keys yet
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-4">
            <button className="btn btn-primary btn-sm" onClick={openCreate}>
              <Plus className="w-4 h-4" /> Add API Key
            </button>
          </div>
        </div>
      </div>

      {/* Modal */}
      <dialog className={`modal ${modalOpen ? "modal-open" : ""}`}>
        <div className="modal-box">
          <h3 className="font-bold text-lg">
            {editingId != null ? "Edit API Key" : "Create API Key"}
          </h3>
          <form onSubmit={handleSubmit} className="mt-4 space-y-4">
            <div className="flex flex-col gap-1">
              <span className="text-sm font-medium">Identifier</span>
              <input
                type="text"
                className="input w-full"
                value={form.ident}
                onChange={(e) => updateField("ident", e.target.value)}
                placeholder="Optional description"
              />
              <span className="text-xs text-base-content/60">
                A label to help you identify this key (e.g. &ldquo;Trunk
                Recorder North Site&rdquo;).
              </span>
            </div>

            {systems && systems.length > 0 && (
              <div className="flex flex-col gap-1">
                <span className="text-sm font-medium">Systems</span>
                <span className="text-xs text-base-content/60">
                  Select which systems this key can upload to. If none are
                  selected, the key has access to all systems.
                </span>
                <div className="flex flex-wrap gap-2 mt-1">
                  {systems.map((sys) => {
                    const selected = selectedSystems.includes(sys.id);
                    return (
                      <button
                        key={sys.id}
                        type="button"
                        className={`badge badge-lg gap-1.5 cursor-pointer transition-colors ${
                          selected
                            ? "badge-primary"
                            : "badge-ghost hover:badge-outline"
                        }`}
                        onClick={() => toggleSystem(sys.id)}
                      >
                        <span
                          className={`inline-block w-2 h-2 rounded-full ${selected ? "bg-primary-content" : "bg-base-content/30"}`}
                        />
                        {sys.label}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            <div className="flex flex-col gap-1">
              <span className="text-sm font-medium">
                Call Rate Limit (per minute)
              </span>
              <input
                type="number"
                min={1}
                max={600}
                className="input w-full"
                value={form.callRateLimit}
                onChange={(e) => updateField("callRateLimit", e.target.value)}
                placeholder={`Global default: ${globalRateLimit}/min`}
              />
              <span className="text-xs text-base-content/60">
                Override the per-key inbound call upload limit. Leave blank to
                use the global default ({globalRateLimit}/min).
              </span>
            </div>

            <div className="modal-action">
              <button
                type="button"
                className="btn"
                onClick={() => setModalOpen(false)}
              >
                Cancel
              </button>
              <button type="submit" className="btn btn-primary">
                {editingId != null ? "Save" : "Create"}
              </button>
            </div>
          </form>
        </div>
        <form method="dialog" className="modal-backdrop">
          <button type="button" onClick={() => setModalOpen(false)}>
            close
          </button>
        </form>
      </dialog>

      <dialog className={`modal ${createdKey ? "modal-open" : ""}`}>
        <div className="modal-box">
          <h3 className="font-bold text-lg">API Key Created</h3>
          <p className="mt-2 text-sm text-base-content/70">
            Copy this key now. For security, it is shown only once and cannot be
            retrieved later.
          </p>
          <div className="mt-4 flex items-center gap-2">
            <input
              type="text"
              className="input w-full font-mono text-sm"
              value={createdKey ?? ""}
              readOnly
            />
            {createdKey && <CopyButton text={createdKey} />}
          </div>
          <div className="modal-action">
            <button
              className="btn btn-primary"
              onClick={() => setCreatedKey(null)}
            >
              Done
            </button>
          </div>
        </div>
      </dialog>

      {toast && (
        <div className="toast toast-end">
          <div className="alert alert-error">
            <span>{toast}</span>
          </div>
        </div>
      )}
    </div>
  );
}
