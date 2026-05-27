import { describe, expect, it } from 'vitest';

import {
  getSensitiveKeyFragments,
  sanitizeContextBackupPayload,
} from './context-backup';

const baseRiskConfig = {
  maxPositionSize: '250',
  maxNotionalValue: '5000',
  notionalCurrency: 'USD',
  orderThrottle: 5,
  orderBurst: 10,
  maxConcurrentOrders: 15,
  priceBandPercent: 2,
  allowedOrderTypes: ['limit', 'market'],
  killSwitchEnabled: true,
  maxRiskBreaches: 3,
  circuitBreaker: {
    enabled: true,
    threshold: 2,
    cooldown: '5m',
  },
};

describe('sanitizeContextBackupPayload', () => {
  it('removes sensitive keys recursively without mutating input', () => {
    const payload = {
      providers: [
        {
          Name: 'alpha',
          Config: {
            api_key: 'should-be-removed',
            secret: 'also-removed',
            nested: {
              token: 'remove-me',
              keep: 'ok',
            },
          },
        },
      ],
      lambdas: [
        {
          id: 'lambda-1',
          strategy: {
            config: {
              secret_token: 'strip',
              ttl: 1,
            },
          },
        },
      ],
      risk: {
        ...baseRiskConfig,
        passphrase: 'remove',
        nested: {
          apiKey: 'remove',
        },
      },
    } satisfies Record<string, unknown>;

    const clone = structuredClone(payload);
    const sanitized = sanitizeContextBackupPayload(payload);

    expect(payload).toEqual(clone);

    expect(sanitized.providers).toHaveLength(1);
    const providerConfig = sanitized.providers[0].Config as Record<string, unknown>;
    expect(providerConfig.api_key).toBeUndefined();
    expect(providerConfig.secret).toBeUndefined();
    expect((providerConfig.nested as Record<string, unknown>).token).toBeUndefined();
    expect((providerConfig.nested as Record<string, unknown>).keep).toBe('ok');

    const lambdaConfig = (sanitized.lambdas[0].strategy as Record<string, unknown>)
      .config as Record<string, unknown>;
    expect(lambdaConfig.secret_token).toBeUndefined();
    expect(lambdaConfig.ttl).toBe(1);

    expect(sanitized.risk.maxPositionSize).toBe('250');
    expect((sanitized.risk as Record<string, unknown>).passphrase).toBeUndefined();
  });

  it('requires risk object and defaults missing collections', () => {
    expect(() => sanitizeContextBackupPayload({})).toThrow(/risk/i);

    const sanitized = sanitizeContextBackupPayload({ risk: baseRiskConfig });

    expect(Array.isArray(sanitized.providers)).toBe(true);
    expect(Array.isArray(sanitized.lambdas)).toBe(true);
    expect(sanitized.providers).toHaveLength(0);
    expect(sanitized.lambdas).toHaveLength(0);
  });
});

describe('getSensitiveKeyFragments', () => {
  it('includes documented fragments', () => {
    const fragments = getSensitiveKeyFragments();
    expect(fragments).toEqual(expect.arrayContaining(['api_key', 'secret', 'token']));
  });
});
