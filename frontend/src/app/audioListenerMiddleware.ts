import { createListenerMiddleware } from "@reduxjs/toolkit";
import type { RootState } from "@/app/store";
import { callReceived } from "@/app/slices/scanner/scannerSlice";
import { audioPlayer } from "@/services/audio/player";

/**
 * Listener middleware that bridges incoming Redux call events to the
 * audio player. Replaces the previous WS → callback → player pipeline:
 * the WS client now only dispatches `callReceived(call)`, and this
 * middleware enqueues into the player when scanner state allows.
 *
 * Filtering rules mirror the scanner UI:
 * - LIVE off → drop (player also clears on LIVE off, see useAudioPlayer).
 * - HOLD TG → only the held talkgroup plays.
 * - HOLD SYSTEM → only that system plays.
 * - AVOID → active avoid entries block their talkgroup.
 * - SELECT → talkgroups explicitly disabled in tgSelection are dropped.
 */
export const audioListenerMiddleware = createListenerMiddleware();

audioListenerMiddleware.startListening({
  actionCreator: callReceived,
  effect: (action, listenerApi) => {
    const state = listenerApi.getState() as RootState;
    if (!state.scanner.isLive) return;

    const call = action.payload;
    const { heldTG, heldSystem, avoidList, tgSelection } = state.scanner;

    if (heldTG !== null) {
      if (call.talkgroup !== heldTG) return;
    } else if (heldSystem !== null) {
      if (call.system !== heldSystem) return;
    }

    const now = Date.now();
    for (const entry of avoidList) {
      if (entry.talkgroupId === call.talkgroup) {
        if (entry.expiresAt === 0 || entry.expiresAt > now) return;
      }
    }

    if (tgSelection[call.talkgroup] === false) return;

    audioPlayer.enqueue(call);
  },
});
