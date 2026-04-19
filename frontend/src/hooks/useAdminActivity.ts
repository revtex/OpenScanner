import { useState, useEffect, useCallback } from "react";
import { adminWsClient } from "@/services/adminWsClient";
import type {
  ActivityStats,
  ActivityChartResponse,
  TopTalkgroupsResponse,
} from "@/app/slices/activitySlice";

const REFRESH_INTERVAL = 30_000;

export function useAdminActivity() {
  const [stats, setStats] = useState<ActivityStats | null>(null);
  const [chart, setChart] = useState<ActivityChartResponse | null>(null);
  const [topTG, setTopTG] = useState<TopTalkgroupsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const fetchAll = useCallback(async () => {
    if (!adminWsClient.isConnected()) return;
    try {
      const [s, c, t] = await Promise.all([
        adminWsClient.request<ActivityStats>("activity.stats"),
        adminWsClient.request<ActivityChartResponse>("activity.chart"),
        adminWsClient.request<TopTalkgroupsResponse>("activity.top-talkgroups"),
      ]);
      setStats(s);
      setChart(c);
      setTopTG(t);
    } catch {
      // Silently fail - will retry on next interval
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
    const interval = setInterval(fetchAll, REFRESH_INTERVAL);
    return () => clearInterval(interval);
  }, [fetchAll]);

  // Also listen for activity.delta events from server
  useEffect(() => {
    const unsub = adminWsClient.on("activity.delta", (_topic, data) => {
      const delta = data as Partial<ActivityStats>;
      setStats((prev) => (prev ? { ...prev, ...delta } : null));
    });
    return unsub;
  }, []);

  return { stats, chart, topTG, isLoading, refetch: fetchAll };
}
