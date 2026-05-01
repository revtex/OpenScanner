// Redux slice that consumes `tr.*` admin WebSocket events and surfaces a
// per-instance live view for the Trunk Recorder dashboards.
//
// The reducer is the single entry point for every WS topic — components
// never parse frames directly. A REST snapshot fetch (via trMqttApi) can
// hydrate `snapshots` independently when a panel mounts.

import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type {
  InstanceConnectionState,
  MessageEntry,
  RateSample,
  SnapshotView,
  TrEventEnvelope,
  UnitEventEntry,
} from "./types";

// Caps mirror the plan: rolling windows are bounded so memory stays flat
// even on a busy P25 system.
const RATE_CAP = 300; // ~5 min at 1 Hz
const UNIT_EVENT_CAP = 200;
const MESSAGE_CAP = 500;

export interface TrMqttState {
  instances: Record<number, InstanceConnectionState>;
  snapshots: Record<number, SnapshotView>;
  rates: Record<number, RateSample[]>;
  recorders: Record<number, unknown>;
  callsActive: Record<number, unknown>;
  systems: Record<number, unknown>;
  config: Record<number, unknown>;
  unitEvents: Record<number, UnitEventEntry[]>;
  trunkingMessages: Record<number, MessageEntry[]>;
  /** Unix millis of the last `tr.warn.lag` event for each instance. */
  lagWarning: Record<number, number>;
}

const initialState: TrMqttState = {
  instances: {},
  snapshots: {},
  rates: {},
  recorders: {},
  callsActive: {},
  systems: {},
  config: {},
  unitEvents: {},
  trunkingMessages: {},
  lagWarning: {},
};

function pushCapped<T>(arr: T[] | undefined, item: T, cap: number): T[] {
  const next = arr ? [...arr, item] : [item];
  if (next.length > cap) {
    return next.slice(next.length - cap);
  }
  return next;
}

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" ? (v as Record<string, unknown>) : null;
}

function aggregateRate(payload: unknown): number {
  // TR `rates` payload is `{rates: [{...}]}` or similar — sum any numeric
  // `decoderate` / `rate` field present, gracefully fall back to 0.
  const rec = asRecord(payload);
  if (!rec) return 0;
  const rates = rec.rates;
  if (!Array.isArray(rates)) {
    const r = rec.rate;
    return typeof r === "number" ? r : 0;
  }
  let total = 0;
  for (const r of rates) {
    const sub = asRecord(r);
    if (!sub) continue;
    const v = sub.decoderate ?? sub.rate;
    if (typeof v === "number") total += v;
  }
  return total;
}

function unitKindFromTopic(topic: string): string {
  // "tr.unit.on" → "on"
  const last = topic.split(".").pop();
  return last ?? topic;
}

export interface ApplyTrEventArgs {
  topic: string;
  envelope: TrEventEnvelope;
  at?: number;
}

export const trMqttSlice = createSlice({
  name: "trMqtt",
  initialState,
  reducers: {
    /** Hydrate the snapshot for one instance from the REST endpoint. */
    setSnapshot: (
      state,
      action: PayloadAction<{ id: number; snapshot: SnapshotView }>,
    ) => {
      const { id, snapshot } = action.payload;
      state.snapshots[id] = snapshot;
      state.instances[id] = {
        connected: snapshot.Connection?.Connected ?? false,
        lastError: snapshot.Connection?.LastError || undefined,
        lastSeenAt: state.instances[id]?.lastSeenAt,
      };
    },
    /** Drop all per-instance state (e.g. when an instance is deleted). */
    forgetInstance: (state, action: PayloadAction<number>) => {
      const id = action.payload;
      delete state.instances[id];
      delete state.snapshots[id];
      delete state.rates[id];
      delete state.recorders[id];
      delete state.callsActive[id];
      delete state.systems[id];
      delete state.config[id];
      delete state.unitEvents[id];
      delete state.trunkingMessages[id];
      delete state.lagWarning[id];
    },
    /** Single entry point for every `tr.*` admin WS event. */
    applyTrEvent: (state, action: PayloadAction<ApplyTrEventArgs>) => {
      const { topic, envelope, at: ts } = action.payload;
      const id = envelope.instanceId;
      const now = ts ?? Date.now();
      const conn: InstanceConnectionState = state.instances[id] ?? {
        connected: false,
      };

      switch (topic) {
        case "tr.instance.connected":
          state.instances[id] = {
            connected: true,
            lastError: undefined,
            lastSeenAt: now,
          };
          return;
        case "tr.instance.disconnected":
          state.instances[id] = {
            connected: false,
            lastError: envelope.error || conn.lastError,
            lastSeenAt: conn.lastSeenAt,
          };
          return;
        case "tr.warn.lag":
          state.lagWarning[id] = now;
          return;
        case "tr.snapshot": {
          const snap = envelope.payload as SnapshotView | undefined;
          if (snap) state.snapshots[id] = snap;
          state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
          return;
        }
        case "tr.rates": {
          state.rates[id] = pushCapped(
            state.rates[id],
            { at: now, rate: aggregateRate(envelope.payload) },
            RATE_CAP,
          );
          state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
          return;
        }
        case "tr.recorders":
          state.recorders[id] = envelope.payload;
          state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
          return;
        case "tr.callsActive":
          state.callsActive[id] = envelope.payload;
          state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
          return;
        case "tr.systems":
        case "tr.system":
          state.systems[id] = envelope.payload;
          state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
          return;
        case "tr.config":
          state.config[id] = envelope.payload;
          state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
          return;
        case "tr.message": {
          const rec = asRecord(envelope.payload);
          state.trunkingMessages[id] = pushCapped(
            state.trunkingMessages[id],
            {
              at: now,
              topic,
              type: rec?.message_type as string | undefined,
              opcode: rec?.opcode != null ? String(rec.opcode) : undefined,
              opcodeDesc: rec?.opcode_desc as string | undefined,
              shortname: rec?.shortname as string | undefined,
              raw: envelope.payload,
            },
            MESSAGE_CAP,
          );
          state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
          return;
        }
        default:
          break;
      }

      if (topic.startsWith("tr.unit.")) {
        const rec = asRecord(envelope.payload);
        state.unitEvents[id] = pushCapped(
          state.unitEvents[id],
          {
            at: now,
            topic,
            kind: unitKindFromTopic(topic),
            shortname: rec?.shortname as string | undefined,
            unitId: rec?.unit_id != null ? String(rec.unit_id) : undefined,
            talkgroupId:
              rec?.talkgroup != null ? String(rec.talkgroup) : undefined,
            raw: envelope.payload,
          },
          UNIT_EVENT_CAP,
        );
        state.instances[id] = { ...conn, connected: true, lastSeenAt: now };
      }
      // Unknown tr.* topics are silently ignored (forward-compatible).
    },
  },
});

export const { applyTrEvent, setSnapshot, forgetInstance } =
  trMqttSlice.actions;
export default trMqttSlice.reducer;
