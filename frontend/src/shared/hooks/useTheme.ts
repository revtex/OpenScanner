import { useState, useEffect, useCallback } from "react";
import { readStored, writeStored } from "@/shared/utils/storage";

const DARK_THEME = "squelch-dark";
const LIGHT_THEME = "squelch-light";
const STORAGE_KEY = "squelch-theme";

export function useTheme() {
  const [isDark, setIsDark] = useState(() => {
    const saved = readStored(localStorage, STORAGE_KEY);
    if (saved === LIGHT_THEME) return false;
    // Default to dark
    return true;
  });

  useEffect(() => {
    const theme = isDark ? DARK_THEME : LIGHT_THEME;
    document.documentElement.setAttribute("data-theme", theme);
    writeStored(localStorage, STORAGE_KEY, theme);
  }, [isDark]);

  const toggle = useCallback(() => {
    setIsDark((prev) => !prev);
  }, []);

  return {
    theme: isDark ? DARK_THEME : LIGHT_THEME,
    toggle,
    isDark,
  };
}
