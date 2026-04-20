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

    // Fetch immediately when WS (re)connects — on first load the
    // socket may not be open yet when fetchAll() runs above.
    const unsubConnect = adminWsClient.on("__connected__", () => {
      fetchAll();
    });

    return () => {
      clearInterval(interval);
      unsubConnect();
    };
  }, [fetchAll]);

  return { stats, chart, topTG, isLoading, refetch: fetchAll };
}
