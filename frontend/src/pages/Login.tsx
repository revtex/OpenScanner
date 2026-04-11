import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { Lock } from "lucide-react";
import { usePostLoginMutation } from "@/app/api";
import { useAppDispatch } from "@/app/store";
import { setCredentials } from "@/app/slices/authSlice";

export default function Login() {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const [postLogin, { isLoading }] = usePostLoginMutation();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  // Change password form
  const [needChange, setNeedChange] = useState(false);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      const result = await postLogin({ username, password }).unwrap();
      if (result.passwordNeedChange) {
        setNeedChange(true);
        dispatch(setCredentials(result));
        return;
      }
      dispatch(setCredentials(result));
      navigate("/", { replace: true });
    } catch {
      setError("Invalid username or password");
    }
  };

  const handleChangePassword = (e: FormEvent) => {
    e.preventDefault();
    if (newPassword.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    if (newPassword !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }
    // Password change API not yet available — block navigation
    setError(
      "Password change is not yet available. Please contact your administrator.",
    );
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
                className="input input-bordered w-full"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                minLength={8}
                required
              />
              <input
                type="password"
                placeholder="Confirm password"
                className="input input-bordered w-full"
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
              className="input input-bordered w-full"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
            />
            <input
              type="password"
              placeholder="Password"
              className="input input-bordered w-full"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
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
