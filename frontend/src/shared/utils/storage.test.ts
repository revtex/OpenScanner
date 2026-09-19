import { describe, it, expect, beforeEach } from "vitest";
import {
  readStored,
  writeStored,
  removeStored,
  migrateThemeValue,
} from "./storage";

describe("storage migration across the rename", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it("reads a value stored under the pre-rename key", () => {
    // The exact case that would otherwise hand a returning user a blank
    // talkgroup selection and leave their real one stranded.
    localStorage.setItem("openscanner-tg-selection-abc", '{"27001":false}');

    expect(readStored(localStorage, "squelch-tg-selection-abc")).toBe(
      '{"27001":false}',
    );
  });

  it("migrates the value forward and drops the old key", () => {
    localStorage.setItem("openscanner-theme", "openscanner-dark");

    readStored(localStorage, "squelch-theme");

    expect(localStorage.getItem("squelch-theme")).toBe("openscanner-dark");
    expect(localStorage.getItem("openscanner-theme")).toBeNull();
  });

  it("prefers the current key when both exist", () => {
    localStorage.setItem("openscanner-theme", "openscanner-light");
    localStorage.setItem("squelch-theme", "squelch-dark");

    expect(readStored(localStorage, "squelch-theme")).toBe("squelch-dark");
  });

  it("returns null when neither key exists", () => {
    expect(readStored(localStorage, "squelch-nothing")).toBeNull();
  });

  it("writes only the current key", () => {
    writeStored(localStorage, "squelch-paused", "true");

    expect(localStorage.getItem("squelch-paused")).toBe("true");
    expect(localStorage.getItem("openscanner-paused")).toBeNull();
  });

  it("removes both spellings", () => {
    localStorage.setItem("openscanner-paused", "true");
    localStorage.setItem("squelch-paused", "true");

    removeStored(localStorage, "squelch-paused");

    expect(localStorage.getItem("squelch-paused")).toBeNull();
    expect(localStorage.getItem("openscanner-paused")).toBeNull();
  });

  it("works on sessionStorage too", () => {
    sessionStorage.setItem("openscanner-display-prefs", '{"x":1}');

    expect(readStored(sessionStorage, "squelch-display-prefs")).toBe('{"x":1}');
  });

  it("does not throw when storage is unavailable", () => {
    const blocked = {
      getItem() {
        throw new Error("blocked");
      },
      setItem() {
        throw new Error("blocked");
      },
      removeItem() {
        throw new Error("blocked");
      },
    } as unknown as Storage;

    expect(() => readStored(blocked, "squelch-theme")).not.toThrow();
    expect(readStored(blocked, "squelch-theme")).toBeNull();
    expect(() => writeStored(blocked, "squelch-theme", "x")).not.toThrow();
    expect(() => removeStored(blocked, "squelch-theme")).not.toThrow();
  });

  describe("migrateThemeValue", () => {
    it("maps a stored theme name forward", () => {
      // A stale "openscanner-dark" would name a theme that no longer
      // exists in the stylesheet, rendering the app unstyled.
      expect(migrateThemeValue("openscanner-dark")).toBe("squelch-dark");
      expect(migrateThemeValue("openscanner-light")).toBe("squelch-light");
    });

    it("leaves a current value alone", () => {
      expect(migrateThemeValue("squelch-dark")).toBe("squelch-dark");
    });

    it("passes null through", () => {
      expect(migrateThemeValue(null)).toBeNull();
    });
  });
});
