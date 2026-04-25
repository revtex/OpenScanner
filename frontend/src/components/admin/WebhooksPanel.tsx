import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useListWebhooksQuery,
  useCreateWebhookMutation,
  useUpdateWebhookMutation,
  useDeleteWebhookMutation,
} from "@/hooks/admin/useAdminWsOps";
import type { AdminWebhook } from "@/types";

interface WebhookFormState {
  url: string;
  type: string;
  secret: string;
  systemsJson: string;
  disabled: number;
}

const emptyForm: WebhookFormState = {
  url: "",
  type: "generic",
  secret: "",
  systemsJson: "",
  disabled: 0,
};

export default function WebhooksPanel() {
  const { data: webhooks, isLoading } = useListWebhooksQuery();
  const [createWebhook] = useCreateWebhookMutation();
  const [updateWebhook] = useUpdateWebhookMutation();
  const [deleteWebhook] = useDeleteWebhookMutation();

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<WebhookFormState>(emptyForm);
  const [toast, setToast] = useState<string | null>(null);

  const showError = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  }, []);

  const sorted = useMemo(
    () => (webhooks ? [...webhooks].sort((a, b) => a.order - b.order) : []),
    [webhooks],
  );

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setModalOpen(true);
  };

  const openEdit = (w: AdminWebhook) => {
    setEditingId(w.id);
    setForm({
      url: w.url,
      type: w.type,
      secret: w.secret ?? "",
      systemsJson: w.systemsJson ?? "",
      disabled: w.disabled,
    });
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload = {
      url: form.url,
      type: form.type,
      secret: form.secret || null,
      systemsJson: form.systemsJson || null,
      disabled: form.disabled,
    };
    try {
      if (editingId != null) {
        await updateWebhook({ id: editingId, ...payload }).unwrap();
      } else {
        await createWebhook({ ...payload, order: sorted.length }).unwrap();
      }
      setModalOpen(false);
    } catch {
      showError(
        editingId ? "Failed to update webhook" : "Failed to create webhook",
      );
    }
  };

  const handleDelete = async (w: AdminWebhook) => {
    if (!window.confirm(`Delete webhook "${w.url}"?`)) return;
    try {
      await deleteWebhook(w.id).unwrap();
    } catch {
      showError("Failed to delete webhook");
    }
  };

  const handleToggleDisabled = async (w: AdminWebhook) => {
    try {
      await updateWebhook({
        id: w.id,
        url: w.url,
        type: w.type,
        secret: w.secret,
        systemsJson: w.systemsJson,
        disabled: w.disabled ? 0 : 1,
        order: w.order,
      }).unwrap();
    } catch {
      showError("Failed to update webhook");
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
      <h1 className="text-xl font-semibold mb-4">Webhooks</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Send HTTP notifications to external services when new calls are
        ingested. Each webhook specifies a URL and an optional HMAC secret for
        payload verification. Use webhooks to integrate with alerting, logging,
        or automation systems.
      </p>
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="flex justify-end mb-2">
            <button className="btn btn-primary btn-sm" onClick={openCreate}>
              <Plus className="w-4 h-4" /> Add Webhook
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="table table-zebra w-full">
              <thead>
                <tr>
                  <th>URL</th>
                  <th>Type</th>
                  <th>Secret</th>
                  <th>Systems</th>
                  <th>Disabled</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((w) => (
                  <tr key={w.id}>
                    <td className="font-mono text-sm">{w.url}</td>
                    <td>
                      <span
                        className={
                          w.type === "discord"
                            ? "badge badge-secondary"
                            : "badge badge-primary"
                        }
                      >
                        {w.type}
                      </span>
                    </td>
                    <td className="font-mono text-sm">
                      {w.secret ? "••••••" : "—"}
                    </td>
                    <td>{systemsList(w.systemsJson)}</td>
                    <td>
                      <input
                        type="checkbox"
                        className="toggle toggle-primary toggle-sm"
                        checked={w.disabled === 1}
                        onChange={() => handleToggleDisabled(w)}
                      />
                    </td>
                    <td className="flex gap-1">
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => openEdit(w)}
                        aria-label="Edit webhook"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => handleDelete(w)}
                        aria-label="Delete webhook"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
                {sorted.length === 0 && (
                  <tr>
                    <td colSpan={6} className="text-center opacity-60">
                      No webhooks configured
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
            {editingId != null ? "Edit Webhook" : "Create Webhook"}
          </h3>
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <label className="flex flex-col w-full">
              <span className="text-sm">URL</span>
              <input
                type="url"
                className="input w-full"
                value={form.url}
                onChange={(e) => setForm({ ...form, url: e.target.value })}
                required
              />
            </label>
            <label className="flex flex-col w-full">
              <span className="text-sm">Type</span>
              <select
                className="select w-full"
                value={form.type}
                onChange={(e) => setForm({ ...form, type: e.target.value })}
              >
                <option value="generic">generic</option>
                <option value="discord">discord</option>
              </select>
            </label>
            <label className="flex flex-col w-full">
              <span className="text-sm">Secret (optional)</span>
              <input
                type="password"
                className="input w-full"
                placeholder={
                  editingId != null ? "Leave blank to keep current" : ""
                }
                value={form.secret}
                onChange={(e) => setForm({ ...form, secret: e.target.value })}
              />
            </label>
            <label className="flex flex-col w-full">
              <span className="text-sm">Systems JSON (optional)</span>
              <textarea
                className="textarea w-full"
                value={form.systemsJson}
                onChange={(e) =>
                  setForm({ ...form, systemsJson: e.target.value })
                }
                placeholder="e.g. [1, 2]"
                rows={2}
              />
            </label>
            <div className="flex flex-col">
              <label className="flex items-center cursor-pointer justify-start gap-3">
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={form.disabled === 1}
                  onChange={(e) =>
                    setForm({ ...form, disabled: e.target.checked ? 1 : 0 })
                  }
                />
                <span className="text-sm">Disabled</span>
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
          <button onClick={() => setModalOpen(false)}>close</button>
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
