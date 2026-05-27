# Theme Palette Onboarding Guide

This guide summarizes the steps we followed to onboard new shadcn.io palettes into the Meltica client. Use it as a checklist whenever you add or refresh themes.

## 1. Collect Palette Tokens from shadcn.io
- Visit the [shadcn.io themes gallery](https://ui.shadcn.com/themes) (Context7 lookup is preferred for repeatable capture).
- Copy the OKLCH values for both light (`:root`) and dark (`.dark`) variants of the palette(s) you plan to add.
- Capture the palette ID, human-friendly label, and the source URL for attribution.

## 2. Register Palette Metadata
- Update `src/components/ui/theme-config.ts`:
  - Extend `ThemePalette` with the new ID.
  - Append a `ThemePaletteMeta` entry that includes `label`, `description`, and `sourceUrl`.
  - If the new palette should become the default, update `DEFAULT_THEME_PALETTE`.

## 3. Define CSS Variable Overrides
- In `src/app/globals.css`, add two blocks per palette:
  - `:root[data-theme-palette="<id>"]` for light tokens.
  - `.dark[data-theme-palette="<id>"]` for dark tokens.
- Keep parity with the existing base variables (`--background`, `--primary`, feedback colors, sidebar colors, etc.) so Tailwind utilities continue to resolve.
- If chart tokens or other custom properties need palette-specific values, define them inside the same blocks.

## 4. Persist Palette Selection in State
- `src/components/ui/theme-provider.tsx` already exposes palette state, dataset syncing, and `meltica-theme-palette` storage through `PaletteProvider`—no new stateful logic is required when onboarding palettes.
- After updating the `ThemePalette` union (Step 2), smoke-test the provider by switching to the new palette and confirming:
  - `document.documentElement.dataset.themePalette` reflects the new ID.
  - `localStorage.getItem('meltica-theme-palette')` stores the value.
- If the new palette should become the default, update `DEFAULT_THEME_PALETTE` (Step 2) so `getStoredPalette` and the initial dataset use it automatically.

## 5. Hydrate Palette Before React Mounts
- Modify `src/components/ui/theme-script.tsx`:
  - Import the palette list to build an allow-list in the inline script.
  - Read the stored palette (falling back to `DEFAULT_THEME_PALETTE`) and set `dataset.themePalette` in the pre-hydration snippet.

## 6. Expose Palette Controls in the UI
- `src/components/theme-toggle.tsx` renders palette options by iterating over `themePalettes`, so new palettes show up automatically once `theme-config.ts` is updated.
- Extend the `PALETTE_SWATCHES` map in the same file so the preview row renders accurate accent chips for the new palette.
- Verify the description text (from `theme-config.ts`) wraps cleanly and the dropdown width still accommodates the copy.

## 7. Add/Update Tests
- Extend `src/components/ui/theme-provider.test.tsx`:
  - Cover palette switching, DOM dataset updates, and `localStorage` writes.
  - Reset `document.documentElement.dataset.themePalette` in `beforeEach`.
- Run the suite:
  ```bash
  pnpm test src/components/ui/theme-provider.test.tsx
  ```

## 8. Document the Changes
- Mention the new palettes in the README’s theming section (and any other user-facing docs) so developers know where to add/edit palettes.
- If you add bespoke palettes (e.g., client-branded), include the palette rationale and any external references in `theme-config.ts`.

Following these steps keeps runtime theming consistent, prevents hydration flicker, and ensures every palette surfaced in the UI has matching CSS + tests.
