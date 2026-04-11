import React, { lazy, Suspense } from "react";
import ReactDOM from "react-dom/client";
import { Provider } from "react-redux";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { store } from "@/app/store";
import "@/index.css";

const Scanner = lazy(() => import("@/pages/Scanner"));
const Login = lazy(() => import("@/pages/Login"));
const Setup = lazy(() => import("@/pages/Setup"));
const Admin = lazy(() => import("@/pages/Admin"));

function App() {
  return (
    <div className="min-h-screen bg-base-100 text-base-content">
      <Suspense
        fallback={
          <div className="flex items-center justify-center min-h-screen">
            <span className="loading loading-spinner loading-lg" />
          </div>
        }
      >
        <Routes>
          <Route path="/" element={<Scanner />} />
          <Route path="/scanner" element={<Scanner />} />
          <Route path="/login" element={<Login />} />
          <Route path="/setup" element={<Setup />} />
          <Route path="/admin/*" element={<Admin />} />
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
