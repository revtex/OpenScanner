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
 * Single-flighted silent token refresh shared by every code path that may
 * trigger POST /auth/refresh (RTK Query 401 retry, scheduled refresh,
 * useAuthInit on mount, WebSocket reauth, audio-element auth recovery).
 *
 * Coalescing avoids redundant network round-trips and keeps every caller
 * in this tab on the same access JWT. The backend additionally tolerates
 * a brief replay of an already-rotated refresh token (see
 * `RefreshReplayGrace` in backend/internal/handler/auth/replay_cache.go),
 * so cross-tab races, service-worker retries, and reload-mid-rotation
 * scenarios converge on identical tokens instead of revoking the family.
 *
 * Implemented as a plain `fetch()` (not via RTK Query's rawBaseQuery) so
 * non-RTK callers (audio recovery in main.tsx) can hit the exact same
 * promise without going through RTK plumbing.
 */
export type RefreshSessionResult =
  | { data: RefreshResponse }
  | { error: FetchBaseQueryError };

type DispatchFn = (action: { type: string; payload?: unknown }) => void;

let refreshInFlight: Promise<RefreshSessionResult> | null = null;

export function refreshSession(
  dispatch: DispatchFn,
): Promise<RefreshSessionResult> {
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      try {
        const res = await fetch("/api/v1/auth/refresh", {
          method: "POST",
          credentials: "include",
        });
        if (!res.ok) {
          return {
            error: {
              status: res.status,
              data: undefined,
            } as FetchBaseQueryError,
          };
        }
        const data = (await res.json()) as RefreshResponse;
        dispatch({
          type: "auth/setCredentials",
          payload: {
            token: data.token,
            role: data.user.role,
            username: data.user.username,
            passwordNeedChange: false,
          },
        });
        return { data };
      } catch (e) {
        return {
          error: {
            status: "FETCH_ERROR",
            error: e instanceof Error ? e.message : String(e),
          } as FetchBaseQueryError,
        };
      } finally {
        refreshInFlight = null;
      }
    })();
  }
  return refreshInFlight;
}

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
  const url = typeof args === "string" ? args : args.url;

  // Coalesce direct calls to /auth/refresh (useAuthInit, useTokenRefresh,
  // WS reauth) onto the same in-flight promise as 401-triggered refreshes
  // and the audio-recovery refresh in main.tsx.
  if (url === "/auth/refresh") {
    const result = await refreshSession(storeApi.dispatch);
    if ("error" in result) {
      storeApi.dispatch({ type: "auth/clearCredentials" });
      return { error: result.error };
    }
    return { data: result.data };
  }

  let result = await rawBaseQuery(args, storeApi, extraOptions);

  if (result.error && result.error.status === 401) {
    const refreshResult = await refreshSession(storeApi.dispatch);
    if ("data" in refreshResult) {
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
