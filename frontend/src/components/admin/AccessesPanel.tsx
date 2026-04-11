import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useListAccessesQuery,
  useCreateAccessMutation,
  useUpdateAccessMutation,
  useDeleteAccessMutation,
} from "@/app/slices/adminSlice";
import type { AdminAccess } from "@/types";

interface AccessFormState {
  code: string;
  ident: string;
  expiration: string;
  limit: string;
  systemsJson: string;
}

const emptyForm: AccessFormState = {
  code: "",
  ident: "",
  expiration: "",
  limit: "",
  systemsJson: "",
};

function formatExpiration(ts: number | null): string {
  if (ts == null) return "—";
  return new Date(ts * 1000).toLocaleString();
}

function tsToDatetimeLocal(ts: number | null): string {
  if (ts == null) return "";
  const d = new Date(ts * 1000);
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export default function AccessesPanel() {
  const { data: accesses, isLoading } = useListAccessesQuery();
  const [createAccess] = useCreateAccessMutation();
  const [updateAccess] = useUpdateAccessMutation();
  const [deleteAccess] = useDeleteAccessMutation();

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<AccessFormState>(emptyForm);
  const [toast, setToast] = useState<string | null>(null);

  const showError = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  }, []);

  const sorted = useMemo(
    () => (accesses ? [...accesses].sort((a, b) => a.order - b.order) : []),
    [accesses],
  );

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setModalOpen(true);
  };

  const openEdit = (a: AdminAccess) => {
    setEditingId(a.id);
    setForm({
      code: a.code,
      ident: a.ident ?? "",
      expiration: tsToDatetimeLocal(a.expiration),
      limit: a.limit != null ? String(a.limit) : "",
      systemsJson: a.systemsJson ?? "",
    });
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload = {
      code: form.code,
      ident: form.ident || null,
      expiration: form.expiration
        ? Math.floor(new Date(form.expiration).getTime() / 1000)
        : null,
      limit: form.limit ? Number(form.limit) : null,
      systemsJson: form.systemsJson || null,
    };
    try {
      if (editingId != null) {
        await updateAccess({ id: editingId, ...payload }).unwrap();
      } else {
        await createAccess({ ...payload, order: sorted.length }).unwrap();
      }
      setModalOpen(false);
    } catch {
      showError(
        editingId ? "Failed to update access" : "Failed to create access",
      );
    }
  };

  const handleDelete = async (a: AdminAccess) => {
    if (!window.confirm(`Delete access "${a.ident || a.code}"?`)) return;
    try {
      await deleteAccess(a.id).unwrap();
    } catch {
      showError("Failed to delete access");
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
      <h1 className="text-xl font-semibold mb-4">Access Codes</h1>
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="flex justify-end mb-2">
            <button className="btn btn-primary btn-sm" onClick={openCreate}>
              <Plus className="w-4 h-4" /> Add Access
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="table table-zebra w-full">
              <thead>
                <tr>
                  <th>Code</th>
                  <th>Ident</th>
                  <th>Expiration</th>
                  <th>Limit</th>
                  <th>Systems</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((a) => (
                  <tr key={a.id}>
                    <td className="font-mono text-sm">{a.code}</td>
                    <td>{a.ident ?? "—"}</td>
                    <td>{formatExpiration(a.expiration)}</td>
                    <td>{a.limit != null ? a.limit : "—"}</td>
                    <td>{systemsList(a.systemsJson)}</td>
                    <td className="flex gap-1">
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => openEdit(a)}
                        aria-label="Edit access"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => handleDelete(a)}
                        aria-label="Delete access"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
                {sorted.length === 0 && (
                  <tr>
                    <td colSpan={6} className="text-center opacity-60">
                      No access codes configured
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
            {editingId != null ? "Edit Access" : "Create Access"}
          </h3>
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Code</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.code}
                onChange={(e) => setForm({ ...form, code: e.target.value })}
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Ident (optional)</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.ident}
                onChange={(e) => setForm({ ...form, ident: e.target.value })}
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Expiration (optional)</span>
              </div>
              <input
                type="datetime-local"
                className="input input-bordered w-full"
                value={form.expiration}
                onChange={(e) =>
                  setForm({ ...form, expiration: e.target.value })
                }
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Limit (optional)</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={form.limit}
                onChange={(e) => setForm({ ...form, limit: e.target.value })}
                min={0}
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
