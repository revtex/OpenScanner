import {
  Sun,
  Moon,
  User,
  LogOut,
  Settings,
  Info,
  KeyRound,
} from "lucide-react";
import { useState, useRef, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useTheme } from "@/hooks/useTheme";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  selectToken,
  selectRole,
  selectUsername,
  clearCredentials,
} from "@/app/slices/authSlice";
import { useChangePasswordMutation } from "@/app/slices/authSlice";

export function LEDPanel() {
  const { isDark, toggle } = useTheme();
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const token = useAppSelector(selectToken);
  const role = useAppSelector(selectRole);
  const username = useAppSelector(selectUsername);
  const config = useAppSelector((s) => s.scanner.config);
  const isLive = useAppSelector((s) => s.scanner.isLive);
  const isPaused = useAppSelector((s) => s.scanner.isPaused);
  const currentCall = useAppSelector((s) => s.scanner.currentCall);
  const isPlaying = !!currentCall;
  const [menuOpen, setMenuOpen] = useState(false);
  const [aboutOpen, setAboutOpen] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [pwToast, setPwToast] = useState<{
    msg: string;
    type: "success" | "error";
  } | null>(null);
  const [changePassword] = useChangePasswordMutation();
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu on outside click
  useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [menuOpen]);

  const branding = config?.branding?.trim() || "OPENSCANNER";

  // LED color logic:
  // Paused:            orange + blink (always)
  // Live + receiving:  green (or TG custom color) with glow
  // Live + idle:       green, dimmer (no glow)
  // Playback/archive:  orange (or TG custom color)
  // No link:           off gray
  let ledColor: string;
  let dimmed: boolean;
  const shouldBlink = isPaused;

  if (isPaused) {
    ledColor = "#ff9100"; // orange blink when paused
    dimmed = false;
  } else if (currentCall?.talkgroupLedColor) {
    ledColor = currentCall.talkgroupLedColor;
    dimmed = false;
  } else if (isPlaying && isLive) {
    ledColor = "#00e676"; // green - live receiving
    dimmed = false;
  } else if (isPlaying && !isLive) {
    ledColor = "#ff9100"; // orange - archive playback
    dimmed = false;
  } else if (isLive && !isPlaying) {
    ledColor = "#00e676"; // green dimmed - live idle
    dimmed = true;
  } else {
    ledColor = "#505050"; // off - no link
    dimmed = true;
  }

  const handleSignOut = () => {
    dispatch(clearCredentials());
    navigate("/login", { replace: true });
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword.length < 8) {
      setPwToast({
        msg: "New password must be at least 8 characters",
        type: "error",
      });
      return;
    }
    if (newPassword !== confirmPassword) {
      setPwToast({ msg: "Passwords do not match", type: "error" });
      return;
    }
    try {
      await changePassword({ currentPassword, newPassword }).unwrap();
      setPwToast({ msg: "Password changed successfully", type: "success" });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setTimeout(() => setPasswordOpen(false), 1500);
    } catch {
      setPwToast({ msg: "Failed to change password", type: "error" });
    }
    setTimeout(() => setPwToast(null), 4000);
  };

  return (
    <div className="flex items-center justify-between mb-6 overflow-visible">
      <span className="led-branding text-sm font-bold tracking-widest uppercase opacity-70">
        {branding}
      </span>
      <div className="flex items-center gap-3">
        {/* User menu */}
        {token ? (
          <div className="relative" ref={menuRef}>
            <button
              className="btn btn-ghost btn-xs gap-1"
              onClick={() => setMenuOpen((v) => !v)}
              aria-label="User menu"
              aria-expanded={menuOpen}
            >
              <User className="w-4 h-4" />
              <span className="text-xs opacity-70 max-w-[6rem] truncate">
                {username}
              </span>
            </button>
            {menuOpen && (
              <ul className="absolute right-0 top-full mt-1 menu p-2 shadow-lg bg-base-200 rounded-box w-48 z-[100] border border-base-300">
                {role === "admin" && (
                  <li>
                    <button
                      onClick={() => {
                        setMenuOpen(false);
                        navigate("/admin/users");
                      }}
                    >
                      <Settings className="w-4 h-4" /> Admin Panel
                    </button>
                  </li>
                )}
                <li>
                  <button
                    onClick={() => {
                      setMenuOpen(false);
                      setPasswordOpen(true);
                    }}
                  >
                    <KeyRound className="w-4 h-4" /> Change Password
                  </button>
                </li>
                <li>
                  <button
                    onClick={() => {
                      setMenuOpen(false);
                      handleSignOut();
                    }}
                  >
                    <LogOut className="w-4 h-4" /> Sign Out
                  </button>
                </li>
              </ul>
            )}
          </div>
        ) : (
          <button
            className="btn btn-ghost btn-xs"
            onClick={() => navigate("/login")}
            aria-label="Sign in"
          >
            <User className="w-4 h-4" />
            <span className="text-xs opacity-70">Sign in</span>
          </button>
        )}

        <button
          className="btn btn-ghost btn-xs btn-circle"
          onClick={toggle}
          aria-label="Toggle theme"
        >
          {isDark ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
        </button>
        <button
          className="btn btn-ghost btn-xs btn-circle"
          onClick={() => setAboutOpen(true)}
          aria-label="About"
        >
          <Info className="w-4 h-4" />
        </button>
        <div
          className={`led-indicator rounded-sm ${shouldBlink ? "animate-pulse" : ""}`}
          style={{
            backgroundColor: ledColor,
            boxShadow: dimmed
              ? "none"
              : `0 0 8px ${ledColor}, 0 0 16px ${ledColor}`,
            opacity: dimmed ? 0.5 : 1,
            animationTimingFunction: shouldBlink ? "step-end" : undefined,
          }}
        />
      </div>

      {/* About modal */}
      {aboutOpen && (
        <dialog
          className="modal modal-open"
          onClick={() => setAboutOpen(false)}
        >
          <div
            className="modal-box max-w-sm"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="font-bold text-lg mb-4">About</h3>
            <div className="space-y-2 text-sm">
              {branding !== "OPENSCANNER" && (
                <div>
                  <span className="opacity-60">Instance:</span>{" "}
                  <span className="font-semibold">{branding}</span>
                </div>
              )}
              <div>
                <span className="opacity-60">Version:</span>{" "}
                <span>{config?.version || "—"}</span>
              </div>
              {config?.email && (
                <div>
                  <span className="opacity-60">Support:</span>{" "}
                  <a
                    href={`mailto:${config.email}`}
                    className="link link-primary"
                  >
                    {config.email}
                  </a>
                </div>
              )}
            </div>
            <div className="modal-action">
              <button
                className="btn btn-sm"
                onClick={() => setAboutOpen(false)}
              >
                Close
              </button>
            </div>
          </div>
        </dialog>
      )}

      {/* Change Password modal */}
      {passwordOpen && (
        <dialog
          className="modal modal-open"
          onClick={() => setPasswordOpen(false)}
        >
          <div
            className="modal-box max-w-sm"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="font-bold text-lg mb-4">Change Password</h3>
            <form onSubmit={handleChangePassword} className="space-y-3">
              <label className="flex flex-col w-full">
                <span className="text-sm">Current Password</span>
                <input
                  type="password"
                  className="input w-full"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
              </label>
              <label className="flex flex-col w-full">
                <span className="text-sm">New Password</span>
                <input
                  type="password"
                  className="input w-full"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  minLength={8}
                  autoComplete="new-password"
                />
              </label>
              <label className="flex flex-col w-full">
                <span className="text-sm">Confirm New Password</span>
                <input
                  type="password"
                  className="input w-full"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  minLength={8}
                  autoComplete="new-password"
                />
              </label>
              {pwToast && (
                <div
                  className={`text-sm ${
                    pwToast.type === "success" ? "text-success" : "text-error"
                  }`}
                >
                  {pwToast.msg}
                </div>
              )}
              <div className="modal-action">
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => {
                    setPasswordOpen(false);
                    setPwToast(null);
                  }}
                >
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary btn-sm">
                  Change Password
                </button>
              </div>
            </form>
          </div>
        </dialog>
      )}
    </div>
  );
}
