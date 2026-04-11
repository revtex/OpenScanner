import { useState, useMemo, useCallback } from "react";
import { Pencil, Trash2, Plus, GripVertical, Copy, Check } from "lucide-react";
import {
  DndContext,
  closestCenter,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
  arrayMove,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  useListApiKeysQuery,
  useCreateApiKeyMutation,
  useUpdateApiKeyMutation,
  useDeleteApiKeyMutation,
  useListSystemsQuery,
} from "@/app/slices/adminSlice";
import type { AdminApiKey } from "@/types";

// ─── Form state ───

interface ApiKeyFormState {
  key: string;
  ident: string;
  disabled: number;
  systemsJson: string;
}

const emptyForm: ApiKeyFormState = {
  key: "",
  ident: "",
  disabled: 0,
  systemsJson: "",
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

// ─── Sortable row ───

function SortableApiKeyRow({
  apiKey,
  onEdit,
  onDelete,
  onToggleDisabled,
}: {
  apiKey: AdminApiKey;
  onEdit: () => void;
  onDelete: () => void;
  onToggleDisabled: () => void;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: apiKey.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const systemsList = apiKey.systemsJson
    ? (() => {
        try {
          return (JSON.parse(apiKey.systemsJson) as number[]).join(", ");
        } catch {
          return apiKey.systemsJson;
        }
      })()
    : "All";

  return (
    <tr ref={setNodeRef} style={style}>
      <td className="w-8">
        <button
          className="btn btn-ghost btn-xs cursor-grab"
          {...attributes}
          {...listeners}
          aria-label="Drag to reorder"
        >
          <GripVertical className="w-4 h-4" />
        </button>
      </td>
      <td>
        <span className="font-mono text-sm">
          {apiKey.key.slice(0, 8)}&hellip;
        </span>
        <CopyButton text={apiKey.key} />
      </td>
      <td>{apiKey.ident ?? "—"}</td>
      <td>
        <input
          type="checkbox"
          className="toggle toggle-primary toggle-sm"
          checked={apiKey.disabled === 1}
          onChange={onToggleDisabled}
        />
      </td>
      <td>{systemsList}</td>
      <td className="flex gap-1">
        <button
          className="btn btn-ghost btn-xs"
          onClick={onEdit}
          aria-label="Edit API key"
        >
          <Pencil className="w-4 h-4" />
        </button>
        <button
          className="btn btn-ghost btn-xs"
          onClick={onDelete}
          aria-label="Delete API key"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </td>
    </tr>
  );
}

// ─── Main panel ───

export default function ApiKeysPanel() {
  const { data: apiKeys, isLoading } = useListApiKeysQuery();
  const { data: systems } = useListSystemsQuery();
  const [createApiKey] = useCreateApiKeyMutation();
  const [updateApiKey] = useUpdateApiKeyMutation();
  const [deleteApiKey] = useDeleteApiKeyMutation();

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<ApiKeyFormState>(emptyForm);
  const [toast, setToast] = useState<string | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor),
  );

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
    setForm({
      ...emptyForm,
      key: crypto.randomUUID(),
    });
    setModalOpen(true);
  };

  const openEdit = (ak: AdminApiKey) => {
    setEditingId(ak.id);
    setForm({
      key: ak.key,
      ident: ak.ident ?? "",
      disabled: ak.disabled,
      systemsJson: ak.systemsJson ?? "",
    });
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingId != null) {
        await updateApiKey({
          id: editingId,
          key: form.key,
          ident: form.ident || null,
          disabled: form.disabled,
          systemsJson: form.systemsJson || null,
        }).unwrap();
      } else {
        await createApiKey({
          key: form.key,
          ident: form.ident || null,
          disabled: form.disabled,
          systemsJson: form.systemsJson || null,
          order: sortedKeys.length,
        }).unwrap();
      }
      setModalOpen(false);
    } catch {
      showError(
        editingId ? "Failed to update API key" : "Failed to create API key",
      );
    }
  };

  const handleDelete = async (ak: AdminApiKey) => {
    if (!window.confirm(`Delete API key "${ak.ident || ak.key.slice(0, 8)}"?`))
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
        disabled: ak.disabled ? 0 : 1,
      }).unwrap();
    } catch {
      showError("Failed to update API key");
    }
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id || !sortedKeys.length) return;

    const oldIndex = sortedKeys.findIndex((k) => k.id === active.id);
    const newIndex = sortedKeys.findIndex((k) => k.id === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    const reordered = arrayMove(sortedKeys, oldIndex, newIndex);

    try {
      await Promise.all(
        reordered.map((ak, idx) => {
          if (ak.order !== idx) {
            return updateApiKey({ id: ak.id, order: idx }).unwrap();
          }
          return Promise.resolve();
        }),
      );
    } catch {
      showError("Failed to reorder API keys");
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
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="overflow-x-auto">
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragEnd={handleDragEnd}
            >
              <SortableContext
                items={sortedKeys.map((k) => k.id)}
                strategy={verticalListSortingStrategy}
              >
                <table className="table table-zebra w-full">
                  <thead>
                    <tr>
                      <th className="w-8" />
                      <th>Key</th>
                      <th>Ident</th>
                      <th>Disabled</th>
                      <th>Systems</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedKeys.map((ak) => (
                      <SortableApiKeyRow
                        key={ak.id}
                        apiKey={ak}
                        onEdit={() => openEdit(ak)}
                        onDelete={() => handleDelete(ak)}
                        onToggleDisabled={() => handleToggleDisabled(ak)}
                      />
                    ))}
                    {sortedKeys.length === 0 && (
                      <tr>
                        <td colSpan={6} className="text-center opacity-60">
                          No API keys yet
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </SortableContext>
            </DndContext>
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
            <div className="form-control">
              <label className="label">
                <span className="label-text">Key</span>
              </label>
              <input
                type="text"
                className="input input-bordered w-full font-mono text-sm"
                value={form.key}
                onChange={(e) => updateField("key", e.target.value)}
                readOnly={editingId != null}
                required
              />
            </div>

            <div className="form-control">
              <label className="label">
                <span className="label-text">Identifier</span>
              </label>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.ident}
                onChange={(e) => updateField("ident", e.target.value)}
                placeholder="Optional description"
              />
            </div>

            {systems && systems.length > 0 && (
              <div className="form-control">
                <label className="label">
                  <span className="label-text">Systems (none = all)</span>
                </label>
                <div className="flex flex-wrap gap-2">
                  {systems.map((sys) => (
                    <label key={sys.id} className="label cursor-pointer gap-2">
                      <input
                        type="checkbox"
                        className="checkbox checkbox-sm"
                        checked={selectedSystems.includes(sys.id)}
                        onChange={() => toggleSystem(sys.id)}
                      />
                      <span className="label-text text-sm">{sys.label}</span>
                    </label>
                  ))}
                </div>
              </div>
            )}

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
