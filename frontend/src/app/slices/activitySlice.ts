import { api } from "@/app/api";

export interface ActivityStats {
  callsToday: number;
  callsThisWeek: number;
  callsTotal: number;
  activeListeners: number;
  uptime: number;
}

export interface ChartBucket {
  hour: number;
  count: number;
}

export interface ActivityChartResponse {
  buckets: ChartBucket[];
}

export interface TopTalkgroup {
  talkgroupId: number;
  talkgroupLabel: string;
  talkgroupName: string;
  systemLabel: string;
  callCount: number;
}

export interface TopTalkgroupsResponse {
  talkgroups: TopTalkgroup[];
}

const activityApi = api.injectEndpoints({
  endpoints: (builder) => ({
    getActivityStats: builder.query<ActivityStats, void>({
      query: () => "/admin/activity/stats",
    }),
    getActivityChart: builder.query<ActivityChartResponse, void>({
      query: () => "/admin/activity/chart",
    }),
    getTopTalkgroups: builder.query<TopTalkgroupsResponse, void>({
      query: () => "/admin/activity/top-talkgroups",
    }),
  }),
});

export const {
  useGetActivityStatsQuery,
  useGetActivityChartQuery,
  useGetTopTalkgroupsQuery,
} = activityApi;
