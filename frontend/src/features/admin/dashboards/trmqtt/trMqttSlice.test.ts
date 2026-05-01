import { describe, it, expect } from "vitest";
import reducer, {
  applyTrEvent,
  setSnapshot,
  forgetInstance,
  type TrMqttState,
} from "./trMqttSlice";
import type { TrEventEnvelope, SnapshotView } from "./types";

const ID = 1;

function envelope(payload: unknown, error?: string): TrEventEnvelope {
  return { instanceId: ID, label: "test", payload, error };
}

function emptyState(): TrMqttState {
  return reducer(undefined, { type: "@@INIT" });
}

describe("trMqttSlice", () => {
  it("tr.instance.connected marks instance connected and clears error", () => {
    const before: TrMqttState = {
      ...emptyState(),
      instances: { [ID]: { connected: false, lastError: "boom" } },
    };
    const next = reducer(
      before,
      applyTrEvent({
        topic: "tr.instance.connected",
        envelope: envelope({}),
        at: 1000,
      }),
    );
    expect(next.instances[ID]).toEqual({
      connected: true,
      lastError: undefined,
      lastSeenAt: 1000,
    });
  });

  it("tr.instance.disconnected captures envelope error", () => {
    const next = reducer(
      emptyState(),
      applyTrEvent({
        topic: "tr.instance.disconnected",
        envelope: envelope({}, "broker down"),
        at: 5,
      }),
    );
    expect(next.instances[ID].connected).toBe(false);
    expect(next.instances[ID].lastError).toBe("broker down");
  });

  it("tr.rates pushes a sample with aggregated decoderate", () => {
    const next = reducer(
      emptyState(),
      applyTrEvent({
        topic: "tr.rates",
        envelope: envelope({
          rates: [{ decoderate: 1.5 }, { decoderate: 2.5 }],
        }),
        at: 100,
      }),
    );
    expect(next.rates[ID]).toEqual([{ at: 100, rate: 4 }]);
  });

  it("tr.rates caps the rolling window to 300 samples", () => {
    let s = emptyState();
    for (let i = 0; i < 305; i++) {
      s = reducer(
        s,
        applyTrEvent({
          topic: "tr.rates",
          envelope: envelope({ rates: [{ decoderate: i }] }),
          at: i,
        }),
      );
    }
    expect(s.rates[ID]).toHaveLength(300);
    expect(s.rates[ID][0].rate).toBe(5);
    expect(s.rates[ID][299].rate).toBe(304);
  });

  it("tr.message captures opcode/desc and caps to 500", () => {
    let s = emptyState();
    s = reducer(
      s,
      applyTrEvent({
        topic: "tr.message",
        envelope: envelope({
          opcode: 0x39,
          opcode_desc: "GRP_V_CH_GRANT",
          shortname: "sys1",
        }),
        at: 1,
      }),
    );
    expect(s.trunkingMessages[ID]).toHaveLength(1);
    const e = s.trunkingMessages[ID][0];
    expect(e.opcode).toBe("57");
    expect(e.opcodeDesc).toBe("GRP_V_CH_GRANT");
    expect(e.shortname).toBe("sys1");

    for (let i = 0; i < 510; i++) {
      s = reducer(
        s,
        applyTrEvent({
          topic: "tr.message",
          envelope: envelope({ opcode: i }),
          at: i,
        }),
      );
    }
    expect(s.trunkingMessages[ID]).toHaveLength(500);
  });

  it("tr.unit.* pushes unit events and caps to 200", () => {
    let s = emptyState();
    s = reducer(
      s,
      applyTrEvent({
        topic: "tr.unit.on",
        envelope: envelope({ unit_id: 4242, talkgroup: 11, shortname: "sys1" }),
        at: 1,
      }),
    );
    expect(s.unitEvents[ID]).toHaveLength(1);
    const e = s.unitEvents[ID][0];
    expect(e.kind).toBe("on");
    expect(e.unitId).toBe("4242");
    expect(e.talkgroupId).toBe("11");

    for (let i = 0; i < 210; i++) {
      s = reducer(
        s,
        applyTrEvent({
          topic: "tr.unit.call",
          envelope: envelope({ unit_id: i }),
          at: i,
        }),
      );
    }
    expect(s.unitEvents[ID]).toHaveLength(200);
  });

  it("tr.warn.lag stamps lagWarning timestamp", () => {
    const next = reducer(
      emptyState(),
      applyTrEvent({
        topic: "tr.warn.lag",
        envelope: envelope({}),
        at: 9999,
      }),
    );
    expect(next.lagWarning[ID]).toBe(9999);
  });

  it("tr.recorders / tr.callsActive / tr.systems / tr.config store payload as-is", () => {
    let s = emptyState();
    const cases = [
      ["tr.recorders", "recorders"],
      ["tr.callsActive", "callsActive"],
      ["tr.systems", "systems"],
      ["tr.config", "config"],
    ] as const;
    for (const [topic, key] of cases) {
      const payload = { tag: topic };
      s = reducer(
        s,
        applyTrEvent({ topic, envelope: envelope(payload), at: 1 }),
      );
      expect((s[key] as Record<number, unknown>)[ID]).toEqual(payload);
    }
  });

  it("setSnapshot hydrates snapshot + connection from REST", () => {
    const snap: SnapshotView = {
      InstanceID: ID,
      Label: "test",
      PluginInstanceID: "tr1",
      Connection: { Connected: true, LastConnected: "now" },
    };
    const next = reducer(emptyState(), setSnapshot({ id: ID, snapshot: snap }));
    expect(next.snapshots[ID]).toBe(snap);
    expect(next.instances[ID].connected).toBe(true);
  });

  it("forgetInstance drops every per-instance map", () => {
    let s = emptyState();
    s = reducer(
      s,
      applyTrEvent({
        topic: "tr.rates",
        envelope: envelope({ rates: [{ decoderate: 1 }] }),
        at: 1,
      }),
    );
    s = reducer(s, forgetInstance(ID));
    expect(s.rates[ID]).toBeUndefined();
    expect(s.instances[ID]).toBeUndefined();
  });

  it("ignores unknown tr.* topics without throwing", () => {
    const next = reducer(
      emptyState(),
      applyTrEvent({
        topic: "tr.someFutureTopic",
        envelope: envelope({}),
        at: 1,
      }),
    );
    expect(next).toEqual(emptyState());
  });
});
