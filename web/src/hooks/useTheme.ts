import { useCallback, useEffect, useState } from 'react';

export type Theme = 'light' | 'dark' | 'auto';

const STORAGE_KEY = 'hermind-theme';

export function getSavedTheme(): Theme {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (raw === 'light' || raw === 'dark' || raw === 'auto') return raw;
  return 'auto';
}

export function applyTheme(theme: Theme) {
  const html = document.documentElement;
  if (theme === 'auto') {
    html.removeAttribute('data-theme');
  } else {
    html.setAttribute('data-theme', theme);
  }
}

/**
 * Reads/writes the user theme preference from localStorage and syncs it
 * to the <html> data-theme attribute so that CSS custom properties switch.
 *
 * Usage: call inside any React component. Safe to call from multiple
 * components — all instances share the same localStorage key.
 */
export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(getSavedTheme);

  useEffect(() => {
    applyTheme(theme);
    localStorage.setItem(STORAGE_KEY, theme);
  }, [theme]);

  // Keep other tabs / instances in sync.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) {
        const next = (e.newValue as Theme | null) ?? 'auto';
        setThemeState(next);
        applyTheme(next);
      }
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t);
  }, []);

  return { theme, setTheme };
}
