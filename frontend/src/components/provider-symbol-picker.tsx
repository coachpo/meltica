'use client';

/* eslint-disable react-hooks/incompatible-library */

import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { InfoIcon, Loader2Icon, XIcon } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import type { Instrument } from '@/lib/types';

const LARGE_SYMBOL_THRESHOLD = 400;
const MIN_FILTER_LENGTH = 2;
const LIST_MAX_HEIGHT_REM = '16rem';

export type ProviderSymbolPickerProps = {
  providerName: string;
  symbols: string[];
  loading: boolean;
  error: string | null;
  filterValue: string;
  selectedSymbols: string[];
  instrumentDetails?: Record<string, Instrument>;
  onFilterChange(value: string): void;
  onToggleSymbol(symbol: string, checked: boolean): void;
  onRetry(): void;
  onSelectAll(symbols: string[]): void;
  onClearAll(): void;
};

export function ProviderSymbolPicker({
  providerName,
  symbols,
  loading,
  error,
  filterValue,
  selectedSymbols,
  instrumentDetails,
  onFilterChange,
  onToggleSymbol,
  onRetry,
  onSelectAll,
  onClearAll,
}: ProviderSymbolPickerProps) {
  const deferredFilter = useDeferredValue(filterValue.trim().toLowerCase());
  const [popoverMounted, setPopoverMounted] = useState(false);
  const selectedSet = useMemo(() => new Set(selectedSymbols), [selectedSymbols]);
  const largeDataset = symbols.length > LARGE_SYMBOL_THRESHOLD;
  const allowFullRender = !largeDataset || deferredFilter.length >= MIN_FILTER_LENGTH;
  const filteredSymbols = useMemo(() => {
    if (symbols.length === 0) {
      return [];
    }
    if (!allowFullRender) {
      return [];
    }
    if (!deferredFilter) {
      return symbols;
    }
    return symbols.filter((symbol) => symbol.toLowerCase().includes(deferredFilter));
  }, [symbols, deferredFilter, allowFullRender]);

  const parentRef = useRef<HTMLDivElement | null>(null);
  const rowVirtualizer = useVirtualizer({
    count: filteredSymbols.length,
    estimateSize: () => 32,
    overscan: 8,
    getScrollElement: () => parentRef.current,
  });
  const virtualItems = rowVirtualizer.getVirtualItems();

  const renderList = allowFullRender && filteredSymbols.length > 0;
  const showTypeHint = largeDataset && !allowFullRender && !loading && !error;
  const hasSelection = selectedSymbols.length > 0;
  const canSelectAll = renderList && filteredSymbols.length > 0;

  const setViewportRef = useCallback((node: HTMLDivElement | null) => {
    parentRef.current = node;
    setPopoverMounted(Boolean(node));
  }, []);

  const handleWheelScroll = useCallback((event: React.WheelEvent<HTMLDivElement>) => {
    const viewport = parentRef.current;
    if (!viewport) {
      return;
    }
    if (event.deltaY !== 0) {
      event.preventDefault();
      viewport.scrollBy({ top: event.deltaY, behavior: 'auto' });
    }
    if (event.deltaX !== 0) {
      event.preventDefault();
      viewport.scrollBy({ left: event.deltaX, behavior: 'auto' });
    }
  }, []);

  useEffect(() => {
    const viewport = parentRef.current;
    if (!popoverMounted || !viewport) {
      return;
    }
    viewport.scrollTop = 0;
    rowVirtualizer.scrollToOffset(0);
    rowVirtualizer.measure();
  }, [popoverMounted, deferredFilter, filteredSymbols.length, rowVirtualizer]);

  const instrumentMetadata = instrumentDetails ?? {};

  const comboboxLabel = hasSelection
    ? `${selectedSymbols.length} symbol${selectedSymbols.length === 1 ? '' : 's'} selected`
    : `Select symbols for ${providerName}`;

  return (
    <div className="space-y-3 rounded-md border p-3">
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span className="font-medium text-foreground">{providerName}</span>
        {symbols.length > 0 ? (
          <span>{symbols.length.toLocaleString()} available</span>
        ) : null}
      </div>

      <Popover>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            role="combobox"
            aria-expanded={popoverMounted}
            className="w-full justify-between"
            disabled={loading || (!!error && symbols.length === 0)}
          >
            <span className="truncate text-left text-sm">{comboboxLabel}</span>
            <span className="text-muted-foreground text-xs">Manage</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[min(26rem,_calc(100vw-3rem))] p-0" align="start">
          <div className="border-b px-3 py-2">
            <Input
              value={filterValue}
              onChange={(event) => onFilterChange(event.target.value)}
              placeholder=
                {largeDataset ? 'Type at least 2 characters to search' : 'Search symbols'}
              disabled={loading}
              autoFocus
            />
          </div>
          <div className="flex items-center justify-between gap-2 px-3 py-2 text-xs text-muted-foreground">
            <span>
              {filteredSymbols.length.toLocaleString()} match{filteredSymbols.length === 1 ? '' : 'es'}
            </span>
            <div className="flex gap-1">
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => onSelectAll(filteredSymbols)}
                disabled={loading || error !== null || !canSelectAll}
              >
                Select all
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={onClearAll}
                disabled={!hasSelection}
              >
                Clear
              </Button>
            </div>
          </div>
          <div className="px-3 pb-3">
            {loading ? (
              <div className="flex items-center justify-center gap-2 py-6 text-xs text-muted-foreground">
                <Loader2Icon className="h-4 w-4 animate-spin" /> Loading symbols...
              </div>
            ) : error ? (
              <div className="space-y-2 text-xs">
                <p className="text-destructive">{error}</p>
                <Button type="button" variant="outline" size="sm" onClick={onRetry}>
                  Retry
                </Button>
              </div>
            ) : symbols.length === 0 ? (
              <p className="text-xs text-muted-foreground">No symbols available for this provider.</p>
            ) : showTypeHint ? (
              <p className="text-xs text-muted-foreground">
                This provider exposes {symbols.length.toLocaleString()} instruments. Type at least{' '}
                {MIN_FILTER_LENGTH} characters to narrow the list.
              </p>
            ) : renderList ? (
              <TooltipProvider delayDuration={150}>
                <ScrollArea
                  className="max-h-64 rounded border"
                  viewportClassName="max-h-64"
                  allowXScroll={false}
                  viewportProps={{
                    ref: setViewportRef,
                    onWheel: handleWheelScroll,
                    'aria-label': `${providerName} available symbols`,
                    style: { maxHeight: LIST_MAX_HEIGHT_REM },
                  }}
                >
                  <div style={{ height: rowVirtualizer.getTotalSize(), position: 'relative' }}>
                    {virtualItems.map((virtualRow) => {
                      const symbol = filteredSymbols[virtualRow.index];
                      const checked = selectedSet.has(symbol);
                      const metadata = instrumentMetadata[symbol];
                      return (
                        <div
                          key={virtualRow.key}
                          className="absolute left-0 right-0 flex cursor-pointer items-center gap-2 border-b px-2 py-1 text-sm text-foreground last:border-none hover:bg-muted/60"
                          style={{ transform: `translateY(${virtualRow.start}px)`, height: `${virtualRow.size}px` }}
                          onClick={(event) => {
                            const target = event.target as HTMLElement | null;
                            if (!target) {
                              return;
                            }
                            if (target.closest('[data-symbol-checkbox]') || target.closest('[data-symbol-info]')) {
                              return;
                            }
                            onToggleSymbol(symbol, !checked);
                          }}
                        >
                          <Checkbox
                            data-symbol-checkbox
                            checked={checked}
                            onCheckedChange={(nextChecked) =>
                              onToggleSymbol(symbol, nextChecked === true)
                            }
                          />
                          <span className="flex-1 truncate" title={symbol}>
                            {symbol}
                          </span>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-7 w-7 rounded-full p-1 text-muted-foreground transition hover:text-foreground"
                                onClick={(event) => event.preventDefault()}
                                data-symbol-info
                                aria-label={metadata ? `View fields for ${symbol}` : `No additional fields for ${symbol}`}
                              >
                                <InfoIcon className="h-4 w-4" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent align="end" side="left" className="max-w-xs space-y-1 text-left">
                              {metadata ? (
                                <InstrumentFields instrument={metadata} />
                              ) : (
                                <span className="text-xs text-muted-foreground">No instrument details available.</span>
                              )}
                            </TooltipContent>
                          </Tooltip>
                        </div>
                      );
                    })}
                  </div>
                </ScrollArea>
              </TooltipProvider>
            ) : (
              <p className="text-xs text-muted-foreground">No matching symbols found.</p>
            )}
          </div>
        </PopoverContent>
      </Popover>

      {hasSelection ? (
        <div className="flex flex-wrap gap-2">
          {selectedSymbols.map((symbol) => (
            <Badge key={symbol} variant="secondary" className="flex items-center gap-1">
              <span>{symbol}</span>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`Remove ${symbol}`}
                className="h-5 w-5 p-0 text-muted-foreground transition hover:text-foreground"
                onClick={() => onToggleSymbol(symbol, false)}
              >
                <XIcon className="h-3 w-3" />
              </Button>
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">No symbols selected yet.</p>
      )}
    </div>
  );
}

type InstrumentFieldsProps = {
  instrument: Instrument;
};

function InstrumentFields({ instrument }: InstrumentFieldsProps) {
  const entries = useMemo(() => {
    return Object.entries(instrument).sort(([a], [b]) => a.localeCompare(b));
  }, [instrument]);

  if (entries.length === 0) {
    return <span className="text-xs text-muted-foreground">No instrument details available.</span>;
  }

  const formatValue = (value: unknown) => {
    if (value === null || value === undefined || value === '') {
      return '—';
    }
    if (typeof value === 'object') {
      try {
        return JSON.stringify(value);
      } catch {
        return '[unserializable]';
      }
    }
    return String(value);
  };

  return (
    <dl className="space-y-1 text-xs">
      {entries.map(([key, value]) => (
        <div key={key} className="flex items-start gap-2">
          <dt className="min-w-[4rem] text-foreground">{key}</dt>
          <dd className="flex-1 break-words text-muted-foreground">{formatValue(value)}</dd>
        </div>
      ))}
    </dl>
  );
}
