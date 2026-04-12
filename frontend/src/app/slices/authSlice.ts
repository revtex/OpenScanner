import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import { api } from "@/app/api";
import type {
  SetupStatus,
  LoginResponse,
  ChangePasswordRequest,
} from "@/types";

interface AuthState {
  token: string | null;
  role: string | null;
  username: string | null;
  passwordNeedChange: boolean;
  setupStatus: SetupStatus | null;
}

function loadPersistedAuth(): Pick<AuthState, "token" | "role" | "username"> {
  try {
    const raw = sessionStorage.getItem("os_auth");
    if (raw) {
      const parsed = JSON.parse(raw) as {
        token?: string;
        role?: string;
        username?: string;
      };
      if (parsed.token && parsed.role && parsed.username) {
        return {
          token: parsed.token,
          role: parsed.role,
          username: parsed.username,
        };
      }
    }
  } catch {
    // ignore parse errors
  }
  return { token: null, role: null, username: null };
}

const persisted = loadPersistedAuth();

const initialState: AuthState = {
  token: persisted.token,
  role: persisted.role,
  username: persisted.username,
  passwordNeedChange: false,
  setupStatus: null,
};

export const authSlice = createSlice({
  name: "auth",
  initialState,
  reducers: {
    setCredentials(
      state,
      action: PayloadAction<{
        token: string;
        role: string;
        username: string;
        passwordNeedChange: boolean;
      }>,
    ) {
      state.token = action.payload.token;
      state.role = action.payload.role;
      state.username = action.payload.username;
      state.passwordNeedChange = action.payload.passwordNeedChange;
      try {
        sessionStorage.setItem(
          "os_auth",
          JSON.stringify({
            token: action.payload.token,
            role: action.payload.role,
            username: action.payload.username,
          }),
        );
      } catch {
        // storage full or unavailable
      }
    },
    clearCredentials(state) {
      state.token = null;
      state.role = null;
      state.username = null;
      state.passwordNeedChange = false;
      try {
        sessionStorage.removeItem("os_auth");
      } catch {
        // ignore
      }
    },
    setSetupStatus(state, action: PayloadAction<SetupStatus>) {
      state.setupStatus = action.payload;
    },
  },
});

export const { setCredentials, clearCredentials, setSetupStatus } =
  authSlice.actions;

export const selectToken = (state: { auth: AuthState }) => state.auth.token;
export const selectRole = (state: { auth: AuthState }) => state.auth.role;
export const selectUsername = (state: { auth: AuthState }) =>
  state.auth.username;

// ── Auth RTK Query endpoints ──

const authApi = api.injectEndpoints({
  endpoints: (builder) => ({
    postLogin: builder.mutation<
      LoginResponse,
      { username: string; password: string }
    >({
      query: (body) => ({
        url: "/auth/login",
        method: "POST",
        body,
      }),
    }),
    changePassword: builder.mutation<void, ChangePasswordRequest>({
      query: (body) => ({
        url: "/auth/password",
        method: "PUT",
        body,
      }),
    }),
  }),
});

export const { usePostLoginMutation, useChangePasswordMutation } = authApi;
