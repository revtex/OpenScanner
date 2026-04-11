import { useState } from "react";
import {
  useGetConfigQuery,
  useUpdateConfigMutation,
} from "@/app/slices/adminSlice";
import type { AdminSetting } from "@/types";

// ─── Known setting keys and their input types ───

const BOOLEAN_KEYS = [
  "publicAccess",
  "shareableLinks",
  "keyboardShortcuts",
  "darkMode",
  "pushNotifications",
  "webhooksEnabled",
  "transcriptionEnabled",
  "activityDashboard",
] as const;

const FFMPEG_MODES: Record<string, string> = {
  "0": "Disabled",
  "1": "Enabled",
  "2": "Normalize",
  "3": "Loudnorm",
};

const TRANSCRIPTION_MODELS = ["tiny", "base", "small", "medium", "large"];

interface SettingSection {
  title: string;
  keys: string[];
}

const SECTIONS: SettingSection[] = [
  { title: "General", keys: ["publicAccess", "darkMode", "keyboardShortcuts"] },
  {
    title: "Sharing & Notifications",
    keys: ["shareableLinks", "pushNotifications"],
  },
  { title: "Audio Processing", keys: ["ffmpegMode", "audioWorkerCount"] },
  { title: "Webhooks", keys: ["webhooksEnabled"] },
  {
    title: "Transcription",
    keys: [
      "transcriptionEnabled",
      "transcriptionBinary",
      "transcriptionModel",
      "transcriptionLanguage",
    ],
  },
  { title: "Dashboard", keys: ["activityDashboard"] },
];

const LABELS: Record<string, string> = {
  publicAccess: "Public Access",
  shareableLinks: "Shareable Links",
  keyboardShortcuts: "Keyboard Shortcuts",
  darkMode: "Dark Mode Default",
  pushNotifications: "Push Notifications",
  webhooksEnabled: "Webhooks Enabled",
  transcriptionEnabled: "Transcription Enabled",
  transcriptionBinary: "Transcription Binary Path",
  transcriptionModel: "Transcription Model",
  transcriptionLanguage: "Transcription Language",
  ffmpegMode: "FFmpeg Mode",
  audioWorkerCount: "Audio Worker Count",
  activityDashboard: "Activity Dashboard",
};

function isBooleanKey(key: string): boolean {
  return (BOOLEAN_KEYS as readonly string[]).includes(key);
}

export default function OptionsPanel() {
  const { data: config, isLoading } = useGetConfigQuery();
  const [updateConfig] = useUpdateConfigMutation();

  const [localSettings, setLocalSettings] = useState<Record<string, string>>(
    {},
  );
  const [configLoaded, setConfigLoaded] = useState(false);
  const [toast, setToast] = useState<{
    message: string;
    type: "success" | "error";
  } | null>(null);

  if (config && !configLoaded) {
    const map: Record<string, string> = {};
    for (const s of config) {
      map[s.key] = s.value;
    }
    setLocalSettings(map);
    setConfigLoaded(true);
  }

  const showToast = (message: string, type: "success" | "error") => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 5000);
  };

  const updateSetting = (key: string, value: string) => {
    setLocalSettings((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    const settings: AdminSetting[] = Object.entries(localSettings).map(
      ([key, value]) => ({ key, value }),
    );
    try {
      await updateConfig(settings).unwrap();
      showToast("Settings saved successfully", "success");
    } catch {
      showToast("Failed to save settings", "error");
    }
  };

  const transcriptionEnabled = localSettings["transcriptionEnabled"] === "true";

  const renderSettingInput = (key: string) => {
    const value = localSettings[key] ?? "";

    if (isBooleanKey(key)) {
      return (
        <div className="form-control">
          <label className="label cursor-pointer justify-start gap-4">
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={value === "true"}
              onChange={(e) =>
                updateSetting(key, e.target.checked ? "true" : "false")
              }
            />
            <span className="label-text">{LABELS[key] ?? key}</span>
            {key === "publicAccess" && (
              <span className="badge badge-warning">
                Warning: Opens scanner to unauthenticated users
              </span>
            )}
          </label>
        </div>
      );
    }

    if (key === "ffmpegMode") {
      return (
        <div className="form-control">
          <label className="label">
            <span className="label-text">{LABELS[key]}</span>
          </label>
          <select
            className="select select-bordered w-full max-w-xs"
            value={value}
            onChange={(e) => updateSetting(key, e.target.value)}
          >
            {Object.entries(FFMPEG_MODES).map(([val, label]) => (
              <option key={val} value={val}>
                {label}
              </option>
            ))}
          </select>
        </div>
      );
    }

    if (key === "transcriptionModel") {
      return (
        <div className="form-control">
          <label className="label">
            <span className="label-text">{LABELS[key]}</span>
          </label>
          <select
            className="select select-bordered w-full max-w-xs"
            value={value}
            onChange={(e) => updateSetting(key, e.target.value)}
          >
            {TRANSCRIPTION_MODELS.map((model) => (
              <option key={model} value={model}>
                {model}
              </option>
            ))}
          </select>
        </div>
      );
    }

    if (key === "audioWorkerCount") {
      return (
        <div className="form-control">
          <label className="label">
            <span className="label-text">{LABELS[key]}</span>
          </label>
          <input
            type="number"
            className="input input-bordered w-full max-w-xs"
            value={value}
            min={1}
            onChange={(e) => updateSetting(key, e.target.value)}
          />
        </div>
      );
    }

    // Default: text input
    return (
      <div className="form-control">
        <label className="label">
          <span className="label-text">{LABELS[key] ?? key}</span>
        </label>
        <input
          type="text"
          className="input input-bordered w-full max-w-xs"
          value={value}
          onChange={(e) => updateSetting(key, e.target.value)}
        />
      </div>
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
      <h1 className="text-xl font-semibold mb-4">Options</h1>
      <div className="card bg-base-200">
        <div className="card-body space-y-6">
          {SECTIONS.map((section) => {
            // Hide transcription sub-settings when disabled
            const keys =
              section.title === "Transcription"
                ? section.keys.filter(
                    (k) => k === "transcriptionEnabled" || transcriptionEnabled,
                  )
                : section.keys;

            // Only show section if we have settings for it
            const hasSettings = keys.some((k) => k in localSettings);
            if (!hasSettings) return null;

            return (
              <div key={section.title}>
                <h2 className="text-lg font-medium mb-3 border-b border-base-300 pb-2">
                  {section.title}
                </h2>
                <div className="space-y-2">
                  {keys.map((key) =>
                    key in localSettings ? (
                      <div key={key}>{renderSettingInput(key)}</div>
                    ) : null,
                  )}
                </div>
              </div>
            );
          })}

          <div className="pt-4">
            <button className="btn btn-primary" onClick={handleSave}>
              Save Changes
            </button>
          </div>
        </div>
      </div>

      {toast && (
        <div className="toast toast-end">
          <div
            className={`alert ${toast.type === "success" ? "alert-success" : "alert-error"}`}
          >
            <span>{toast.message}</span>
          </div>
        </div>
      )}
    </div>
  );
}
