'use client';

import Link from 'next/link';
import { useMemo } from 'react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartLegend, StackedBarChart, type ChartSegment } from '@/components/ui/chart';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { useInstancesQuery, useProvidersQuery, useRiskLimitsQuery } from '@/lib/hooks';
import { computeRiskPresence, type RiskPresence } from '@/lib/risk-presence';
import { cn } from '@/lib/utils';

const numberFormatter = new Intl.NumberFormat('en-US');

const severityStyles: Record<DashboardEvent['severity'], string> = {
  critical: 'border-destructive/30 bg-destructive/10 text-destructive dark:text-destructive-foreground/90',
  warning: 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400',
  info: 'border-primary/30 bg-primary/10 text-primary',
};

function formatNumber(value: number | null | undefined): string {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '—';
  }
  return numberFormatter.format(value);
}

type DashboardEvent = {
  id: string;
  title: string;
  description: string;
  severity: 'critical' | 'warning' | 'info';
  href?: string;
};

function buildRiskChecks(presence: RiskPresence) {
  const checks: { label: string; configured: boolean }[] = [
    { label: 'Max position size', configured: presence.maxPositionSize },
    { label: 'Max notional value', configured: presence.maxNotionalValue },
    { label: 'Notional currency', configured: presence.notionalCurrency },
    { label: 'Order throttle', configured: presence.orderThrottle },
    { label: 'Order burst', configured: presence.orderBurst },
    { label: 'Max concurrent orders', configured: presence.maxConcurrentOrders },
    { label: 'Price band percent', configured: presence.priceBandPercent },
    { label: 'Allowed order types', configured: presence.allowedOrderTypes },
    { label: 'Kill switch', configured: presence.killSwitchEnabled },
    { label: 'Max risk breaches', configured: presence.maxRiskBreaches },
  ];
  if (!presence.circuitBreaker.enabled) {
    checks.push({ label: 'Circuit breaker', configured: false });
  } else {
    checks.push({ label: 'Circuit breaker', configured: true });
    checks.push({ label: 'Circuit breaker threshold', configured: presence.circuitBreaker.threshold });
    checks.push({ label: 'Circuit breaker cooldown', configured: presence.circuitBreaker.cooldown });
  }
  return checks;
}

