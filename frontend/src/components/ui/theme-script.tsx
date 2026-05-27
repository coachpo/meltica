import { DEFAULT_THEME_PALETTE, themePalettes } from '@/components/ui/theme-config';

const PALETTE_STORAGE_KEY = 'meltica-theme-palette';

const paletteIds = JSON.stringify(themePalettes.map((palette) => palette.id));

const paletteInitializer = `
(function() {
  try {
    var root = document.documentElement;
    var storedPalette = localStorage.getItem('${PALETTE_STORAGE_KEY}');
    var allowedPalettes = ${paletteIds};
    var palette = allowedPalettes.includes(storedPalette) ? storedPalette : '${DEFAULT_THEME_PALETTE}';
    root.dataset.themePalette = palette;
  } catch (error) {
    // ignore
  }
})();
`;

export function ThemeScript() {
  return <script dangerouslySetInnerHTML={{ __html: paletteInitializer }} />;
}
