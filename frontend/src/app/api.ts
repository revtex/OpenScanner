import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";
import type { SetupStatus, LoginResponse } from "@/types";

export const api = createApi({
  reducerPath: "api",
  tagTypes: [
    "Users",
    "Systems",
    "Talkgroups",
    "Units",
    "Groups",
    "Tags",
    "ApiKeys",
    "Accesses",
    "Dirwatches",
    "Downstreams",
    "Webhooks",
    "Config",
    "Logs",
  ],
  baseQuery: fetchBaseQuery({
    baseUrl: "/api",
    prepareHeaders: (headers, { getState }) => {
      const state = getState() as { auth: { token: string | null } };
      const token = state.auth?.token;
      if (token) {
        headers.set("Authorization", `Bearer ${token}`);
      }
      return headers;
    },
  }),
  endpoints: (builder) => ({
    getSetupStatus: builder.query<SetupStatus, void>({
      query: () => "/setup/status",
    }),
    postSetup: builder.mutation<void, { username: string; password: string }>({
      query: (body) => ({
        url: "/setup",
        method: "POST",
        body,
      }),
    }),
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
  }),
});

export const {
  useGetSetupStatusQuery,
  usePostSetupMutation,
  usePostLoginMutation,
} = api;
