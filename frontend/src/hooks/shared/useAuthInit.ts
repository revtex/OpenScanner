import { useEffect } from "react";
import { useAppDispatch } from "@/app/store";
import {
  setCredentials,
  setAuthReady,
  usePostRefreshMutation,
} from "@/app/slices/authSlice";

/**
 * Attempts a silent token refresh on app mount.
 * If a valid refresh cookie exists, the user is logged in automatically
 * without seeing the login screen. Sets `authReady` when complete.
 */
export function useAuthInit(): boolean {
  const dispatch = useAppDispatch();
  const [postRefresh] = usePostRefreshMutation();

  useEffect(() => {
    let cancelled = false;

    postRefresh()
      .unwrap()
      .then(({ token, user }) => {
        if (!cancelled) {
          dispatch(
            setCredentials({
              token,
              role: user.role,
              username: user.username,
              passwordNeedChange: false,
            }),
          );
        }
      })
      .catch(() => {
        // No valid refresh cookie — user must log in manually.
      })
      .finally(() => {
        if (!cancelled) {
          dispatch(setAuthReady());
        }
      });

    return () => {
      cancelled = true;
    };
  }, [dispatch, postRefresh]);

  return true;
}
