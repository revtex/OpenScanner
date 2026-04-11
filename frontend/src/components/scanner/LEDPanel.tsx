import { Sun, Moon, User, LogOut, Settings } from "lucide-react";
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
    </div>
  );
}
