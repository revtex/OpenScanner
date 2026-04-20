import { useState, useEffect, useCallback, useRef } from "react";
import { adminWsClient } from "@/services/adminWsClient";
import type {
  ActivityStats,
  ActivityChartResponse,
  TopTalkgroupsResponse,
} from "@/app/slices/activitySlice";

const REFRESH_INTERVAL = 30_000;
const DEBOUNCE_MS = 3_000; // debounce rapid call bursts

export function useAdminActivity() {
  const [stats, setStats] = useState<ActivityStats | null>(null);
  const [chart, setChart] = useState<ActivityChartResponse | null>(null);
  const [topTG, setTopTG] = useState<TopTalkgroupsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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

  // Debounced fetch — coalesces rapid activity.updated bursts
  const debouncedFetch = useCallback(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      fetchAll();
    }, DEBOUNCE_MS);
  }, [fetchAll]);

  useEffect(() => {
    fetchAll();
    const interval = setInterval(fetchAll, REFRESH_INTERVAL);

    // Fetch immediately when WS (re)connects
    const unsubConnect = adminWsClient.on("__connected__", () => {
      fetchAll();
    });

    // Refresh when new calls arrive (debounced to avoid request storms)
    const unsubActivity = adminWsClient.on("activity.updated", () => {
      debouncedFetch();
    });

    return () => {
      clearInterval(interval);
      unsubConnect();
      unsubActivity();
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [fetchAll, debouncedFetch]);

  return { stats, chart, topTG, isLoading, refetch: fetchAll };
}
