import { useState, useRef, useCallback } from "react";
import { Upload, Download, KeyRound } from "lucide-react";
import {
  useImportTalkgroupsMutation,
  useImportUnitsMutation,
  useLazyExportConfigQuery,
  useImportConfigMutation,
  useChangePasswordMutation,
} from "@/app/slices/adminSlice";

export default function ToolsPanel() {
  const [importTalkgroups] = useImportTalkgroupsMutation();
  const [importUnits] = useImportUnitsMutation();
  const [triggerExport] = useLazyExportConfigQuery();
  const [importConfig] = useImportConfigMutation();
  const [changePassword] = useChangePasswordMutation();

  const [toast, setToast] = useState<string | null>(null);
  const [toastType, setToastType] = useState<"error" | "success">("error");
  const tgFileRef = useRef<HTMLInputElement>(null);
  const unitFileRef = useRef<HTMLInputElement>(null);
  const configFileRef = useRef<HTMLInputElement>(null);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const showToast = useCallback(
    (msg: string, type: "error" | "success" = "error") => {
      setToast(msg);
      setToastType(type);
      setTimeout(() => setToast(null), 5000);
    },
    [],
  );

  const handleImportTalkgroups = async () => {
    const file = tgFileRef.current?.files?.[0];
    if (!file) return;
    const formData = new FormData();
    formData.append("file", file);
    try {
      await importTalkgroups(formData).unwrap();
      showToast("Talkgroups imported successfully", "success");
      if (tgFileRef.current) tgFileRef.current.value = "";
    } catch {
      showToast("Failed to import talkgroups");
    }
  };

  const handleImportUnits = async () => {
    const file = unitFileRef.current?.files?.[0];
    if (!file) return;
    const formData = new FormData();
    formData.append("file", file);
    try {
      await importUnits(formData).unwrap();
      showToast("Units imported successfully", "success");
      if (unitFileRef.current) unitFileRef.current.value = "";
    } catch {
      showToast("Failed to import units");
    }
  };

  const handleExportConfig = async () => {
    try {
      const result = await triggerExport().unwrap();
      const blob = new Blob([JSON.stringify(result, null, 2)], {
        type: "application/json",
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "openscanner-config.json";
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast("Failed to export config");
    }
  };

  const handleImportConfig = async () => {
    const file = configFileRef.current?.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const json: unknown = JSON.parse(text);
      await importConfig(json).unwrap();
      showToast("Config imported successfully", "success");
      if (configFileRef.current) configFileRef.current.value = "";
    } catch {
      showToast("Failed to import config");
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword.length < 8) {
      showToast("New password must be at least 8 characters");
      return;
    }
    if (newPassword !== confirmPassword) {
      showToast("Passwords do not match");
      return;
    }
    try {
      await changePassword({
        currentPassword,
        newPassword,
      }).unwrap();
      showToast("Password changed successfully", "success");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch {
      showToast("Failed to change password");
    }
  };

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Tools</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Import and export data. Upload CSV files to bulk-import talkgroups or
        units into a system, and export or import the full server configuration
        as JSON for backup or migration.
      </p>

      {/* CSV Import: Talkgroups */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <Upload className="w-4 h-4" /> Import Talkgroups (CSV)
          </h2>
          <div className="flex items-center gap-3">
            <input
              ref={tgFileRef}
              type="file"
              accept=".csv"
              className="file-input file-input-bordered file-input-sm"
            />
            <button
              className="btn btn-primary btn-sm"
              onClick={handleImportTalkgroups}
            >
              Upload
            </button>
          </div>
        </div>
      </div>

      {/* CSV Import: Units */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <Upload className="w-4 h-4" /> Import Units (CSV)
          </h2>
          <div className="flex items-center gap-3">
            <input
              ref={unitFileRef}
              type="file"
              accept=".csv"
              className="file-input file-input-bordered file-input-sm"
            />
            <button
              className="btn btn-primary btn-sm"
              onClick={handleImportUnits}
            >
              Upload
            </button>
          </div>
        </div>
      </div>

      {/* JSON Config Export */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <Download className="w-4 h-4" /> Export Config (JSON)
          </h2>
          <div>
            <button className="btn btn-primary" onClick={handleExportConfig}>
              Export Config
            </button>
          </div>
        </div>
      </div>

      {/* JSON Config Import */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <Upload className="w-4 h-4" /> Import Config (JSON)
          </h2>
          <div className="flex items-center gap-3">
            <input
              ref={configFileRef}
              type="file"
              accept=".json"
              className="file-input file-input-bordered file-input-sm"
            />
            <button
              className="btn btn-primary btn-sm"
              onClick={handleImportConfig}
            >
              Upload
            </button>
          </div>
        </div>
      </div>

      {/* Change Password */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <KeyRound className="w-4 h-4" /> Change Password
          </h2>
          <form
            onSubmit={handleChangePassword}
            className="flex flex-col gap-3 max-w-sm"
          >
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Current Password</span>
              </div>
              <input
                type="password"
                className="input input-bordered w-full"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">New Password</span>
              </div>
              <input
                type="password"
                className="input input-bordered w-full"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                minLength={8}
                autoComplete="new-password"
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Confirm New Password</span>
              </div>
              <input
                type="password"
                className="input input-bordered w-full"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                minLength={8}
                autoComplete="new-password"
              />
            </label>
            <button type="submit" className="btn btn-primary btn-sm w-fit">
              Change Password
            </button>
          </form>
        </div>
      </div>

      {toast && (
        <div className="toast toast-end">
          <div
            className={`alert ${toastType === "success" ? "alert-success" : "alert-error"}`}
          >
            <span>{toast}</span>
          </div>
        </div>
      )}
    </div>
  );
}
