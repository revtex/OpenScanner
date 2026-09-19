import { useState, useEffect, useCallback } from "react";
import {
  readStored,
  writeStored,
  migrateThemeValue,
} from "@/shared/utils/storage";

const DARK_THEME = "squelch-dark";
const LIGHT_THEME = "squelch-light";
const STORAGE_KEY = "squelch-theme";

export function useTheme() {
  const [isDark, setIsDark] = useState(() => {
    // The stored value is a theme name, which carried the old product
    // name, so it is mapped forward as well as the key it lives under.
    const saved = migrateThemeValue(readStored(localStorage, STORAGE_KEY));
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
