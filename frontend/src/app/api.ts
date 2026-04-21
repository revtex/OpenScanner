import {
  createApi,
  fetchBaseQuery,
  type BaseQueryFn,
  type FetchArgs,
  type FetchBaseQueryError,
} from "@reduxjs/toolkit/query/react";
import type { SetupStatus, RefreshResponse } from "@/types";

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

/**
 * Wrapper that intercepts 401 responses and attempts a silent token refresh.
 * If the refresh succeeds, the original request is retried with the new token.
 * If the refresh fails, credentials are cleared (user sees login screen).
 */
const baseQueryWithRefresh: BaseQueryFn<
  string | FetchArgs,
  unknown,
  FetchBaseQueryError
> = async (args, storeApi, extraOptions) => {
  let result = await rawBaseQuery(args, storeApi, extraOptions);

  if (result.error && result.error.status === 401) {
    // Don't try to refresh if the failing request IS the refresh endpoint.
    const url = typeof args === "string" ? args : args.url;
    if (url === "/auth/refresh") {
      storeApi.dispatch({ type: "auth/clearCredentials" });
      return result;
    }

    // Attempt silent refresh.
    const refreshResult = await rawBaseQuery(
      { url: "/auth/refresh", method: "POST" },
      storeApi,
      extraOptions,
    );

    if (refreshResult.data) {
      const refreshData = refreshResult.data as RefreshResponse;
      storeApi.dispatch({
        type: "auth/setCredentials",
        payload: {
          token: refreshData.token,
          role: refreshData.user.role,
          username: refreshData.user.username,
          passwordNeedChange: false,
        },
      });
      // Retry original request with new token.
      result = await rawBaseQuery(args, storeApi, extraOptions);
    } else {
      storeApi.dispatch({ type: "auth/clearCredentials" });
    }
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
    "DirMonitors",
    "Downstreams",
    "Webhooks",
    "Config",
    "Logs",
    "Calls",
    "Bookmarks",
    "Setup",
    "SharedLinks",
  ],
  baseQuery: baseQueryWithRefresh,
  endpoints: (builder) => ({
    getSetupStatus: builder.query<SetupStatus, void>({
      query: () => "/setup/status",
      providesTags: ["Setup"],
    }),
    postSetup: builder.mutation<void, { username: string; password: string }>({
      query: (body) => ({
        url: "/setup",
        method: "POST",
        body,
      }),
      invalidatesTags: ["Setup"],
    }),
    getBookmarkIDs: builder.query<{ callIds: number[] }, void>({
      query: () => "/bookmarks",
      providesTags: ["Bookmarks"],
    }),
    toggleBookmark: builder.mutation<
      { bookmarked: boolean; id?: number },
      number
    >({
      query: (callId) => ({
        url: "/bookmarks",
        method: "POST",
        body: { callId },
      }),
      invalidatesTags: ["Bookmarks"],
    }),
    getBookmarkCalls: builder.query<
      {
        calls: {
          id: number;
          audioName: string;
          audioType: string;
          dateTime: number;
          systemId: number;
          talkgroupId: number;
          systemLabel: string;
          talkgroupLabel: string;
          talkgroupName: string;
          talkgroupLed: string;
          frequency?: number;
          duration?: number;
          source?: number;
          errorCount?: number;
          spikeCount?: number;
          transcript?: string;
          bookmarked: boolean;
        }[];
      },
      void
    >({
      query: () => "/bookmarks/calls",
      providesTags: ["Bookmarks"],
    }),
  }),
});

export const {
  useGetSetupStatusQuery,
  usePostSetupMutation,
  useGetBookmarkIDsQuery,
  useToggleBookmarkMutation,
  useGetBookmarkCallsQuery,
} = api;
