import { useState, useRef, useCallback } from "react";
import { Upload, Download, ExternalLink } from "lucide-react";
import {
  useImportTalkgroupsMutation,
  useImportUnitsMutation,
} from "@/app/slices/adminSlice";
import {
  useLazyExportConfigQuery,
  useLazyExportTalkgroupsQuery,
  useLazyExportUnitsQuery,
  useImportConfigMutation,
  useListSystemsQuery,
} from "@/hooks/useAdminWsOps";
import { selectToken } from "@/app/slices/authSlice";
import { useAppSelector } from "@/app/store";
import RadioReferenceCard from "@/components/admin/RadioReferenceCard";

const SWAGGER_URL = "/api/admin/docs/index.html";

export default function ToolsPanel() {
  const token = useAppSelector(selectToken);
  const [importTalkgroups] = useImportTalkgroupsMutation();
  const [importUnits] = useImportUnitsMutation();
  const [triggerExport] = useLazyExportConfigQuery();
  const [triggerExportTalkgroups] = useLazyExportTalkgroupsQuery();
  const [triggerExportUnits] = useLazyExportUnitsQuery();
  const [importConfig] = useImportConfigMutation();
  const { data: systems } = useListSystemsQuery();

  const [toast, setToast] = useState<string | null>(null);
  const [toastType, setToastType] = useState<"error" | "success">("error");
  const tgFileRef = useRef<HTMLInputElement>(null);
  const unitFileRef = useRef<HTMLInputElement>(null);
  const configFileRef = useRef<HTMLInputElement>(null);
  const [selectedTgSystemId, setSelectedTgSystemId] = useState<string>("");
  const [tgImportMode, setTgImportMode] = useState<"overwrite" | "skip">(
    "overwrite",
  );
  const [selectedUnitSystemId, setSelectedUnitSystemId] = useState<string>("");
  const [unitImportMode, setUnitImportMode] = useState<"overwrite" | "skip">(
    "overwrite",
  );
  const [exportTgSystemId, setExportTgSystemId] = useState<string>("");
  const [exportUnitSystemId, setExportUnitSystemId] = useState<string>("");

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
    if (!selectedTgSystemId) {
      showToast("Please select a system");
      return;
    }
    const formData = new FormData();
    formData.append("file", file);
    formData.append("system_id", selectedTgSystemId);
    formData.append("mode", tgImportMode);
    try {
      const result = await importTalkgroups(formData).unwrap();
      const msg = `Talkgroups imported: ${result.inserted} inserted, ${result.updated} updated, ${result.skipped} skipped`;
      showToast(msg, "success");
      if (tgFileRef.current) tgFileRef.current.value = "";
    } catch {
      showToast("Failed to import talkgroups");
    }
  };

  const handleImportUnits = async () => {
    const file = unitFileRef.current?.files?.[0];
    if (!file) return;
    if (!selectedUnitSystemId) {
      showToast("Please select a system");
      return;
    }
    const formData = new FormData();
    formData.append("file", file);
    formData.append("system_id", selectedUnitSystemId);
    formData.append("mode", unitImportMode);
    try {
      const result = await importUnits(formData).unwrap();
      const msg = `Units imported: ${result.inserted} inserted, ${result.updated} updated, ${result.skipped} skipped`;
      showToast(msg, "success");
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

  const handleExportTalkgroups = async () => {
    try {
      const csv = await triggerExportTalkgroups(
        exportTgSystemId ? { systemId: Number(exportTgSystemId) } : {},
      ).unwrap();
      const systemLabel =
        systems?.find((s) => String(s.id) === exportTgSystemId)?.label ?? "all";
      const blob = new Blob([csv], { type: "text/csv" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `talkgroups-${systemLabel}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast("Failed to export talkgroups");
    }
  };

  const handleExportUnits = async () => {
    try {
      const csv = await triggerExportUnits(
        exportUnitSystemId ? { systemId: Number(exportUnitSystemId) } : {},
      ).unwrap();
      const systemLabel =
        systems?.find((s) => String(s.id) === exportUnitSystemId)?.label ??
        "all";
      const blob = new Blob([csv], { type: "text/csv" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `units-${systemLabel}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast("Failed to export units");
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
          <div className="space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <div>
                <label className="label">
                  <span className="label-text text-sm">System</span>
                </label>
                <select
                  value={selectedTgSystemId}
                  onChange={(e) => setSelectedTgSystemId(e.target.value)}
                  className="select select-bordered select-sm w-full"
                >
                  <option value="">--- Select a system ---</option>
                  {systems?.map((sys) => (
                    <option key={sys.id} value={sys.id}>
                      {sys.label}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="label">
                  <span className="label-text text-sm">Duplicate Mode</span>
                </label>
                <select
                  value={tgImportMode}
                  onChange={(e) =>
                    setTgImportMode(e.target.value as "overwrite" | "skip")
                  }
                  className="select select-bordered select-sm w-full"
                >
                  <option value="overwrite">Overwrite (update existing)</option>
                  <option value="skip">Skip (keep existing)</option>
                </select>
              </div>
              <div className="flex items-end">
                <input
                  ref={tgFileRef}
                  type="file"
                  accept=".csv"
                  className="file-input file-input-bordered file-input-sm w-full"
                />
              </div>
            </div>
            <div className="text-xs text-base-content/70">
              CSV format: talkgroup_id, label (optional), name, tag_id,
              group_id, frequency, led, order. Mode: "Overwrite" updates
              existing talkgroup properties, "Skip" ignores duplicates.
            </div>
            <button
              className="btn btn-primary btn-sm w-full"
              onClick={handleImportTalkgroups}
            >
              Upload Talkgroups
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
          <div className="space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <div>
                <label className="label">
                  <span className="label-text text-sm">System</span>
                </label>
                <select
                  value={selectedUnitSystemId}
                  onChange={(e) => setSelectedUnitSystemId(e.target.value)}
                  className="select select-bordered select-sm w-full"
                >
                  <option value="">--- Select a system ---</option>
                  {systems?.map((sys) => (
                    <option key={sys.id} value={sys.id}>
                      {sys.label}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="label">
                  <span className="label-text text-sm">Duplicate Mode</span>
                </label>
                <select
                  value={unitImportMode}
                  onChange={(e) =>
                    setUnitImportMode(e.target.value as "overwrite" | "skip")
                  }
                  className="select select-bordered select-sm w-full"
                >
                  <option value="overwrite">Overwrite (update existing)</option>
                  <option value="skip">Skip (keep existing)</option>
                </select>
              </div>
              <div className="flex items-end">
                <input
                  ref={unitFileRef}
                  type="file"
                  accept=".csv"
                  className="file-input file-input-bordered file-input-sm w-full"
                />
              </div>
            </div>
            <div className="text-xs text-base-content/70">
              CSV format: unit_id, label (optional), order (optional). Mode:
              "Overwrite" updates existing unit labels, "Skip" ignores
              duplicates.
            </div>
            <button
              className="btn btn-primary btn-sm w-full"
              onClick={handleImportUnits}
            >
              Upload Units
            </button>
          </div>
        </div>
      </div>

      {/* RadioReference Enrichment */}
      <RadioReferenceCard />

      {/* CSV Export: Talkgroups */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <Download className="w-4 h-4" /> Export Talkgroups (CSV)
          </h2>
          <div className="space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div>
                <label className="label">
                  <span className="label-text text-sm">System</span>
                </label>
                <select
                  value={exportTgSystemId}
                  onChange={(e) => setExportTgSystemId(e.target.value)}
                  className="select select-bordered select-sm w-full"
                >
                  <option value="">All systems</option>
                  {systems?.map((sys) => (
                    <option key={sys.id} value={sys.id}>
                      {sys.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex items-end">
                <button
                  className="btn btn-primary btn-sm w-full"
                  onClick={handleExportTalkgroups}
                >
                  Download CSV
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* CSV Export: Units */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <Download className="w-4 h-4" /> Export Units (CSV)
          </h2>
          <div className="space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div>
                <label className="label">
                  <span className="label-text text-sm">System</span>
                </label>
                <select
                  value={exportUnitSystemId}
                  onChange={(e) => setExportUnitSystemId(e.target.value)}
                  className="select select-bordered select-sm w-full"
                >
                  <option value="">All systems</option>
                  {systems?.map((sys) => (
                    <option key={sys.id} value={sys.id}>
                      {sys.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex items-end">
                <button
                  className="btn btn-primary btn-sm w-full"
                  onClick={handleExportUnits}
                >
                  Download CSV
                </button>
              </div>
            </div>
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
              className="file-input file-input-sm"
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

      {/* API Documentation */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">
            <ExternalLink className="w-4 h-4" /> API Documentation
          </h2>
          <p className="text-sm text-base-content/70">
            Browse the interactive Swagger UI to explore and test all API
            endpoints. Use the token below to authorize requests via the padlock
            icon in Swagger UI. Your docs session expires after 1 hour — click
            "Open Swagger UI" again to refresh it.
          </p>
          <div className="flex items-center gap-2 flex-wrap">
            <button
              className="btn btn-primary btn-sm"
              onClick={async () => {
                const res = await fetch("/api/admin/docs/session", {
                  method: "POST",
                  headers: { Authorization: `Bearer ${token}` },
                });
                if (res.ok) {
                  window.open(SWAGGER_URL, "_blank", "noopener");
                }
              }}
            >
              Open Swagger UI
              <ExternalLink className="w-4 h-4" />
            </button>
            <button
              className="btn btn-outline btn-sm"
              onClick={() => {
                if (token) {
                  navigator.clipboard.writeText(`Bearer ${token}`);
                  showToast("Bearer token copied to clipboard", "success");
                }
              }}
            >
              Copy Bearer Token
            </button>
          </div>
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
