import { describe, it, expect, afterEach, vi } from "vitest";
import { isIOS, isAndroid, isMobilePlatform } from "./platform";

const ANDROID_UA =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36";
const DESKTOP_UA =
  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36";
const IPHONE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";

/**
 * Describe the device: user agent, touch points, and primary pointer.
 * jsdom does not define `maxTouchPoints` at all, so these are redefined
 * rather than spied on.
 */
function asDevice(ua: string, touchPoints: number, coarsePointer: boolean) {
  define("userAgent", ua);
  define("maxTouchPoints", touchPoints);
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: query.includes("pointer: coarse") ? coarsePointer : false,
    media: query,
  }));
}

function define(prop: string, value: unknown) {
  Object.defineProperty(navigator, prop, { value, configurable: true });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("isMobilePlatform", () => {
  it("is true on an ordinary Android phone", () => {
    asDevice(ANDROID_UA, 5, true);
    expect(isAndroid()).toBe(true);
    expect(isMobilePlatform()).toBe(true);
  });

  it("is true on an iPhone", () => {
    asDevice(IPHONE_UA, 5, true);
    expect(isIOS()).toBe(true);
    expect(isMobilePlatform()).toBe(true);
  });

  it("stays true on Android with 'Desktop site' enabled", () => {
    // Chrome swaps in a desktop UA, so the UA checks alone see a PC and the
    // background-audio control disappears from a phone that needs it.
    asDevice(DESKTOP_UA, 5, true);
    expect(isAndroid()).toBe(false);
    expect(isMobilePlatform()).toBe(true);
  });

  it("is false on a desktop browser", () => {
    asDevice(DESKTOP_UA, 0, false);
    expect(isMobilePlatform()).toBe(false);
  });

  it("is false on a touch-screen laptop driven by a trackpad", () => {
    // A touch screen is present, but the primary pointer is fine, so this
    // is a desktop and the control would just be clutter.
    asDevice(DESKTOP_UA, 10, false);
    expect(isMobilePlatform()).toBe(false);
  });

  it("does not throw where matchMedia is unavailable", () => {
    asDevice(DESKTOP_UA, 5, true);
    vi.stubGlobal("matchMedia", undefined);
    expect(() => isMobilePlatform()).not.toThrow();
    expect(isMobilePlatform()).toBe(false);
  });
});
