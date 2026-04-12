import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus, ClipboardCopy, Check } from "lucide-react";
import {
  useListDownstreamsQuery,
  useCreateDownstreamMutation,
  useUpdateDownstreamMutation,
  useDeleteDownstreamMutation,
} from "@/app/slices/adminSlice";
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
      aria-label="Copy API key"
    >
      {copied ? (
        <Check className="w-3 h-3 text-success" />
      ) : (
        <ClipboardCopy className="w-3 h-3" />
      )}
    </button>
  );
}

export default function DownstreamsPanel() {
  const { data: downstreams, isLoading } = useListDownstreamsQuery();
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
      apiKey: d.apiKey,
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
        await updateDownstream({ id: editingId, ...payload }).unwrap();
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
        apiKey: d.apiKey,
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
      return (JSON.parse(json) as number[]).join(", ");
    } catch {
      return json;
    }
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
                      <span className="font-mono text-sm">
                        {d.apiKey.slice(0, 8)}&hellip;
                      </span>
                      <CopyButton text={d.apiKey} />
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
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">URL</span>
              </div>
              <input
                type="url"
                className="input input-bordered w-full"
                value={form.url}
                onChange={(e) => setForm({ ...form, url: e.target.value })}
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">API Key</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.apiKey}
                onChange={(e) => setForm({ ...form, apiKey: e.target.value })}
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Systems JSON (optional)</span>
              </div>
              <textarea
                className="textarea textarea-bordered w-full"
                value={form.systemsJson}
                onChange={(e) =>
                  setForm({ ...form, systemsJson: e.target.value })
                }
                placeholder="e.g. [1, 2]"
                rows={2}
              />
            </label>
            <div className="form-control">
              <label className="label cursor-pointer justify-start gap-3">
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={form.disabled === 1}
                  onChange={(e) =>
                    setForm({ ...form, disabled: e.target.checked ? 1 : 0 })
                  }
                />
                <span className="label-text">Disabled</span>
              </label>
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
