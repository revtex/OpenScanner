import { describe, it, expect } from "vitest";
import {
  authSlice,
  setCredentials,
  clearCredentials,
  setAuthReady,
  setSetupStatus,
} from "@/app/slices/shared/authSlice";

const { reducer } = authSlice;

describe("authSlice", () => {
  it("has the expected initial state", () => {
    const state = reducer(undefined, { type: "@@INIT" });
    expect(state.token).toBeNull();
    expect(state.role).toBeNull();
    expect(state.username).toBeNull();
    expect(state.passwordNeedChange).toBe(false);
    expect(state.setupStatus).toBeNull();
    expect(state.authReady).toBe(false);
  });

  it("setCredentials stores token, role, username, and flag", () => {
    const state = reducer(
      undefined,
      setCredentials({
        token: "jwt.abc.def",
        role: "admin",
        username: "alice",
        passwordNeedChange: true,
      }),
    );
    expect(state.token).toBe("jwt.abc.def");
    expect(state.role).toBe("admin");
    expect(state.username).toBe("alice");
    expect(state.passwordNeedChange).toBe(true);
  });

  it("clearCredentials wipes token, role, username, and password flag", () => {
    const filled = reducer(
      undefined,
      setCredentials({
        token: "jwt.abc.def",
        role: "admin",
        username: "alice",
        passwordNeedChange: true,
      }),
    );
    const cleared = reducer(filled, clearCredentials());
    expect(cleared.token).toBeNull();
    expect(cleared.role).toBeNull();
    expect(cleared.username).toBeNull();
    expect(cleared.passwordNeedChange).toBe(false);
  });

  it("setAuthReady flips authReady to true", () => {
    const state = reducer(undefined, setAuthReady());
    expect(state.authReady).toBe(true);
  });

  it("setSetupStatus stores the payload", () => {
    const state = reducer(
      undefined,
      setSetupStatus({ needsSetup: true, publicAccess: false }),
    );
    expect(state.setupStatus).toEqual({
      needsSetup: true,
      publicAccess: false,
    });
  });

  it("setCredentials then clearCredentials leaves a clean state (no leftover token)", () => {
    let state = reducer(undefined, setAuthReady());
    state = reducer(
      state,
      setCredentials({
        token: "jwt.abc.def",
        role: "listener",
        username: "bob",
        passwordNeedChange: false,
      }),
    );
    state = reducer(state, clearCredentials());
    // Critically: no lingering token after logout.
    expect(state.token).toBeNull();
    expect(state.role).toBeNull();
    expect(state.username).toBeNull();
    // authReady is preserved (it's not about login state).
    expect(state.authReady).toBe(true);
  });
});
