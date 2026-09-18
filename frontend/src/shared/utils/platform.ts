/**
 * Platform checks used to decide which controls are worth showing.
 *
 * Deliberately user-agent based rather than feature based: the question
 * these answer is "is this a phone or tablet whose browser suspends a
 * backgrounded page", which no feature test reports. A touch-screen laptop
 * is not the same case, so `maxTouchPoints` alone would be wrong.
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
 * True on the mobile platforms that suspend a backgrounded page, which is
 * the only place the continuous audio stream earns its keep. A desktop
 * browser keeps a background tab running and plays each call normally, so
 * offering the toggle there is just clutter.
 */
export function isMobilePlatform(): boolean {
  return isIOS() || isAndroid();
}
