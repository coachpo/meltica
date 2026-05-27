import type { RiskConfig, RuntimeConfig, RuntimeConfigSnapshot, RuntimeConfigSource } from '@/lib/types';

const runtimeKeys: Array<keyof RuntimeConfig> = ['eventbus', 'pools', 'risk', 'apiServer', 'telemetry'];

function isRuntimeConfig(value: unknown): value is RuntimeConfig {
  if (!value || typeof value !== 'object') {
    return false;
  }
  const record = value as Record<string, unknown>;
  return runtimeKeys.every((key) => Object.prototype.hasOwnProperty.call(record, key));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function pickValue(source: Record<string, unknown>, keys: string[]): unknown {
  for (const key of keys) {
    if (Object.prototype.hasOwnProperty.call(source, key)) {
      return source[key];
    }
  }
  return undefined;
}

function toStringValue(value: unknown): string | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  const result = String(value);
  return result.trim();
}

function toNumberValue(value: unknown): number | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  const candidate = typeof value === 'number' ? value : Number(String(value).trim());
  return Number.isFinite(candidate) ? candidate : undefined;
}

function toBooleanValue(value: unknown): boolean | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  if (typeof value === 'boolean') {
    return value;
  }
  if (typeof value === 'number') {
    return value !== 0;
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (!normalized) {
      return undefined;
    }
    if (['true', '1', 'yes', 'on', 'enabled'].includes(normalized)) {
      return true;
    }
    if (['false', '0', 'no', 'off', 'disabled'].includes(normalized)) {
      return false;
    }
  }
  return undefined;
}

function toStringArray(value: unknown): string[] | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  const source = Array.isArray(value) ? value : typeof value === 'string' ? value.split(',') : null;
  if (!source) {
    return undefined;
  }
  const items = source
    .map((entry) => String(entry).trim())
    .filter((entry) => entry.length > 0);
  return items;
}

function assertPresent<T>(value: T | undefined, field: string): T {
  if (value === undefined) {
    throw new Error(`Risk configuration is missing required field: ${field}`);
  }
  return value;
}

export function normalizeRiskConfig(payload: unknown): RiskConfig {
  if (!isRecord(payload)) {
    throw new Error('Risk configuration payload must be an object');
  }
  const source = payload as Record<string, unknown>;

  const maxPositionSize = toStringValue(pickValue(source, ['maxPositionSize', 'MaxPositionSize']));
  const maxNotionalValue = toStringValue(pickValue(source, ['maxNotionalValue', 'MaxNotionalValue']));
  const notionalCurrency = toStringValue(pickValue(source, ['notionalCurrency', 'NotionalCurrency']));
  const orderThrottle = toNumberValue(pickValue(source, ['orderThrottle', 'OrderThrottle']));
  const orderBurst = toNumberValue(pickValue(source, ['orderBurst', 'OrderBurst']));
  const maxConcurrentOrders = toNumberValue(pickValue(source, ['maxConcurrentOrders', 'MaxConcurrentOrders']));
  const priceBandPercent = toNumberValue(pickValue(source, ['priceBandPercent', 'PriceBandPercent']));
  const allowedOrderTypes = toStringArray(pickValue(source, ['allowedOrderTypes', 'AllowedOrderTypes']));
  const killSwitchEnabled = toBooleanValue(pickValue(source, ['killSwitchEnabled', 'KillSwitchEnabled']));
  const maxRiskBreaches = toNumberValue(pickValue(source, ['maxRiskBreaches', 'MaxRiskBreaches']));

  const circuitSource = pickValue(source, ['circuitBreaker', 'CircuitBreaker']);
  if (!isRecord(circuitSource)) {
    throw new Error('Risk configuration circuitBreaker block is missing');
  }
  const circuitEnabled = toBooleanValue(pickValue(circuitSource, ['enabled', 'Enabled']));
  const circuitThreshold = toNumberValue(pickValue(circuitSource, ['threshold', 'Threshold']));
  const circuitCooldown = toStringValue(pickValue(circuitSource, ['cooldown', 'Cooldown']));

  return {
    maxPositionSize: assertPresent(maxPositionSize, 'maxPositionSize'),
    maxNotionalValue: assertPresent(maxNotionalValue, 'maxNotionalValue'),
    notionalCurrency: assertPresent(notionalCurrency, 'notionalCurrency'),
    orderThrottle: assertPresent(orderThrottle, 'orderThrottle'),
    orderBurst: assertPresent(orderBurst, 'orderBurst'),
    maxConcurrentOrders: assertPresent(maxConcurrentOrders, 'maxConcurrentOrders'),
    priceBandPercent: assertPresent(priceBandPercent, 'priceBandPercent'),
    allowedOrderTypes: assertPresent(allowedOrderTypes, 'allowedOrderTypes'),
    killSwitchEnabled: assertPresent(killSwitchEnabled, 'killSwitchEnabled'),
    maxRiskBreaches: assertPresent(maxRiskBreaches, 'maxRiskBreaches'),
    circuitBreaker: {
      enabled: assertPresent(circuitEnabled, 'circuitBreaker.enabled'),
      threshold: assertPresent(circuitThreshold, 'circuitBreaker.threshold'),
      cooldown: assertPresent(circuitCooldown, 'circuitBreaker.cooldown'),
    },
  } satisfies RiskConfig;
}

