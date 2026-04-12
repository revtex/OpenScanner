import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useLazyListServerDirectoriesQuery,
  useListDirwatchesQuery,
  useCreateDirwatchMutation,
  useUpdateDirwatchMutation,
  useDeleteDirwatchMutation,
  useListSystemsQuery,
  useListTalkgroupsQuery,
} from "@/app/slices/adminSlice";
import type { AdminDirwatch } from "@/types";

const DIRWATCH_TYPES = [
  { value: "default", label: "Default (mask-based)" },
  { value: "dsdplus", label: "DSDPlus Fast Lane" },
  { value: "sdr-trunk", label: "SDR Trunk" },
  { value: "trunk-recorder", label: "Trunk Recorder" },
] as const;

const MASK_HELP = `Extract metadata from the filename using these tokens:
  #DATE — date: 20201231, 2020-12-31, or 2020_12_31
  #TIME — local time: 085430, 08-54-30, or 08:54:30
  #ZTIME — zulu/UTC time
  #SYS — system ID (decimal)
  #SYSLBL — system label
  #TG — talkgroup ID (decimal)
  #TGLBL — talkgroup label
  #TGAFS — talkgroup ID in AFS format (11-061)
  #HZ — frequency in Hz
  #KHZ — frequency in kHz
  #MHZ — frequency in MHz
  #TGHZ / #TGKHZ / #TGMHZ — frequency → talkgroup ID
  #GROUP — group label
  #TAG — tag label
  #UNIT — unit ID
Example: cymx_#TG_#DATE_#TIME_#HZ`;

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
  const { data: systems } = useListSystemsQuery();
  const { data: allTalkgroups } = useListTalkgroupsQuery();
  const [loadServerDirs, { data: serverDirs, isFetching: loadingServerDirs }] =
    useLazyListServerDirectoriesQuery();

  const [modalOpen, setModalOpen] = useState(false);
  const [directoryBrowserOpen, setDirectoryBrowserOpen] = useState(false);
  const [directoryBrowserPath, setDirectoryBrowserPath] = useState("/");
  const [directoryJumpInput, setDirectoryJumpInput] = useState("/");
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

  // Talkgroups indexed by their DB system row ID
  const tgBySystem = useMemo(() => {
    const map = new Map<
      number,
      { id: number; talkgroupId: number; label: string | null }[]
    >();
    if (allTalkgroups) {
      for (const tg of allTalkgroups) {
        const list = map.get(tg.systemId) ?? [];
        list.push(tg);
        map.set(tg.systemId, list);
      }
    }
    return map;
  }, [allTalkgroups]);

  const talkgroupsForSelectedSystem = useMemo(() => {
    if (!form.systemId) return [];
    return tgBySystem.get(Number(form.systemId)) ?? [];
  }, [tgBySystem, form.systemId]);

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
    } catch (err) {
      const fallback =
        editingId != null
          ? "Failed to update directory watch"
          : "Failed to create directory watch";
      const msg =
        typeof err === "object" &&
        err !== null &&
        "data" in err &&
        typeof (err as { data?: unknown }).data === "object" &&
        (err as { data?: { error?: unknown } }).data?.error &&
        typeof (err as { data?: { error?: unknown } }).data?.error === "string"
          ? (err as { data: { error: string } }).data.error
          : fallback;
      showError(msg);
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
        directory: d.directory,
        type: d.type,
        mask: d.mask,
        extension: d.extension,
        frequency: d.frequency,
        delay: d.delay,
        deleteAfter: d.deleteAfter,
        usePolling: d.usePolling,
        disabled: d.disabled ? 0 : 1,
        systemId: d.systemId,
        talkgroupId: d.talkgroupId,
        order: d.order,
      }).unwrap();
    } catch {
      showError("Failed to update directory watch");
    }
  };

  const handlePickDirectory = async () => {
    const startPath = form.directory.startsWith("/") ? form.directory : "/";
    setDirectoryBrowserPath(startPath);
    setDirectoryJumpInput(startPath);
    setDirectoryBrowserOpen(true);
    try {
      await loadServerDirs({ path: startPath }).unwrap();
    } catch (err) {
      const msg =
        typeof err === "object" &&
        err !== null &&
        "data" in err &&
        typeof (err as { data?: unknown }).data === "object" &&
        (err as { data?: { error?: unknown } }).data?.error &&
        typeof (err as { data?: { error?: unknown } }).data?.error === "string"
          ? (err as { data: { error: string } }).data.error
          : "Failed to load directories";
      showError(msg);
    }
  };

  const navigateDirectory = async (path: string) => {
    setDirectoryBrowserPath(path);
    setDirectoryJumpInput(path);
    try {
      await loadServerDirs({ path }).unwrap();
    } catch (err) {
      const msg =
        typeof err === "object" &&
        err !== null &&
        "data" in err &&
        typeof (err as { data?: unknown }).data === "object" &&
        (err as { data?: { error?: unknown } }).data?.error &&
        typeof (err as { data?: { error?: unknown } }).data?.error === "string"
          ? (err as { data: { error: string } }).data.error
          : "Failed to load directories";
      showError(msg);
    }
  };

  if (isLoading) return <div className="loading loading-spinner loading-md" />;

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Directory Watches</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Monitor local directories for new audio files and automatically ingest
        them as calls. Configure the directory path, file mask pattern, and how
        metadata (system, talkgroup, frequency) is extracted from filenames.
      </p>
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
                    <td>
                      {DIRWATCH_TYPES.find((t) => t.value === d.type)?.label ??
                        d.type}
                    </td>
                    <td>{d.mask ?? "—"}</td>
                    <td>{d.extension ?? "—"}</td>
                    <td>{d.delay != null ? `${d.delay} ms` : "—"}</td>
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
        <div className="modal-box max-w-lg">
          <h3 className="font-bold text-lg mb-4">
            {editingId != null
              ? "Edit Directory Watch"
              : "Create Directory Watch"}
          </h3>
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            {/* Always-shown fields */}
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Type</span>
              </div>
              <select
                className="select select-bordered w-full"
                value={form.type}
                onChange={(e) =>
                  setForm({
                    ...form,
                    type: e.target.value,
                    systemId: "",
                    talkgroupId: "",
                  })
                }
              >
                {DIRWATCH_TYPES.map(({ value, label }) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Directory</span>
              </div>
              <div className="join w-full">
                <input
                  type="text"
                  className="input input-bordered join-item w-full font-mono text-sm"
                  value={form.directory}
                  onChange={(e) =>
                    setForm({ ...form, directory: e.target.value })
                  }
                  required
                  placeholder="/path/to/recordings"
                />
                <button
                  type="button"
                  className="btn join-item"
                  onClick={handlePickDirectory}
                >
                  Browse
                </button>
              </div>
              <div className="label">
                <span className="label-text-alt text-base-content/60">
                  Browse uses server directories and returns full paths.
                </span>
              </div>
            </label>

            {/* Extension — shown for default, dsdplus, trunk-recorder */}
            {["default", "dsdplus", "trunk-recorder"].includes(form.type) && (
              <label className="form-control w-full">
                <div className="label">
                  <span className="label-text">Extension</span>
                  <span className="label-text-alt text-base-content/60">
                    e.g. mp3, wav — without the dot
                  </span>
                </div>
                <input
                  type="text"
                  className="input input-bordered w-full"
                  value={form.extension}
                  placeholder="mp3"
                  onChange={(e) =>
                    setForm({ ...form, extension: e.target.value })
                  }
                />
              </label>
            )}

            {/* System dropdown — shown for default and dsdplus */}
            {["default", "dsdplus"].includes(form.type) && (
              <label className="form-control w-full">
                <div className="label">
                  <span className="label-text">System</span>
                  <span className="label-text-alt text-base-content/60">
                    Override: send all files to this system
                  </span>
                </div>
                <select
                  className="select select-bordered w-full"
                  value={form.systemId}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      systemId: e.target.value,
                      talkgroupId: "",
                    })
                  }
                >
                  <option value="">— extract from mask / filename —</option>
                  {(systems ?? [])
                    .slice()
                    .sort((a, b) => a.order - b.order)
                    .map((s) => (
                      <option key={s.id} value={String(s.id)}>
                        {s.label} ({s.systemId})
                      </option>
                    ))}
                </select>
              </label>
            )}

            {/* Talkgroup dropdown — shown for default and dsdplus, only when a system is selected */}
            {["default", "dsdplus"].includes(form.type) && form.systemId && (
              <label className="form-control w-full">
                <div className="label">
                  <span className="label-text">Talkgroup</span>
                  <span className="label-text-alt text-base-content/60">
                    Override: send all files to this talkgroup
                  </span>
                </div>
                <select
                  className="select select-bordered w-full"
                  value={form.talkgroupId}
                  onChange={(e) =>
                    setForm({ ...form, talkgroupId: e.target.value })
                  }
                >
                  <option value="">— extract from mask / filename —</option>
                  {talkgroupsForSelectedSystem.map((tg) => (
                    <option key={tg.id} value={String(tg.id)}>
                      {tg.label ?? tg.talkgroupId} ({tg.talkgroupId})
                    </option>
                  ))}
                </select>
              </label>
            )}

            {/* Mask — shown for default only */}
            {form.type === "default" && (
              <div className="form-control w-full">
                <label className="label">
                  <span className="label-text">Mask</span>
                </label>
                <input
                  type="text"
                  className="input input-bordered w-full font-mono text-sm"
                  value={form.mask}
                  placeholder="e.g. site_#TG_#DATE_#TIME_#HZ"
                  onChange={(e) => setForm({ ...form, mask: e.target.value })}
                />
                <details className="mt-1">
                  <summary className="text-xs text-base-content/60 cursor-pointer select-none">
                    Available mask tokens
                  </summary>
                  <pre className="text-xs text-base-content/70 bg-base-300 rounded p-2 mt-1 whitespace-pre-wrap">
                    {MASK_HELP}
                  </pre>
                </details>
              </div>
            )}

            {/* Frequency — shown for default only */}
            {form.type === "default" && (
              <label className="form-control w-full">
                <div className="label">
                  <span className="label-text">Frequency (Hz)</span>
                  <span className="label-text-alt text-base-content/60">
                    Display-only fake frequency
                  </span>
                </div>
                <input
                  type="number"
                  className="input input-bordered w-full"
                  value={form.frequency}
                  min={0}
                  placeholder="e.g. 155325000"
                  onChange={(e) =>
                    setForm({ ...form, frequency: e.target.value })
                  }
                />
              </label>
            )}

            {/* Delay — shown for default only */}
            {form.type === "default" && (
              <label className="form-control w-full">
                <div className="label">
                  <span className="label-text">Delay (ms)</span>
                  <span className="label-text-alt text-base-content/60">
                    Min 2000 — wait before ingesting file
                  </span>
                </div>
                <input
                  type="number"
                  className="input input-bordered w-full"
                  value={form.delay}
                  min={2000}
                  step={100}
                  placeholder="2000"
                  onChange={(e) => setForm({ ...form, delay: e.target.value })}
                />
              </label>
            )}

            {/* Toggles */}
            <div className="divider my-1" />
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
                <div>
                  <span className="label-text">Delete after import</span>
                  <p className="text-xs text-base-content/60">
                    Remove audio file from disk after ingestion
                  </p>
                </div>
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
                <div>
                  <span className="label-text">Use polling</span>
                  <p className="text-xs text-base-content/60">
                    Use filesystem polling instead of inotify (for NFS/CIFS)
                  </p>
                </div>
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

      <dialog className={`modal ${directoryBrowserOpen ? "modal-open" : ""}`}>
        <div className="modal-box max-w-2xl">
          <h3 className="font-bold text-lg mb-2">Select Server Directory</h3>
          <div className="join w-full mb-3">
            <input
              type="text"
              className="input input-bordered join-item w-full font-mono text-sm"
              value={directoryJumpInput}
              onChange={(e) => setDirectoryJumpInput(e.target.value)}
              placeholder="/absolute/path"
            />
            <button
              type="button"
              className="btn join-item"
              disabled={loadingServerDirs}
              onClick={() => {
                const raw = directoryJumpInput.trim();
                const target = raw === "" ? "/" : raw;
                const normalized = target.startsWith("/")
                  ? target
                  : `/${target}`;
                void navigateDirectory(normalized);
              }}
            >
              Go
            </button>
          </div>
          <p className="text-sm text-base-content/70 mb-3 font-mono break-all">
            {serverDirs?.path ?? directoryBrowserPath}
          </p>
          <div className="max-h-80 overflow-auto border border-base-300 rounded-md">
            <button
              type="button"
              className="btn btn-ghost btn-sm w-full justify-start rounded-none"
              disabled={!serverDirs?.parent || loadingServerDirs}
              onClick={() => {
                if (serverDirs?.parent) {
                  void navigateDirectory(serverDirs.parent);
                }
              }}
            >
              ..
            </button>
            {(serverDirs?.directories ?? []).map((d) => (
              <button
                key={d.path}
                type="button"
                className="btn btn-ghost btn-sm w-full justify-start rounded-none font-mono"
                disabled={loadingServerDirs}
                onClick={() => {
                  void navigateDirectory(d.path);
                }}
              >
                {d.name}
              </button>
            ))}
            {(serverDirs?.directories ?? []).length === 0 && (
              <div className="p-3 text-sm text-base-content/60">
                No child directories
              </div>
            )}
          </div>

          <div className="modal-action">
            <button
              type="button"
              className="btn"
              onClick={() => setDirectoryBrowserOpen(false)}
            >
              Cancel
            </button>
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => {
                const chosen = serverDirs?.path ?? directoryBrowserPath;
                setForm((prev) => ({ ...prev, directory: chosen }));
                setDirectoryBrowserOpen(false);
              }}
            >
              Use This Directory
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
