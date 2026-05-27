'use client';

import { useEffect, useState } from 'react';
import { Check, LaptopMinimal, Moon, Sun } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { themePalettes, type ThemePalette } from '@/components/ui/theme-config';
import { useTheme, type ThemePreference } from '@/components/ui/theme-provider';
import { cn } from '@/lib/utils';

const THEME_OPTIONS: Array<{
  value: ThemePreference;
  label: string;
  icon: typeof Sun;
}> = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: LaptopMinimal },
];

const PALETTE_SWATCHES: Record<ThemePalette, string[]> = {
  claude: ['#fdf6ec', '#f8d3a4', '#f29c66', '#b45309'],
  'amber-minimal': ['#fff7ed', '#fde68a', '#fbbf24', '#713f12'],
  corporate: ['#eef2ff', '#c4b5fd', '#8b5cf6', '#312e81'],
  'modern-minimal': ['#f8fafc', '#dbeafe', '#a78bfa', '#312e81'],
  claymorphism: ['#fef3c7', '#fde68a', '#fb923c', '#92400e'],
  'art-deco': ['#f5f0dc', '#e4c680', '#d97706', '#1f1b2c'],
  cyberpunk: ['#fdf4ff', '#f472b6', '#c026d3', '#0f172a'],
  'ghibli-studio': ['#f1f5d8', '#b5e2d1', '#6fb1a0', '#1f2937'],
  'vs-code': ['#f2f5ff', '#3b82f6', '#1d4ed8', '#0f172a'],
};

export function ThemeToggle() {
  const { theme, resolvedTheme, setTheme, palette, setPalette } = useTheme();
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMounted(true);
  }, []);

  const displayTheme = mounted ? theme : 'system';
  const displayResolvedTheme: 'light' | 'dark' = mounted ? resolvedTheme : 'light';
  const activeMode = THEME_OPTIONS.find((option) => option.value === displayTheme);
  const activePalette = mounted ? themePalettes.find((item) => item.id === palette) : null;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          className="min-w-[180px] justify-between gap-2 text-sm"
          aria-label="Toggle theme"
        >
          <span className="flex items-center gap-2 text-left">
            <Sun
              className={cn(
                'h-4 w-4 rotate-0 scale-100 transition-all',
                displayResolvedTheme === 'dark' && '-rotate-90 scale-0',
              )}
            />
            <Moon
              className={cn(
                'absolute h-4 w-4 rotate-90 scale-0 transition-all',
                displayResolvedTheme === 'dark' && 'rotate-0 scale-100',
              )}
            />
            <span className="flex flex-col leading-tight">
              <span className="font-medium">
                {activeMode ? activeMode.label : 'Theme'}
              </span>
              <span className="text-xs text-muted-foreground">
                {activePalette ? activePalette.label : 'Default palette'}
              </span>
            </span>
          </span>
          <span className="text-muted-foreground text-xs">Change</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-0" align="end">
        <Command>
          <CommandInput placeholder="Search theme..." className="h-9" />
          <ScrollArea className="max-h-[360px]" viewportClassName="pr-1" allowXScroll={false}>
            <CommandList className="max-h-none overflow-visible">
              <CommandEmpty>No results found.</CommandEmpty>
              <CommandGroup heading="Mode">
                {THEME_OPTIONS.map((option) => (
                  <CommandItem
                    key={option.value}
                    value={option.label}
                    className="flex items-center justify-between"
                    onSelect={() => {
                      setTheme(option.value);
                      setOpen(false);
                    }}
                  >
                    <span className="flex items-center gap-2">
                      <option.icon className="h-4 w-4" />
                      {option.label}
                    </span>
                    {theme === option.value ? <Check className="h-4 w-4" /> : null}
                  </CommandItem>
                ))}
              </CommandGroup>
              <CommandSeparator />
              <CommandGroup heading="Palette">
                {themePalettes.map((option) => (
                  <CommandItem
                    key={option.id}
                    value={option.label}
                    className="flex items-center justify-between"
                    onSelect={() => {
                      setPalette(option.id);
                      setOpen(false);
                    }}
                  >
                    <span className="flex flex-col">
                      <span className="font-medium">{option.label}</span>
                      {option.description ? (
                        <span className="text-xs text-muted-foreground">{option.description}</span>
                      ) : null}
                      <span className="mt-2 flex items-center gap-1">
                        {(PALETTE_SWATCHES[option.id] ?? []).map((color) => (
                          <span
                            key={color}
                            className="h-2.5 w-2.5 rounded-full border border-white/60 shadow-sm dark:border-white/20"
                            style={{ backgroundColor: color }}
                          />
                        ))}
                      </span>
                    </span>
                    {palette === option.id ? <Check className="h-4 w-4" /> : null}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </ScrollArea>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
