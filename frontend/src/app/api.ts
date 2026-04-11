import {
  createApi,
  fetchBaseQuery,
  type BaseQueryFn,
  type FetchArgs,
  type FetchBaseQueryError,
} from "@reduxjs/toolkit/query/react";
import type { SetupStatus, LoginResponse } from "@/types";
import { clearCredentials } from "@/app/slices/authSlice";

const rawBaseQuery = fetchBaseQuery({
  baseUrl: "/api",
  prepareHeaders: (headers, { getState }) => {
    const state = getState() as { auth: { token: string | null } };
    const token = state.auth?.token;
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
    return headers;
  },
});

const baseQueryWith401Redirect: BaseQueryFn<
  string | FetchArgs,
  unknown,
  FetchBaseQueryError
> = async (args, api, extraOptions) => {
  const result = await rawBaseQuery(args, api, extraOptions);
  if (result.error && result.error.status === 401) {
    api.dispatch(clearCredentials());
  }
  return result;
};

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
    "Dirwatches",
    "Downstreams",
    "Webhooks",
    "Config",
    "Logs",
  ],
  baseQuery: baseQueryWith401Redirect,
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
