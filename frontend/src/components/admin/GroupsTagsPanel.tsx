import { useState } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useListGroupsQuery,
  useCreateGroupMutation,
  useUpdateGroupMutation,
  useDeleteGroupMutation,
  useListTagsQuery,
  useCreateTagMutation,
  useUpdateTagMutation,
  useDeleteTagMutation,
} from "@/hooks/useAdminWsOps";
import type { AdminGroup, AdminTag } from "@/types";

// ─── Generic label CRUD table ───

function LabelTable<T extends { id: number; label: string }>({
  title,
  items,
  isLoading,
  onCreate,
  onUpdate,
  onDelete,
}: {
  title: string;
  items: T[] | undefined;
  isLoading: boolean;
  onCreate: (label: string) => Promise<void>;
  onUpdate: (id: number, label: string) => Promise<void>;
  onDelete: (item: T) => Promise<void>;
}) {
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [label, setLabel] = useState("");
  const [toast, setToast] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  };

  const openCreate = () => {
    setEditingId(null);
    setLabel("");
    setModalOpen(true);
  };

  const openEdit = (item: T) => {
    setEditingId(item.id);
    setLabel(item.label);
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingId != null) {
        await onUpdate(editingId, label);
      } else {
        await onCreate(label);
      }
      setModalOpen(false);
    } catch {
      showToast(
        editingId
          ? `Failed to update ${title.toLowerCase()}`
          : `Failed to create ${title.toLowerCase()}`,
      );
    }
  };

  const handleDelete = async (item: T) => {
    if (!window.confirm(`Delete ${title.toLowerCase()} "${item.label}"?`))
      return;
    try {
      await onDelete(item);
    } catch {
      showToast(`Failed to delete ${title.toLowerCase()}`);
    }
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner loading-lg" />
      </div>
    );
  }

  return (
    <div className="card bg-base-200">
      <div className="card-body">
        <div className="flex justify-between items-center mb-2">
          <h2 className="card-title text-lg">{title}</h2>
          <button className="btn btn-primary btn-sm" onClick={openCreate}>
            <Plus className="w-4 h-4" /> Add
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="table table-zebra w-full">
            <thead>
              <tr>
                <th>ID</th>
                <th>Label</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {items?.map((item) => (
                <tr key={item.id}>
                  <td>{item.id}</td>
                  <td>{item.label}</td>
                  <td className="flex gap-1">
                    <button
                      className="btn btn-ghost btn-xs"
                      onClick={() => openEdit(item)}
                      aria-label={`Edit ${title.toLowerCase()}`}
                    >
                      <Pencil className="w-4 h-4" />
                    </button>
                    <button
                      className="btn btn-ghost btn-xs"
                      onClick={() => handleDelete(item)}
                      aria-label={`Delete ${title.toLowerCase()}`}
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
              {items?.length === 0 && (
                <tr>
                  <td colSpan={3} className="text-center opacity-60">
                    No {title.toLowerCase()} yet
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Modal */}
        <dialog className={`modal ${modalOpen ? "modal-open" : ""}`}>
          <div className="modal-box">
            <h3 className="font-bold text-lg">
              {editingId != null ? `Edit ${title}` : `Add ${title}`}
            </h3>
            <form onSubmit={handleSubmit} className="mt-4 space-y-4">
              <div className="flex flex-col">
                <span className="text-sm">Label</span>
                <input
                  type="text"
                  className="input w-full"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                  required
                />
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
    </div>
  );
}

// ─── Main panel ───

export default function GroupsTagsPanel() {
  const { data: groups, isLoading: loadingGroups } = useListGroupsQuery();
  const [createGroup] = useCreateGroupMutation();
  const [updateGroup] = useUpdateGroupMutation();
  const [deleteGroup] = useDeleteGroupMutation();

  const { data: tags, isLoading: loadingTags } = useListTagsQuery();
  const [createTag] = useCreateTagMutation();
  const [updateTag] = useUpdateTagMutation();
  const [deleteTag] = useDeleteTagMutation();

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Groups & Tags</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Groups organize talkgroups into categories displayed in the Select
        Talkgroup panel. Tags provide additional classification for filtering.
        Assign groups and tags to talkgroups in the Systems page.
      </p>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <LabelTable<AdminGroup>
          title="Group"
          items={groups}
          isLoading={loadingGroups}
          onCreate={async (label) => {
            await createGroup({ label }).unwrap();
          }}
          onUpdate={async (id, label) => {
            await updateGroup({ id, label }).unwrap();
          }}
          onDelete={async (item) => {
            await deleteGroup(item.id).unwrap();
          }}
        />
        <LabelTable<AdminTag>
          title="Tag"
          items={tags}
          isLoading={loadingTags}
          onCreate={async (label) => {
            await createTag({ label }).unwrap();
          }}
          onUpdate={async (id, label) => {
            await updateTag({ id, label }).unwrap();
          }}
          onDelete={async (item) => {
            await deleteTag(item.id).unwrap();
          }}
        />
      </div>
    </div>
  );
}
