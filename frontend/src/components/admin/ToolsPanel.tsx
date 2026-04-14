import { useState, useRef, useCallback } from "react";
import { Upload, Download, ExternalLink } from "lucide-react";
import {
  useImportTalkgroupsMutation,
  useImportUnitsMutation,
  useLazyExportConfigQuery,
  useImportConfigMutation,
  useLazyGetMissingAudioCallsQuery,
  useCleanupMissingAudioCallsMutation,
  useListSystemsQuery,
  type MissingAudioResponse,
} from "@/app/slices/adminSlice";
import { selectToken } from "@/app/slices/authSlice";
import { useAppSelector } from "@/app/store";

const SWAGGER_URL = "/api/admin/docs/index.html";

export default function ToolsPanel() {
  const token = useAppSelector(selectToken);
  const [importTalkgroups] = useImportTalkgroupsMutation();
  const [importUnits] = useImportUnitsMutation();
  const [triggerExport] = useLazyExportConfigQuery();
  const [importConfig] = useImportConfigMutation();
  const [getMissingAudioCalls, { isFetching: scanningMissingAudio }] =
    useLazyGetMissingAudioCallsQuery();
  const [cleanupMissingAudioCalls, { isLoading: cleaningMissingAudio }] =
    useCleanupMissingAudioCallsMutation();
  const { data: systems } = useListSystemsQuery();

  const [toast, setToast] = useState<string | null>(null);
  const [toastType, setToastType] = useState<"error" | "success">("error");
  const tgFileRef = useRef<HTMLInputElement>(null);
  const unitFileRef = useRef<HTMLInputElement>(null);
  const configFileRef = useRef<HTMLInputElement>(null);
  const [selectedSystemId, setSelectedSystemId] = useState<string>("");
  const [unitImportMode, setUnitImportMode] = useState<"overwrite" | "skip">(
    "overwrite",
  );

  const [missingAudioResult, setMissingAudioResult] =
    useState<MissingAudioResponse | null>(null);
  const [confirmMissingCleanup, setConfirmMissingCleanup] = useState(false);
  const [scanProgress, setScanProgress] = useState<{
    checked: number;
    total: number;
  } | null>(null);

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
    if (!selectedSystemId) {
      showToast("Please select a system");
      return;
    }
    const formData = new FormData();
    formData.append("file", file);
    formData.append("system_id", selectedSystemId);
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

  const handleScanMissingAudio = async () => {
    const PAGE_SIZE = 500;
    let offset = 0;
    const allMissing: MissingAudioResponse["missing"] = [];
    let totalChecked = 0;
    let lastResult!: MissingAudioResponse;
    setScanProgress({ checked: 0, total: 0 });

    try {
      for (;;) {
        const result = await getMissingAudioCalls({
          limit: PAGE_SIZE,
          offset,
        }).unwrap();
        lastResult = result;
        totalChecked += result.checked;
        allMissing.push(...result.missing);
        setScanProgress({ checked: totalChecked, total: result.totalCalls });

        if (result.checked < PAGE_SIZE || totalChecked >= result.totalCalls)
          break;
        offset += PAGE_SIZE;
      }

      const combined: MissingAudioResponse = {
        recordingsDir: lastResult.recordingsDir,
        limit: totalChecked,
        offset: 0,
        totalCalls: lastResult.totalCalls,
        checked: totalChecked,
        missing: allMissing,
      };
      setMissingAudioResult(combined);
      showToast(
        `Scan complete: ${allMissing.length} missing in ${totalChecked} checked calls`,
        "success",
      );
    } catch {
      showToast("Failed to scan for missing audio files");
    } finally {
      setScanProgress(null);
    }
  };

  const handleCleanupMissingAudio = async () => {
    if (!missingAudioResult || missingAudioResult.missing.length === 0) {
      showToast("No missing rows to delete");
      return;
    }
    if (!confirmMissingCleanup) {
      showToast("Please confirm deletion first");
      return;
    }
    try {
      const callIds = missingAudioResult.missing.map((row) => row.id);
      const result = await cleanupMissingAudioCalls({
        confirm: true,
        callIds,
      }).unwrap();
      showToast(
        `Cleanup complete: deleted ${result.deleted} of ${result.requested}`,
        "success",
      );
      setMissingAudioResult(null);
      setConfirmMissingCleanup(false);
    } catch {
      showToast("Failed to delete missing audio rows");
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
              className="file-input file-input-sm"
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
          <div className="space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <div>
                <label className="label">
                  <span className="label-text text-sm">System</span>
                </label>
                <select
                  value={selectedSystemId}
                  onChange={(e) => setSelectedSystemId(e.target.value)}
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

      {/* Missing audio scan */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <h2 className="card-title text-base">Find Missing Call Audio</h2>
          <p className="text-sm text-base-content/70">
            Scan recent archived calls and report DB records whose stored audio
            file is missing from the configured base directory.
          </p>
          <div className="flex items-center gap-3 flex-wrap">
            <button
              className="btn btn-primary"
              onClick={handleScanMissingAudio}
              disabled={scanningMissingAudio}
            >
              {scanningMissingAudio
                ? scanProgress
                  ? `Scanning... ${scanProgress.checked} / ${scanProgress.total}`
                  : "Scanning..."
                : "Scan Missing Audio"}
            </button>
            <label className="flex items-center cursor-pointer gap-2">
              <input
                type="checkbox"
                className="checkbox checkbox-sm"
                checked={confirmMissingCleanup}
                onChange={(e) => setConfirmMissingCleanup(e.target.checked)}
              />
              <span className="text-sm">Confirm delete missing rows</span>
            </label>
            <button
              className="btn btn-error"
              onClick={handleCleanupMissingAudio}
              disabled={
                cleaningMissingAudio ||
                !missingAudioResult ||
                missingAudioResult.missing.length === 0
              }
            >
              {cleaningMissingAudio ? "Deleting..." : "Delete Missing Rows"}
            </button>
            {missingAudioResult && (
              <span className="text-sm text-base-content/70">
                Recordings dir: {missingAudioResult.recordingsDir} | Checked:{" "}
                {missingAudioResult.checked} / {missingAudioResult.totalCalls} |
                Missing: {missingAudioResult.missing.length}
              </span>
            )}
          </div>
          {missingAudioResult && missingAudioResult.missing.length > 0 && (
            <div className="overflow-x-auto mt-3">
              <table className="table table-zebra table-xs">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Audio Name</th>
                    <th>Audio Path</th>
                    <th>Reason</th>
                  </tr>
                </thead>
                <tbody>
                  {missingAudioResult.missing.slice(0, 50).map((row) => (
                    <tr key={row.id}>
                      <td>{row.id}</td>
                      <td>{row.audioName || "-"}</td>
                      <td className="font-mono text-xs">{row.audioPath}</td>
                      <td>{row.reason}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {missingAudioResult.missing.length > 50 && (
                <p className="text-xs text-base-content/70 mt-2">
                  Showing first 50 missing rows.
                </p>
              )}
            </div>
          )}
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
