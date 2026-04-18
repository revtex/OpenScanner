import { useState, type FormEvent } from "react";
import { useLocation, useNavigate, Navigate } from "react-router-dom";
import { Lock } from "lucide-react";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { useGetSetupStatusQuery } from "@/app/api";
import {
  setCredentials,
  selectToken,
  usePostLoginMutation,
  useChangePasswordMutation,
} from "@/app/slices/authSlice";

interface LoginLocationState {
  from?: string;
}

function isSafePostLoginPath(path: string): boolean {
  // Allow only simple in-app absolute paths (no scheme, host, query, or traversal-like chars).
  return /^\/(?!\/)[A-Za-z0-9/_-]*$/.test(path);
}

export default function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const dispatch = useAppDispatch();
  const token = useAppSelector(selectToken);
  const { data: setupStatus, isLoading: setupLoading } =
    useGetSetupStatusQuery();
  const [postLogin, { isLoading }] = usePostLoginMutation();
  const [changePassword] = useChangePasswordMutation();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [rememberMe, setRememberMe] = useState(true);
  const [error, setError] = useState("");

  // Change password form
  const [needChange, setNeedChange] = useState(false);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const from = (location.state as LoginLocationState | null)?.from;

  if (setupLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <span className="loading loading-spinner loading-lg" />
      </div>
    );
  }

  // While first-login setup is active, login should be unreachable.
  if (setupStatus?.needsSetup) {
    return <Navigate to="/setup" replace />;
  }

  // Already authenticated — redirect away without touching credentials.
  if (token && !needChange) {
    return <Navigate to="/" replace />;
  }

  const getPostLoginPath = (role: string) => {
    if (typeof from !== "string" || !isSafePostLoginPath(from)) {
      return "/";
    }
    if (from.startsWith("/admin") && role !== "admin") {
      return "/";
    }
    return from;
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      const result = await postLogin({
        username,
        password,
        rememberMe,
      }).unwrap();
      const creds = {
        token: result.token,
        role: result.user.role,
        username: result.user.username,
        passwordNeedChange: result.passwordNeedChange,
      };
      if (creds.passwordNeedChange) {
        setNeedChange(true);
        dispatch(setCredentials(creds));
        return;
      }
      dispatch(setCredentials(creds));
      navigate(getPostLoginPath(creds.role), { replace: true });
    } catch {
      setError("Invalid username or password");
    }
  };

  const handleChangePassword = async (e: FormEvent) => {
    e.preventDefault();
    if (newPassword.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    if (newPassword !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }
    try {
      await changePassword({ currentPassword: password, newPassword }).unwrap();
      navigate(getPostLoginPath("admin"), { replace: true });
    } catch {
      setError("Failed to change password");
    }
  };

  if (needChange) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="card max-w-sm w-full bg-base-200 shadow-xl">
          <div className="card-body items-center text-center">
            <Lock className="w-10 h-10 text-primary mb-2" />
            <h2 className="card-title">Change Password</h2>
            <p className="text-sm opacity-60">Please set a new password</p>
            <form
              onSubmit={handleChangePassword}
              className="w-full space-y-3 mt-2"
            >
              <input
                type="password"
                placeholder="New password"
                className="input w-full"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                minLength={8}
                required
              />
              <input
                type="password"
                placeholder="Confirm password"
                className="input w-full"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                minLength={8}
                required
              />
              {error && <p className="text-error text-sm">{error}</p>}
              <button type="submit" className="btn btn-primary btn-block">
                Update Password
              </button>
            </form>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="card max-w-sm w-full bg-base-200 shadow-xl">
        <div className="card-body items-center text-center">
          <Lock className="w-10 h-10 text-primary mb-2" />
          <h2 className="card-title tracking-widest">OPENSCANNER</h2>
          <form onSubmit={handleSubmit} className="w-full space-y-3 mt-4">
            <input
              type="text"
              placeholder="Username"
              className="input w-full"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
            />
            <input
              type="password"
              placeholder="Password"
              className="input w-full"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
            <label className="flex items-center gap-2 cursor-pointer self-start">
              <input
                type="checkbox"
                className="checkbox checkbox-sm checkbox-primary"
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
              />
              <span className="text-sm">Remember me</span>
            </label>
            {error && <p className="text-error text-sm">{error}</p>}
            <button
              type="submit"
              className="btn btn-primary btn-block"
              disabled={isLoading}
            >
              {isLoading ? (
                <span className="loading loading-spinner loading-sm" />
              ) : (
                "Sign In"
              )}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
