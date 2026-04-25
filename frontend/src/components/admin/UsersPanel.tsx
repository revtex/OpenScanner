import { useState } from "react";
import { Pencil, Trash2, Plus } from "lucide-react";
import {
  useListUsersQuery,
  useListSystemsQuery,
  useCreateUserMutation,
  useUpdateUserMutation,
  useDeleteUserMutation,
} from "@/hooks/admin/useAdminWsOps";
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
  const { data: systems } = useListSystemsQuery();
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
    if (user.id === 1) {
      showError("Cannot disable the primary admin account");
      return;
    }
    try {
      await updateUser({
        id: user.id,
        username: user.username,
        role: user.role,
        disabled: user.disabled ? 0 : 1,
        systemsJson: user.systemsJson ?? null,
        expiration: user.expiration ?? null,
        limit: user.limit ?? null,
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
      <h1 className="text-xl font-semibold mb-4">Users</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Manage user accounts that can access the scanner. Each user has a role
        (admin or listener), and can optionally be restricted to specific
        systems, given an expiration date, or rate-limited.
      </p>
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
                        disabled={user.id === 1}
                        onChange={() => handleToggleDisabled(user)}
                        title={
                          user.id === 1
                            ? "Cannot disable the primary admin"
                            : undefined
                        }
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
                      {user.id !== 1 && (
                        <button
                          className="btn btn-ghost btn-xs"
                          onClick={() => handleDelete(user)}
                          aria-label="Delete user"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
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
        <div className="modal-box max-w-lg">
          <h3 className="font-bold text-lg mb-1">
            {editingId != null ? "Edit User" : "Create User"}
          </h3>
          <p className="text-sm text-base-content/60 mb-4">
            {editingId != null
              ? "Update this user's account settings and access controls."
              : "Add a new user account. They can log in immediately after creation."}
          </p>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {/* Account section */}
            <fieldset className="fieldset bg-base-200 border-base-300 rounded-box border p-4">
              <legend className="fieldset-legend px-1 text-sm font-semibold">
                Account
              </legend>
              <div className="grid grid-cols-2 gap-3">
                <label className="flex flex-col w-full">
                  <span className="text-sm font-medium mb-1">Username</span>
                  <input
                    type="text"
                    className="input w-full"
                    value={form.username}
                    onChange={(e) => updateField("username", e.target.value)}
                    required
                  />
                </label>

                <label className="flex flex-col w-full">
                  <span className="text-sm font-medium mb-1">Password</span>
                  <input
                    type="password"
                    className="input w-full"
                    value={form.password}
                    onChange={(e) => updateField("password", e.target.value)}
                    required={editingId == null}
                  />
                  {editingId != null && (
                    <span className="text-xs text-base-content/50 mt-1">
                      Leave blank to keep current.
                    </span>
                  )}
                </label>

                <label className="flex flex-col w-full">
                  <span className="text-sm font-medium mb-1">Role</span>
                  <select
                    className="select w-full"
                    value={form.role}
                    disabled={editingId === 1}
                    onChange={(e) =>
                      updateField(
                        "role",
                        e.target.value as "admin" | "listener",
                      )
                    }
                  >
                    <option value="listener">Listener</option>
                    <option value="admin">Admin</option>
                  </select>
                </label>

                <label className="flex flex-col w-full">
                  <span className="text-sm font-medium mb-1">Disabled</span>
                  <div className="flex items-center h-12">
                    <input
                      type="checkbox"
                      className="toggle toggle-primary"
                      checked={form.disabled === 1}
                      disabled={editingId === 1}
                      onChange={(e) =>
                        updateField("disabled", e.target.checked ? 1 : 0)
                      }
                    />
                  </div>
                </label>
              </div>
            </fieldset>

            {/* Access Controls section */}
            <fieldset className="fieldset bg-base-200 border-base-300 rounded-box border p-4">
              <legend className="fieldset-legend px-1 text-sm font-semibold">
                Access Controls
              </legend>
              <div className="grid grid-cols-2 gap-3">
                <label className="flex flex-col w-full">
                  <span className="text-sm font-medium mb-1">Expiration</span>
                  <input
                    type="date"
                    className="input w-full"
                    value={form.expiration}
                    disabled={editingId === 1}
                    onChange={(e) => updateField("expiration", e.target.value)}
                  />
                  <span className="text-xs text-base-content/50 mt-1">
                    {editingId === 1
                      ? "Locked for primary admin."
                      : "Optional. Account disabled after this date."}
                  </span>
                </label>

                <label className="flex flex-col w-full">
                  <span className="text-sm font-medium mb-1">Max Sessions</span>
                  <input
                    type="number"
                    className="input w-full"
                    value={form.limit}
                    disabled={editingId === 1}
                    onChange={(e) => updateField("limit", e.target.value)}
                    min={0}
                    placeholder="Unlimited"
                  />
                  <span className="text-xs text-base-content/50 mt-1">
                    {editingId === 1
                      ? "Locked for primary admin."
                      : "Simultaneous logins. Empty = unlimited."}
                  </span>
                </label>
              </div>

              {/* System badges */}
              {systems && systems.length > 0 && (
                <div className="flex flex-col gap-1 mt-3">
                  <span className="text-sm font-medium">Allowed Systems</span>
                  <span className="text-xs text-base-content/60">
                    Select which systems this user can access. If none selected,
                    all systems are allowed.
                  </span>
                  <div className="flex flex-wrap gap-2 mt-1">
                    {systems
                      .slice()
                      .sort((a, b) => a.order - b.order)
                      .map((sys) => {
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
                              className={`inline-block w-2 h-2 rounded-full ${
                                selected
                                  ? "bg-primary-content"
                                  : "bg-base-content/30"
                              }`}
                            />
                            {sys.label}
                          </button>
                        );
                      })}
                  </div>
                </div>
              )}
            </fieldset>

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
