import { z } from 'zod';
import type {
  AdapterMetadata,
  BalanceHistoryResponse,
  Provider,
  ProviderDetail,
  ProviderRequest,
} from '@/lib/types';
import {
  adapterMetadataSchema,
  adaptersResponseSchema,
  balanceHistoryResponseSchema,
  providerDetailSchema,
  providersResponseSchema,
} from './schemas';
import { requestJson } from './http';

const providerMutationResponseSchema = providerDetailSchema;

type ProviderDetailResponse = z.infer<typeof providerDetailSchema>;

function normalizeProviderDetailResponse(detail: ProviderDetailResponse): ProviderDetail {
  return {
    ...detail,
    instruments: detail.instruments ?? [],
  } as ProviderDetail;
}

export async function fetchProviders(): Promise<Provider[]> {
  const data = await requestJson({
    path: '/providers',
    schema: providersResponseSchema,
  });
  return data.providers;
}

export async function fetchProvider(name: string): Promise<ProviderDetail> {
  const detail = await requestJson({
    path: `/providers/${encodeURIComponent(name)}`,
    schema: providerDetailSchema,
  });
  return normalizeProviderDetailResponse(detail);
}

export async function createProvider(payload: ProviderRequest): Promise<ProviderDetail> {
  const detail = await requestJson({
    path: '/providers',
    method: 'POST',
    body: payload,
    schema: providerMutationResponseSchema,
  });
  return normalizeProviderDetailResponse(detail);
}

export async function updateProvider(name: string, payload: ProviderRequest): Promise<ProviderDetail> {
  const detail = await requestJson({
    path: `/providers/${encodeURIComponent(name)}`,
    method: 'PUT',
    body: payload,
    schema: providerMutationResponseSchema,
  });
  return normalizeProviderDetailResponse(detail);
}

export async function deleteProvider(name: string): Promise<void> {
  await requestJson({
    path: `/providers/${encodeURIComponent(name)}`,
    method: 'DELETE',
  });
}

export async function startProvider(name: string): Promise<ProviderDetail> {
  const detail = await requestJson({
    path: `/providers/${encodeURIComponent(name)}/start`,
    method: 'POST',
    schema: providerMutationResponseSchema,
  });
  return normalizeProviderDetailResponse(detail);
}

export async function stopProvider(name: string): Promise<ProviderDetail> {
  const detail = await requestJson({
    path: `/providers/${encodeURIComponent(name)}/stop`,
    method: 'POST',
    schema: providerMutationResponseSchema,
  });
  return normalizeProviderDetailResponse(detail);
}

export async function fetchAdapters(): Promise<AdapterMetadata[]> {
  const data = await requestJson({
    path: '/adapters',
    schema: adaptersResponseSchema,
  });
  return data.adapters;
}

export async function fetchAdapter(identifier: string): Promise<AdapterMetadata> {
  return requestJson({
    path: `/adapters/${encodeURIComponent(identifier)}`,
    schema: adapterMetadataSchema,
  });
}

export interface BalanceHistoryFilters {
  limit?: number;
  asset?: string;
}

export async function fetchProviderBalances(
  name: string,
  filters?: BalanceHistoryFilters,
): Promise<BalanceHistoryResponse> {
  const searchParams = filters
    ? {
        ...(filters.limit !== undefined ? { limit: filters.limit } : {}),
        ...(filters.asset ? { asset: filters.asset } : {}),
      }
    : undefined;
  return requestJson({
    path: `/providers/${encodeURIComponent(name)}/balances`,
    searchParams,
    schema: balanceHistoryResponseSchema,
  });
}
