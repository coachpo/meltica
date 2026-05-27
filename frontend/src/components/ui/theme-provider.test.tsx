import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { act } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DEFAULT_THEME_PALETTE, themePalettes } from './theme-config';
import { ThemeProvider, useTheme } from './theme-provider';

const mediaListeners = new Set<(event: MediaQueryListEvent) => void>();
const MEDIA_QUERY = '(prefers-color-scheme: dark)';
let prefersDark = false;

function ThemeHarness() {
  const { theme, resolvedTheme, setTheme, palette, setPalette } = useTheme();

  return (
    <div>
      <span data-testid="theme-preference">{theme}</span>
      <span data-testid="resolved-theme">{resolvedTheme}</span>
      <span data-testid="palette">{palette}</span>
      <button type="button" onClick={() => setTheme('dark')}>
        set-dark
      </button>
      <button type="button" onClick={() => setTheme('system')}>
        set-system
      </button>
      {themePalettes.map((config) => (
        <button
          key={config.id}
          type="button"
          onClick={() => setPalette(config.id)}
        >
          {`set-${config.id}`}
        </button>
      ))}
    </div>
  );
}

function emitSystemPreferenceChange(isDark: boolean) {
  prefersDark = isDark;
  const event = { matches: isDark, media: MEDIA_QUERY } as MediaQueryListEvent;
  mediaListeners.forEach((listener) => listener(event));
}

beforeEach(() => {
  prefersDark = false;
  mediaListeners.clear();
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation(() => ({
      media: MEDIA_QUERY,
      get matches() {
        return prefersDark;
      },
      onchange: null,
      addEventListener: (_event, listener) => {
        mediaListeners.add(listener);
      },
      removeEventListener: (_event, listener) => {
        mediaListeners.delete(listener);
      },
      addListener: (listener) => {
        mediaListeners.add(listener);
      },
      removeListener: (listener) => {
        mediaListeners.delete(listener);
      },
      dispatchEvent: () => true,
    }) as MediaQueryList),
  });
  window.localStorage.clear();
  document.documentElement.classList.remove('dark');
  delete document.documentElement.dataset.theme;
  delete document.documentElement.dataset.themePreference;
  delete document.documentElement.dataset.themePalette;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('ThemeProvider', () => {
  it('defaults to the system preference', async () => {
    render(
      <ThemeProvider>
        <ThemeHarness />
      </ThemeProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('resolved-theme').textContent).toBe('light');
    });
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(document.documentElement.dataset.themePreference).toBe('system');
  });

  it('switches to dark mode when requested', async () => {
    render(
      <ThemeProvider>
        <ThemeHarness />
      </ThemeProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('resolved-theme').textContent).toBe('light');
    });

    fireEvent.click(screen.getByText('set-dark'));

    await waitFor(() => {
      expect(screen.getByTestId('resolved-theme').textContent).toBe('dark');
    });
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('follows system changes when preference is system', async () => {
    render(
      <ThemeProvider>
        <ThemeHarness />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText('set-system'));

    act(() => {
      emitSystemPreferenceChange(true);
    });

    await waitFor(() => {
      expect(screen.getByTestId('resolved-theme').textContent).toBe('dark');
    });
    expect(document.documentElement.dataset.theme).toBe('dark');

    act(() => {
      emitSystemPreferenceChange(false);
    });

    await waitFor(() => {
      expect(screen.getByTestId('resolved-theme').textContent).toBe('light');
    });
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('switches palettes and syncs the dataset/localStorage', async () => {
    render(
      <ThemeProvider>
        <ThemeHarness />
      </ThemeProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('palette').textContent).toBe(DEFAULT_THEME_PALETTE);
    });

    const selectablePalettes = themePalettes.filter((palette) => palette.id !== DEFAULT_THEME_PALETTE);
    for (const paletteMeta of selectablePalettes) {
      fireEvent.click(screen.getByText(`set-${paletteMeta.id}`));

      await waitFor(() => {
        expect(screen.getByTestId('palette').textContent).toBe(paletteMeta.id);
      });
      expect(document.documentElement.dataset.themePalette).toBe(paletteMeta.id);
      expect(window.localStorage.getItem('meltica-theme-palette')).toBe(paletteMeta.id);
    }
  });
});
