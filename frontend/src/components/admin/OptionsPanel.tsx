import { useState, useEffect, useMemo, useRef } from "react";
import {
  useGetConfigQuery,
  useUpdateConfigMutation,
} from "@/hooks/useAdminWsOps";
import { useNavigationGuard } from "@/components/admin/NavigationGuardContext";
import type { AdminSetting } from "@/types";

// ─── Known setting keys and their input types ───

const BOOLEAN_KEYS = [
  "publicAccess",
  "shareableLinks",
  "pushNotifications",
  "webhooksEnabled",
  "transcriptionEnabled",
  "transcriptionDiarize",
  "time12hFormat",
  "disableDuplicateDetection",
  "sortTalkgroups",
  "tagsToggle",
  "playbackGoesLive",
  "searchPatchedTalkgroups",
  "showListenersCount",
] as const;

const AUDIO_CONVERSION_MODES: Record<string, string> = {
  "0": "Disabled",
  "1": "Enabled (no normalization)",
  "2": "Normalize",
  "3": "Loudnorm",
};

const AUDIO_ENCODING_PRESETS: Record<string, string> = {
  mp3_32k: "MP3 32 kbps (default)",
  mp3_24k: "MP3 24 kbps",
  mp3_16k: "MP3 16 kbps",
  aac_lc_32k: "AAC-LC 32 kbps",
  aac_lc_24k: "AAC-LC 24 kbps",
  aac_lc_16k: "AAC-LC 16 kbps",
  he_aac_12k: "HE-AAC 12 kbps",
  he_aac_8k: "HE-AAC 8 kbps",
};

const HE_AAC_PRESETS = new Set(["he_aac_12k", "he_aac_8k"]);

const KEYPAD_BEEPS = ["disabled", "uniden", "whistler"] as const;

const TRANSCRIPTION_MODELS = [
  "ggml-tiny",
  "ggml-base",
  "ggml-small",
  "ggml-medium",
  "ggml-large",
  "ggml-small.en-tdrz",
];
interface SettingSection {
  title: string;
  keys: string[];
}

// Keys listed here are persisted but currently not wired to runtime behavior.
const PLANNED_ONLY_KEYS = [
  "pushNotifications",
  "webhooksEnabled",
  "sortTalkgroups",
  "tagsToggle",
  "playbackGoesLive",
  "searchPatchedTalkgroups",
  "afsSystems",
] as const;

const SECTIONS: SettingSection[] = [
  {
    title: "General",
    keys: ["branding", "email", "publicAccess"],
  },
  {
    title: "Scanner Behavior",
    keys: [
      "sortTalkgroups",
      "tagsToggle",
      "time12hFormat",
      "showListenersCount",
      "playbackGoesLive",
      "afsSystems",
      "maxClients",
    ],
  },
  {
    title: "Call Processing",
    keys: [
      "audioConversion",
      "audioEncodingPreset",
      "disableDuplicateDetection",
      "duplicateDetectionTimeFrame",
      "pruneDays",
      "searchPatchedTalkgroups",
    ],
  },
  {
    title: "Display",
    keys: ["keypadBeeps"],
  },
  {
    title: "Sharing & Notifications",
    keys: ["shareableLinks", "pushNotifications"],
  },
  { title: "Webhooks", keys: ["webhooksEnabled"] },
  {
    title: "Transcription",
    keys: [
      "transcriptionEnabled",
      "transcriptionUrl",
      "transcriptionModel",
      "transcriptionLanguage",
      "transcriptionDiarize",
    ],
  },
];

// Only keys that appear in SECTIONS should be sent to the backend.
// The DB may contain additional client-side-only keys (e.g. darkMode,
// dimmerDelay) that are not in the backend's allowedSettingKeys whitelist.
const SAVEABLE_KEYS = new Set(SECTIONS.flatMap((s) => s.keys));

const LABELS: Record<string, string> = {
  publicAccess: "Public Access",
  shareableLinks: "Shareable Links",
  pushNotifications: "Push Notifications",
  webhooksEnabled: "Webhooks Enabled",
  transcriptionEnabled: "Transcription Enabled",
  transcriptionUrl: "go-whisper Server URL",
  transcriptionModel: "Transcription Model",
  transcriptionLanguage: "Transcription Language",
  transcriptionDiarize: "Speaker Diarization",
  audioConversion: "Audio Conversion (FFmpeg)",
  audioEncodingPreset: "Audio Encoding Preset",
  branding: "Branding Label",
  email: "Support Email",
  time12hFormat: "12-Hour Time Format",
  afsSystems: "AFS Systems",
  maxClients: "Max Simultaneous Clients",
  keypadBeeps: "Keypad Beep Style",
  disableDuplicateDetection: "Disable Duplicate Call Detection",
  duplicateDetectionTimeFrame: "Duplicate Detection Time Frame (ms)",
  pruneDays: "Prune Database After (days)",
  sortTalkgroups: "Sort Talkgroups by ID",
  tagsToggle: "Allow Toggle by Tag",
  playbackGoesLive: "Playback Mode Goes Live",
  searchPatchedTalkgroups: "Search Patched Talkgroups",
  showListenersCount: "Show Listeners Count",
};

