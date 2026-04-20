import { useState, useEffect, useCallback, useRef } from "react";
import { adminWsClient } from "@/services/adminWsClient";
import type { AdminLog } from "@/types";

interface LogQueryParams {
  from?: number;
  to?: number;
  level?: string;
  q?: string;
  limit?: number;
}

const LIVE_POLL_MS = 5_000;
const DEBOUNCE_MS = 2_000;

export function useAdminLogs(params: LogQueryParams, autoRefresh: boolean) {
  const [logs, setLogs] = useState<AdminLog[] | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isFetching, setIsFetching] = useState(false);
  const paramsRef = useRef(params);
  paramsRef.current = params;
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchLogs = useCallback(async () => {
    if (!adminWsClient.isConnected()) return;
    setIsFetching(true);
    try {
      const result = await adminWsClient.request<AdminLog[]>(
        "logs.query",
        paramsRef.current as Record<string, unknown>,
      );
      setLogs(result);
    } catch {
      // Silent fail
    } finally {
      setIsLoading(false);
      setIsFetching(false);
    }
  }, []);

  // Initial fetch and when params change
  useEffect(() => {
    fetchLogs();
  }, [fetchLogs, params.from, params.to, params.level, params.q, params.limit]);

  // Re-fetch when WS (re)connects — first load may race with socket open
  useEffect(() => {
    return adminWsClient.on("__connected__", () => {
      fetchLogs();
    });
  }, [fetchLogs]);

  // Live mode: poll on interval + refresh on new call activity (debounced)
  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(fetchLogs, LIVE_POLL_MS);

    const unsubActivity = adminWsClient.on("activity.updated", () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(fetchLogs, DEBOUNCE_MS);
    });

    return () => {
      clearInterval(interval);
      unsubActivity();
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [autoRefresh, fetchLogs]);

  return { logs, isLoading, isFetching, refetch: fetchLogs };
}

export function useAdminLogLevel() {
  const [level, setLevel] = useState<string>("info");

  const fetchLevel = useCallback(async () => {
    if (!adminWsClient.isConnected()) return;
    try {
      const result = await adminWsClient.request<{ level: string }>(
        "logs.level",
      );
      setLevel(result.level);
    } catch {
      // Silent fail
    }
  }, []);

  useEffect(() => {
    fetchLevel();
    // Re-fetch on WS connect and config changes
    const unsubConnect = adminWsClient.on("__connected__", () => {
      fetchLevel();
    });
    const unsubConfig = adminWsClient.on("config.updated", () => {
      fetchLevel();
    });
    return () => {
      unsubConnect();
      unsubConfig();
    };
  }, [fetchLevel]);

  return { level, refetch: fetchLevel };
}
