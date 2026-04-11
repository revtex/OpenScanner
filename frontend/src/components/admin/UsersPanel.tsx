import { useState } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useListUsersQuery,
  useCreateUserMutation,
  useUpdateUserMutation,
  useDeleteUserMutation,
} from "@/app/slices/adminSlice";
import type { AdminUser, CreateUserPayload, UpdateUserPayload } from "@/types";

interface UserFormState {
  username: string;
  password: string;
  role: "admin" | "listener";
  disabled: number;
  systemsJson: string;
  expiration: string; // ISO date string for input
  limit: string; // string for input
}

const emptyForm: UserFormState = {
  username: "",
  password: "",
  role: "listener",
  disabled: 0,
  systemsJson: "",
  expiration: "",
  limit: "",
};

function userToForm(user: AdminUser): UserFormState {
  return {
    username: user.username,
    password: "",
    role: user.role,
    disabled: user.disabled,
    systemsJson: user.systemsJson ?? "",
    expiration: user.expiration
      ? new Date(user.expiration * 1000).toISOString().slice(0, 10)
      : "",
    limit: user.limit != null ? String(user.limit) : "",
  };
}

function formToCreatePayload(form: UserFormState): CreateUserPayload {
  return {
    username: form.username,
    password: form.password,
    role: form.role,
    disabled: form.disabled,
    systemsJson: form.systemsJson || null,
    expiration: form.expiration
      ? Math.floor(new Date(form.expiration).getTime() / 1000)
      : null,
    limit: form.limit ? Number(form.limit) : null,
  };
}

function formToUpdatePayload(form: UserFormState): UpdateUserPayload {
  const payload: UpdateUserPayload = {
    username: form.username,
    role: form.role,
    disabled: form.disabled,
    systemsJson: form.systemsJson || null,
    expiration: form.expiration
      ? Math.floor(new Date(form.expiration).getTime() / 1000)
      : null,
    limit: form.limit ? Number(form.limit) : null,
  };
  if (form.password) {
    payload.password = form.password;
  }
  return payload;
}

export default function UsersPanel() {
  const { data: users, isLoading } = useListUsersQuery();
  const [createUser] = useCreateUserMutation();
  const [updateUser] = useUpdateUserMutation();
  const [deleteUser] = useDeleteUserMutation();

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<UserFormState>(emptyForm);
  const [toast, setToast] = useState<string | null>(null);

  const showError = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  };

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setModalOpen(true);
  };

  const openEdit = (user: AdminUser) => {
    setEditingId(user.id);
    setForm(userToForm(user));
    setModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingId != null) {
        await updateUser({
          id: editingId,
          ...formToUpdatePayload(form),
        }).unwrap();
      } else {
        await createUser(formToCreatePayload(form)).unwrap();
      }
      setModalOpen(false);
    } catch {
      showError(editingId ? "Failed to update user" : "Failed to create user");
    }
  };

  const handleDelete = async (user: AdminUser) => {
    if (!window.confirm(`Delete user "${user.username}"?`)) return;
    try {
      await deleteUser(user.id).unwrap();
    } catch {
      showError("Failed to delete user");
    }
  };

  const handleToggleDisabled = async (user: AdminUser) => {
    try {
      await updateUser({
        id: user.id,
        disabled: user.disabled ? 0 : 1,
      }).unwrap();
    } catch {
      showError("Failed to update user");
    }
  };

  const updateField = <K extends keyof UserFormState>(
    key: K,
    value: UserFormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
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
      <h1 className="text-xl font-semibold mb-4">Users</h1>
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="overflow-x-auto">
            <table className="table table-zebra w-full">
              <thead>
                <tr>
                  <th>Username</th>
                  <th>Role</th>
                  <th>Disabled</th>
                  <th>Expiration</th>
                  <th>Limit</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {users?.map((user) => (
                  <tr key={user.id}>
                    <td>{user.username}</td>
                    <td>
                      <span
                        className={`badge ${user.role === "admin" ? "badge-primary" : "badge-secondary"}`}
                      >
                        {user.role}
                      </span>
                    </td>
                    <td>
                      <input
                        type="checkbox"
                        className="toggle toggle-primary toggle-sm"
                        checked={user.disabled === 1}
                        onChange={() => handleToggleDisabled(user)}
                      />
                    </td>
                    <td>
                      {user.expiration
                        ? new Date(user.expiration * 1000).toLocaleDateString()
                        : "—"}
                    </td>
                    <td>{user.limit != null ? user.limit : "—"}</td>
                    <td className="flex gap-1">
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => openEdit(user)}
                        aria-label="Edit user"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => handleDelete(user)}
                        aria-label="Delete user"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
                {users?.length === 0 && (
                  <tr>
                    <td colSpan={6} className="text-center opacity-60">
                      No users found
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-4">
            <button className="btn btn-primary" onClick={openCreate}>
              <Plus className="w-4 h-4" />
              Add User
            </button>
          </div>
        </div>
      </div>

      {/* Create / Edit Modal */}
      <dialog className={`modal ${modalOpen ? "modal-open" : ""}`}>
        <div className="modal-box">
          <h3 className="font-bold text-lg mb-4">
            {editingId != null ? "Edit User" : "Create User"}
          </h3>
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Username</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={form.username}
                onChange={(e) => updateField("username", e.target.value)}
                required
              />
            </label>

            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">
                  Password{editingId != null ? " (leave blank to keep)" : ""}
                </span>
              </div>
              <input
                type="password"
                className="input input-bordered w-full"
                value={form.password}
                onChange={(e) => updateField("password", e.target.value)}
                required={editingId == null}
              />
            </label>

            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Role</span>
              </div>
              <select
                className="select select-bordered w-full"
                value={form.role}
                onChange={(e) =>
                  updateField("role", e.target.value as "admin" | "listener")
                }
              >
                <option value="listener">Listener</option>
                <option value="admin">Admin</option>
              </select>
            </label>

            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Expiration</span>
              </div>
              <input
                type="date"
                className="input input-bordered w-full"
                value={form.expiration}
                onChange={(e) => updateField("expiration", e.target.value)}
              />
            </label>

            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Concurrent Limit</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={form.limit}
                onChange={(e) => updateField("limit", e.target.value)}
                min={0}
              />
            </label>

            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Systems JSON</span>
              </div>
              <textarea
                className="textarea textarea-bordered w-full"
                rows={3}
                value={form.systemsJson}
                onChange={(e) => updateField("systemsJson", e.target.value)}
                placeholder="e.g. [1, 2, 3]"
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
