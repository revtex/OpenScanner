import { createListenerMiddleware } from "@reduxjs/toolkit";
import type { RootState } from "@/app/store";
import { callReceived } from "@/features/scanner";
import { audioPlayer } from "@/shared/services/audio/player";

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
 * - Background audio on → the server stream plays instead; drop everything.
 */
/** True while an AVOID entry for this talkgroup is still in force. */
function isAvoided(
  avoidList: RootState["scanner"]["avoidList"],
  talkgroupId: number,
): boolean {
  const now = Date.now();
  for (const entry of avoidList) {
    if (entry.talkgroupId === talkgroupId) {
      if (entry.expiresAt === 0 || entry.expiresAt > now) return true;
    }
  }
  return false;
}

export const audioListenerMiddleware = createListenerMiddleware();

audioListenerMiddleware.startListening({
  actionCreator: callReceived,
  effect: (action, listenerApi) => {
    const state = listenerApi.getState() as RootState;
    const call = action.payload;
    const { heldTG, heldSystem, avoidList, tgSelection, backgroundAudio } =
      state.scanner;

    // Background-audio mode moves playback to the server's continuous
    // stream, which applies the selection itself — enqueuing here too would
    // play every call twice. The lock screen still needs labelling though,
    // and nothing else does it in that mode, so mirror the server's filter
    // (selection + AVOID, but not HOLD, which the server cannot see) and
    // hand the call to the media session.
    if (backgroundAudio) {
      if (isAvoided(avoidList, call.talkgroup)) return;
      if (tgSelection[call.talkgroup] === false) return;
      audioPlayer.setNowPlaying(call);
      return;
    }

    if (!state.scanner.isLive) return;

    if (heldTG !== null) {
      if (call.talkgroup !== heldTG) return;
    } else if (heldSystem !== null) {
      if (call.system !== heldSystem) return;
    }

    if (isAvoided(avoidList, call.talkgroup)) return;

    if (tgSelection[call.talkgroup] === false) return;

    audioPlayer.enqueue(call);
  },
});