export default function Home() {
  const instancesQuery = useInstancesQuery();
  const providersQuery = useProvidersQuery();
  const riskQuery = useRiskLimitsQuery();

  const instances = useMemo(() => [...(instancesQuery.data ?? [])], [instancesQuery.data]);
  const providers = useMemo(() => [...(providersQuery.data ?? [])], [providersQuery.data]);
  const riskLimits = riskQuery.data?.limits ?? null;

  const runningInstances = instances.filter((instance) => instance.running).length;
  const stoppedInstances = Math.max(instances.length - runningInstances, 0);

  const instanceSegments: ChartSegment[] = [
    { label: 'Running', value: runningInstances, color: 'success' },
    { label: 'Stopped', value: stoppedInstances, color: 'warning' },
  ];

  const instanceSymbolSummary = useMemo(() => {
    const symbols = new Set<string>();
    instances.forEach((instance) => {
      (instance.aggregatedSymbols ?? []).forEach((symbol) => {
        if (symbol) {
          symbols.add(symbol);
        }
      });
    });
    return symbols;
  }, [instances]);

  const topInstances = useMemo(() => {
    return instances
      .slice()
      .sort((a, b) => {
        if (a.running === b.running) {
          return a.id.localeCompare(b.id);
        }
        return a.running ? -1 : 1;
      })
      .slice(0, 4);
  }, [instances]);

  const providerStatusCounts = providers.reduce<Record<string, number>>((acc, provider) => {
    acc[provider.status] = (acc[provider.status] ?? 0) + 1;
    return acc;
  }, { pending: 0, starting: 0, running: 0, stopped: 0, failed: 0 });

  const providerSegments: ChartSegment[] = [
    { label: 'Running', value: providerStatusCounts.running, color: 'success' },
    { label: 'Starting', value: providerStatusCounts.starting + providerStatusCounts.pending, color: 'info' },
    { label: 'Stopped', value: providerStatusCounts.stopped, color: 'warning' },
    { label: 'Failed', value: providerStatusCounts.failed, color: 'destructive' },
  ];

  const topProviders = useMemo(() => {
    const severityOrder: Record<string, number> = {
      failed: 0,
      stopped: 1,
      starting: 2,
      pending: 3,
      running: 4,
    };
    return providers
      .slice()
      .sort((a, b) => {
        const severityDelta = (severityOrder[a.status] ?? 5) - (severityOrder[b.status] ?? 5);
        if (severityDelta !== 0) {
          return severityDelta;
        }
        return a.name.localeCompare(b.name);
      })
      .slice(0, 4);
  }, [providers]);

  const dependentInstanceTotal = providers.reduce((sum, provider) => {
    if (typeof provider.dependentInstanceCount === 'number') {
      return sum + provider.dependentInstanceCount;
    }
    if (Array.isArray(provider.dependentInstances)) {
      return sum + provider.dependentInstances.length;
    }
    return sum;
  }, 0);

  const instrumentCoverage = providers.reduce((sum, provider) => sum + provider.instrumentCount, 0);
  const portfolioCoverage = instrumentCoverage > 0
    ? Math.min(100, Math.round((instanceSymbolSummary.size / instrumentCoverage) * 100))
    : null;

  const riskPresence = useMemo(() => {
    if (!riskLimits) {
      return null;
    }
    return computeRiskPresence(riskLimits);
  }, [riskLimits]);

  const riskCoverage = useMemo(() => {
    if (!riskPresence) {
      return { configured: 0, total: 0, missing: [] as string[] };
    }
    const checks = buildRiskChecks(riskPresence);
    const missing = checks.filter((check) => !check.configured).map((check) => check.label);
    return {
      configured: checks.length - missing.length,
      total: checks.length,
      missing,
    };
  }, [riskPresence]);

  const riskSegments: ChartSegment[] = riskCoverage.total > 0
    ? [
        { label: 'Configured', value: riskCoverage.configured, color: 'success' },
        { label: 'Missing', value: riskCoverage.total - riskCoverage.configured, color: 'warning' },
      ]
    : [];

  const dashboardEvents = useMemo(() => {
    const events: DashboardEvent[] = [];
    providers.forEach((provider) => {
      if (provider.status === 'failed' || provider.status === 'stopped') {
        events.push({
          id: `provider-${provider.name}`,
          title: `${provider.name} ${provider.status === 'failed' ? 'failed' : 'stopped'}`,
          description:
            provider.status === 'failed'
              ? provider.startupError ?? 'Review logs and restart the provider.'
              : 'Start this provider to resume streaming.',
          severity: provider.status === 'failed' ? 'critical' : 'warning',
          href: '/providers',
        });
      }
    });
    instances.forEach((instance) => {
      if (!instance.running) {
        events.push({
          id: `instance-${instance.id}`,
          title: `${instance.id} is offline`,
          description: `${instance.strategyIdentifier} has been stopped. Start it to resume trading.`,
          severity: 'warning',
          href: '/instances',
        });
      }
    });
    if (riskCoverage.missing.length > 0) {
      events.push({
        id: 'risk-missing-fields',
        title: 'Risk guardrails incomplete',
        description: `Configure ${riskCoverage.missing.slice(0, 3).join(', ')} to enable full protection.`,
        severity: 'warning',
        href: '/risk',
      });
    } else if (riskPresence && !riskPresence.killSwitchEnabled) {
      events.push({
        id: 'risk-kill-switch',
        title: 'Kill switch disabled',
        description: 'Consider enabling the kill switch for emergency shutdowns.',
        severity: 'info',
        href: '/risk',
      });
    }
    return events.slice(0, 5);
  }, [providers, instances, riskCoverage, riskPresence]);

  const queryErrors: string[] = [];
  if (instancesQuery.isError) {
    queryErrors.push(
      instancesQuery.error instanceof Error
        ? instancesQuery.error.message
        : 'Failed to load instances.',
    );
  }
  if (providersQuery.isError) {
    queryErrors.push(
      providersQuery.error instanceof Error
        ? providersQuery.error.message
        : 'Failed to load providers.',
    );
  }
  if (riskQuery.isError) {
    queryErrors.push(
      riskQuery.error instanceof Error
        ? riskQuery.error.message
        : 'Failed to load risk limits.',
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-muted-foreground">
            Live snapshot of strategy instances, provider health, and guardrails.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild>
            <Link href="/instances">View instances</Link>
          </Button>
          <Button variant="secondary" asChild>
            <Link href="/providers">Provider console</Link>
          </Button>
        </div>
      </div>

      {queryErrors.length > 0 && (
        <Alert variant="destructive">
          <AlertDescription>
            {queryErrors.join(' ')}
          </AlertDescription>
        </Alert>
      )}

      <div className="grid gap-4 xl:grid-cols-12">
        <Card className="xl:col-span-7 h-full rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_60px_rgba(15,23,42,0.3)] backdrop-blur">
          <CardHeader>
            <CardTitle>Strategy Instances</CardTitle>
            <CardDescription>Running signals, tracked symbols, and quick status.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <p className="text-xs uppercase text-muted-foreground">Instances online</p>
                <p className="text-3xl font-semibold">{formatNumber(runningInstances)}</p>
                <p className="text-sm text-muted-foreground">
                  {stoppedInstances === 0 ? 'All instances running' : `${formatNumber(stoppedInstances)} stopped`}
                </p>
              </div>
              <div className="flex-1 min-w-[220px]">
                <StackedBarChart segments={instanceSegments} />
                <ChartLegend segments={instanceSegments} className="mt-2" />
              </div>
              <div className="text-right">
                <p className="text-xs uppercase text-muted-foreground">Tracked symbols</p>
                <p className="text-3xl font-semibold">{formatNumber(instanceSymbolSummary.size)}</p>
              </div>
            </div>
            <Separator />
            {topInstances.length === 0 ? (
              <p className="text-sm text-muted-foreground">No instances configured yet.</p>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2">
                {topInstances.map((instance) => (
                  <div key={instance.id} className="rounded-md border p-3">
                    <div className="flex items-center justify-between gap-2">
                      <p className="font-semibold text-foreground">{instance.id}</p>
                      <Badge variant={instance.running ? 'default' : 'secondary'}>
                        {instance.running ? 'Running' : 'Stopped'}
                      </Badge>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {instance.providers.length} provider{instance.providers.length === 1 ? '' : 's'} ·{' '}
                      {instance.aggregatedSymbols.length} instrument{instance.aggregatedSymbols.length === 1 ? '' : 's'}
                    </p>
                  </div>
                ))}
              </div>
            )}
            <div className="flex justify-end">
              <Button variant="ghost" asChild>
                <Link href="/instances">Open instances</Link>
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="xl:col-span-5 h-full rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_60px_rgba(15,23,42,0.3)] backdrop-blur">
          <CardHeader>
            <CardTitle>Provider Health</CardTitle>
            <CardDescription>Adapter lifecycles and dependency coverage.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <div>
                <p className="text-xs uppercase text-muted-foreground">Providers running</p>
                <p className="text-3xl font-semibold">{formatNumber(providerStatusCounts.running)}</p>
                <p className="text-sm text-muted-foreground">
                  {providerStatusCounts.failed + providerStatusCounts.stopped === 0
                    ? 'All providers healthy'
                    : `${formatNumber(providerStatusCounts.failed + providerStatusCounts.stopped)} need attention`}
                </p>
              </div>
              <div className="flex-1 min-w-[180px]">
                <StackedBarChart segments={providerSegments} />
                <ChartLegend segments={providerSegments} className="mt-2" />
              </div>
            </div>
            <Separator />
            {topProviders.length === 0 ? (
              <p className="text-sm text-muted-foreground">No providers configured.</p>
            ) : (
              <div className="space-y-3">
                {topProviders.map((provider) => (
                  <div key={provider.name} className="flex items-center justify-between text-sm">
                    <div className="space-y-0.5">
                      <p className="font-medium text-foreground">{provider.name}</p>
                      <p className="text-xs text-muted-foreground">{provider.adapter}</p>
                    </div>
                    <Badge
                      variant="outline"
                      className={cn(
                        provider.status === 'failed' && 'border-destructive/40 text-destructive',
                        provider.status === 'stopped' && 'border-amber-500/40 text-amber-600',
                        provider.status === 'running' && 'border-green-500/40 text-green-600 dark:text-green-400',
                        'uppercase'
                      )}
                    >
                      {provider.status}
                    </Badge>
                  </div>
                ))}
              </div>
            )}
            <div className="flex justify-end">
              <Button variant="ghost" asChild>
                <Link href="/providers">Manage providers</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card className="h-full rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_50px_rgba(15,23,42,0.25)] backdrop-blur">
          <CardHeader>
            <CardTitle>Latest Activity</CardTitle>
            <CardDescription>Derived alerts from providers, instances, and risk.</CardDescription>
          </CardHeader>
          <CardContent>
            {dashboardEvents.length === 0 ? (
              <div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
                All systems look healthy. Alerts will appear here when action is required.
              </div>
            ) : (
              <ScrollArea className="max-h-[280px] pr-2">
                <ul className="space-y-4">
                  {dashboardEvents.map((event) => (
                    <li key={event.id} className="flex items-start gap-3">
                      <Badge variant="outline" className={cn('text-xs', severityStyles[event.severity])}>
                        {event.severity === 'critical'
                          ? 'Critical'
                          : event.severity === 'warning'
                            ? 'Warning'
                            : 'Info'}
                      </Badge>
                      <div className="flex-1 space-y-1">
                        <p className="text-sm font-medium text-foreground">{event.title}</p>
                        <p className="text-sm text-muted-foreground">{event.description}</p>
                      </div>
                      {event.href ? (
                        <Button variant="ghost" size="sm" asChild>
                          <Link href={event.href}>Open</Link>
                        </Button>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </ScrollArea>
            )}
          </CardContent>
        </Card>

        <Card className="h-full rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_50px_rgba(15,23,42,0.25)] backdrop-blur">
          <CardHeader>
            <CardTitle>Portfolio Coverage</CardTitle>
            <CardDescription>Unique symbols versus catalog instruments.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs uppercase text-muted-foreground">Tracked symbols</p>
                <p className="text-3xl font-semibold">{formatNumber(instanceSymbolSummary.size)}</p>
              </div>
              <div className="text-right">
                <p className="text-xs uppercase text-muted-foreground">Coverage</p>
                <p className="text-3xl font-semibold">
                  {portfolioCoverage === null ? '—' : `${portfolioCoverage}%`}
                </p>
              </div>
            </div>
            <Separator />
            <div className="space-y-1 text-sm text-muted-foreground">
              <p>{formatNumber(instrumentCoverage)} instruments published by providers.</p>
              <p>{formatNumber(dependentInstanceTotal)} dependent instances declared.</p>
            </div>
            <Button variant="ghost" className="w-full" asChild>
              <Link href="/providers">Review provider balances</Link>
            </Button>
          </CardContent>
        </Card>

        <Card className="h-full rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_50px_rgba(15,23,42,0.25)] backdrop-blur">
          <CardHeader>
            <CardTitle>Risk Guardrails</CardTitle>
            <CardDescription>Configuration coverage and escrows.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {riskSegments.length > 0 ? (
              <>
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <p className="text-xs uppercase text-muted-foreground">Coverage</p>
                    <p className="text-3xl font-semibold">
                      {riskCoverage.total > 0
                        ? `${Math.round((riskCoverage.configured / riskCoverage.total) * 100)}%`
                        : '—'}
                    </p>
                  </div>
                  <div className="flex-1 min-w-[180px]">
                    <StackedBarChart segments={riskSegments} />
                  </div>
                </div>
                <ChartLegend segments={riskSegments} />
              </>
            ) : (
              <p className="text-sm text-muted-foreground">Risk limits not configured.</p>
            )}
            <div className="space-y-1 text-sm text-muted-foreground">
              <p>
                Kill switch:
                <span className="ml-2 font-semibold text-foreground">
                  {riskPresence?.killSwitchEnabled ? 'Enabled' : 'Disabled'}
                </span>
              </p>
              <p>
                Max risk breaches:{' '}
                <span className="font-semibold text-foreground">
                  {riskLimits?.maxRiskBreaches ?? '—'}
                </span>
              </p>
              {riskCoverage.missing.length > 0 ? (
                <p>Missing: {riskCoverage.missing.slice(0, 3).join(', ')}.</p>
              ) : null}
            </div>
            <Button variant="ghost" className="w-full" asChild>
              <Link href="/risk">Edit risk limits</Link>
            </Button>
          </CardContent>
        </Card>
      </div>

      <div className="space-y-4">
        <div>
          <h2 className="text-xl font-semibold tracking-tight">Navigate</h2>
          <p className="text-sm text-muted-foreground">
            Jump into the detailed consoles for deeper configuration.
          </p>
        </div>
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <Link href="/instances" className="block">
            <Card className="h-full cursor-pointer rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_40px_rgba(15,23,42,0.3)] transition-colors hover:border-primary backdrop-blur">
              <CardHeader>
                <CardTitle>Strategy Instances</CardTitle>
                <CardDescription>Manage running strategy instances</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  Create, start, stop, and configure strategy instances
                </p>
              </CardContent>
            </Card>
          </Link>

          <Link href="/strategies/modules" className="block">
            <Card className="h-full cursor-pointer rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_40px_rgba(15,23,42,0.3)] transition-colors hover:border-primary backdrop-blur">
              <CardHeader>
                <CardTitle>Strategy Modules</CardTitle>
                <CardDescription>Manage JavaScript strategy source files</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  Upload, edit, and refresh runtime strategy modules
                </p>
              </CardContent>
            </Card>
          </Link>

          <Link href="/providers" className="block">
            <Card className="h-full cursor-pointer rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_40px_rgba(15,23,42,0.3)] transition-colors hover:border-primary backdrop-blur">
              <CardHeader>
                <CardTitle>Providers</CardTitle>
                <CardDescription>Monitor exchange providers</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  View provider metadata and instrument catalogs
                </p>
              </CardContent>
            </Card>
          </Link>

          <Link href="/adapters" className="block">
            <Card className="h-full cursor-pointer rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_40px_rgba(15,23,42,0.3)] transition-colors hover:border-primary backdrop-blur">
              <CardHeader>
                <CardTitle>Adapters</CardTitle>
                <CardDescription>View exchange adapter definitions</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  Explore adapter capabilities and configuration schemas
                </p>
              </CardContent>
            </Card>
          </Link>

          <Link href="/risk" className="block">
            <Card className="h-full cursor-pointer rounded-[22px] border border-white/10 bg-background/95 shadow-[0_10px_40px_rgba(15,23,42,0.3)] transition-colors hover:border-primary backdrop-blur">
              <CardHeader>
                <CardTitle>Risk Limits</CardTitle>
                <CardDescription>Configure risk management settings</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  Adjust position limits, order throttling, and circuit breakers
                </p>
              </CardContent>
            </Card>
          </Link>
        </div>
      </div>
    </div>
  );
}
