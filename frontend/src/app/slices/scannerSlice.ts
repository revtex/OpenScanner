import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type {
  Call,
  AvoidEntry,
  ConnectionStatus,
  ScannerConfig,
} from "@/types";

const MAX_HISTORY = 5;

interface ScannerState {
  isLive: boolean;
  isPaused: boolean;
  heldSystem: number | null;
  heldTG: number | null;
  avoidList: AvoidEntry[];
  currentCall: Call | null;
  history: Call[];
  listenerCount: number;
  connectionStatus: ConnectionStatus;
  config: ScannerConfig | null;
  tgSelection: Record<number, boolean>;
}

const initialState: ScannerState = {
  isLive: true,
  isPaused: false,
  heldSystem: null,
  heldTG: null,
  avoidList: [],
  currentCall: null,
  history: [],
  listenerCount: 0,
  connectionStatus: "disconnected",
  config: null,
  tgSelection: {},
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
                call.talkgroupLedColor = tg.ledColor;
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
      state.currentCall = null;
    },
    togglePause(state) {
      state.isPaused = !state.isPaused;
    },
    toggleLive(state) {
      state.isLive = !state.isLive;
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
    },
    removeAvoid(state, action: PayloadAction<number>) {
      state.avoidList = state.avoidList.filter(
        (a) => a.talkgroupId !== action.payload,
      );
    },
    clearAvoids(state) {
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
          playbackGoesLive: false,
          shareableLinks: false,
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
    expireAvoids(state) {
      const now = Date.now();
      state.avoidList = state.avoidList.filter(
        (a) => a.expiresAt === 0 || a.expiresAt > now,
      );
    },
    transcriptReceived(
      state,
      action: PayloadAction<{ callId: number; text: string }>,
    ) {
      const { callId, text } = action.payload;
      if (state.currentCall?.id === callId) {
        state.currentCall.transcript = text;
      }
      const histItem = state.history.find((c) => c.id === callId);
      if (histItem) {
        histItem.transcript = text;
      }
    },
  },
});

export const {
  callReceived,
  setCurrentCall,
  clearCurrentCall,
  togglePause,
  toggleLive,
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
  setAllTGs,
  setTGsBySystem,
  transcriptReceived,
} = scannerSlice.actions;
