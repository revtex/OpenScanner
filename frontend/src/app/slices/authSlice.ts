import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type { SetupStatus } from "@/types";

interface AuthState {
  token: string | null;
  role: string | null;
  username: string | null;
  passwordNeedChange: boolean;
  setupStatus: SetupStatus | null;
}

const initialState: AuthState = {
  token: null,
  role: null,
  username: null,
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
    },
    clearCredentials(state) {
      state.token = null;
      state.role = null;
      state.username = null;
      state.passwordNeedChange = false;
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
