import { useState, useEffect, useCallback } from "react";
import {
  Wifi,
  WifiOff,
  Trash2,
  Download,
  Check,
  Loader2,
  Mic,
} from "lucide-react";
import {
  useTranscriptionStatusQuery,
  useTranscriptionModelsQuery,
  useTranscriptionDownloadMutation,
  useTranscriptionDeleteMutation,
  useTranscriptionStatsQuery,
  useUpdateConfigMutation,
} from "@/hooks/useAdminWsOps";
import type { WhisperModel } from "@/types";

const KNOWN_MODELS = [
  "ggml-tiny",
  "ggml-tiny.en",
  "ggml-base",
  "ggml-base.en",
  "ggml-small",
  "ggml-small.en",
  "ggml-medium",
  "ggml-medium.en",
  "ggml-large-v3",
  "ggml-large-v3-turbo",
  "ggml-small.en-tdrz",
];

const LANGUAGES = [
  { value: "auto", label: "Auto-detect" },
  { value: "en", label: "English" },
  { value: "es", label: "Spanish" },
  { value: "fr", label: "French" },
  { value: "de", label: "German" },
  { value: "it", label: "Italian" },
  { value: "pt", label: "Portuguese" },
  { value: "nl", label: "Dutch" },
  { value: "ja", label: "Japanese" },
  { value: "zh", label: "Chinese" },
  { value: "ko", label: "Korean" },
  { value: "ru", label: "Russian" },
  { value: "ar", label: "Arabic" },
  { value: "pl", label: "Polish" },
  { value: "sv", label: "Swedish" },
];

