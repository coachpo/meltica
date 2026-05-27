import type { RiskConfig } from '@/lib/types';

export type RiskPresence = {
  maxPositionSize: boolean;
  maxNotionalValue: boolean;
  notionalCurrency: boolean;
  orderThrottle: boolean;
  orderBurst: boolean;
  maxConcurrentOrders: boolean;
  priceBandPercent: boolean;
  allowedOrderTypes: boolean;
  killSwitchEnabled: boolean;
  maxRiskBreaches: boolean;
  circuitBreaker: {
    enabled: boolean;
    threshold: boolean;
    cooldown: boolean;
  };
};

export const computeRiskPresence = (config?: Partial<RiskConfig> | null): RiskPresence => {
  const source = (config ?? {}) as Partial<RiskConfig>;
  const circuit = (source.circuitBreaker ?? {}) as Partial<RiskConfig['circuitBreaker']>;

  const hasValue = <K extends keyof RiskConfig>(key: K) =>
    Object.prototype.hasOwnProperty.call(source, key) &&
    source[key] !== undefined &&
    source[key] !== null &&
    (typeof source[key] !== 'string' || (source[key] as unknown as string).trim() !== '');

  const hasCircuitValue = <K extends keyof RiskConfig['circuitBreaker']>(key: K) =>
    Object.prototype.hasOwnProperty.call(circuit, key) &&
    circuit[key] !== undefined &&
    circuit[key] !== null &&
    (typeof circuit[key] !== 'string' || (circuit[key] as unknown as string).trim() !== '');

  return {
    maxPositionSize: hasValue('maxPositionSize'),
    maxNotionalValue: hasValue('maxNotionalValue'),
    notionalCurrency: hasValue('notionalCurrency'),
    orderThrottle: hasValue('orderThrottle'),
    orderBurst: hasValue('orderBurst'),
    maxConcurrentOrders: hasValue('maxConcurrentOrders'),
    priceBandPercent: hasValue('priceBandPercent'),
    allowedOrderTypes:
      Object.prototype.hasOwnProperty.call(source, 'allowedOrderTypes') &&
      Array.isArray(source.allowedOrderTypes) &&
      source.allowedOrderTypes.length > 0,
    killSwitchEnabled: Object.prototype.hasOwnProperty.call(source, 'killSwitchEnabled'),
    maxRiskBreaches: hasValue('maxRiskBreaches'),
    circuitBreaker: {
      enabled: Object.prototype.hasOwnProperty.call(circuit, 'enabled'),
      threshold: hasCircuitValue('threshold'),
      cooldown: hasCircuitValue('cooldown'),
    },
  };
};
