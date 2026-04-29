import {
  createApi,
  fetchBaseQuery,
  type BaseQueryFn,
  type FetchArgs,
  type FetchBaseQueryError,
} from "@reduxjs/toolkit/query/react";
import type {
  SetupStatus,
  RefreshResponse,
  LegacyUsageResponse,
} from "@/types";

const rawBaseQuery = fetchBaseQuery({
  baseUrl: "/api/v1",
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
 *
 * Refresh is single-flighted: if multiple requests 401 simultaneously (typical
 * on tab wake / network resume), or multiple call sites trigger
 * POST /auth/refresh in parallel (RTK Query 401 handler + scheduled refresh
 * + WS reconnect + audio recovery), only one network refresh actually goes
 * out and the rest await the same promise. Without this, parallel refresh
 * attempts present the same single-use refresh token; the server detects
 * "replay" on the loser and revokes the entire token family — forcing
 * re-login even though the refresh cookie is nowhere near its TTL.
 */
type RefreshQueryResult = Awaited<ReturnType<typeof rawBaseQuery>>;
let refreshInFlight: Promise<RefreshQueryResult> | null = null;

function runRefresh(
  storeApi: Parameters<typeof rawBaseQuery>[1],
  extraOptions: Parameters<typeof rawBaseQuery>[2],
): Promise<RefreshQueryResult> {
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      try {
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
        }
        return refreshResult;
      } finally {
        refreshInFlight = null;
      }
    })();
  }
  return refreshInFlight;
}

const baseQueryWithRefresh: BaseQueryFn<
  string | FetchArgs,
  unknown,
  FetchBaseQueryError
> = async (args, storeApi, extraOptions) => {
  const url = typeof args === "string" ? args : args.url;

  // Coalesce direct calls to /auth/refresh (useAuthInit, useTokenRefresh,
  // WS reauth) onto the same in-flight promise as 401-triggered refreshes.
  if (url === "/auth/refresh") {
    const result = await runRefresh(storeApi, extraOptions);
    if (result.error && result.error.status === 401) {
      storeApi.dispatch({ type: "auth/clearCredentials" });
    }
    return result;
  }

  let result = await rawBaseQuery(args, storeApi, extraOptions);

  if (result.error && result.error.status === 401) {
    const refreshResult = await runRefresh(storeApi, extraOptions);
    if (refreshResult.data) {
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
    "LegacyUsage",
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
    getLegacyUsage: builder.query<LegacyUsageResponse, void>({
      query: () => "/admin/legacy-usage",
      providesTags: ["LegacyUsage"],
    }),
  }),
});

export const {
  useGetSetupStatusQuery,
  usePostSetupMutation,
  useGetBookmarkIDsQuery,
  useToggleBookmarkMutation,
  useGetBookmarkCallsQuery,
  useGetLegacyUsageQuery,
} = api;
