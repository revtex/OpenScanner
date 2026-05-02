import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type { AvoidEntry, ConnectionStatus, ScannerConfig } from "@/types";
import type { Call, TranscriptionSegment } from "./types";

const MAX_HISTORY = 5;

interface PendingTranscript {
  text: string;
  segments?: TranscriptionSegment[];
}

interface ScannerState {
  isLive: boolean;
  isPaused: boolean;
  isAudioActive: boolean;
  heldSystem: number | null;
  heldTG: number | null;
  avoidList: AvoidEntry[];
  currentCall: Call | null;
  history: Call[];
  listenerCount: number;
  connectionStatus: ConnectionStatus;
  config: ScannerConfig | null;
  tgSelection: Record<number, boolean>;
  tgSelectionReady: boolean;
  pendingTranscripts: Record<number, PendingTranscript>;
}

const initialState: ScannerState = {
  isLive: false,
  isPaused:
    typeof sessionStorage !== "undefined" &&
    sessionStorage.getItem("openscanner-paused") === "true",
  isAudioActive: false,
  heldSystem: null,
  heldTG: null,
  avoidList: [],
  currentCall: null,
  history: [],
  listenerCount: 0,
  connectionStatus: "disconnected",
  config: null,
  tgSelection: {},
  tgSelectionReady: false,
  pendingTranscripts: {},
};

