import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type {
  Call,
  AvoidEntry,
  ConnectionStatus,
  ScannerConfig,
} from "@/types";

const MAX_HISTORY = 5;
const MAX_QUEUE = 50;

interface ScannerState {
  isLive: boolean;
  isPaused: boolean;
  heldSystem: number | null;
  heldTG: number | null;
  avoidList: AvoidEntry[];
  callQueue: Call[];
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
  callQueue: [],
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
      // Add to history (front), cap at MAX_HISTORY
      state.history = [call, ...state.history].slice(0, MAX_HISTORY);

      if (!state.currentCall) {
        state.currentCall = call;
      } else {
        state.callQueue.push(call);
        if (state.callQueue.length > MAX_QUEUE) {
          state.callQueue = state.callQueue.slice(-MAX_QUEUE);
        }
      }
    },
    setCurrentCall(state, action: PayloadAction<Call | null>) {
      state.currentCall = action.payload;
    },
    skipCall(state) {
      const next = state.callQueue.shift();
      state.currentCall = next ?? null;
    },
    clearCurrentCall(state) {
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
      state.config = action.payload;
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
        state.config = { systems: [], ...action.payload };
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
      const queueItem = state.callQueue.find((c) => c.id === callId);
      if (queueItem) {
        queueItem.transcript = text;
      }
    },
  },
});

export const {
  callReceived,
  setCurrentCall,
  skipCall,
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
