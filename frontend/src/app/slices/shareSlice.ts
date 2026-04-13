import { api } from "@/app/api";

export interface SharedCall {
  id: number;
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

const shareApi = api.injectEndpoints({
  endpoints: (builder) => ({
    getSharedCall: builder.query<SharedCall, number>({
      query: (id) => `/calls/${id}/share`,
    }),
  }),
});

export const { useGetSharedCallQuery } = shareApi;
