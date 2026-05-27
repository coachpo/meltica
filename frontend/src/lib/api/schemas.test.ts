import { describe, expect, it } from 'vitest';
import { contextBackupSchema, strategyModuleSummarySchema } from './schemas';

const riskConfig = {
  maxPositionSize: '0',
  maxNotionalValue: '0',
  notionalCurrency: 'USD',
  orderThrottle: 0,
  orderBurst: 0,
  maxConcurrentOrders: 0,
  priceBandPercent: 0,
  allowedOrderTypes: [],
  killSwitchEnabled: false,
  maxRiskBreaches: 0,
  circuitBreaker: {
    enabled: false,
    threshold: 0,
    cooldown: '0s',
  },
};

describe('contextBackupSchema', () => {
  it('defaults missing lambdas to an empty array', () => {
    const parsed = contextBackupSchema.parse({
      providers: [],
      risk: riskConfig,
    });

    expect(parsed.lambdas).toEqual([]);
  });

  it('defaults missing providers to an empty array', () => {
    const parsed = contextBackupSchema.parse({
      risk: riskConfig,
    });

    expect(parsed.providers).toEqual([]);
  });
});

describe('strategyModuleSummarySchema', () => {
  const baseModule = {
    name: 'logging',
    file: 'logging.js',
    path: 'strategies/logging.js',
    selectedRevisionHash: 'sha256:123',
    selectedRevisionTag: '1.0.0',
    tags: ['stable'],
    size: 1,
    metadata: {
      name: 'logging',
      displayName: 'Logging',
      description: 'desc',
      config: [],
      events: [],
    },
  };

  it('requires selected revision metadata', () => {
    const parsed = strategyModuleSummarySchema.parse(baseModule);

    expect(parsed.selectedRevisionHash).toEqual('sha256:123');
    expect(parsed.selectedRevisionTag).toEqual('1.0.0');
  });

  it('rejects summaries without a tag list', () => {
    expect(() =>
      strategyModuleSummarySchema.parse({
        ...baseModule,
        tags: undefined,
      }),
    ).toThrow('Invalid input: expected array, received undefined');
  });
});
