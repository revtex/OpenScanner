import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useListDirwatchesQuery,
  useCreateDirwatchMutation,
  useUpdateDirwatchMutation,
  useDeleteDirwatchMutation,
} from "@/app/slices/adminSlice";
import type { AdminDirwatch } from "@/types";

const DIRWATCH_TYPES = ["trunk-recorder", "sdr-trunk", "rdio-scanner"] as const;

interface DirwatchFormState {
  directory: string;
  type: string;
  mask: string;
  extension: string;
  frequency: string;
  delay: string;
  deleteAfter: number;
  usePolling: number;
  disabled: number;
  systemId: string;
  talkgroupId: string;
}

const emptyForm: DirwatchFormState = {
  directory: "",
  type: "trunk-recorder",
  mask: "",
  extension: "",
  frequency: "",
  delay: "",
  deleteAfter: 0,
  usePolling: 0,
  disabled: 0,
  systemId: "",
  talkgroupId: "",
};

export default function DirWatchPanel() {
  const { data: dirwatches, isLoading } = useListDirwatchesQuery();
  const [createDirwatch] = useCreateDirwatchMutation();
  const [updateDirwatch] = useUpdateDirwatchMutation();
  const [deleteDirwatch] = useDeleteDirwatchMutation();

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<DirwatchFormState>(emptyForm);
  const [toast, setToast] = useState<string | null>(null);

  const showError = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  }, []);

  const sorted = useMemo(
    () => (dirwatches ? [...dirwatches].sort((a, b) => a.order - b.order) : []),
    [dirwatches],
  );

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setModalOpen(true);
  };

  const openEdit = (d: AdminDirwatch) => {
    setEditingId(d.id);
    setForm({
      directory: d.directory,
      type: d.type,
      mask: d.mask ?? "",
      extension: d.extension ?? "",
      frequency: d.frequency != null ? String(d.frequency) : "",
      delay: d.delay != null ? String(d.delay) : "",
      deleteAfter: d.deleteAfter,
      usePolling: d.usePolling,
      disabled: d.disabled,
      systemId: d.systemId != null ? String(d.systemId) : "",
      talkgroupId: d.talkgroupId != null ? String(d.talkgroupId) : "",
    });
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload = {
      directory: form.directory,
      type: form.type,
      mask: form.mask || null,
      extension: form.extension || null,
      frequency: form.frequency ? Number(form.frequency) : null,
      delay: form.delay ? Number(form.delay) : null,
      deleteAfter: form.deleteAfter,
      usePolling: form.usePolling,
      disabled: form.disabled,
      systemId: form.systemId ? Number(form.systemId) : null,
      talkgroupId: form.talkgroupId ? Number(form.talkgroupId) : null,
    };
    try {
      if (editingId != null) {
        await updateDirwatch({ id: editingId, ...payload }).unwrap();
      } else {
        await createDirwatch({ ...payload, order: sorted.length }).unwrap();
      }
      setModalOpen(false);
    } catch {
      showError(
        editingId
          ? "Failed to update directory watch"
          : "Failed to create directory watch",
      );
    }
  };

  const handleDelete = async (d: AdminDirwatch) => {
    if (!window.confirm(`Delete directory watch "${d.directory}"?`)) return;
    try {
      await deleteDirwatch(d.id).unwrap();
    } catch {
      showError("Failed to delete directory watch");
    }
  };

  const handleToggleDisabled = async (d: AdminDirwatch) => {
    try {
      await updateDirwatch({
        id: d.id,
        disabled: d.disabled ? 0 : 1,
      }).unwrap();
    } catch {
      showError("Failed to update directory watch");
    }
  };

  if (isLoading) return <div className="loading loading-spinner loading-md" />;

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Directory Watches</h1>
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="flex justify-end mb-2">
            <button className="btn btn-primary btn-sm" onClick={openCreate}>
              <Plus className="w-4 h-4" /> Add Directory Watch
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="table table-zebra w-full">
              <thead>
                <tr>
                  <th>Directory</th>
                  <th>Type</th>
                  <th>Mask</th>
                  <th>Extension</th>
                  <th>Delay</th>
                  <th>Delete After</th>
                  <th>Disabled</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((d) => (
                  <tr key={d.id}>
                    <td className="font-mono text-sm">{d.directory}</td>
                    <td>{d.type}</td>
                    <td>{d.mask ?? "—"}</td>
                    <td>{d.extension ?? "—"}</td>
                    <td>{d.delay != null ? `${d.delay}s` : "—"}</td>
                    <td>{d.deleteAfter ? "Yes" : "No"}</td>
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
                        aria-label="Edit directory watch"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => handleDelete(d)}
                        aria-label="Delete directory watch"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
                {sorted.length === 0 && (
                  <tr>
                    <td colSpan={8} className="text-center opacity-60">
                      No directory watches configured
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
            {editingId != null
              ? "Edit Directory Watch"
              : "Create Directory Watch"}
          </h3>
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Directory</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.directory}
                onChange={(e) =>
                  setForm({ ...form, directory: e.target.value })
                }
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Type</span>
              </div>
              <select
                className="select select-bordered w-full"
                value={form.type}
                onChange={(e) => setForm({ ...form, type: e.target.value })}
              >
                {DIRWATCH_TYPES.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Mask (optional)</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.mask}
                onChange={(e) => setForm({ ...form, mask: e.target.value })}
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Extension (optional)</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.extension}
                onChange={(e) =>
                  setForm({ ...form, extension: e.target.value })
                }
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Frequency (optional)</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={form.frequency}
                onChange={(e) =>
                  setForm({ ...form, frequency: e.target.value })
                }
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Delay in seconds (optional)</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={form.delay}
                onChange={(e) => setForm({ ...form, delay: e.target.value })}
                min={0}
              />
            </label>
            <div className="form-control">
              <label className="label cursor-pointer justify-start gap-3">
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={form.deleteAfter === 1}
                  onChange={(e) =>
                    setForm({ ...form, deleteAfter: e.target.checked ? 1 : 0 })
                  }
                />
                <span className="label-text">Delete after import</span>
              </label>
            </div>
            <div className="form-control">
              <label className="label cursor-pointer justify-start gap-3">
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={form.usePolling === 1}
                  onChange={(e) =>
                    setForm({ ...form, usePolling: e.target.checked ? 1 : 0 })
                  }
                />
                <span className="label-text">Use polling</span>
              </label>
            </div>
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
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">System ID (optional)</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={form.systemId}
                onChange={(e) => setForm({ ...form, systemId: e.target.value })}
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Talkgroup ID (optional)</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={form.talkgroupId}
                onChange={(e) =>
                  setForm({ ...form, talkgroupId: e.target.value })
                }
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