export const scannerSlice = createSlice({
  name: "scanner",
  initialState,
  reducers: {
    callReceived(state, action: PayloadAction<Call>) {
      const call = action.payload;

      // Enrich call with labels from config
      if (state.config) {
        for (const sys of state.config.systems) {
          if (sys.id === call.system) {
            call.systemLabel = sys.label;
            for (const tg of sys.talkgroups) {
              if (tg.id === call.talkgroup) {
                call.talkgroupLabel = tg.label;
                call.talkgroupName = tg.name;
                call.talkgroupTag = tg.tag;
                call.talkgroupGroup = tg.group;
                call.talkgroupLedColor = tg.ledColor || sys.ledColor;
                break;
              }
            }
            break;
          }
        }
      }
    },
    setCurrentCall(state, action: PayloadAction<Call | null>) {
      // Move the previous call into history (skip if already there)
      if (
        state.currentCall &&
        !state.history.some((h) => h.id === state.currentCall!.id)
      ) {
        state.history = [state.currentCall, ...state.history].slice(
          0,
          MAX_HISTORY,
        );
      }
      state.currentCall = action.payload;

      // Merge any transcript that arrived while the call was queued.
      if (state.currentCall) {
        const pending = state.pendingTranscripts[state.currentCall.id];
        if (pending) {
          state.currentCall.transcript = pending.text;
          state.currentCall.transcriptSegments = pending.segments;
          delete state.pendingTranscripts[state.currentCall.id];
        }

        // Purge stale entries — any pending transcript for a call ID older
        // than the current call was skipped/filtered and will never play.
        const currentId = state.currentCall.id;
        for (const key of Object.keys(state.pendingTranscripts)) {
          if (Number(key) < currentId) {
            delete state.pendingTranscripts[Number(key)];
          }
        }
      }
    },
    clearCurrentCall(state) {
      // Move the finished call into history (skip if already there)
      if (
        state.currentCall &&
        !state.history.some((h) => h.id === state.currentCall!.id)
      ) {
        state.history = [state.currentCall, ...state.history].slice(
          0,
          MAX_HISTORY,
        );
      }
      // Keep the last call visible on the display until a new call starts.
    },
    resetDisplay(state) {
      state.currentCall = null;
      state.history = [];
    },
    resetTGSelection(state) {
      state.tgSelection = {};
      state.tgSelectionReady = false;
    },
    togglePause(state) {
      state.isPaused = !state.isPaused;
      try {
        sessionStorage.setItem("openscanner-paused", String(state.isPaused));
      } catch {
        /* quota exceeded or SSR */
      }
    },
    setAudioActive(state, action: PayloadAction<boolean>) {
      state.isAudioActive = action.payload;
    },
    setPaused(state, action: PayloadAction<boolean>) {
      state.isPaused = action.payload;
      try {
        sessionStorage.setItem("openscanner-paused", String(state.isPaused));
      } catch {
        /* quota exceeded or SSR */
      }
    },
    toggleLive(state) {
      state.isLive = !state.isLive;
    },
    setLive(state, action: PayloadAction<boolean>) {
      state.isLive = action.payload;
    },
    holdSystem(state, action: PayloadAction<number | null>) {
      state.heldSystem = action.payload;
    },
    holdTG(state, action: PayloadAction<number | null>) {
      state.heldTG = action.payload;
    },
    addAvoid(state, action: PayloadAction<AvoidEntry>) {
      // Replace if already present
      state.avoidList = state.avoidList.filter(
        (a) => a.talkgroupId !== action.payload.talkgroupId,
      );
      state.avoidList.push(action.payload);
      // Avoided talkgroups are filtered talkgroups: mark unchecked.
      state.tgSelection[action.payload.talkgroupId] = false;
    },
    removeAvoid(state, action: PayloadAction<number>) {
      state.avoidList = state.avoidList.filter(
        (a) => a.talkgroupId !== action.payload,
      );
    },
    clearAvoids(state) {
      for (const entry of state.avoidList) {
        state.tgSelection[entry.talkgroupId] = true;
      }
      state.avoidList = [];
    },
    setListenerCount(state, action: PayloadAction<number>) {
      state.listenerCount = action.payload;
    },
    setConnectionStatus(state, action: PayloadAction<ConnectionStatus>) {
      state.connectionStatus = action.payload;
    },
    setConfig(state, action: PayloadAction<ScannerConfig>) {
      const incoming = action.payload;
      state.config = {
        ...incoming,
        branding: incoming.branding ?? state.config?.branding ?? "",
        email: incoming.email ?? state.config?.email ?? "",
        version: incoming.version ?? state.config?.version ?? "",
      };
      // Cache display prefs so the next page load avoids a flash of defaults.
      try {
        sessionStorage.setItem(
          "openscanner-display-prefs",
          JSON.stringify({
            time12hFormat: state.config.time12hFormat,
            showListenersCount: state.config.showListenersCount,
          }),
        );
      } catch {
        // sessionStorage unavailable — ignore
      }
    },
    setBranding(
      state,
      action: PayloadAction<{
        branding: string;
        email: string;
        version: string;
      }>,
    ) {
      if (state.config) {
        state.config.branding = action.payload.branding;
        state.config.email = action.payload.email;
        state.config.version = action.payload.version;
      } else {
        state.config = {
          systems: [],
          time12hFormat: false,
          showListenersCount: false,
          shareableLinks: false,
          transcriptionEnabled: false,
          liveTranscriptDisplay: false,
          keypadBeeps: "",
          ...action.payload,
        };
      }
    },
    toggleTG(state, action: PayloadAction<number>) {
      const id = action.payload;
      state.tgSelection[id] = !state.tgSelection[id];
    },
    restoreTGSelection(state, action: PayloadAction<Record<number, boolean>>) {
      state.tgSelection = action.payload;
      state.tgSelectionReady = true;
    },
    restoreFromDisabledTGs(state, action: PayloadAction<number[]>) {
      const disabled = new Set(action.payload);
      const selection: Record<number, boolean> = {};
      if (state.config) {
        for (const sys of state.config.systems) {
          for (const tg of sys.talkgroups) {
            selection[tg.id] = !disabled.has(tg.id);
          }
        }
      }
      state.tgSelection = selection;
      state.tgSelectionReady = true;
    },
    restoreAvoidList(state, action: PayloadAction<AvoidEntry[]>) {
      const now = Date.now();
      state.avoidList = action.payload.filter(
        (a) => a.expiresAt === 0 || a.expiresAt > now,
      );
      // Ensure active avoids are reflected as unchecked.
      for (const entry of state.avoidList) {
        state.tgSelection[entry.talkgroupId] = false;
      }
    },
    setAllTGs(state, action: PayloadAction<boolean>) {
      const enabled = action.payload;
      if (state.config) {
        for (const sys of state.config.systems) {
          for (const tg of sys.talkgroups) {
            state.tgSelection[tg.id] = enabled;
          }
        }
      }
    },
    setTGsBySystem(
      state,
      action: PayloadAction<{ systemId: number; enabled: boolean }>,
    ) {
      const { systemId, enabled } = action.payload;
      const sys = state.config?.systems.find((s) => s.id === systemId);
      if (sys) {
        for (const tg of sys.talkgroups) {
          state.tgSelection[tg.id] = enabled;
        }
      }
    },
    setTGsByGroup(
      state,
      action: PayloadAction<{ group: string; enabled: boolean }>,
    ) {
      const { group, enabled } = action.payload;
      if (!state.config) return;
      for (const sys of state.config.systems) {
        for (const tg of sys.talkgroups) {
          if (tg.group === group) {
            state.tgSelection[tg.id] = enabled;
          }
        }
      }
    },
    setTGsByTag(
      state,
      action: PayloadAction<{ tag: string; enabled: boolean }>,
    ) {
      const { tag, enabled } = action.payload;
      if (!state.config) return;
      for (const sys of state.config.systems) {
        for (const tg of sys.talkgroups) {
          if (tg.tag === tag) {
            state.tgSelection[tg.id] = enabled;
          }
        }
      }
    },
    expireAvoids(state) {
      const now = Date.now();
      const kept: AvoidEntry[] = [];
      for (const entry of state.avoidList) {
        if (entry.expiresAt === 0 || entry.expiresAt > now) {
          kept.push(entry);
        } else {
          // Timed avoid expired: auto re-enable the talkgroup.
          state.tgSelection[entry.talkgroupId] = true;
        }
      }
      state.avoidList = kept;
    },
    transcriptReceived(
      state,
      action: PayloadAction<{
        callId: number;
        text: string;
        segments?: TranscriptionSegment[];
      }>,
    ) {
      const { callId, text, segments } = action.payload;
      let matched = false;

      if (state.currentCall?.id === callId) {
        state.currentCall.transcript = text;
        state.currentCall.transcriptSegments = segments;
        matched = true;
      }
      const histItem = state.history.find((c) => c.id === callId);
      if (histItem) {
        histItem.transcript = text;
        histItem.transcriptSegments = segments;
        matched = true;
      }

      // Call is likely still in the audioPlayer queue — stash for later.
      // Consumed by setCurrentCall when the call starts playing.
      if (!matched) {
        state.pendingTranscripts[callId] = { text, segments };
      }
    },
  },
});

export const {
  callReceived,
  setCurrentCall,
  clearCurrentCall,
  resetDisplay,
  resetTGSelection,
  togglePause,
  setAudioActive,
  setPaused,
  toggleLive,
  setLive,
  holdSystem,
  holdTG,
  addAvoid,
  removeAvoid,
  clearAvoids,
  expireAvoids,
  setListenerCount,
  setConnectionStatus,
  setConfig,
  setBranding,
  toggleTG,
  restoreTGSelection,
  restoreFromDisabledTGs,
  restoreAvoidList,
  setAllTGs,
  setTGsBySystem,
  setTGsByGroup,
  setTGsByTag,
  transcriptReceived,
} = scannerSlice.actions;
