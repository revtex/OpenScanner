import { describe, it, expect } from "vitest";
import {
  scannerSlice,
  callReceived,
  skipCall,
  togglePause,
  toggleLive,
  holdSystem,
  holdTG,
  addAvoid,
  removeAvoid,
  clearAvoids,
  toggleTG,
  setAllTGs,
  setConfig,
  transcriptReceived,
} from "@/app/slices/scannerSlice";
import type { Call, ScannerConfig } from "@/types";

const reducer = scannerSlice.reducer;

function makeCall(overrides: Partial<Call> = {}): Call {
  return {
    id: 1,
    audioName: "test.wav",
    audioType: "audio/wav",
    dateTime: Date.now(),
    systemId: 100,
    system: 1,
    talkgroupId: 200,
    talkgroup: 2,
    ...overrides,
  };
}

describe("scannerSlice", () => {
  describe("callReceived", () => {
    it("sets currentCall when queue is empty", () => {
      const state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      expect(state.currentCall?.id).toBe(1);
      expect(state.callQueue).toHaveLength(0);
    });

    it("queues call when currentCall already exists", () => {
      let state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      state = reducer(state, callReceived(makeCall({ id: 2 })));
      expect(state.currentCall?.id).toBe(1);
      expect(state.callQueue).toHaveLength(1);
      expect(state.callQueue[0].id).toBe(2);
    });

    it("adds to history (front)", () => {
      const state = reducer(undefined, callReceived(makeCall({ id: 10 })));
      expect(state.history[0].id).toBe(10);
    });

    it("caps history at 5 items", () => {
      let state = reducer(undefined, { type: "init" });
      for (let i = 1; i <= 7; i++) {
        state = reducer(state, callReceived(makeCall({ id: i })));
      }
      expect(state.history).toHaveLength(5);
      // Most recent first
      expect(state.history[0].id).toBe(7);
      expect(state.history[4].id).toBe(3);
    });
  });

  describe("skipCall", () => {
    it("advances to next call in queue", () => {
      let state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      state = reducer(state, callReceived(makeCall({ id: 2 })));
      state = reducer(state, callReceived(makeCall({ id: 3 })));
      state = reducer(state, skipCall());
      expect(state.currentCall?.id).toBe(2);
      expect(state.callQueue).toHaveLength(1);
    });

    it("sets currentCall to null when queue is empty", () => {
      let state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      state = reducer(state, skipCall());
      expect(state.currentCall).toBeNull();
    });
  });

  describe("togglePause", () => {
    it("toggles isPaused from false to true", () => {
      const state = reducer(undefined, togglePause());
      expect(state.isPaused).toBe(true);
    });

    it("toggles isPaused from true to false", () => {
      let state = reducer(undefined, togglePause());
      state = reducer(state, togglePause());
      expect(state.isPaused).toBe(false);
    });
  });

  describe("toggleLive", () => {
    it("toggles isLive from true to false", () => {
      const state = reducer(undefined, toggleLive());
      expect(state.isLive).toBe(false);
    });

    it("toggles isLive from false to true", () => {
      let state = reducer(undefined, toggleLive());
      state = reducer(state, toggleLive());
      expect(state.isLive).toBe(true);
    });
  });

  describe("holdSystem / holdTG", () => {
    it("sets heldSystem", () => {
      const state = reducer(undefined, holdSystem(42));
      expect(state.heldSystem).toBe(42);
    });

    it("clears heldSystem with null", () => {
      let state = reducer(undefined, holdSystem(42));
      state = reducer(state, holdSystem(null));
      expect(state.heldSystem).toBeNull();
    });

    it("sets heldTG", () => {
      const state = reducer(undefined, holdTG(99));
      expect(state.heldTG).toBe(99);
    });

    it("clears heldTG with null", () => {
      let state = reducer(undefined, holdTG(99));
      state = reducer(state, holdTG(null));
      expect(state.heldTG).toBeNull();
    });
  });

  describe("addAvoid / removeAvoid / clearAvoids", () => {
    it("adds an avoid entry", () => {
      const state = reducer(
        undefined,
        addAvoid({ talkgroupId: 10, expiresAt: 0 }),
      );
      expect(state.avoidList).toHaveLength(1);
      expect(state.avoidList[0].talkgroupId).toBe(10);
    });

    it("replaces existing avoid for same talkgroup", () => {
      let state = reducer(
        undefined,
        addAvoid({ talkgroupId: 10, expiresAt: 1000 }),
      );
      state = reducer(state, addAvoid({ talkgroupId: 10, expiresAt: 2000 }));
      expect(state.avoidList).toHaveLength(1);
      expect(state.avoidList[0].expiresAt).toBe(2000);
    });

    it("removes an avoid entry by talkgroupId", () => {
      let state = reducer(
        undefined,
        addAvoid({ talkgroupId: 10, expiresAt: 0 }),
      );
      state = reducer(state, removeAvoid(10));
      expect(state.avoidList).toHaveLength(0);
    });

    it("clears all avoids", () => {
      let state = reducer(
        undefined,
        addAvoid({ talkgroupId: 10, expiresAt: 0 }),
      );
      state = reducer(state, addAvoid({ talkgroupId: 20, expiresAt: 0 }));
      state = reducer(state, clearAvoids());
      expect(state.avoidList).toHaveLength(0);
    });
  });

  describe("toggleTG", () => {
    it("flips talkgroup selection from undefined to true", () => {
      const state = reducer(undefined, toggleTG(5));
      expect(state.tgSelection[5]).toBe(true);
    });

    it("flips talkgroup selection from true to false", () => {
      let state = reducer(undefined, toggleTG(5));
      state = reducer(state, toggleTG(5));
      expect(state.tgSelection[5]).toBe(false);
    });
  });

  describe("setAllTGs", () => {
    const config: ScannerConfig = {
      systems: [
        {
          id: 1,
          systemId: 100,
          label: "System 1",
          talkgroups: [
            {
              id: 10,
              talkgroupId: 200,
              label: "TG1",
              name: "Talkgroup 1",
              tag: "Law",
              group: "Police",
              ledColor: "#00e676",
            },
            {
              id: 11,
              talkgroupId: 201,
              label: "TG2",
              name: "Talkgroup 2",
              tag: "Fire",
              group: "Fire",
              ledColor: "#ff0000",
            },
          ],
        },
      ],
      branding: "TEST",
      email: "",
      version: "1.0",
    };

    it("enables all talkgroups", () => {
      let state = reducer(undefined, setConfig(config));
      state = reducer(state, setAllTGs(true));
      expect(state.tgSelection[10]).toBe(true);
      expect(state.tgSelection[11]).toBe(true);
    });

    it("disables all talkgroups", () => {
      let state = reducer(undefined, setConfig(config));
      state = reducer(state, setAllTGs(true));
      state = reducer(state, setAllTGs(false));
      expect(state.tgSelection[10]).toBe(false);
      expect(state.tgSelection[11]).toBe(false);
    });

    it("does nothing without config", () => {
      const state = reducer(undefined, setAllTGs(true));
      expect(Object.keys(state.tgSelection)).toHaveLength(0);
    });
  });

  describe("transcriptReceived", () => {
    it("updates transcript on currentCall", () => {
      let state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      state = reducer(
        state,
        transcriptReceived({ callId: 1, text: "hello world" }),
      );
      expect(state.currentCall?.transcript).toBe("hello world");
    });

    it("updates transcript in history", () => {
      let state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      state = reducer(
        state,
        transcriptReceived({ callId: 1, text: "transcript text" }),
      );
      expect(state.history[0].transcript).toBe("transcript text");
    });

    it("updates transcript in callQueue", () => {
      let state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      state = reducer(state, callReceived(makeCall({ id: 2 })));
      state = reducer(
        state,
        transcriptReceived({ callId: 2, text: "queued transcript" }),
      );
      expect(state.callQueue[0].transcript).toBe("queued transcript");
    });

    it("does not fail if callId not found", () => {
      let state = reducer(undefined, callReceived(makeCall({ id: 1 })));
      state = reducer(state, transcriptReceived({ callId: 999, text: "nope" }));
      expect(state.currentCall?.transcript).toBeUndefined();
    });
  });
});