export function serializeRiskLimitsPayload(config: RiskConfig): Record<string, unknown> {
  return {
    MaxPositionSize: config.maxPositionSize,
    MaxNotionalValue: config.maxNotionalValue,
    NotionalCurrency: config.notionalCurrency,
    OrderThrottle: config.orderThrottle,
    OrderBurst: config.orderBurst,
    MaxConcurrentOrders: config.maxConcurrentOrders,
    PriceBandPercent: config.priceBandPercent,
    AllowedOrderTypes: config.allowedOrderTypes,
    KillSwitchEnabled: config.killSwitchEnabled,
    MaxRiskBreaches: config.maxRiskBreaches,
    CircuitBreaker: {
      Enabled: config.circuitBreaker?.enabled ?? false,
      Threshold: config.circuitBreaker?.threshold ?? 0,
      Cooldown: config.circuitBreaker?.cooldown ?? '',
    },
  };
}

export function normalizeRuntimeConfigSnapshot(payload: unknown): RuntimeConfigSnapshot {
  if (!payload) {
    throw new Error('Empty runtime configuration payload');
  }
  if (isRuntimeConfig(payload)) {
    return {
      config: payload,
      source: 'runtime',
    };
  }

  if (typeof payload !== 'object') {
    throw new Error('Malformed runtime configuration payload');
  }

  const data = payload as Record<string, unknown>;
  const configCandidate = [data.config, data.runtime].find(isRuntimeConfig);

  if (!configCandidate) {
    throw new Error('Runtime configuration missing from response');
  }

  const sourceRaw =
    typeof data.source === 'string' ? (data.source as RuntimeConfigSource) : undefined;
  const source: RuntimeConfigSource = ['runtime', 'file', 'bootstrap'].includes(String(sourceRaw))
    ? (sourceRaw as RuntimeConfigSource)
    : 'runtime';

  const persistedAt =
    typeof data.persistedAt === 'string'
      ? (data.persistedAt as string)
      : typeof data.persisted_at === 'string'
        ? (data.persisted_at as string)
        : null;

  const filePath =
    typeof data.filePath === 'string'
      ? (data.filePath as string)
      : typeof data.path === 'string'
        ? (data.path as string)
        : null;

  const metadata =
    data.metadata && typeof data.metadata === 'object'
      ? (data.metadata as Record<string, unknown>)
      : null;

  return {
    config: configCandidate,
    source,
    persistedAt,
    filePath,
    metadata,
  };
}
