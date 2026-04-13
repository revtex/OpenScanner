import { useState, useEffect, useMemo, useRef } from "react";
import {
  useGetConfigQuery,
  useUpdateConfigMutation,
} from "@/app/slices/adminSlice";
import { useNavigationGuard } from "@/components/admin/NavigationGuardContext";
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
  "autoPopulate",
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

const KEYPAD_BEEPS = ["disabled", "uniden", "whistler"] as const;

const TRANSCRIPTION_MODELS = ["tiny", "base", "small", "medium", "large"];

interface SettingSection {
  title: string;
  keys: string[];
}

const SECTIONS: SettingSection[] = [
  {
    title: "General",
    keys: [
      "branding",
      "email",
      "publicAccess",
      "darkMode",
      "keyboardShortcuts",
    ],
  },
  {
    title: "Scanner Behavior",
    keys: [
      "autoPopulate",
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
      "disableDuplicateDetection",
      "duplicateDetectionTimeFrame",
      "pruneDays",
      "searchPatchedTalkgroups",
    ],
  },
  {
    title: "Display",
    keys: ["dimmerDelay", "keypadBeeps"],
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
  audioConversion: "Audio Conversion (FFmpeg)",
  activityDashboard: "Activity Dashboard",
  branding: "Branding Label",
  email: "Support Email",
  autoPopulate: "Auto-Populate Systems & Talkgroups",
  time12hFormat: "12-Hour Time Format",
  afsSystems: "AFS Systems",
  maxClients: "Max Simultaneous Clients",
  dimmerDelay: "Dimmer Delay (ms)",
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
  autoPopulate:
    "Automatically create new systems and talkgroups from incoming calls.",
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
  disableDuplicateDetection:
    "Disable automatic rejection of duplicate incoming calls.",
  duplicateDetectionTimeFrame:
    "Calls within ±this many milliseconds of an existing call with the same system/talkgroup are rejected as duplicates.",
  pruneDays:
    "Automatically delete calls older than this many days. Set to 0 to disable.",
  dimmerDelay: "Milliseconds of inactivity before the screen dims on mobile.",
  keypadBeeps:
    "Audio feedback style when pressing buttons. Disabled turns off beeps.",
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

  // Build a map of original server values for dirty comparison.
  const serverSettings = useMemo(() => {
    const map: Record<string, string> = {};
    if (config) {
      for (const s of config) {
        map[s.key] = s.value;
      }
    }
    return map;
  }, [config]);

  if (config && !configLoaded) {
    const map: Record<string, string> = {};
    for (const s of config) {
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
    setLocalSettings((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    const settings: AdminSetting[] = Object.entries(localSettings).map(
      ([key, value]) => ({ key, value }),
    );
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

  const renderSettingInput = (key: string) => {
    const value = localSettings[key] ?? "";

    if (isBooleanKey(key)) {
      return (
        <div className="flex flex-col">
          <label className="flex items-center cursor-pointer justify-start gap-4">
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={value === "true"}
              onChange={(e) =>
                updateSetting(key, e.target.checked ? "true" : "false")
              }
            />
            <div>
              <span className="text-sm font-medium">
                {LABELS[key] ?? key}
              </span>
              {DESCRIPTIONS[key] && (
                <p className="text-xs text-base-content/60 mt-0.5">
                  {DESCRIPTIONS[key]}
                </p>
              )}
            </div>
          </label>
        </div>
      );
    }

    const label = (
      <div className="pb-0">
        <span className="text-sm font-medium">{LABELS[key] ?? key}</span>
      </div>
    );
    const description = DESCRIPTIONS[key] ? (
      <p className="text-xs text-base-content/60 mb-1">{DESCRIPTIONS[key]}</p>
    ) : null;

    if (key === "audioConversion") {
      return (
        <div className="flex flex-col">
          {label}
          {description}
          <select
            className="select w-full max-w-xs"
            value={value}
            onChange={(e) => updateSetting(key, e.target.value)}
          >
            {Object.entries(AUDIO_CONVERSION_MODES).map(([val, lbl]) => (
              <option key={val} value={val}>
                {lbl}
              </option>
            ))}
          </select>
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
            onChange={(e) => updateSetting(key, e.target.value)}
          >
            {KEYPAD_BEEPS.map((style) => (
              <option key={style} value={style}>
                {style.charAt(0).toUpperCase() + style.slice(1)}
              </option>
            ))}
          </select>
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

    if (
      key === "maxClients" ||
      key === "dimmerDelay" ||
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
            onChange={(e) => updateSetting(key, e.target.value)}
          />
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
            placeholder="e.g. 1,2,5"
            rows={2}
            onChange={(e) => updateSetting(key, e.target.value)}
          />
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
      <p className="text-sm text-base-content/70 mb-4">
        Global settings for the scanner. Configure audio processing, public
        access, branding, duplicate detection, auto-pruning, transcription, and
        UI behavior. Changes take effect immediately for all connected clients.
      </p>
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
                      <div key={key} className="relative">
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
            <button className="btn btn-primary" onClick={handleSave}>
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
