'use client';

import * as React from 'react';
import {
  motion,
  useMotionValue,
  useReducedMotion,
  useSpring,
  type SpringOptions,
} from 'framer-motion';

import { useTheme } from '@/components/ui/theme-provider';
import { cn } from '@/lib/utils';

export type ColorConfig = {
  first: string;
  second: string;
  third: string;
  fourth: string;
  fifth: string;
  sixth: string;
};

const DEFAULT_COLORS: ColorConfig = {
  first: '18,113,255',
  second: '221,74,255',
  third: '0,220,255',
  fourth: '200,50,50',
  fifth: '180,180,50',
  sixth: '140,100,255',
};

const LIGHT_MODE_COLORS: ColorConfig = {
  first: '255,183,0',
  second: '255,105,180',
  third: '133,199,255',
  fourth: '255,148,112',
  fifth: '255,241,181',
  sixth: '120,200,255',
};

const DEFAULT_TRANSITION: SpringOptions = {
  stiffness: 100,
  damping: 20,
};

export type BackgroundCanvasProps = React.ComponentProps<'div'> & {
  interactive?: boolean;
  transition?: SpringOptions;
  colors?: ColorConfig;
};

export const BackgroundCanvas = React.forwardRef<HTMLDivElement, BackgroundCanvasProps>(
  (
    {
      className,
      children,
      interactive = false,
      transition = DEFAULT_TRANSITION,
      colors: customColors,
      style,
      ...props
    },
    ref,
  ) => {
    const containerRef = React.useRef<HTMLDivElement>(null);
    React.useImperativeHandle(ref, () => containerRef.current as HTMLDivElement);

    const { resolvedTheme } = useTheme();
    const [mounted, setMounted] = React.useState(false);

    React.useEffect(() => {
      setMounted(true);
    }, []);

    const shouldReduceMotion = useReducedMotion();
    const mouseX = useMotionValue(0);
    const mouseY = useMotionValue(0);
    const springX = useSpring(mouseX, transition);
    const springY = useSpring(mouseY, transition);
    const rectRef = React.useRef<DOMRect | null>(null);
    const lastPointerRef = React.useRef<{ x: number; y: number } | null>(null);
    const pointerFrameRef = React.useRef<number | null>(null);

    const effectiveTheme: 'light' | 'dark' = mounted ? resolvedTheme : 'dark';
    const isLightTheme = effectiveTheme === 'light';
    const palette = customColors ?? (isLightTheme ? LIGHT_MODE_COLORS : DEFAULT_COLORS);
    const blendMode = isLightTheme ? 'mix-blend-multiply' : 'mix-blend-hard-light';
    const overlayStyle = shouldReduceMotion
      ? undefined
      : {
          filter: isLightTheme ? 'blur(24px) saturate(1.1)' : 'blur(30px)',
        };

    const colorVars = React.useMemo(
      () =>
        ({
          '--first-color': palette.first,
          '--second-color': palette.second,
          '--third-color': palette.third,
          '--fourth-color': palette.fourth,
          '--fifth-color': palette.fifth,
          '--sixth-color': palette.sixth,
        }) as React.CSSProperties,
      [palette],
    );
    const mergedStyle = style ? { ...colorVars, ...style } : colorVars;

    const resetPointer = React.useCallback(() => {
      mouseX.set(0);
      mouseY.set(0);
    }, [mouseX, mouseY]);

    const updateContainerRect = React.useCallback(() => {
      if (!containerRef.current) {
        rectRef.current = null;
        return null;
      }
      const rect = containerRef.current.getBoundingClientRect();
      rectRef.current = rect;
      return rect;
    }, []);

    const flushPointer = React.useCallback(() => {
      const coords = lastPointerRef.current;
      if (!coords) {
        return;
      }
      lastPointerRef.current = null;
      const rect = rectRef.current ?? updateContainerRect();
      if (!rect || rect.width === 0 || rect.height === 0) {
        resetPointer();
        return;
      }
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      mouseX.set(coords.x - centerX);
      mouseY.set(coords.y - centerY);
    }, [mouseX, mouseY, resetPointer, updateContainerRect]);

    const schedulePointerUpdate = React.useCallback(
      (event: PointerEvent) => {
        lastPointerRef.current = { x: event.clientX, y: event.clientY };
        if (pointerFrameRef.current != null) {
          return;
        }
        pointerFrameRef.current = window.requestAnimationFrame(() => {
          pointerFrameRef.current = null;
          flushPointer();
        });
      },
      [flushPointer],
    );

    React.useEffect(() => {
      return () => {
        if (pointerFrameRef.current != null) {
          cancelAnimationFrame(pointerFrameRef.current);
        }
      };
    }, []);

    React.useEffect(() => {
      if (!interactive) {
        return;
      }

      const handleResize = () => {
        updateContainerRect();
      };

      window.addEventListener('resize', handleResize);
      return () => {
        window.removeEventListener('resize', handleResize);
      };
    }, [interactive, updateContainerRect]);

    React.useEffect(() => {
      if (!interactive) {
        resetPointer();
        rectRef.current = null;
        return;
      }

      updateContainerRect();

      const node = containerRef.current;
      if (!node) {
        return;
      }

      const handlePointerEnter = () => {
        updateContainerRect();
      };

      const handlePointerMove = (event: PointerEvent) => {
        schedulePointerUpdate(event);
      };

      const handlePointerLeave = () => {
        resetPointer();
      };

      node.addEventListener('pointerenter', handlePointerEnter);
      node.addEventListener('pointermove', handlePointerMove, { passive: true });
      node.addEventListener('pointerleave', handlePointerLeave);

      return () => {
        node.removeEventListener('pointerenter', handlePointerEnter);
        node.removeEventListener('pointermove', handlePointerMove);
        node.removeEventListener('pointerleave', handlePointerLeave);
      };
    }, [interactive, resetPointer, schedulePointerUpdate, updateContainerRect]);

    return (
      <div
        ref={containerRef}
        data-slot="bubble-background"
        className={cn(
          'relative size-full overflow-hidden rounded-3xl transition-colors duration-500',
          isLightTheme
            ? 'bg-gradient-to-br from-orange-50 via-amber-50/70 to-sky-100 shadow-[inset_0_1px_0_rgba(255,255,255,0.6)]'
            : 'bg-gradient-to-br from-violet-950 via-indigo-950 to-blue-900',
          className,
        )}
        style={mergedStyle}
        {...props}
      >
        <div className={cn('absolute inset-0', !shouldReduceMotion && 'will-change-transform')} style={overlayStyle}>
          <motion.div
            className={cn(
              'absolute left-[10%] top-[10%] size-[80%] rounded-full bg-[radial-gradient(circle_at_center,rgba(var(--first-color),0.8)_0%,rgba(var(--first-color),0)_50%)]',
              blendMode,
            )}
            animate={shouldReduceMotion ? undefined : { y: [-50, 50, -50] }}
            transition={
              shouldReduceMotion
                ? undefined
                : { duration: 30, ease: 'easeInOut', repeat: Infinity }
            }
          />

          <div
            className={cn(
              'absolute inset-0 flex items-center justify-center origin-[calc(50%-400px)] will-change-transform',
              shouldReduceMotion ? 'animate-none' : 'animate-spin',
            )}
            style={shouldReduceMotion ? undefined : { animationDuration: '20s' }}
          >
            <div
              className={cn(
                'size-[80%] rounded-full bg-[radial-gradient(circle_at_center,rgba(var(--second-color),0.8)_0%,rgba(var(--second-color),0)_50%)]',
                blendMode,
              )}
            />
          </div>

          <div
            className={cn(
              'absolute inset-0 flex items-center justify-center origin-[calc(50%+400px)] will-change-transform',
              shouldReduceMotion ? 'animate-none' : 'animate-spin',
            )}
            style={shouldReduceMotion ? undefined : { animationDuration: '40s' }}
          >
            <div
              className={cn(
                'absolute left-[calc(50%-500px)] top-[calc(50%+200px)] size-[80%] rounded-full bg-[radial-gradient(circle_at_center,rgba(var(--third-color),0.8)_0%,rgba(var(--third-color),0)_50%)]',
                blendMode,
              )}
            />
          </div>

          <motion.div
            className={cn(
              'absolute left-[10%] top-[10%] size-[80%] rounded-full bg-[radial-gradient(circle_at_center,rgba(var(--fourth-color),0.8)_0%,rgba(var(--fourth-color),0)_50%)] opacity-80',
              blendMode,
            )}
            animate={shouldReduceMotion ? undefined : { x: [-40, 40, -40] }}
            transition={
              shouldReduceMotion
                ? undefined
                : { duration: 38, ease: 'easeInOut', repeat: Infinity }
            }
          />

          <div
            className={cn(
              'absolute inset-0 flex items-center justify-center origin-[calc(50%_-_800px)_calc(50%_+_200px)] will-change-transform',
              shouldReduceMotion ? 'animate-none' : 'animate-spin',
            )}
            style={shouldReduceMotion ? undefined : { animationDuration: '26s' }}
          >
            <div
              className={cn(
                'absolute left-[calc(50%-80%)] top-[calc(50%-80%)] size-[160%] rounded-full bg-[radial-gradient(circle_at_center,rgba(var(--fifth-color),0.8)_0%,rgba(var(--fifth-color),0)_50%)]',
                blendMode,
              )}
            />
          </div>

          {interactive && (
            <motion.div
              className={cn(
                'absolute size-full rounded-full bg-[radial-gradient(circle_at_center,rgba(var(--sixth-color),0.8)_0%,rgba(var(--sixth-color),0)_50%)] opacity-70',
                blendMode,
              )}
              style={{ x: springX, y: springY }}
            />
          )}
        </div>

        {children}
      </div>
    );
  },
);

BackgroundCanvas.displayName = 'BackgroundCanvas';