export default function TranscriptionPanel() {
  const {
    data: status,
    isLoading: statusLoading,
    refetch: refetchStatus,
  } = useTranscriptionStatusQuery();
  const {
    data: modelsData,
    isLoading: modelsLoading,
    refetch: refetchModels,
  } = useTranscriptionModelsQuery();
  const [downloadModel, { isLoading: isDownloading }] =
    useTranscriptionDownloadMutation();
  const [deleteModel] = useTranscriptionDeleteMutation();
  const [updateConfig] = useUpdateConfigMutation();
  const { data: stats } = useTranscriptionStatsQuery();

  const [url, setUrl] = useState("");
  const [language, setLanguage] = useState("en");
  const [diarize, setDiarize] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const [toastType, setToastType] = useState<"error" | "success">("error");
  const [selectedDownload, setSelectedDownload] = useState("");
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Sync form state from server status
  useEffect(() => {
    if (status) {
      setUrl(status.url);
      setLanguage(status.language || "en");
      setDiarize(status.diarize);
      setDirty(false);
    }
  }, [status]);

  const showToast = useCallback((msg: string, type: "error" | "success") => {
    setToast(msg);
    setToastType(type);
    setTimeout(() => setToast(null), 4000);
  }, []);

  const handleToggleEnabled = async () => {
    if (!status) return;
    try {
      await updateConfig([
        { key: "transcriptionEnabled", value: String(!status.enabled) },
      ]).unwrap();
      refetchStatus();
      showToast(
        status.enabled ? "Transcription disabled" : "Transcription enabled",
        "success",
      );
    } catch {
      showToast("Failed to toggle transcription", "error");
    }
  };

  const handleSaveSettings = async () => {
    try {
      await updateConfig([
        { key: "transcriptionUrl", value: url },
        { key: "transcriptionLanguage", value: language },
        { key: "transcriptionDiarize", value: String(diarize) },
      ]).unwrap();
      setDirty(false);
      refetchStatus();
      showToast("Settings saved", "success");
    } catch {
      showToast("Failed to save settings", "error");
    }
  };

  const handleSetActiveModel = async (modelId: string) => {
    try {
      await updateConfig([
        { key: "transcriptionModel", value: modelId },
      ]).unwrap();
      refetchStatus();
      showToast(`Active model set to ${modelId}`, "success");
    } catch {
      showToast("Failed to set active model", "error");
    }
  };

  const handleDownload = async () => {
    if (!selectedDownload) return;
    try {
      await downloadModel({ model: selectedDownload }).unwrap();
      showToast(`Model ${selectedDownload} downloaded`, "success");
      setSelectedDownload("");
      // Delay refetch slightly — go-whisper needs a moment to rescan its store
      setTimeout(() => refetchModels(), 500);
    } catch {
      showToast(`Failed to download ${selectedDownload}`, "error");
    }
  };

  const handleDelete = async (model: WhisperModel) => {
    if (!window.confirm(`Delete model "${model.id}"?`)) return;
    setDeletingId(model.id);
    try {
      await deleteModel({ id: model.id }).unwrap();
      showToast(`Model ${model.id} deleted`, "success");
      refetchModels();
      refetchStatus();
    } catch {
      showToast(`Failed to delete ${model.id}`, "error");
    } finally {
      setDeletingId(null);
    }
  };

  const models = modelsData?.models ?? [];
  const downloadedIds = new Set(models.map((m) => m.id));
  const availableForDownload = KNOWN_MODELS.filter(
    (m) => !downloadedIds.has(m),
  );

  if (statusLoading) {
    return <div className="loading loading-spinner loading-md" />;
  }

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Transcription</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Manage the go-whisper transcription sidecar. Enable transcription,
        configure the connection, and manage whisper models.
      </p>

      {/* ─── Settings Card ─── */}
      <div className="card bg-base-200 mb-4">
        <div className="card-body">
          <div className="flex items-center justify-between mb-4">
            <h2 className="card-title text-base">Status &amp; Settings</h2>
            <div className="flex items-center gap-3">
              {status?.connected ? (
                <div className="flex items-center gap-1.5 text-success text-sm">
                  <Wifi className="w-4 h-4" />
                  Connected
                </div>
              ) : (
                <div className="flex items-center gap-1.5 text-error text-sm">
                  <WifiOff className="w-4 h-4" />
                  Disconnected
                </div>
              )}
            </div>
          </div>

          <div className="flex flex-col gap-4">
            {/* Enable toggle */}
            <label className="flex items-center justify-between cursor-pointer">
              <span className="text-sm font-medium">Enable Transcription</span>
              <input
                type="checkbox"
                className="toggle toggle-primary"
                checked={status?.enabled ?? false}
                onChange={handleToggleEnabled}
              />
            </label>

            {/* URL */}
            <label className="flex flex-col w-full">
              <span className="text-sm mb-1">go-whisper Server URL</span>
              <input
                type="url"
                className="input w-full"
                placeholder="http://localhost:9673"
                value={url}
                onChange={(e) => {
                  setUrl(e.target.value);
                  setDirty(true);
                }}
              />
            </label>

            {/* Language */}
            <label className="flex flex-col w-full">
              <span className="text-sm mb-1">Language</span>
              <select
                className="select w-full"
                value={language}
                onChange={(e) => {
                  setLanguage(e.target.value);
                  setDirty(true);
                }}
              >
                {LANGUAGES.map((l) => (
                  <option key={l.value} value={l.value}>
                    {l.label}
                  </option>
                ))}
              </select>
            </label>

            {/* Diarize */}
            <label className="flex items-center justify-between cursor-pointer">
              <div>
                <span className="text-sm font-medium">Speaker Diarization</span>
                <p className="text-xs text-base-content/60">
                  Requires a tdrz model (e.g. ggml-small.en-tdrz)
                </p>
              </div>
              <input
                type="checkbox"
                className="toggle toggle-primary"
                checked={diarize}
                onChange={(e) => {
                  setDiarize(e.target.checked);
                  setDirty(true);
                }}
              />
            </label>

            {/* Save */}
            <div className="flex justify-end">
              <button
                className="btn btn-primary btn-sm"
                disabled={!dirty}
                onClick={handleSaveSettings}
              >
                Save Settings
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* ─── Stats Card ─── */}
      {stats && stats.total > 0 && (
        <div className="card bg-base-200 mb-4">
          <div className="card-body">
            <h2 className="card-title text-base mb-3">Statistics</h2>

            {/* Summary row */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
              <div className="stat bg-base-300 rounded-lg p-3">
                <div className="stat-title text-xs">Total</div>
                <div className="stat-value text-lg">
                  {stats.total.toLocaleString()}
                </div>
              </div>
              <div className="stat bg-base-300 rounded-lg p-3">
                <div className="stat-title text-xs">Last 24h</div>
                <div className="stat-value text-lg">
                  {stats.recent24h.toLocaleString()}
                </div>
              </div>
              <div className="stat bg-base-300 rounded-lg p-3">
                <div className="stat-title text-xs">Avg Duration</div>
                <div className="stat-value text-lg">
                  {stats.avgDurationMs > 0
                    ? `${(stats.avgDurationMs / 1000).toFixed(1)}s`
                    : "—"}
                </div>
              </div>
              <div className="stat bg-base-300 rounded-lg p-3">
                <div className="stat-title text-xs">Queue</div>
                <div className="stat-value text-lg">
                  {stats.poolEnabled ? stats.queueDepth : "Off"}
                </div>
              </div>
            </div>

            {/* Duration range */}
            {stats.minDurationMs > 0 && (
              <p className="text-xs text-base-content/60 mb-3">
                Processing range: {(stats.minDurationMs / 1000).toFixed(1)}s –{" "}
                {(stats.maxDurationMs / 1000).toFixed(1)}s
              </p>
            )}

            {/* Breakdowns */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {stats.byLanguage.length > 0 && (
                <div>
                  <h3 className="text-sm font-medium mb-2">By Language</h3>
                  <div className="space-y-1">
                    {stats.byLanguage.map((l) => (
                      <div
                        key={l.language}
                        className="flex justify-between text-sm"
                      >
                        <span className="uppercase">{l.language}</span>
                        <span className="text-base-content/60">
                          {l.count.toLocaleString()}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {stats.byModel.length > 0 && (
                <div>
                  <h3 className="text-sm font-medium mb-2">By Model</h3>
                  <div className="space-y-1">
                    {stats.byModel.map((m) => (
                      <div
                        key={m.model}
                        className="flex justify-between text-sm"
                      >
                        <span className="font-mono text-xs">{m.model}</span>
                        <span className="text-base-content/60">
                          {m.count.toLocaleString()}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* ─── Models Card ─── */}
      <div className="card bg-base-200">
        <div className="card-body">
          <h2 className="card-title text-base mb-2">Model Management</h2>

          {/* Download section */}
          {availableForDownload.length > 0 && (
            <div className="flex flex-col sm:flex-row gap-2 mb-4">
              <select
                className="select flex-1"
                value={selectedDownload}
                onChange={(e) => setSelectedDownload(e.target.value)}
                disabled={isDownloading}
              >
                <option value="">Select a model to download…</option>
                {availableForDownload.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
              <button
                className="btn btn-primary btn-sm gap-2"
                disabled={!selectedDownload || isDownloading}
                onClick={handleDownload}
              >
                {isDownloading ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Download className="w-4 h-4" />
                )}
                {isDownloading ? "Downloading…" : "Download"}
              </button>
            </div>
          )}

          {/* Models table */}
          {modelsLoading ? (
            <div className="loading loading-spinner loading-sm" />
          ) : (
            <div className="overflow-x-auto">
              <table className="table table-zebra w-full">
                <thead>
                  <tr>
                    <th>Model</th>
                    <th>Created</th>
                    <th>Active</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {models.map((m) => {
                    const isActive = m.id === status?.model;
                    return (
                      <tr
                        key={m.id}
                        className={isActive ? "bg-primary/10" : ""}
                      >
                        <td className="font-mono text-sm flex items-center gap-2">
                          <Mic className="w-4 h-4 shrink-0 opacity-50" />
                          {m.id}
                        </td>
                        <td className="text-sm">
                          {new Date(m.created * 1000).toLocaleDateString()}
                        </td>
                        <td>
                          {isActive ? (
                            <span className="badge badge-success badge-sm gap-1">
                              <Check className="w-3 h-3" /> Active
                            </span>
                          ) : (
                            <button
                              className="btn btn-ghost btn-xs"
                              onClick={() => handleSetActiveModel(m.id)}
                            >
                              Set Active
                            </button>
                          )}
                        </td>
                        <td>
                          <button
                            className="btn btn-ghost btn-xs"
                            onClick={() => handleDelete(m)}
                            disabled={deletingId === m.id}
                            aria-label={`Delete model ${m.id}`}
                          >
                            {deletingId === m.id ? (
                              <Loader2 className="w-4 h-4 animate-spin" />
                            ) : (
                              <Trash2 className="w-4 h-4" />
                            )}
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                  {models.length === 0 && (
                    <tr>
                      <td colSpan={4} className="text-center opacity-60">
                        No models downloaded
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      {/* Toast */}
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
