// Activity types used by the WebSocket-based useAdminActivity hook.

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
