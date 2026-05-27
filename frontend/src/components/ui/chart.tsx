'use client'

import * as React from 'react'
import {
  Bar,
  BarChart,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { cn } from '@/lib/utils'

const COLOR_MAP = {
  success: 'var(--color-success)',
  warning: 'var(--color-warning)',
  info: 'var(--color-info)',
  destructive: 'var(--color-destructive)',
  muted: 'var(--color-muted)',
} as const

export type ChartSegmentColor = keyof typeof COLOR_MAP

export type ChartSegment = {
  label: string
  value: number
  color: ChartSegmentColor
}

type StackedBarChartProps = {
  segments: ChartSegment[]
  className?: string
  height?: number
}

export function StackedBarChart({
  segments,
  className,
  height = 24,
}: StackedBarChartProps) {
  const visibleSegments = React.useMemo(
    () => segments.filter((segment) => segment.value > 0),
    [segments],
  )

  const total = React.useMemo(
    () =>
      visibleSegments.reduce(
        (sum, segment) => sum + Math.max(segment.value, 0),
        0,
      ),
    [visibleSegments],
  )

  const data = React.useMemo(() => {
    if (!visibleSegments.length) {
      return []
    }

    return [
      visibleSegments.reduce<Record<string, number | string>>(
        (acc, segment) => {
          acc[segment.label] = Math.max(segment.value, 0)
          return acc
        },
        { category: 'stack' } as Record<string, number | string>,
      ),
    ]
  }, [visibleSegments])

  if (!total || data.length === 0) {
    return null
  }

  return (
    <div data-slot="chart-bar" className={cn('w-full', className)}>
      <ResponsiveContainer width="100%" height={height}>
        <BarChart
          data={data}
          layout="vertical"
          stackOffset="none"
          margin={{ top: 0, right: 0, left: 0, bottom: 0 }}
        >
          <XAxis type="number" hide domain={[0, total]} />
          <YAxis type="category" dataKey="category" hide />
          <RechartsTooltip
            cursor={{ fill: 'transparent' }}
            content={<StackedBarTooltipContent total={total} />}
          />
          {visibleSegments.map((segment, index) => {
            const isFirst = index === 0
            const isLast = index === visibleSegments.length - 1
            return (
              <Bar
                key={segment.label}
                dataKey={segment.label}
                stackId="stack"
                fill={COLOR_MAP[segment.color]}
                radius={[
                  isLast ? 4 : 0,
                  isLast ? 4 : 0,
                  isFirst ? 4 : 0,
                  isFirst ? 4 : 0,
                ]}
                isAnimationActive={false}
              />
            )
          })}
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

type ChartLegendProps = {
  segments: ChartSegment[]
  className?: string
  showValues?: boolean
  totalLabel?: string
}

export function ChartLegend({
  segments,
  className,
  showValues = true,
  totalLabel,
}: ChartLegendProps) {
  const visibleSegments = segments.filter((segment) => segment.value > 0)

  if (visibleSegments.length === 0) {
    return null
  }

  const total = visibleSegments.reduce(
    (sum, segment) => sum + Math.max(segment.value, 0),
    0,
  )

  return (
    <div
      data-slot="chart-legend"
      className={cn(
        'flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground',
        className,
      )}
    >
      {visibleSegments.map((segment) => (
        <span
          key={`${segment.label}-${segment.color}`}
          className="inline-flex items-center gap-2"
        >
          <span
            className="h-2 w-2 rounded-full"
            style={{ backgroundColor: COLOR_MAP[segment.color] }}
          />
          <span className="text-foreground">
            {segment.label}
            {showValues && (
              <span className="ml-1 font-medium text-muted-foreground">
                {segment.value.toLocaleString()} (
                {total ? ((segment.value / total) * 100).toFixed(1) : '0.0'}%)
              </span>
            )}
          </span>
        </span>
      ))}
      {totalLabel ? (
        <span className="inline-flex items-center gap-1 font-medium text-foreground">
          {totalLabel}
          <span className="text-muted-foreground">
            {total.toLocaleString()}
          </span>
        </span>
      ) : null}
    </div>
  )
}

type StackedBarTooltipEntry = {
  value?: number
  name?: string
  color?: string
}

type StackedBarTooltipProps = {
  active?: boolean
  payload?: StackedBarTooltipEntry[]
  total: number
}

function StackedBarTooltipContent({
  active,
  payload,
  total,
}: StackedBarTooltipProps) {
  if (!active || !payload?.length) {
    return null
  }

  return (
    <div className="rounded-md border bg-popover px-3 py-2 text-xs shadow-md">
      {payload.map((entry) => {
        if (!entry.value || Number(entry.value) <= 0) {
          return null
        }
        const value = Number(entry.value)
        const percent = total ? ((value / total) * 100).toFixed(1) : '0.0'

        return (
          <div
            key={entry.name}
            className="flex items-center justify-between gap-4"
          >
            <div className="flex items-center gap-2">
              <span
                className="h-2 w-2 rounded-full"
                style={{ backgroundColor: entry.color ?? 'var(--primary)' }}
              />
              <span className="text-foreground">{entry.name}</span>
            </div>
            <span className="font-medium text-muted-foreground">
              {value.toLocaleString()} ({percent}%)
            </span>
          </div>
        )
      })}
    </div>
  )
}
