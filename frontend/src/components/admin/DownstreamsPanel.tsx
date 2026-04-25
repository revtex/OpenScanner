import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useListDownstreamsQuery,
  useCreateDownstreamMutation,
  useUpdateDownstreamMutation,
  useDeleteDownstreamMutation,
  useListSystemsQuery,
} from "@/hooks/admin/useAdminWsOps";
import type { AdminDownstream } from "@/types";

interface DownstreamFormState {
  url: string;
  apiKey: string;
  systemsJson: string;
  disabled: number;
}

const emptyForm: DownstreamFormState = {
  url: "",
  apiKey: "",
  systemsJson: "",
  disabled: 0,
};

export default function DownstreamsPanel() {
  const { data: downstreams, isLoading } = useListDownstreamsQuery();
  const { data: systems } = useListSystemsQuery();
  const [createDownstream] = useCreateDownstreamMutation();
  const [updateDownstream] = useUpdateDownstreamMutation();
  const [deleteDownstream] = useDeleteDownstreamMutation();

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<DownstreamFormState>(emptyForm);
  const [toast, setToast] = useState<string | null>(null);

  const showError = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  }, []);

  const sorted = useMemo(
    () =>
      downstreams ? [...downstreams].sort((a, b) => a.order - b.order) : [],
    [downstreams],
  );

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setModalOpen(true);
  };

  const openEdit = (d: AdminDownstream) => {
    setEditingId(d.id);
    setForm({
      url: d.url,
      apiKey: "",
      systemsJson: d.systemsJson ?? "",
      disabled: d.disabled,
    });
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload = {
      url: form.url,
      apiKey: form.apiKey,
      systemsJson: form.systemsJson || null,
      disabled: form.disabled,
    };
    try {
      if (editingId != null) {
        const existing = sorted.find((d) => d.id === editingId);
        await updateDownstream({
          id: editingId,
          order: existing?.order ?? 0,
          ...payload,
        }).unwrap();
      } else {
        await createDownstream({ ...payload, order: sorted.length }).unwrap();
      }
      setModalOpen(false);
    } catch {
      showError(
        editingId
          ? "Failed to update downstream"
          : "Failed to create downstream",
      );
    }
  };

  const handleDelete = async (d: AdminDownstream) => {
    if (!window.confirm(`Delete downstream "${d.url}"?`)) return;
    try {
      await deleteDownstream(d.id).unwrap();
    } catch {
      showError("Failed to delete downstream");
    }
  };

  const handleToggleDisabled = async (d: AdminDownstream) => {
    try {
      await updateDownstream({
        id: d.id,
        url: d.url,
        apiKey: "",
        systemsJson: d.systemsJson,
        disabled: d.disabled ? 0 : 1,
        order: d.order,
      }).unwrap();
    } catch {
      showError("Failed to update downstream");
    }
  };

  const systemsList = (json: string | null) => {
    if (!json) return "All";
    try {
      const ids = JSON.parse(json) as number[];
      if (!systems || systems.length === 0) return ids.join(", ");
      return ids
        .map((id) => systems.find((s) => s.id === id)?.label ?? String(id))
        .join(", ");
    } catch {
      return json;
    }
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
    setForm({
      ...form,
      systemsJson: updated.length > 0 ? JSON.stringify(updated) : "",
    });
  };

  if (isLoading) return <div className="loading loading-spinner loading-md" />;

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Downstreams</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Forward ingested calls to other OpenScanner instances. Each downstream
        specifies the target server URL, API key, and which systems to forward.
        Use this to chain multiple servers or distribute calls across sites.
      </p>
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="flex justify-end mb-2">
            <button className="btn btn-primary btn-sm" onClick={openCreate}>
              <Plus className="w-4 h-4" /> Add Downstream
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="table table-zebra w-full">
              <thead>
                <tr>
                  <th>URL</th>
                  <th>API Key</th>
                  <th>Systems</th>
                  <th>Disabled</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((d) => (
                  <tr key={d.id}>
                    <td className="font-mono text-sm">{d.url}</td>
                    <td>
                      <span className="text-base-content/50">
                        {d.hasApiKey ? "••••••••••••" : "Not set"}
                      </span>
                    </td>
                    <td>{systemsList(d.systemsJson)}</td>
                    <td>
                      <input
                        type="checkbox"
                        className="toggle toggle-primary toggle-sm"
                        checked={d.disabled === 1}
                        onChange={() => handleToggleDisabled(d)}
                      />
                    </td>
                    <td className="flex gap-1">
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => openEdit(d)}
                        aria-label="Edit downstream"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => handleDelete(d)}
                        aria-label="Delete downstream"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
                {sorted.length === 0 && (
                  <tr>
                    <td colSpan={5} className="text-center opacity-60">
                      No downstreams configured
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Create / Edit modal */}
      <dialog className={`modal ${modalOpen ? "modal-open" : ""}`}>
        <div className="modal-box">
          <h3 className="font-bold text-lg mb-4">
            {editingId != null ? "Edit Downstream" : "Create Downstream"}
          </h3>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1">
              <span className="text-sm font-medium">URL</span>
              <input
                type="url"
                className="input w-full"
                value={form.url}
                onChange={(e) => setForm({ ...form, url: e.target.value })}
                placeholder="https://remote-server/api/call-upload"
                required
              />
              <span className="text-xs text-base-content/60">
                The call-upload endpoint of the remote OpenScanner instance.
              </span>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-sm font-medium">API Key</span>
              <input
                type="password"
                className="input w-full font-mono"
                value={form.apiKey}
                onChange={(e) => setForm({ ...form, apiKey: e.target.value })}
                placeholder={
                  editingId != null ? "Leave blank to keep current key" : ""
                }
                required={editingId == null}
                autoComplete="off"
              />
              <span className="text-xs text-base-content/60">
                {editingId != null
                  ? "Enter a new key to replace the existing one, or leave blank to keep it."
                  : "The API key configured on the remote server for authentication."}
              </span>
            </div>

            {systems && systems.length > 0 && (
              <div className="flex flex-col gap-1">
                <span className="text-sm font-medium">Systems</span>
                <span className="text-xs text-base-content/60">
                  Select which systems to forward. If none are selected, all
                  systems are forwarded.
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

            <div className="flex items-center gap-3">
              <input
                type="checkbox"
                className="toggle toggle-primary"
                checked={form.disabled === 1}
                onChange={(e) =>
                  setForm({ ...form, disabled: e.target.checked ? 1 : 0 })
                }
              />
              <span className="text-sm font-medium">Disabled</span>
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
