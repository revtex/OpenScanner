import { useEffect, useRef, useCallback } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { wsClient } from "@/services/ws/client";
import { setCredentials, usePostRefreshMutation } from "@/app/slices/shared/authSlice";
import type { ConnectionStatus } from "@/types";

export function useWebSocket(): { connectionStatus: ConnectionStatus } {
  const dispatch = useAppDispatch();
  const token = useAppSelector((s) => s.auth.token);
  const setupStatus = useAppSelector((s) => s.auth.setupStatus);
  const connectionStatus = useAppSelector((s) => s.scanner.connectionStatus);
  const connectedRef = useRef(false);
  const [postRefresh] = usePostRefreshMutation();

  // Provide the WS client with a refresh callback so it can attempt
  // a silent token refresh on XPR instead of immediately logging out.
  const handleTokenExpired = useCallback(async (): Promise<string | null> => {
    try {
      const { token: newToken, user } = await postRefresh().unwrap();
      dispatch(
        setCredentials({
          token: newToken,
          role: user.role,
          username: user.username,
          passwordNeedChange: false,
        }),
      );
      return newToken;
    } catch {
      return null;
    }
  }, [dispatch, postRefresh]);

  useEffect(() => {
    wsClient.onTokenExpired(handleTokenExpired);
  }, [handleTokenExpired]);

  useEffect(() => {
    // Wait for setup status to load before deciding whether to connect.
    if (!setupStatus) return;

    // Don't connect if setup is needed
    if (setupStatus.needsSetup) return;

    const auth: { token?: string; publicAccess?: boolean } = {};
    if (token) {
      auth.token = token;
    } else if (setupStatus.publicAccess) {
      auth.publicAccess = true;
    } else {
      // Not authenticated and not public — don't connect
      return;
    }

    const connectTimer = window.setTimeout(() => {
      wsClient.connect(dispatch, auth);
      connectedRef.current = true;
    }, 0);

    return () => {
      window.clearTimeout(connectTimer);
      if (connectedRef.current) {
        wsClient.disconnect();
        connectedRef.current = false;
      }
    };
  }, [dispatch, token, setupStatus]);

  return { connectionStatus };
}
