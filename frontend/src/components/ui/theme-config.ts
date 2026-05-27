export type ThemePalette =
  | 'claude'
  | 'amber-minimal'
  | 'corporate'
  | 'modern-minimal'
  | 'claymorphism'
  | 'art-deco'
  | 'cyberpunk'
  | 'ghibli-studio'
  | 'vs-code';

export const DEFAULT_THEME_PALETTE: ThemePalette = 'claude';

export interface ThemePaletteMeta {
  id: ThemePalette;
  label: string;
  description: string;
  sourceUrl: string;
}

export const themePalettes: ThemePaletteMeta[] = [
  {
    id: 'claude',
    label: 'Claude',
    description: 'Anthropic Claude-inspired warm neutrals lifted from shadcn.io.',
    sourceUrl: 'https://www.shadcn.io/theme/claude',
  },
  {
    id: 'amber-minimal',
    label: 'Amber Minimal',
    description: 'Modern cream + amber minimal palette from the shadcn gallery.',
    sourceUrl: 'https://www.shadcn.io/theme/amber-minimal',
  },
  {
    id: 'corporate',
    label: 'Corporate',
    description: 'Cool violets and grayscale mix designed for corporate dashboards.',
    sourceUrl: 'https://www.shadcn.io/theme/corporate',
  },
  {
    id: 'modern-minimal',
    label: 'Modern Minimal',
    description: 'Muted lavender + charcoal minimalism captured from shadcn.io.',
    sourceUrl: 'https://www.shadcn.io/theme/modern-minimal',
  },
  {
    id: 'claymorphism',
    label: 'Claymorphism',
    description: 'High-radius claymorphism palette with citrus gradients.',
    sourceUrl: 'https://www.shadcn.io/theme/claymorphism',
  },
  {
    id: 'art-deco',
    label: 'Art Deco',
    description: 'Gold, teal, and onyx tones inspired by art deco motifs.',
    sourceUrl: 'https://www.shadcn.io/theme/art-deco',
  },
  {
    id: 'cyberpunk',
    label: 'Cyberpunk',
    description: 'Neon magenta/teal accents built for cyberpunk-inspired UI.',
    sourceUrl: 'https://www.shadcn.io/theme/cyberpunk',
  },
  {
    id: 'ghibli-studio',
    label: 'Ghibli Studio',
    description: 'Studio Ghibli pastels mixing mossy greens and coral skies.',
    sourceUrl: 'https://www.shadcn.io/theme/ghibli-studio',
  },
  {
    id: 'vs-code',
    label: 'VS Code',
    description: 'Visual Studio Code blues and charcoal UI replica from shadcn.',
    sourceUrl: 'https://www.shadcn.io/theme/vs-code',
  },
];

export function isThemePalette(value: unknown): value is ThemePalette {
  if (typeof value !== 'string') {
    return false;
  }
  return themePalettes.some((palette) => palette.id === value);
}
