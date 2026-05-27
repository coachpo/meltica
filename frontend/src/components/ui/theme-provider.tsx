'use client';

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { ThemeProvider as NextThemesProvider, useTheme as useNextTheme } from 'next-themes';

import { DEFAULT_THEME_PALETTE, type ThemePalette, isThemePalette } from '@/components/ui/theme-config';

export type ThemePreference = 'light' | 'dark' | 'system';
type ResolvedTheme = 'light' | 'dark';

interface ThemeContextValue {
  theme: ThemePreference;
  resolvedTheme: ResolvedTheme;
  palette: ThemePalette;
  toggleTheme: () => void;
  setTheme: (theme: ThemePreference) => void;
  setPalette: (palette: ThemePalette) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);
const STORAGE_KEY = 'meltica-theme';
const PALETTE_STORAGE_KEY = 'meltica-theme-palette';
const MEDIA_QUERY = '(prefers-color-scheme: dark)';

function getStoredPalette(): ThemePalette {
  if (typeof window === 'undefined') {
    return DEFAULT_THEME_PALETTE;
  }
  const stored = window.localStorage.getItem(PALETTE_STORAGE_KEY);
  if (stored && isThemePalette(stored)) {
    return stored;
  }
  return DEFAULT_THEME_PALETTE;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
      storageKey={STORAGE_KEY}
    >
      <PaletteProvider>{children}</PaletteProvider>
    </NextThemesProvider>
  );
}

function PaletteProvider({ children }: { children: ReactNode }) {
  const { theme: nextTheme, setTheme: setNextTheme, resolvedTheme } = useNextTheme();
  const [palette, setPaletteState] = useState<ThemePalette>(getStoredPalette);

  const normalizedTheme = (nextTheme ?? 'system') as ThemePreference;
  const fallbackResolved: ResolvedTheme = typeof window === 'undefined'
    ? 'light'
    : window.matchMedia?.(MEDIA_QUERY).matches
      ? 'dark'
      : 'light';
  const normalizedResolved = (resolvedTheme ?? fallbackResolved) as ResolvedTheme;

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.theme = normalizedResolved;
    root.dataset.themePreference = normalizedTheme;
    root.classList.toggle('dark', normalizedResolved === 'dark');
  }, [normalizedResolved, normalizedTheme]);

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.themePalette = palette;
  }, [palette]);

  const setPalette = useCallback((next: ThemePalette) => {
    setPaletteState(next);
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(PALETTE_STORAGE_KEY, next);
    }
  }, []);

  const toggleTheme = useCallback(() => {
    const current = normalizedTheme === 'system' ? normalizedResolved : normalizedTheme;
    setNextTheme(current === 'dark' ? 'light' : 'dark');
  }, [normalizedTheme, normalizedResolved, setNextTheme]);

  const setTheme = useCallback(
    (value: ThemePreference) => {
      setNextTheme(value);
    },
    [setNextTheme],
  );

  const value = useMemo(
    () => ({
      theme: normalizedTheme,
      resolvedTheme: normalizedResolved,
      palette,
      toggleTheme,
      setTheme,
      setPalette,
    }),
    [normalizedTheme, normalizedResolved, palette, toggleTheme, setTheme, setPalette],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }
  return context;
}