const DESCRIPTIONS: Record<string, string> = {
  publicAccess: "Allow unauthenticated listeners to access the scanner.",
  branding:
    "Short label shown above the scanner display to identify this instance.",
  email: "Support contact email shown to users.",
  time12hFormat: "Display timestamps in 12-hour (AM/PM) format.",
  afsSystems:
    "Comma-separated system IDs whose talkgroup IDs should be shown in AFS (agency-fleet-subfleet) format.",
  maxClients: "Maximum number of simultaneous WebSocket listeners.",
  sortTalkgroups:
    "Sort talkgroups by their numeric ID instead of display order.",
  tagsToggle: "Allow toggling entire groups of talkgroups by tag.",
  playbackGoesLive:
    "Automatically switch from playback to live mode when the search list is exhausted.",
  searchPatchedTalkgroups:
    "Include patched talkgroups in search results (may slow search).",
  showListenersCount:
    "Display the active listener count on the main scanner screen.",
  audioConversion: "Convert incoming audio to AAC/M4A using FFmpeg.",
  audioEncodingPreset:
    "Codec and bitrate for converted audio. HE-AAC presets require libfdk_aac in your FFmpeg build. Lower bitrates save storage; choose based on your quality needs.",
  disableDuplicateDetection:
    "Disable automatic rejection of duplicate incoming calls.",
  duplicateDetectionTimeFrame:
    "Calls within ±this many milliseconds of an existing call with the same system/talkgroup are rejected as duplicates.",
  pruneDays:
    "Automatically delete calls older than this many days. Set to 0 to disable.",
  keypadBeeps:
    "Audio feedback style when pressing buttons. Disabled turns off beeps.",
};

function isBooleanKey(key: string): boolean {
  return (BOOLEAN_KEYS as readonly string[]).includes(key);
}

function isPlannedKey(key: string): boolean {
  return (PLANNED_ONLY_KEYS as readonly string[]).includes(key);
}

