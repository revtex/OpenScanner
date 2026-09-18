import { describe, it, expect, vi, beforeEach } from "vitest";
import { configureStore } from "@reduxjs/toolkit";

const enqueue = vi.fn();
const setNowPlaying = vi.fn();
const hold = vi.fn();

vi.mock("@/shared/services/audio/player", () => ({
  audioPlayer: {
    enqueue: (...a: unknown[]) => enqueue(...a),
    setNowPlaying: (...a: unknown[]) => setNowPlaying(...a),
  },
}));

vi.mock("@/shared/services/audio/streamCues", () => ({
  streamCues: {
    hold: (...a: unknown[]) => hold(...a),
    configure: () => {},
    reset: () => {},
  },
}));

vi.mock("@/shared/services/audio/streamPlayer", () => ({
  streamPlayer: { currentTime: () => null, setOnReset: () => {} },
}));

import {
  scannerSlice,
  callReceived,
  setLive,
  setBackgroundAudio,
  toggleTG,
  addAvoid,
  type Call,
} from "@/features/scanner";
import { audioListenerMiddleware } from "./audioListenerMiddleware";

function makeStore() {
  return configureStore({
    reducer: { scanner: scannerSlice.reducer },
    middleware: (getDefault) =>
      getDefault().prepend(audioListenerMiddleware.middleware),
  });
}

/**
 * RTK's listener middleware schedules effects rather than running them
 * inside dispatch, so every assertion has to yield first.
 */
async function settle(): Promise<void> {
  await new Promise<void>((resolve) => {
    setTimeout(resolve, 0);
  });
}

function makeCall(talkgroup: number): Call {
  return {
    id: 1000 + talkgroup,
    audioName: "c.m4a",
    audioType: "audio/mp4",
    dateTime: Math.floor(Date.now() / 1000),
    systemId: 100,
    system: 1,
    talkgroupId: 27000 + talkgroup,
    talkgroup,
  };
}

describe("audioListenerMiddleware", () => {
  beforeEach(() => {
    enqueue.mockClear();
    setNowPlaying.mockClear();
    hold.mockClear();
  });

  it("enqueues locally when live and not streaming", async () => {
    const store = makeStore();
    store.dispatch(setLive(true));
    store.dispatch(callReceived(makeCall(165)));
    await settle();

    expect(enqueue).toHaveBeenCalledTimes(1);
    expect(hold).not.toHaveBeenCalled();
  });

  it("labels the media session instead of playing when streaming", async () => {
    const store = makeStore();
    store.dispatch(setBackgroundAudio(true));
    store.dispatch(callReceived(makeCall(165)));
    await settle();

    // The server plays the audio; nothing here should touch the element.
    // The label is handed to the cue scheduler rather than published now,
    // because the audio is still seconds down the stream's buffer.
    expect(enqueue).not.toHaveBeenCalled();
    expect(setNowPlaying).not.toHaveBeenCalled();
    expect(hold).toHaveBeenCalledTimes(1);
  });

  it("labels while streaming even with LIVE off", async () => {
    const store = makeStore();
    store.dispatch(setLive(false));
    store.dispatch(setBackgroundAudio(true));
    store.dispatch(callReceived(makeCall(165)));
    await settle();

    // LIVE is disabled in the UI during streaming, so it must not gate the
    // label — otherwise the lock screen stays blank for the whole session.
    expect(hold).toHaveBeenCalledTimes(1);
  });

  it("does not label a talkgroup the listener has disabled", async () => {
    const store = makeStore();
    store.dispatch(setBackgroundAudio(true));
    store.dispatch(toggleTG(165)); // no stored preference -> disabled
    store.dispatch(callReceived(makeCall(165)));
    await settle();

    expect(hold).not.toHaveBeenCalled();
  });

  it("does not label a talkgroup the listener is avoiding", async () => {
    const store = makeStore();
    store.dispatch(setBackgroundAudio(true));
    store.dispatch(addAvoid({ talkgroupId: 165, expiresAt: 0 }));
    store.dispatch(callReceived(makeCall(165)));
    await settle();

    expect(hold).not.toHaveBeenCalled();
  });

  it("still labels once an avoid has expired", async () => {
    const store = makeStore();
    store.dispatch(setBackgroundAudio(true));
    store.dispatch(
      addAvoid({ talkgroupId: 165, expiresAt: Date.now() - 1000 }),
    );
    store.dispatch(callReceived(makeCall(165)));
    await settle();

    expect(hold).toHaveBeenCalledTimes(1);
  });

  it("drops calls entirely when neither live nor streaming", async () => {
    const store = makeStore();
    store.dispatch(callReceived(makeCall(165)));
    await settle();

    expect(enqueue).not.toHaveBeenCalled();
    expect(hold).not.toHaveBeenCalled();
  });
});
