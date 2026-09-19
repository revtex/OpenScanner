import { describe, it, expect, beforeEach, vi } from "vitest";

import { readStored, writeStored, removeStored } from "@/shared/utils/storage";

/** A Storage stand-in whose accessors can be made to throw. */
function makeStorage(throwing = false): Storage {
  const data = new Map<string, string>();
  const guard = () => {
    if (throwing) throw new DOMException("blocked", "SecurityError");
  };
  return {
    get length() {
      return data.size;
    },
    clear: () => {
      guard();
      data.clear();
    },
    getItem: (k: string) => {
      guard();
      return data.get(k) ?? null;
    },
    key: (i: number) => Array.from(data.keys())[i] ?? null,
    removeItem: (k: string) => {
      guard();
      data.delete(k);
    },
    setItem: (k: string, v: string) => {
      guard();
      data.set(k, v);
    },
  } as Storage;
}

describe("readStored", () => {
  let storage: Storage;
  beforeEach(() => {
    storage = makeStorage();
  });

  it("returns a stored value", () => {
    storage.setItem("squelch-theme", "squelch-dark");
    expect(readStored(storage, "squelch-theme")).toBe("squelch-dark");
  });

  it("returns null for a key that was never set", () => {
    expect(readStored(storage, "squelch-theme")).toBeNull();
  });

  it("returns null instead of throwing when storage is blocked", () => {
    expect(readStored(makeStorage(true), "squelch-theme")).toBeNull();
  });
});

describe("writeStored", () => {
  it("stores a value", () => {
    const storage = makeStorage();
    writeStored(storage, "squelch-paused", "true");
    expect(storage.getItem("squelch-paused")).toBe("true");
  });

  it("does not throw when storage is blocked", () => {
    expect(() =>
      writeStored(makeStorage(true), "squelch-paused", "true"),
    ).not.toThrow();
  });
});

describe("removeStored", () => {
  it("removes a value", () => {
    const storage = makeStorage();
    storage.setItem("squelch-paused", "true");
    removeStored(storage, "squelch-paused");
    expect(storage.getItem("squelch-paused")).toBeNull();
  });

  it("does not throw when storage is blocked", () => {
    expect(() => removeStored(makeStorage(true), "squelch-paused")).not.toThrow();
  });
});

describe("no legacy key handling", () => {
  it("does not read a pre-v3 openscanner- key", () => {
    // The migration shim was removed in v3.0.0. A value left under the
    // old key must be ignored, not silently resurrected.
    const storage = makeStorage();
    storage.setItem("openscanner-theme", "openscanner-dark");
    expect(readStored(storage, "squelch-theme")).toBeNull();
    expect(vi.isMockFunction(storage.getItem)).toBe(false);
  });
});
