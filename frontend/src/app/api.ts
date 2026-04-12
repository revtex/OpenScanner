import {
  createApi,
  fetchBaseQuery,
  type BaseQueryFn,
  type FetchArgs,
  type FetchBaseQueryError,
} from "@reduxjs/toolkit/query/react";
import type { SetupStatus } from "@/types";
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
    "Accesses",
    "Config",
    "Logs",
    "Bookmarks",
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
  }),
});

export const {
  useGetSetupStatusQuery,
  usePostSetupMutation,
  useGetBookmarkIDsQuery,
  useToggleBookmarkMutation,
} = api;
