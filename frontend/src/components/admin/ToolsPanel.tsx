import { useState, useRef, useCallback, type RefObject } from "react";
import {
  Upload,
  Download,
  ExternalLink,
  Database,
  FileText,
} from "lucide-react";
import {
  useImportTalkgroupsMutation,
  useImportUnitsMutation,
  useImportGroupsMutation,
  useImportTagsMutation,
} from "@/app/slices/adminSlice";
import {
  useLazyExportConfigQuery,
  useLazyExportTalkgroupsQuery,
  useLazyExportUnitsQuery,
  useLazyExportGroupsQuery,
  useLazyExportTagsQuery,
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
  const [importGroups] = useImportGroupsMutation();
  const [importTags] = useImportTagsMutation();
  const [triggerExport] = useLazyExportConfigQuery();
  const [triggerExportTalkgroups] = useLazyExportTalkgroupsQuery();
  const [triggerExportUnits] = useLazyExportUnitsQuery();
  const [triggerExportGroups] = useLazyExportGroupsQuery();
  const [triggerExportTags] = useLazyExportTagsQuery();
  const [importConfig] = useImportConfigMutation();
  const { data: systems } = useListSystemsQuery();

  const [toast, setToast] = useState<string | null>(null);
  const [toastType, setToastType] = useState<"error" | "success">("error");
  const tgFileRef = useRef<HTMLInputElement>(null);
  const unitFileRef = useRef<HTMLInputElement>(null);
  const configFileRef = useRef<HTMLInputElement>(null);
  const groupsFileRef = useRef<HTMLInputElement>(null);
  const tagsFileRef = useRef<HTMLInputElement>(null);
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
      const failed = result.failed ?? 0;
      const parts = [
        `${result.inserted} inserted`,
        `${result.updated} updated`,
        `${result.skipped} skipped`,
      ];
      if (failed > 0) parts.push(`${failed} failed`);
      const msg = result.message
        ? `Talkgroups: ${result.message}`
        : `Talkgroups imported: ${parts.join(", ")}`;
      const tone = result.inserted + result.updated === 0 ? "error" : "success";
      showToast(msg, tone);
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
      const failed = result.failed ?? 0;
      const parts = [
        `${result.inserted} inserted`,
        `${result.updated} updated`,
        `${result.skipped} skipped`,
      ];
      if (failed > 0) parts.push(`${failed} failed`);
      const tone = result.inserted + result.updated === 0 ? "error" : "success";
      showToast(`Units imported: ${parts.join(", ")}`, tone);
      if (unitFileRef.current) unitFileRef.current.value = "";
    } catch {
      showToast("Failed to import units");
    }
  };

  const handleImportLabels = async (
    kind: "groups" | "tags",
    fileRef: RefObject<HTMLInputElement | null>,
    mutate: (fd: FormData) => {
      unwrap: () => Promise<{
        inserted: number;
        skipped: number;
        failed?: number;
        message?: string;
      }>;
    },
  ) => {
    const file = fileRef.current?.files?.[0];
    if (!file) return;
    const formData = new FormData();
    formData.append("file", file);
    try {
      const result = await mutate(formData).unwrap();
      const failed = result.failed ?? 0;
      const parts = [
        `${result.inserted} inserted`,
        `${result.skipped} skipped`,
      ];
      if (failed > 0) parts.push(`${failed} failed`);
      const msg = result.message
        ? `${kind}: ${result.message}`
        : `${kind} imported: ${parts.join(", ")}`;
      const tone = result.inserted === 0 && failed > 0 ? "error" : "success";
      showToast(msg, tone);
      if (fileRef.current) fileRef.current.value = "";
    } catch {
      showToast(`Failed to import ${kind}`);
    }
  };

  const handleImportGroups = () =>
    handleImportLabels("groups", groupsFileRef, importGroups);
  const handleImportTags = () =>
    handleImportLabels("tags", tagsFileRef, importTags);

  const handleExportLabels = async (
    kind: "groups" | "tags",
    trigger: () => { unwrap: () => Promise<unknown> },
  ) => {
    try {
      const csv = await trigger().unwrap();
      if (typeof csv !== "string") {
        showToast("Export returned unexpected payload");
        return;
      }
      const blob = new Blob([csv], { type: "text/csv" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${kind}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast(`Failed to export ${kind}`);
    }
  };

  const handleExportGroups = () =>
    handleExportLabels("groups", triggerExportGroups);
  const handleExportTags = () => handleExportLabels("tags", triggerExportTags);

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
    if (!exportTgSystemId) {
      showToast("Please select a system");
      return;
    }
    try {
      const csv = await triggerExportTalkgroups({
        systemId: Number(exportTgSystemId),
      }).unwrap();
      if (typeof csv !== "string") {
        showToast("Export returned unexpected payload");
        return;
      }
      const sys = systems?.find((s) => String(s.id) === exportTgSystemId);
      const slug =
        sys?.label.replace(/[^a-zA-Z0-9._-]+/g, "_").replace(/^_+|_+$/g, "") ||
        `system-${exportTgSystemId}`;
      const blob = new Blob([csv], { type: "text/csv" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `talkgroups-${slug}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast("Failed to export talkgroups");
    }
  };

  const handleExportUnits = async () => {
    if (!exportUnitSystemId) {
      showToast("Please select a system");
      return;
    }
    try {
      const csv = await triggerExportUnits({
        systemId: Number(exportUnitSystemId),
      }).unwrap();
      if (typeof csv !== "string") {
        showToast("Export returned unexpected payload");
        return;
      }
      const sys = systems?.find((s) => String(s.id) === exportUnitSystemId);
      const slug =
        sys?.label.replace(/[^a-zA-Z0-9._-]+/g, "_").replace(/^_+|_+$/g, "") ||
        `system-${exportUnitSystemId}`;
      const blob = new Blob([csv], { type: "text/csv" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `units-${slug}.csv`;
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
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold mb-1">Tools</h1>
        <p className="text-sm text-base-content/70">
          Import, export, and enrich your scanner data.
        </p>
      </div>

      {/* ── Import ─────────────────────────────────────── */}
      <section>
        <h2 className="flex items-center gap-2 text-base font-semibold mb-3">
          <Upload className="w-4 h-4" /> Import
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {/* Import Talkgroups */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Talkgroups (CSV)</h3>
              <select
                value={selectedTgSystemId}
                onChange={(e) => setSelectedTgSystemId(e.target.value)}
                className="select select-bordered select-sm w-full"
              >
                <option value="">Select a system…</option>
                {systems?.map((sys) => (
                  <option key={sys.id} value={sys.id}>
                    {sys.label}
                  </option>
                ))}
              </select>
              <select
                value={tgImportMode}
                onChange={(e) =>
                  setTgImportMode(e.target.value as "overwrite" | "skip")
                }
                className="select select-bordered select-sm w-full"
              >
                <option value="overwrite">Overwrite existing</option>
                <option value="skip">Skip existing</option>
              </select>
              <input
                ref={tgFileRef}
                type="file"
                accept=".csv"
                className="file-input file-input-bordered file-input-sm w-full"
              />
              <p className="text-xs text-base-content/50">
                Supports OpenScanner and rdio-scanner CSV formats. Headers are
                auto-detected; tag and group names are resolved automatically.
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleImportTalkgroups}
                disabled={!selectedTgSystemId}
              >
                Upload
              </button>
            </div>
          </div>

          {/* Import Units */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Units (CSV)</h3>
              <select
                value={selectedUnitSystemId}
                onChange={(e) => setSelectedUnitSystemId(e.target.value)}
                className="select select-bordered select-sm w-full"
              >
                <option value="">Select a system…</option>
                {systems?.map((sys) => (
                  <option key={sys.id} value={sys.id}>
                    {sys.label}
                  </option>
                ))}
              </select>
              <select
                value={unitImportMode}
                onChange={(e) =>
                  setUnitImportMode(e.target.value as "overwrite" | "skip")
                }
                className="select select-bordered select-sm w-full"
              >
                <option value="overwrite">Overwrite existing</option>
                <option value="skip">Skip existing</option>
              </select>
              <input
                ref={unitFileRef}
                type="file"
                accept=".csv"
                className="file-input file-input-bordered file-input-sm w-full"
              />
              <p className="text-xs text-base-content/50">
                Columns: unit_id, label, order
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleImportUnits}
                disabled={!selectedUnitSystemId}
              >
                Upload
              </button>
            </div>
          </div>

          {/* Import Groups */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Groups (CSV)</h3>
              <input
                ref={groupsFileRef}
                type="file"
                accept=".csv"
                className="file-input file-input-bordered file-input-sm w-full"
              />
              <p className="text-xs text-base-content/50">
                One label per line, optional 'label' header. Existing labels are
                skipped.
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleImportGroups}
              >
                Upload
              </button>
            </div>
          </div>

          {/* Import Tags */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Tags (CSV)</h3>
              <input
                ref={tagsFileRef}
                type="file"
                accept=".csv"
                className="file-input file-input-bordered file-input-sm w-full"
              />
              <p className="text-xs text-base-content/50">
                One label per line, optional 'label' header. Existing labels are
                skipped.
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleImportTags}
              >
                Upload
              </button>
            </div>
          </div>

          {/* Import Config */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Server Config (JSON)</h3>
              <input
                ref={configFileRef}
                type="file"
                accept=".json"
                className="file-input file-input-bordered file-input-sm w-full"
              />
              <p className="text-xs text-base-content/50">
                Restore a full server configuration from a previously exported
                JSON backup.
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleImportConfig}
              >
                Upload
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* ── Export ─────────────────────────────────────── */}
      <section>
        <h2 className="flex items-center gap-2 text-base font-semibold mb-3">
          <Download className="w-4 h-4" /> Export
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {/* Export Talkgroups */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Talkgroups (CSV)</h3>
              <select
                value={exportTgSystemId}
                onChange={(e) => setExportTgSystemId(e.target.value)}
                className="select select-bordered select-sm w-full"
              >
                <option value="">Select a system…</option>
                {systems?.map((sys) => (
                  <option key={sys.id} value={sys.id}>
                    {sys.label}
                  </option>
                ))}
              </select>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleExportTalkgroups}
                disabled={!exportTgSystemId}
              >
                Download CSV
              </button>
            </div>
          </div>

          {/* Export Units */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Units (CSV)</h3>
              <select
                value={exportUnitSystemId}
                onChange={(e) => setExportUnitSystemId(e.target.value)}
                className="select select-bordered select-sm w-full"
              >
                <option value="">Select a system…</option>
                {systems?.map((sys) => (
                  <option key={sys.id} value={sys.id}>
                    {sys.label}
                  </option>
                ))}
              </select>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleExportUnits}
                disabled={!exportUnitSystemId}
              >
                Download CSV
              </button>
            </div>
          </div>

          {/* Export Groups */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Groups (CSV)</h3>
              <p className="text-xs text-base-content/50">
                All talkgroup groups (one label per row).
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleExportGroups}
              >
                Download CSV
              </button>
            </div>
          </div>

          {/* Export Tags */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Tags (CSV)</h3>
              <p className="text-xs text-base-content/50">
                All talkgroup tags (one label per row).
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleExportTags}
              >
                Download CSV
              </button>
            </div>
          </div>

          {/* Export Config */}
          <div className="card bg-base-200">
            <div className="card-body gap-3">
              <h3 className="card-title text-sm">Server Config (JSON)</h3>
              <p className="text-xs text-base-content/50">
                Download a full snapshot of systems, talkgroups, units, groups,
                tags, and settings.
              </p>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleExportConfig}
              >
                Download JSON
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* ── Enrich ─────────────────────────────────────── */}
      <section>
        <h2 className="flex items-center gap-2 text-base font-semibold mb-3">
          <Database className="w-4 h-4" /> Enrich
        </h2>
        <RadioReferenceCard />
      </section>

      {/* ── API Docs ───────────────────────────────────── */}
      <section>
        <h2 className="flex items-center gap-2 text-base font-semibold mb-3">
          <FileText className="w-4 h-4" /> API Documentation
        </h2>
        <div className="card bg-base-200">
          <div className="card-body gap-3">
            <p className="text-sm text-base-content/70">
              Interactive Swagger UI for exploring and testing API endpoints.
              Sessions expire after 1 hour.
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
      </section>

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
