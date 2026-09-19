/**
 * Platform checks used to decide which controls are worth showing.
 *
 * Primarily user-agent based: the question these answer is "is this a phone
 * or tablet whose browser suspends a backgrounded page", which no feature
 * test reports directly. A touch-screen laptop is not the same case, so
 * `maxTouchPoints` alone would be wrong.
 *
 * The user agent alone is not enough, though — see `isTouchFirst`.
 */

/** iOS and iPadOS, including the desktop-UA iPad. */
export function isIOS(): boolean {
  if (typeof navigator === "undefined") return false;
  if (/iP(hone|od|ad)/.test(navigator.userAgent)) return true;
  // iPadOS 13+ reports a desktop user agent; touch points disambiguate it.
  return navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
}

/** Android phones and tablets. */
export function isAndroid(): boolean {
  if (typeof navigator === "undefined") return false;
  return /Android/.test(navigator.userAgent);
}

/**
 * A device whose *primary* input is a touch screen.
 *
 * This backstops the user-agent checks, which a browser will happily lie
 * about: Chrome for Android's "Desktop site" switch replaces the UA with a
 * Linux desktop string containing no "Android", so `isAndroid()` returns
 * false on a phone that is still very much a phone. That hid the
 * background-audio control on a real device with no way for the user to
 * tell why. Media queries describe the hardware and are not rewritten by
 * that switch (verified: a desktop UA with touch still reports
 * `pointer: coarse`).
 *
 * `pointer: coarse` asks about the primary pointer, so a laptop with both a
 * touch screen and a trackpad answers false — that is the case the UA
 * checks alone were protecting, and it is still protected. Note this is the
 * one case browser emulation cannot reproduce, so it rests on the spec
 * rather than on a measurement here. If it ever does misfire, the cost is
 * one extra control on a laptop, where the stream works anyway; the cost of
 * the opposite mistake is a phone that cannot play in the background at
 * all.
 */
function isTouchFirst(): boolean {
  if (typeof navigator === "undefined" || typeof matchMedia === "undefined") {
    return false;
  }
  return matchMedia("(pointer: coarse)").matches && navigator.maxTouchPoints > 0;
}

/**
 * True on the mobile platforms that suspend a backgrounded page, which is
 * the only place the continuous audio stream earns its keep. A desktop
 * browser keeps a background tab running and plays each call normally, so
 * offering the toggle there is just clutter.
 */
export function isMobilePlatform(): boolean {
  return isIOS() || isAndroid() || isTouchFirst();
}
