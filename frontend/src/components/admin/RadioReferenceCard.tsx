import { useState, useRef, useCallback } from "react";
import {
  Upload,
  Globe,
  CheckCircle,
  XCircle,
  AlertTriangle,
  LogIn,
} from "lucide-react";
import {
  useRrPreviewCSVMutation,
  useRrApplyMutation,
  useRrLoginMutation,
  useLazyRrCountriesQuery,
  useLazyRrStatesQuery,
  useLazyRrCountiesQuery,
  useLazyRrSystemsQuery,
  useRrPreviewAPIMutation,
  useListSystemsQuery,
} from "@/app/slices/adminSlice";
import type {
  RRPreviewResponse,
  RRPreviewRow,
  RRTalkgroupCandidate,
  RRApplyResponse,
  RRCountry,
  RRState,
  RRCounty,
  RRSystem,
} from "@/types";

const UPDATABLE_FIELDS = ["label", "name", "group", "tag", "led", "order"];
type Mode = "csv" | "api";

export default function RadioReferenceCard() {
  const { data: localSystems } = useListSystemsQuery();
  const [previewCSV] = useRrPreviewCSVMutation();
  const [previewAPI] = useRrPreviewAPIMutation();
  const [applyEnrichment, { isLoading: applying }] = useRrApplyMutation();
  const [rrLogin] = useRrLoginMutation();
  const [fetchCountries] = useLazyRrCountriesQuery();
  const [fetchStates] = useLazyRrStatesQuery();
  const [fetchCounties] = useLazyRrCountiesQuery();
  const [fetchSystems] = useLazyRrSystemsQuery();

  // Shared state
  const [mode, setMode] = useState<Mode>("csv");
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

  // API state
  const [rrSession, setRrSession] = useState<string | null>(null);
  const [rrUsername, setRrUsername] = useState("");
  const [rrPassword, setRrPassword] = useState("");
  const [loginLoading, setLoginLoading] = useState(false);
  const [countries, setCountries] = useState<RRCountry[]>([]);
  const [states, setStates] = useState<RRState[]>([]);
  const [counties, setCounties] = useState<RRCounty[]>([]);
  const [rrSystems, setRrSystems] = useState<RRSystem[]>([]);
  const [selectedCountry, setSelectedCountry] = useState("");
  const [selectedState, setSelectedState] = useState("");
  const [selectedCounty, setSelectedCounty] = useState("");
  const [selectedRrSystem, setSelectedRrSystem] = useState("");
  const [hierarchyLoading, setHierarchyLoading] = useState(false);

  const showError = useCallback((msg: string) => {
    setError(msg);
    setTimeout(() => setError(null), 6000);
  }, []);

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
    } catch {
      showError("Failed to parse CSV or preview enrichment");
    } finally {
      setLoading(false);
    }
  };

  // --- API handlers ---
  const handleLogin = async () => {
    if (!rrUsername || !rrPassword) {
      showError("Username and password are required");
      return;
    }
    setLoginLoading(true);
    setError(null);
    try {
      const result = await rrLogin({
        username: rrUsername,
        password: rrPassword,
      }).unwrap();
      setRrSession(result.rrSession);
      setRrPassword(""); // Clear password from state after login
      // Auto-load countries
      const countriesResult = await fetchCountries().unwrap();
      setCountries(countriesResult);
    } catch {
      showError("RadioReference login failed — check credentials and API key");
    } finally {
      setLoginLoading(false);
    }
  };

  const handleCountryChange = async (countryId: string) => {
    setSelectedCountry(countryId);
    setStates([]);
    setCounties([]);
    setRrSystems([]);
    setSelectedState("");
    setSelectedCounty("");
    setSelectedRrSystem("");
    if (!countryId || !rrSession) return;
    setHierarchyLoading(true);
    try {
      const result = await fetchStates({
        countryId: Number(countryId),
        rrSession,
      }).unwrap();
      setStates(result);
    } catch {
      showError("Failed to fetch states");
    } finally {
      setHierarchyLoading(false);
    }
  };

  const handleStateChange = async (stateId: string) => {
    setSelectedState(stateId);
    setCounties([]);
    setRrSystems([]);
    setSelectedCounty("");
    setSelectedRrSystem("");
    if (!stateId || !rrSession) return;
    setHierarchyLoading(true);
    try {
      const result = await fetchCounties({
        stateId: Number(stateId),
        rrSession,
      }).unwrap();
      setCounties(result);
    } catch {
      showError("Failed to fetch counties");
    } finally {
      setHierarchyLoading(false);
    }
  };

  const handleCountyChange = async (countyId: string) => {
    setSelectedCounty(countyId);
    setRrSystems([]);
    setSelectedRrSystem("");
    if (!countyId || !rrSession) return;
    setHierarchyLoading(true);
    try {
      const result = await fetchSystems({
        countyId: Number(countyId),
        rrSession,
      }).unwrap();
      setRrSystems(result);
    } catch {
      showError("Failed to fetch systems");
    } finally {
      setHierarchyLoading(false);
    }
  };

  const handleAPIPreview = async () => {
    if (!selectedRrSystem || !systemId || !rrSession) {
      showError("Please select both a RadioReference system and a local system");
      return;
    }
    setLoading(true);
    setPreview(null);
    setApplyResult(null);
    setError(null);
    try {
      const result = await previewAPI({
        body: {
          rrSystemId: Number(selectedRrSystem),
          systemId: Number(systemId),
        },
        rrSession,
      }).unwrap();
      setPreview(result);
    } catch {
      showError("Failed to fetch talkgroups from RadioReference");
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
    } catch {
      showError("Failed to apply enrichment");
    }
  };

  const handleReset = () => {
    setPreview(null);
    setApplyResult(null);
    setError(null);
    if (fileRef.current) fileRef.current.value = "";
  };

  const handleLogout = () => {
    setRrSession(null);
    setRrUsername("");
    setRrPassword("");
    setCountries([]);
    setStates([]);
    setCounties([]);
    setRrSystems([]);
    setSelectedCountry("");
    setSelectedState("");
    setSelectedCounty("");
    setSelectedRrSystem("");
    handleReset();
  };

  const updatableCount = preview?.rows.filter((r) => r.wouldUpdate).length ?? 0;
  const showControls = !preview && !applyResult;

  return (
    <div className="card bg-base-200 mb-4">
      <div className="card-body">
        <h2 className="card-title text-base">
          <Globe className="w-4 h-4" /> RadioReference Enrichment
        </h2>
        <p className="text-sm text-base-content/70">
          Fill in missing talkgroup details (label, name, group, tag) from
          RadioReference data. Use CSV mode to upload a RadioReference export
          file, or API mode to browse and fetch directly. Frequency is never
          updated. Talkgroups are matched by their decimal ID within the
          selected system.
        </p>

        {/* Mode selector — only when not in preview/result state */}
        {showControls && (
          <>
            <div className="tabs tabs-boxed w-fit">
              <button
                className={`tab tab-sm ${mode === "csv" ? "tab-active" : ""}`}
                onClick={() => setMode("csv")}
              >
                <Upload className="w-3 h-3 mr-1" /> CSV Upload
              </button>
              <button
                className={`tab tab-sm ${mode === "api" ? "tab-active" : ""}`}
                onClick={() => setMode("api")}
              >
                <Globe className="w-3 h-3 mr-1" /> API Browse
              </button>
            </div>

            {/* --- CSV Mode Controls --- */}
            {mode === "csv" && (
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
                  disabled={loading}
                >
                  {loading ? "Parsing..." : "Preview Enrichment"}
                </button>
              </div>
            )}

            {/* --- API Mode Controls --- */}
            {mode === "api" && !rrSession && (
              <div className="space-y-3">
                <p className="text-xs text-base-content/70">
                  Log in with your RadioReference premium account. Credentials
                  are used only for this session and are not stored.
                </p>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <input
                    type="text"
                    placeholder="RR Username"
                    value={rrUsername}
                    onChange={(e) => setRrUsername(e.target.value)}
                    className="input input-bordered input-sm w-full"
                    autoComplete="off"
                  />
                  <input
                    type="password"
                    placeholder="RR Password"
                    value={rrPassword}
                    onChange={(e) => setRrPassword(e.target.value)}
                    className="input input-bordered input-sm w-full"
                    autoComplete="off"
                    onKeyDown={(e) => e.key === "Enter" && handleLogin()}
                  />
                </div>
                <button
                  className="btn btn-primary btn-sm w-full"
                  onClick={handleLogin}
                  disabled={loginLoading}
                >
                  <LogIn className="w-4 h-4" />
                  {loginLoading ? "Logging in..." : "Log in to RadioReference"}
                </button>
              </div>
            )}

            {mode === "api" && rrSession && (
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-success">
                    Logged in as <strong>{rrUsername}</strong>
                  </span>
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={handleLogout}
                  >
                    Log out
                  </button>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {/* Country */}
                  <div>
                    <label className="label">
                      <span className="label-text text-sm">Country</span>
                    </label>
                    <select
                      value={selectedCountry}
                      onChange={(e) => handleCountryChange(e.target.value)}
                      className="select select-bordered select-sm w-full"
                      disabled={hierarchyLoading}
                    >
                      <option value="">--- Select country ---</option>
                      {countries.map((c) => (
                        <option key={c.coid} value={c.coid}>
                          {c.countryName}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* State */}
                  <div>
                    <label className="label">
                      <span className="label-text text-sm">
                        State / Province
                      </span>
                    </label>
                    <select
                      value={selectedState}
                      onChange={(e) => handleStateChange(e.target.value)}
                      className="select select-bordered select-sm w-full"
                      disabled={!selectedCountry || hierarchyLoading}
                    >
                      <option value="">--- Select state ---</option>
                      {states.map((s) => (
                        <option key={s.stid} value={s.stid}>
                          {s.stateName}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* County */}
                  <div>
                    <label className="label">
                      <span className="label-text text-sm">County</span>
                    </label>
                    <select
                      value={selectedCounty}
                      onChange={(e) => handleCountyChange(e.target.value)}
                      className="select select-bordered select-sm w-full"
                      disabled={!selectedState || hierarchyLoading}
                    >
                      <option value="">--- Select county ---</option>
                      {counties.map((c) => (
                        <option key={c.ctid} value={c.ctid}>
                          {c.countyName}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* RR System */}
                  <div>
                    <label className="label">
                      <span className="label-text text-sm">
                        RR Trunked System
                      </span>
                    </label>
                    <select
                      value={selectedRrSystem}
                      onChange={(e) => setSelectedRrSystem(e.target.value)}
                      className="select select-bordered select-sm w-full"
                      disabled={!selectedCounty || hierarchyLoading}
                    >
                      <option value="">--- Select system ---</option>
                      {rrSystems.map((s) => (
                        <option key={s.sid} value={s.sid}>
                          {s.sName}
                          {s.sCity ? ` (${s.sCity})` : ""}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                {/* Local system to match against */}
                <div>
                  <label className="label">
                    <span className="label-text text-sm">
                      Local System (match target)
                    </span>
                  </label>
                  <select
                    value={systemId}
                    onChange={(e) => setSystemId(e.target.value)}
                    className="select select-bordered select-sm w-full"
                  >
                    <option value="">--- Select local system ---</option>
                    {localSystems?.map((sys) => (
                      <option key={sys.id} value={sys.id}>
                        {sys.label}
                      </option>
                    ))}
                  </select>
                </div>

                <MergeControls
                  mergeMode={mergeMode}
                  setMergeMode={setMergeMode}
                  selectedFields={selectedFields}
                  toggleField={toggleField}
                />
                <button
                  className="btn btn-primary btn-sm w-full"
                  onClick={handleAPIPreview}
                  disabled={
                    loading || !selectedRrSystem || !systemId || !rrSession
                  }
                >
                  {loading ? "Fetching..." : "Fetch & Preview Enrichment"}
                </button>
              </div>
            )}
          </>
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
          <option value="overwrite_selected">
            Overwrite selected fields
          </option>
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
        <div className="overflow-x-auto max-h-64">
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
              {preview.rows.slice(0, 100).map((row: RRPreviewRow) => (
                <tr key={row.row}>
                  <td>{row.row}</td>
                  <td>{row.talkgroupId}</td>
                  <td>
                    {row.wouldUpdate ? (
                      <CheckCircle className="w-4 h-4 text-success inline" />
                    ) : row.matched ? (
                      <span className="text-base-content/50">no change</span>
                    ) : (
                      <span className="text-warning text-xs">
                        {row.skipReason ?? "skip"}
                      </span>
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
          {preview.rows.length > 100 && (
            <p className="text-xs text-base-content/70 mt-1">
              Showing first 100 of {preview.rows.length} rows.
            </p>
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
            <XCircle className="w-4 h-4 inline" /> {result.rowErrors.length}{" "}
            row errors
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
