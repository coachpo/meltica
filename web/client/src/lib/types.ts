// API Types based on docs/lambdas-api.md

export interface StrategyConfig {
  name: string;
  type: string;
  description: string;
  default?: unknown;
  required: boolean;
}

export interface Strategy {
  name: string;
  displayName: string;
  description: string;
  config: StrategyConfig[];
  events: string[];
}

export type ProviderSettings = Record<string, unknown>;

export interface Provider {
  name: string;
  adapter: string;
  identifier: string;
  instrumentCount: number;
  settings: ProviderSettings;
  running: boolean;
}

export interface Instrument {
  symbol: string;
  type?: string;
  baseAsset?: string | null;
  baseCurrency?: string | null;
  quoteAsset?: string | null;
  quoteCurrency?: string | null;
  venue?: string | null;
  expiry?: string | null;
  contractValue?: number | null;
  contractCurrency?: string | null;
  strike?: number | null;
  optionType?: string | null;
  priceIncrement?: number | string | null;
  quantityIncrement?: number | string | null;
  pricePrecision?: number | null;
  quantityPrecision?: number | null;
  notionalPrecision?: number | null;
  minNotional?: number | string | null;
  minQuantity?: number | string | null;
  maxQuantity?: number | string | null;
  [key: string]: unknown;
}

export interface SettingsSchema {
  name: string;
  type: string;
  default?: unknown;
  required: boolean;
}

export interface AdapterMetadata {
  identifier: string;
  displayName: string;
  venue: string;
  description?: string;
  capabilities: string[];
  settingsSchema: SettingsSchema[];
}

export interface ProviderDetail extends Provider {
  instruments: Instrument[];
  adapter: AdapterMetadata;
}

export interface ProviderRequest {
  name: string;
  adapter: {
    identifier: string;
    config: Record<string, unknown>;
  };
  enabled?: boolean;
}

export interface InstanceSummary {
  id: string;
  strategyIdentifier: string;
  providers: string[];
  aggregatedSymbols: string[];
  autoStart: boolean;
  running: boolean;
}

export interface ProviderSymbols {
  symbols: string[];
}

export interface InstanceSpec {
  id: string;
  strategy: {
    identifier: string;
    config: Record<string, unknown>;
  };
  scope: Record<string, ProviderSymbols>;
  providers?: string[];
  aggregatedSymbols?: string[];
  autoStart?: boolean;
  running?: boolean;
}

export interface CircuitBreakerConfig {
  enabled: boolean;
  threshold: number;
  cooldown: string;
}

export interface RiskConfig {
  maxPositionSize: string;
  maxNotionalValue: string;
  notionalCurrency: string;
  orderThrottle: number;
  orderBurst: number;
  maxConcurrentOrders: number;
  priceBandPercent: number;
  allowedOrderTypes: string[];
  killSwitchEnabled: boolean;
  maxRiskBreaches: number;
  circuitBreaker: CircuitBreakerConfig;
}

export interface ApiError {
  status: string;
  error: string;
}

export type FanoutWorkersSetting = number | 'auto' | 'default' | string;

export interface EventbusRuntimeConfig {
  bufferSize: number;
  fanoutWorkers: FanoutWorkersSetting;
}

export interface ObjectPoolRuntimeConfig {
  size: number;
  waitQueueSize: number;
}

export interface PoolRuntimeConfig {
  event: ObjectPoolRuntimeConfig;
  orderRequest: ObjectPoolRuntimeConfig;
}

export interface ApiServerConfig {
  addr: string;
}

export interface TelemetryConfig {
  otlpEndpoint: string;
  serviceName: string;
  otlpInsecure: boolean;
  enableMetrics: boolean;
}

export interface RuntimeConfig {
  eventbus: EventbusRuntimeConfig;
  pools: PoolRuntimeConfig;
  risk: RiskConfig;
  apiServer: ApiServerConfig;
  telemetry: TelemetryConfig;
}
