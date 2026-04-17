import { useEffect, useRef } from "react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { wsClient } from "@/services/wsClient";
import type { ConnectionStatus } from "@/types";

export function useWebSocket(): { connectionStatus: ConnectionStatus } {
  const dispatch = useAppDispatch();
  const token = useAppSelector((s) => s.auth.token);
  const setupStatus = useAppSelector((s) => s.auth.setupStatus);
  const connectionStatus = useAppSelector((s) => s.scanner.connectionStatus);
  const connectedRef = useRef(false);

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
