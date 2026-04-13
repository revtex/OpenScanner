import { api } from "@/app/api";

export interface SharedCall {
  token: string;
  dateTime: number;
  systemLabel: string;
  talkgroupLabel: string;
  talkgroupName: string;
  frequency: number;
  duration: number;
  source: number;
  transcript?: string;
  audioUrl: string;
}

export interface ShareCreateResponse {
  token: string;
  url: string;
}

export interface SharedLinkAdmin {
  id: number;
  callId: number;
  token: string;
  createdAt: number;
  sharedBy: string;
  dateTime: number;
  duration: number;
  systemLabel: string;
  talkgroupLabel: string;
  talkgroupName: string;
}

const shareApi = api.injectEndpoints({
  endpoints: (builder) => ({
    getSharedCall: builder.query<SharedCall, string>({
      query: (token) => `/shared/${token}`,
    }),
    shareCall: builder.mutation<ShareCreateResponse, number>({
      query: (callId) => ({
        url: `/calls/${callId}/share`,
        method: "POST",
      }),
    }),
    unshareCall: builder.mutation<{ shared: boolean }, number>({
      query: (callId) => ({
        url: `/calls/${callId}/share`,
        method: "DELETE",
      }),
    }),
    getSharedLinks: builder.query<SharedLinkAdmin[], void>({
      query: () => "/admin/shared-links",
      providesTags: ["SharedLinks"],
    }),
    deleteSharedLink: builder.mutation<{ deleted: boolean }, number>({
      query: (id) => ({
        url: `/admin/shared-links/${id}`,
        method: "DELETE",
      }),
      invalidatesTags: ["SharedLinks"],
    }),
  }),
});

export const {
  useGetSharedCallQuery,
  useShareCallMutation,
  useUnshareCallMutation,
  useGetSharedLinksQuery,
  useDeleteSharedLinkMutation,
} = shareApi;
