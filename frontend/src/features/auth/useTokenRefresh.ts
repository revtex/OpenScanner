import { useEffect } from "react";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  selectToken,
  setCredentials,
  clearCredentials,
  usePostRefreshMutation,
} from "./authSlice";

/**
 * Schedules a silent access token refresh 1 minute before the current
 * JWT expires. Re-arms whenever the token changes (after a successful
 * refresh or login).
 */
export function useTokenRefresh(): void {
  const token = useAppSelector(selectToken);
  const dispatch = useAppDispatch();
  const [postRefresh] = usePostRefreshMutation();

  useEffect(() => {
    if (!token) return;

    // Decode JWT payload (base64url) to read `exp` claim.
    let expiresAtMs: number;
    try {
      const payloadB64 = token.split(".")[1];
      const payload = JSON.parse(atob(payloadB64)) as { exp: number };
      expiresAtMs = payload.exp * 1000;
    } catch {
      return;
    }

    // Refresh 1 minute before expiry, but at least immediately if already close.
    const refreshAt = expiresAtMs - 60_000;
    const delay = Math.max(refreshAt - Date.now(), 0);

    const timer = setTimeout(() => {
      postRefresh()
        .unwrap()
        .then(({ token: newToken, user }) => {
          dispatch(
            setCredentials({
              token: newToken,
              role: user.role,
              username: user.username,
              passwordNeedChange: false,
            }),
          );
        })
        .catch(() => {
          dispatch(clearCredentials());
        });
    }, delay);

    return () => clearTimeout(timer);
  }, [token, dispatch, postRefresh]);
}
