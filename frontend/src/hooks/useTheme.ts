import { useState, useEffect, useCallback } from 'react';

const DARK_THEME = 'openscanner-dark';
const LIGHT_THEME = 'openscanner-light';
const STORAGE_KEY = 'openscanner-theme';

export function useTheme() {
  const [isDark, setIsDark] = useState(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === LIGHT_THEME) return false;
    // Default to dark
    return true;
  });

  useEffect(() => {
    const theme = isDark ? DARK_THEME : LIGHT_THEME;
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem(STORAGE_KEY, theme);
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