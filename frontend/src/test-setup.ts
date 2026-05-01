import "@testing-library/jest-dom";

// jsdom doesn't implement Web Audio. The audioPlayer singleton (loaded
// via @/app/store) registers a document.body click handler that calls
// `new AudioContext()` on the first user gesture, so any test using
// userEvent.click() would otherwise fire an unhandled rejection.
// Stubbing here keeps tests jsdom-pure without touching app code.
if (typeof globalThis.AudioContext === "undefined") {
  class StubAudioContext {
    state = "running";
    onstatechange: (() => void) | null = null;
    async resume() {
      /* noop */
    }
    async suspend() {
      /* noop */
    }
    async close() {
      /* noop */
    }
    createOscillator() {
      return {
        type: "sine",
        frequency: { value: 0, setValueAtTime() {} },
        connect() {},
        start() {},
        stop() {},
        disconnect() {},
      };
    }
    createGain() {
      return {
        gain: {
          value: 0,
          setValueAtTime() {},
          linearRampToValueAtTime() {},
          exponentialRampToValueAtTime() {},
        },
        connect() {},
        disconnect() {},
      };
    }
    get destination() {
      return {};
    }
    get currentTime() {
      return 0;
    }
  }
  // @ts-expect-error — minimal stub for jsdom
  globalThis.AudioContext = StubAudioContext;
}
