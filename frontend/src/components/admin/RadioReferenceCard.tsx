import { useState, useRef, useCallback } from "react";
import { Upload, CheckCircle, XCircle, AlertTriangle } from "lucide-react";
import { useRrPreviewCSVMutation } from "@/app/slices/adminSlice";
import { useRrApplyMutation, useListSystemsQuery } from "@/hooks/admin/useAdminWsOps";
import type {
  RRPreviewResponse,
  RRPreviewRow,
  RRTalkgroupCandidate,
  RRApplyResponse,
} from "@/types";

const UPDATABLE_FIELDS = ["label", "name", "group", "tag", "led", "order"];

export default function RadioReferenceCard() {
  const { data: localSystems } = useListSystemsQuery();
  const [previewCSV] = useRrPreviewCSVMutation();
  const [applyEnrichment, { isLoading: applying }] = useRrApplyMutation();

  const [systemId, setSystemId] = useState("");
  const [preview, setPreview] = useState<RRPreviewResponse | null>(null);
  const [applyResult, setApplyResult] = useState<RRApplyResponse | null>(null);
  const [mergeMode, setMergeMode] = useState<
    "fill_missing" | "overwrite_selected"
  >("fill_missing");
  const [selectedFields, setSelectedFields] = useState<string[]>(
    () => UPDATABLE_FIELDS,
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // CSV state
  const fileRef = useRef<HTMLInputElement>(null);
  const [hasFile, setHasFile] = useState(false);

  const showError = useCallback((msg: string) => {
    setError(msg);
    setTimeout(() => setError(null), 6000);
  }, []);

  const extractError = (err: unknown, fallback: string): string => {
    if (
      err != null &&
      typeof err === "object" &&
      "data" in err &&
      err.data != null &&
      typeof err.data === "object" &&
      "error" in err.data &&
      typeof (err.data as Record<string, unknown>).error === "string"
    ) {
      return (err.data as Record<string, unknown>).error as string;
    }
    return fallback;
  };

  const toggleField = (field: string) => {
    setSelectedFields((prev) =>
      prev.includes(field) ? prev.filter((f) => f !== field) : [...prev, field],
    );
  };

  // --- CSV handlers ---
  const handleCSVPreview = async () => {
    const file = fileRef.current?.files?.[0];
    if (!file) {
      showError("Please select a CSV file");
      return;
    }
    if (!systemId) {
      showError("Please select a system");
      return;
    }
    setLoading(true);
    setPreview(null);
    setApplyResult(null);
    setError(null);
    try {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("system_id", systemId);
      const result = await previewCSV(formData).unwrap();
      setPreview(result);
    } catch (err: unknown) {
      showError(extractError(err, "Failed to parse CSV or preview enrichment"));
    } finally {
      setLoading(false);
    }
  };

  // --- Shared handlers ---
  const handleApply = async () => {
    if (!preview || !systemId) return;

    const candidates: RRTalkgroupCandidate[] = preview.rows
      .filter((r) => r.matched && r.wouldUpdate)
      .map((r) => ({
        row: r.row,
        talkgroupId: r.talkgroupId,
        label: r.label,
        name: r.name,
        group: r.group,
        tag: r.tag,
        led: r.led,
        order: r.order,
      }));

    if (candidates.length === 0) {
      showError("No rows to update");
      return;
    }

    try {
      const result = await applyEnrichment({
        systemId: Number(systemId),
        candidates,
        mergeMode,
        selectedFields:
          mergeMode === "overwrite_selected" ? selectedFields : [],
      }).unwrap();
      setApplyResult(result);
      setPreview(null);
    } catch (err: unknown) {
      showError(extractError(err, "Failed to apply enrichment"));
    }
  };

  const handleReset = () => {
    setPreview(null);
    setApplyResult(null);
    setError(null);
    if (fileRef.current) fileRef.current.value = "";
    setHasFile(false);
  };

  const updatableCount = preview?.rows.filter((r) => r.wouldUpdate).length ?? 0;
  const showControls = !preview && !applyResult;

  return (
    <div className="card bg-base-200 mb-4">
      <div className="card-body">
        <h2 className="card-title text-base">
          <Upload className="w-4 h-4" /> RadioReference CSV Enrichment
        </h2>
        <p className="text-sm text-base-content/70">
          Fill in missing talkgroup details (label, name, group, tag) from
          RadioReference data using a CSV export file. Frequency is never
          updated. Talkgroups are matched by their decimal ID within the
          selected system.
        </p>

        {/* Controls */}
        {showControls && (
          <div className="space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div>
                <label className="label">
                  <span className="label-text text-sm">Local System</span>
                </label>
                <select
                  value={systemId}
                  onChange={(e) => setSystemId(e.target.value)}
                  className="select select-bordered select-sm w-full"
                >
                  <option value="">--- Select a system ---</option>
                  {localSystems?.map((sys) => (
                    <option key={sys.id} value={sys.id}>
                      {sys.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex items-end">
                <input
                  ref={fileRef}
                  type="file"
                  accept=".csv"
                  onChange={(e) => setHasFile(!!e.target.files?.length)}
                  className="file-input file-input-bordered file-input-sm w-full"
                />
              </div>
            </div>
            <MergeControls
              mergeMode={mergeMode}
              setMergeMode={setMergeMode}
              selectedFields={selectedFields}
              toggleField={toggleField}
            />
            <button
              className="btn btn-primary btn-sm w-full"
              onClick={handleCSVPreview}
              disabled={loading || !systemId || !hasFile}
            >
              {loading ? "Parsing..." : "Preview Enrichment"}
            </button>
          </div>
        )}

        {/* Preview table (shared) */}
        {preview && !applyResult && (
          <PreviewTable
            preview={preview}
            updatableCount={updatableCount}
            applying={applying}
            onApply={handleApply}
            onCancel={handleReset}
          />
        )}

        {/* Apply result (shared) */}
        {applyResult && (
          <ApplyResult result={applyResult} onDone={handleReset} />
        )}

        {error && (
          <div className="alert alert-error text-sm mt-2">
            <span>{error}</span>
          </div>
        )}
      </div>
    </div>
  );
}

// --- Extracted shared sub-components ---

function MergeControls({
  mergeMode,
  setMergeMode,
  selectedFields,
  toggleField,
}: {
  mergeMode: string;
  setMergeMode: (m: "fill_missing" | "overwrite_selected") => void;
  selectedFields: string[];
  toggleField: (f: string) => void;
}) {
  return (
    <>
      <div>
        <label className="label">
          <span className="label-text text-sm">Update Mode</span>
        </label>
        <select
          value={mergeMode}
          onChange={(e) =>
            setMergeMode(
              e.target.value as "fill_missing" | "overwrite_selected",
            )
          }
          className="select select-bordered select-sm w-full md:w-auto"
        >
          <option value="fill_missing">Fill missing only (default)</option>
          <option value="overwrite_selected">Overwrite selected fields</option>
        </select>
      </div>
      {mergeMode === "overwrite_selected" && (
        <div>
          <label className="label">
            <span className="label-text text-sm">Fields to overwrite</span>
          </label>
          <div className="flex flex-wrap gap-2">
            {UPDATABLE_FIELDS.map((field) => (
              <label
                key={field}
                className="flex items-center gap-1 cursor-pointer"
              >
                <input
                  type="checkbox"
                  className="checkbox checkbox-xs"
                  checked={selectedFields.includes(field)}
                  onChange={() => toggleField(field)}
                />
                <span className="text-sm capitalize">{field}</span>
              </label>
            ))}
          </div>
        </div>
      )}
    </>
  );
}

const PREVIEW_PAGE_SIZE = 50;

function PreviewTable({
  preview,
  updatableCount,
  applying,
  onApply,
  onCancel,
}: {
  preview: RRPreviewResponse;
  updatableCount: number;
  applying: boolean;
  onApply: () => void;
  onCancel: () => void;
}) {
  const [page, setPage] = useState(1);
  const totalPages = Math.max(
    1,
    Math.ceil(preview.rows.length / PREVIEW_PAGE_SIZE),
  );
  const pageRows = preview.rows.slice(
    (page - 1) * PREVIEW_PAGE_SIZE,
    page * PREVIEW_PAGE_SIZE,
  );

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-4 text-sm">
        <span>
          Processed: <strong>{preview.processed}</strong>
        </span>
        <span>
          Matched: <strong>{preview.matched}</strong>
        </span>
        <span className="text-success">
          Would update: <strong>{updatableCount}</strong>
        </span>
        <span>
          Skipped: <strong>{preview.skipped}</strong>
        </span>
        {preview.errors > 0 && (
          <span className="text-error">
            Errors: <strong>{preview.errors}</strong>
          </span>
        )}
      </div>

      {preview.rows.length > 0 && (
        <div className="space-y-2">
          <div className="overflow-x-auto max-h-96">
            <table className="table table-zebra table-xs">
              <thead>
                <tr>
                  <th>Row</th>
                  <th>TG ID</th>
                  <th>Status</th>
                  <th>Label</th>
                  <th>Name</th>
                  <th>Group</th>
                  <th>Tag</th>
                  <th>Fields</th>
                </tr>
              </thead>
              <tbody>
                {pageRows.map((row: RRPreviewRow) => (
                  <tr key={row.row}>
                    <td>{row.row}</td>
                    <td>{row.talkgroupId}</td>
                    <td>
                      {row.wouldUpdate ? (
                        <CheckCircle className="w-4 h-4 text-success inline" />
                      ) : (
                        <span className="text-base-content/50">no change</span>
                      )}
                    </td>
                    <td className="max-w-32 truncate">{row.label ?? "-"}</td>
                    <td className="max-w-40 truncate">{row.name ?? "-"}</td>
                    <td>{row.group ?? "-"}</td>
                    <td>{row.tag ?? "-"}</td>
                    <td className="text-xs">
                      {row.wouldUpdateFields?.join(", ") || "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {totalPages > 1 && (
            <div className="flex items-center justify-between text-sm">
              <button
                className="btn btn-ghost btn-xs"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
              >
                ← Prev
              </button>
              <span className="text-base-content/70">
                Page {page} of {totalPages} ({preview.rows.length} rows)
              </span>
              <button
                className="btn btn-ghost btn-xs"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
              >
                Next →
              </button>
            </div>
          )}
        </div>
      )}

      {preview.rowErrors.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer text-error">
            <AlertTriangle className="w-4 h-4 inline" />{" "}
            {preview.rowErrors.length} row errors
          </summary>
          <ul className="list-disc list-inside text-xs mt-1 max-h-32 overflow-auto">
            {preview.rowErrors.map((e, i) => (
              <li key={i}>
                Row {e.row}: {e.reason}
              </li>
            ))}
          </ul>
        </details>
      )}

      <div className="flex gap-2">
        <button
          className="btn btn-primary btn-sm flex-1"
          onClick={onApply}
          disabled={applying || updatableCount === 0}
        >
          {applying
            ? "Applying..."
            : `Apply ${updatableCount} Update${updatableCount !== 1 ? "s" : ""}`}
        </button>
        <button className="btn btn-ghost btn-sm" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  );
}

function ApplyResult({
  result,
  onDone,
}: {
  result: RRApplyResponse;
  onDone: () => void;
}) {
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-4 text-sm">
        <span>
          Processed: <strong>{result.processed}</strong>
        </span>
        <span>
          Matched: <strong>{result.matched}</strong>
        </span>
        <span className="text-success">
          Updated: <strong>{result.updated}</strong>
        </span>
        <span>
          Skipped: <strong>{result.skipped}</strong>
        </span>
        {result.errors > 0 && (
          <span className="text-error">
            Errors: <strong>{result.errors}</strong>
          </span>
        )}
      </div>

      {result.rowErrors?.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer text-error">
            <XCircle className="w-4 h-4 inline" /> {result.rowErrors.length} row
            errors
          </summary>
          <ul className="list-disc list-inside text-xs mt-1 max-h-32 overflow-auto">
            {result.rowErrors.map((e, i) => (
              <li key={i}>
                Row {e.row}: {e.reason}
              </li>
            ))}
          </ul>
        </details>
      )}

      <button className="btn btn-ghost btn-sm" onClick={onDone}>
        Done
      </button>
    </div>
  );
}