export default function OptionsPanel() {
  const { data: config, isLoading } = useGetConfigQuery();
  const [updateConfig] = useUpdateConfigMutation();

  const capabilities = config?.capabilities;
  const settings = config?.settings;

  const [localSettings, setLocalSettings] = useState<Record<string, string>>(
    {},
  );
  const [configLoaded, setConfigLoaded] = useState(false);
  const [toast, setToast] = useState<{
    message: string;
    type: "success" | "error";
  } | null>(null);

  // Build a map of original server values for dirty comparison.
  const serverSettings = useMemo(() => {
    const map: Record<string, string> = {};
    if (settings) {
      for (const s of settings) {
        map[s.key] = s.value;
      }
    }
    return map;
  }, [settings]);

  if (settings && !configLoaded) {
    const map: Record<string, string> = {};
    for (const s of settings) {
      map[s.key] = s.value;
    }
    setLocalSettings(map);
    setConfigLoaded(true);
  }

  const isDirty = useMemo(() => {
    if (!configLoaded) return false;
    for (const key of Object.keys(localSettings)) {
      if (localSettings[key] !== serverSettings[key]) return true;
    }
    return false;
  }, [localSettings, serverSettings, configLoaded]);

  // Warn on browser/tab close with unsaved changes.
  useEffect(() => {
    if (!isDirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [isDirty]);

  // Register navigation guard so AdminLayout can block tab switches.
  const { setGuard } = useNavigationGuard();
  const isDirtyRef = useRef(isDirty);
  useEffect(() => {
    isDirtyRef.current = isDirty;
  }, [isDirty]);
  useEffect(() => {
    setGuard(() => isDirtyRef.current);
    return () => setGuard(null);
  }, [setGuard]);

  const showToast = (message: string, type: "success" | "error") => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 5000);
  };

  const updateSetting = (key: string, value: string) => {
    if (key === "transcriptionDiarize" && value === "true") {
      setLocalSettings((prev) => ({
        ...prev,
        [key]: value,
        transcriptionModel: "ggml-small.en-tdrz",
      }));
      return;
    }
    setLocalSettings((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    if (!isDirty) return;

    const settings: AdminSetting[] = Object.entries(localSettings)
      .filter(([key]) => SAVEABLE_KEYS.has(key))
      .map(([key, value]) => ({ key, value }));
    try {
      await updateConfig(settings).unwrap();
      showToast("Settings saved successfully", "success");
      // RTK Query invalidates "Config" tag → refetch updates serverSettings
      // automatically via the useMemo, so isDirty clears on its own.
    } catch {
      showToast("Failed to save settings", "error");
    }
  };

  const transcriptionEnabled = localSettings["transcriptionEnabled"] === "true";
  const diarizeEnabled = localSettings["transcriptionDiarize"] === "true";

  const renderSettingInput = (key: string) => {
    const value = localSettings[key] ?? "";
    const plannedOnly = isPlannedKey(key);

    const statusBadge = plannedOnly ? (
      <span className="badge badge-sm border-warning/30 bg-warning/10 text-warning">
        Planned
      </span>
    ) : (
      <span className="badge badge-sm border-success/30 bg-success/10 text-success">
        Active
      </span>
    );

    if (isBooleanKey(key)) {
      return (
        <div className="flex flex-col">
          <label className="flex items-center cursor-pointer justify-start gap-4">
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={value === "true"}
              disabled={plannedOnly}
              onChange={(e) =>
                updateSetting(key, e.target.checked ? "true" : "false")
              }
            />
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">
                  {LABELS[key] ?? key}
                </span>
                {statusBadge}
              </div>
              {DESCRIPTIONS[key] && (
                <p className="text-xs text-base-content/60 mt-0.5">
                  {DESCRIPTIONS[key]}
                </p>
              )}
              {plannedOnly && (
                <p className="text-xs text-warning/80 mt-1">
                  Saved to config, but not wired to runtime behavior yet.
                </p>
              )}
            </div>
          </label>
        </div>
      );
    }

    const label = (
      <div className="pb-0 flex items-center gap-2">
        <span className="text-sm font-medium">{LABELS[key] ?? key}</span>
        {statusBadge}
      </div>
    );
    const description = DESCRIPTIONS[key] ? (
      <p className="text-xs text-base-content/60 mb-1">{DESCRIPTIONS[key]}</p>
    ) : null;

    if (key === "audioConversion") {
      const ffmpegMissing = capabilities && !capabilities.ffmpeg;
      return (
        <div className="flex flex-col">
          {label}
          {description}
          <select
            className="select w-full max-w-xs"
            value={value}
            onChange={(e) => updateSetting(key, e.target.value)}
            disabled={!!ffmpegMissing}
          >
            {Object.entries(AUDIO_CONVERSION_MODES).map(([val, lbl]) => (
              <option key={val} value={val}>
                {lbl}
              </option>
            ))}
          </select>
          {plannedOnly && (
            <p className="text-xs text-warning/80 mt-1">
              Saved to config, but not wired to runtime behavior yet.
            </p>
          )}
          {ffmpegMissing && (
            <p className="text-xs text-warning mt-1">
              FFmpeg is not installed. Install it and restart the service to
              enable audio conversion.
            </p>
          )}
        </div>
      );
    }

    if (key === "audioEncodingPreset") {
      const conversionDisabled =
        (localSettings["audioConversion"] ?? "0") === "0";
      const ffmpegMissing = capabilities && !capabilities.ffmpeg;
      const fdkAacAvailable = !!capabilities?.fdkAac;
      const visiblePresets = Object.entries(AUDIO_ENCODING_PRESETS).filter(
        ([preset]) => fdkAacAvailable || !HE_AAC_PRESETS.has(preset),
      );
      const selectedValue =
        value && (!HE_AAC_PRESETS.has(value) || fdkAacAvailable)
          ? value
          : "mp3_32k";
      return (
        <div className="flex flex-col">
          {label}
          {description}
          <select
            className="select w-full max-w-xs"
            value={selectedValue}
            onChange={(e) => updateSetting(key, e.target.value)}
            disabled={!!ffmpegMissing || conversionDisabled}
          >
            {visiblePresets.map(([val, lbl]) => (
              <option key={val} value={val}>
                {lbl}
              </option>
            ))}
          </select>
          {!fdkAacAvailable && (
            <p className="text-xs text-warning mt-1">
              libfdk_aac not detected in FFmpeg; HE-AAC presets are unavailable.
            </p>
          )}
          {conversionDisabled && (
            <p className="text-xs text-base-content/50 mt-1">
              Enable audio conversion above to activate this setting.
            </p>
          )}
        </div>
      );
    }

    if (key === "keypadBeeps") {
      return (
        <div className="flex flex-col">
          {label}
          {description}
          <select
            className="select w-full max-w-xs"
            value={value}
            disabled={plannedOnly}
            onChange={(e) => updateSetting(key, e.target.value)}
          >
            {KEYPAD_BEEPS.map((style) => (
              <option key={style} value={style}>
                {style.charAt(0).toUpperCase() + style.slice(1)}
              </option>
            ))}
          </select>
          {plannedOnly && (
            <p className="text-xs text-warning/80 mt-1">
              Saved to config, but not wired to runtime behavior yet.
            </p>
          )}
        </div>
      );
    }

    if (key === "transcriptionModel") {
      return (
        <div className="flex flex-col">
          {label}
          {description}
          <select
            className="select w-full max-w-xs"
            value={value}
            disabled={diarizeEnabled}
            onChange={(e) => updateSetting(key, e.target.value)}
          >
            {TRANSCRIPTION_MODELS.map((model) => (
              <option key={model} value={model}>
                {model}
              </option>
            ))}
          </select>
          {diarizeEnabled && (
            <p className="text-xs text-info/80 mt-1">
              Model locked to ggml-small.en-tdrz while Speaker Diarization is
              enabled.
            </p>
          )}
        </div>
      );
    }

    if (
      key === "maxClients" ||
      key === "pruneDays" ||
      key === "duplicateDetectionTimeFrame"
    ) {
      return (
        <div className="flex flex-col">
          {label}
          {description}
          <input
            type="number"
            className="input w-full max-w-xs"
            value={value}
            min={0}
            disabled={plannedOnly}
            onChange={(e) => updateSetting(key, e.target.value)}
          />
          {plannedOnly && (
            <p className="text-xs text-warning/80 mt-1">
              Saved to config, but not wired to runtime behavior yet.
            </p>
          )}
        </div>
      );
    }

    if (key === "afsSystems") {
      return (
        <div className="flex flex-col">
          {label}
          {description}
          <textarea
            className="textarea w-full max-w-xs"
            value={value}
            disabled={plannedOnly}
            placeholder="e.g. 1,2,5"
            rows={2}
            onChange={(e) => updateSetting(key, e.target.value)}
          />
          {plannedOnly && (
            <p className="text-xs text-warning/80 mt-1">
              Saved to config, but not wired to runtime behavior yet.
            </p>
          )}
        </div>
      );
    }

    // Default: text input
    return (
      <div className="flex flex-col">
        {label}
        {description}
        <input
          type="text"
          className="input w-full max-w-xs"
          value={value}
          disabled={plannedOnly}
          onChange={(e) => updateSetting(key, e.target.value)}
        />
        {plannedOnly && (
          <p className="text-xs text-warning/80 mt-1">
            Saved to config, but not wired to runtime behavior yet.
          </p>
        )}
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
      <p className="text-sm text-base-content/70 mb-4">
        Global settings for the scanner. Configure audio processing, public
        access, branding, duplicate detection, auto-pruning, transcription, and
        UI behavior. Changes take effect immediately for all connected clients.
      </p>
      <div className="mb-4 rounded-lg border border-base-300 bg-base-200/60 p-3">
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="font-medium">Status:</span>
          <span className="badge badge-sm border-success/30 bg-success/10 text-success">
            Active
          </span>
          <span className="text-base-content/60">Fully wired and in use.</span>
          <span className="badge badge-sm border-warning/30 bg-warning/10 text-warning">
            Planned
          </span>
          <span className="text-base-content/60">
            Saved to config but not active yet (grayed out).
          </span>
        </div>
      </div>
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
                <div className="space-y-3">
                  {keys.map((key) =>
                    key in localSettings ? (
                      <div
                        key={key}
                        className={`relative rounded-lg border border-base-300 bg-base-100/60 p-3 ${isPlannedKey(key) ? "opacity-60" : ""}`}
                      >
                        {localSettings[key] !== serverSettings[key] && (
                          <span
                            className="absolute -left-3 top-3 w-2 h-2 rounded-full bg-warning"
                            title="Modified"
                          />
                        )}
                        {renderSettingInput(key)}
                      </div>
                    ) : null,
                  )}
                </div>
              </div>
            );
          })}

          <div className="pt-4 flex items-center gap-3">
            <button
              className="btn btn-primary"
              onClick={handleSave}
              disabled={!isDirty}
            >
              Save Changes
            </button>
            {isDirty && (
              <span className="text-warning text-sm">Unsaved changes</span>
            )}
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
