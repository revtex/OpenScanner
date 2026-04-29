import React, { lazy, Suspense } from "react";
import ReactDOM from "react-dom/client";
import { Provider } from "react-redux";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { store } from "@/app/store";
import { useAppSelector } from "@/app/store";
import { selectAuthReady, setCredentials } from "@/app/slices/shared/authSlice";
import { useAuthInit } from "@/hooks/shared/useAuthInit";
import { useTokenRefresh } from "@/hooks/shared/useTokenRefresh";
import { audioPlayer } from "@/services/audio/player";
import type { RefreshResponse } from "@/types";
import "@/index.css";

// Wire a silent auth-recovery hook into the audio player so that when an
// `<audio>` fetch returns 401 (e.g. another device pushed our access JWT
// out of the per-user concurrent-token cap) we transparently refresh the
// session cookie and retry the same call once. Bypasses RTK Query's
// retry-on-401 plumbing because media-element fetches don't go through it.
audioPlayer.setAuthRecovery(async () => {
  try {
    const res = await fetch("/api/v1/auth/refresh", {
      method: "POST",
      credentials: "include",
    });
    if (!res.ok) return false;
    const data = (await res.json()) as RefreshResponse;
    store.dispatch(
      setCredentials({
        token: data.token,
        role: data.user.role,
        username: data.user.username,
        passwordNeedChange: false,
      }),
    );
    return true;
  } catch {
    return false;
  }
});

const Scanner = lazy(() => import("@/pages/Scanner"));
const Login = lazy(() => import("@/pages/Login"));
const Setup = lazy(() => import("@/pages/Setup"));
const Admin = lazy(() => import("@/pages/Admin"));
const SharedCall = lazy(() => import("@/pages/SharedCall"));

const LoadingSpinner = (
  <div className="flex items-center justify-center min-h-screen">
    <span className="loading loading-spinner loading-lg" />
  </div>
);

function App() {
  useAuthInit();
  useTokenRefresh();
  const authReady = useAppSelector(selectAuthReady);

  if (!authReady) {
    return (
      <div className="min-h-screen bg-base-100 text-base-content">
        {LoadingSpinner}
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-base-100 text-base-content">
      <Suspense fallback={LoadingSpinner}>
        <Routes>
          <Route path="/" element={<Scanner />} />
          <Route path="/scanner" element={<Scanner />} />
          <Route path="/login" element={<Login />} />
          <Route path="/setup" element={<Setup />} />
          <Route path="/admin/*" element={<Admin />} />
          <Route path="/call/:token" element={<SharedCall />} />
        </Routes>
      </Suspense>
    </div>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Provider store={store}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </Provider>
  </React.StrictMode>,
);
