"use client";

import * as React from 'react';
import * as ScrollAreaPrimitive from '@radix-ui/react-scroll-area';

import { cn } from '@/lib/utils';

type ScrollAreaViewportProps = React.ComponentPropsWithoutRef<typeof ScrollAreaPrimitive.Viewport> & {
  ref?: React.Ref<HTMLDivElement>;
};

type ScrollAreaProps = React.ComponentPropsWithoutRef<typeof ScrollAreaPrimitive.Root> & {
  viewportClassName?: string;
  viewportProps?: ScrollAreaViewportProps;
  allowXScroll?: boolean;
  allowYScroll?: boolean;
};

function setRefValue<T>(ref: React.Ref<T> | undefined, value: T | null) {
  if (!ref) {
    return;
  }
  if (typeof ref === 'function') {
    ref(value);
  } else {
    (ref as React.MutableRefObject<T | null>).current = value;
  }
}

const ScrollArea = React.forwardRef<React.ElementRef<typeof ScrollAreaPrimitive.Root>, ScrollAreaProps>(
  (
    {
      className,
      viewportClassName,
      viewportProps,
      children,
      allowXScroll = true,
      allowYScroll = true,
      ...props
    },
    ref,
  ) => {
    const [fallbackScroll, setFallbackScroll] = React.useState({ x: false, y: false });
    const [contentOverflow, setContentOverflow] = React.useState({ x: false, y: false });
    const viewportNodeRef = React.useRef<HTMLDivElement | null>(null);
    const { style: viewportStyle, ref: viewportForwardedRef, ...restViewportProps } = viewportProps ?? {};

    const setViewportNodeRef = React.useCallback((node: HTMLDivElement | null) => {
      viewportNodeRef.current = node;
    }, []);

    const composedViewportRef = React.useCallback(
      (node: HTMLDivElement | null) => {
        setViewportNodeRef(node);
        setRefValue(viewportForwardedRef, node);
      },
      [setViewportNodeRef, viewportForwardedRef],
    );

    React.useLayoutEffect(() => {
      const viewportEl = viewportNodeRef.current;
      if (!viewportEl || typeof window === 'undefined') {
        return;
      }

      const updateOverflowState = () => {
        const nextOverflow = {
          x: viewportEl.scrollWidth > viewportEl.clientWidth,
          y: viewportEl.scrollHeight > viewportEl.clientHeight,
        };

        setContentOverflow((prev) =>
          prev.x === nextOverflow.x && prev.y === nextOverflow.y ? prev : nextOverflow,
        );

        setFallbackScroll((prev) => {
          const nextFallback = {
            x: !allowXScroll && nextOverflow.x,
            y: !allowYScroll && nextOverflow.y,
          };

          return prev.x === nextFallback.x && prev.y === nextFallback.y ? prev : nextFallback;
        });
      };

      updateOverflowState();

      const resizeObserver = 'ResizeObserver' in window ? new ResizeObserver(updateOverflowState) : null;
      resizeObserver?.observe(viewportEl);

      const mutationObserver = 'MutationObserver' in window ? new MutationObserver(updateOverflowState) : null;
      mutationObserver?.observe(viewportEl, {
        childList: true,
        subtree: true,
        characterData: true,
        attributes: true,
      });

      window.addEventListener('resize', updateOverflowState);

      return () => {
        resizeObserver?.disconnect();
        mutationObserver?.disconnect();
        window.removeEventListener('resize', updateOverflowState);
      };
    }, [allowXScroll, allowYScroll]);

    const shouldScrollX = allowXScroll || fallbackScroll.x;
    const shouldScrollY = allowYScroll || fallbackScroll.y;
    const showScrollbarX = contentOverflow.x && shouldScrollX;
    const showScrollbarY = contentOverflow.y && shouldScrollY;

    return (
      <ScrollAreaPrimitive.Root
        ref={ref}
        data-slot="scroll-area"
        className={cn('relative max-h-full max-w-full min-h-0 min-w-0 overflow-hidden', className)}
        {...props}
      >
        <ScrollAreaPrimitive.Viewport
          ref={composedViewportRef}
          data-slot="scroll-area-viewport"
          className={cn('h-full w-full max-h-full min-h-0 rounded-[inherit]', viewportClassName)}
          style={{
            height: '100%',
            maxHeight: 'inherit',
            overflowX: shouldScrollX ? 'auto' : 'visible',
            overflowY: shouldScrollY ? 'auto' : 'visible',
            ...viewportStyle,
          }}
          {...restViewportProps}
        >
          {children}
        </ScrollAreaPrimitive.Viewport>
        {showScrollbarY ? <ScrollBar orientation="vertical" /> : null}
        {showScrollbarX ? <ScrollBar orientation="horizontal" /> : null}
        <ScrollAreaPrimitive.Corner />
      </ScrollAreaPrimitive.Root>
    );
  },
);
ScrollArea.displayName = ScrollAreaPrimitive.Root.displayName;

const ScrollBar = React.forwardRef<
  React.ElementRef<typeof ScrollAreaPrimitive.Scrollbar>,
  React.ComponentPropsWithoutRef<typeof ScrollAreaPrimitive.Scrollbar>
>(({ className, orientation = 'vertical', ...props }, ref) => (
  <ScrollAreaPrimitive.Scrollbar
    ref={ref}
    orientation={orientation}
    data-slot="scroll-area-scrollbar"
    className={cn(
      'flex touch-none select-none transition-colors p-[1px]',
      orientation === 'vertical' && 'h-full w-2.5 border-l border-l-transparent',
      orientation === 'horizontal' && 'h-2.5 border-t border-t-transparent',
      className,
    )}
    {...props}
  >
    <ScrollAreaPrimitive.Thumb
      data-slot="scroll-area-thumb"
      className="bg-border relative flex-1 rounded-full"
    />
  </ScrollAreaPrimitive.Scrollbar>
));
ScrollBar.displayName = ScrollAreaPrimitive.Scrollbar.displayName;

export { ScrollArea, ScrollBar };
