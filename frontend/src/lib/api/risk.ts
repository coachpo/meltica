import { z } from 'zod';
import type { RiskConfig } from '@/lib/types';
import { requestJson } from './http';
import { normalizeRiskConfig, serializeRiskLimitsPayload } from './normalizers';

const riskLimitsResponseSchema = z
  .object({
    limits: z.unknown().optional(),
    status: z.string().optional(),
  })
  .passthrough();

const riskLimitsUpdateResponseSchema = z
  .object({
    status: z.string().optional(),
    limits: z.unknown().optional(),
  })
  .passthrough();

export interface RiskLimitsResult {
  status?: string;
  limits: RiskConfig;
}

export async function fetchRiskLimits(): Promise<RiskLimitsResult> {
  const payload = await requestJson({
    path: '/risk/limits',
    schema: riskLimitsResponseSchema,
  });
  const rawLimits = payload.limits ?? payload;
  const limits = normalizeRiskConfig(rawLimits);
  return { limits };
}

export async function updateRiskLimits(config: RiskConfig): Promise<RiskLimitsResult> {
  const payload = await requestJson({
    path: '/risk/limits',
    method: 'PUT',
    body: serializeRiskLimitsPayload(config),
    schema: riskLimitsUpdateResponseSchema,
  });
  return {
    status: payload.status,
    limits: payload.limits ? normalizeRiskConfig(payload.limits) : config,
  };
}
