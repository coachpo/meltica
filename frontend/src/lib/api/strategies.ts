import type {
  Strategy,
  StrategyModuleOperationResponse,
  StrategyModulePayload,
  StrategyModuleSummary,
  StrategyModuleUsageResponse,
  StrategyModulesResponse,
  StrategyRefreshRequest,
  StrategyRefreshResponse,
  StrategyRegistryExport,
  StrategyTagMutationResponse,
  StrategyTagAssignmentRequest,
} from '@/lib/types';
import {
  strategyListSchema,
  strategyModuleOperationResponseSchema,
  strategyModuleSummarySchema,
  strategyModuleUsageResponseSchema,
  strategyModulesResponseSchema,
  strategyRegistryExportSchema,
  strategySchema,
  strategyRefreshResponseSchema,
  strategyTagMutationResponseSchema,
} from './schemas';
import { requestJson, requestText } from './http';

export interface StrategyModulesFilters {
  strategy?: string;
  hash?: string;
  limit?: number;
  offset?: number;
}

export interface StrategyModuleUsageFilters {
  limit?: number;
  offset?: number;
  includeStopped?: boolean;
}

export async function fetchStrategies(): Promise<Strategy[]> {
  const data = await requestJson({
    path: '/strategies',
    schema: strategyListSchema,
  });
  return data.strategies;
}

export async function fetchStrategy(name: string): Promise<Strategy> {
  return requestJson({
    path: `/strategies/${encodeURIComponent(name)}`,
    schema: strategySchema,
  });
}

export async function fetchStrategyModules(filters?: StrategyModulesFilters): Promise<StrategyModulesResponse> {
  const searchParams = filters
    ? {
        ...(filters.strategy ? { strategy: filters.strategy } : {}),
        ...(filters.hash ? { hash: filters.hash } : {}),
        ...(filters.limit !== undefined ? { limit: filters.limit } : {}),
        ...(filters.offset !== undefined ? { offset: filters.offset } : {}),
      }
    : undefined;
  const response = await requestJson({
    path: '/strategies/modules',
    searchParams,
    schema: strategyModulesResponseSchema,
  });
  return response as StrategyModulesResponse;
}

export async function fetchStrategyModule(identifier: string): Promise<StrategyModuleSummary> {
  const summary = await requestJson({
    path: `/strategies/modules/${encodeURIComponent(identifier)}`,
    schema: strategyModuleSummarySchema,
  });
  return summary as StrategyModuleSummary;
}

export async function fetchStrategyModuleUsage(
  selector: string,
  filters?: StrategyModuleUsageFilters,
): Promise<StrategyModuleUsageResponse> {
  const searchParams = filters
    ? {
        ...(filters.limit !== undefined ? { limit: filters.limit } : {}),
        ...(filters.offset !== undefined ? { offset: filters.offset } : {}),
        ...(filters.includeStopped !== undefined ? { includeStopped: filters.includeStopped } : {}),
      }
    : undefined;
  const usage = await requestJson({
    path: `/strategies/modules/${encodeURIComponent(selector)}/usage`,
    searchParams,
    schema: strategyModuleUsageResponseSchema,
  });
  return usage as StrategyModuleUsageResponse;
}

export async function fetchStrategyModuleSource(identifier: string): Promise<string> {
  return requestText({
    path: `/strategies/modules/${encodeURIComponent(identifier)}/source`,
  });
}

export async function createStrategyModule(
  payload: StrategyModulePayload,
): Promise<StrategyModuleOperationResponse> {
  const response = await requestJson({
    path: '/strategies/modules',
    method: 'POST',
    body: payload,
    schema: strategyModuleOperationResponseSchema,
  });
  return response as StrategyModuleOperationResponse;
}

export async function updateStrategyModule(
  identifier: string,
  payload: StrategyModulePayload,
): Promise<StrategyModuleOperationResponse> {
  const response = await requestJson({
    path: `/strategies/modules/${encodeURIComponent(identifier)}`,
    method: 'PUT',
    body: payload,
    schema: strategyModuleOperationResponseSchema,
  });
  return response as StrategyModuleOperationResponse;
}

export async function deleteStrategyModule(identifier: string): Promise<void> {
  await requestJson({
    path: `/strategies/modules/${encodeURIComponent(identifier)}`,
    method: 'DELETE',
  });
}

export interface DeleteStrategyTagOptions {
	allowOrphan?: boolean;
}

export async function assignStrategyTag(
	strategy: string,
	tag: string,
	payload: StrategyTagAssignmentRequest,
): Promise<StrategyTagMutationResponse> {
	const response = await requestJson({
		path: `/strategies/modules/${encodeURIComponent(strategy)}/tags/${encodeURIComponent(tag)}`,
		method: 'PUT',
		body: payload,
		schema: strategyTagMutationResponseSchema,
	});
	return response as StrategyTagMutationResponse;
}

export async function deleteStrategyTag(
	strategy: string,
	tag: string,
	options?: DeleteStrategyTagOptions,
): Promise<StrategyTagMutationResponse> {
	const searchParams = options?.allowOrphan ? { allowOrphan: options.allowOrphan } : undefined;
	const response = await requestJson({
		path: `/strategies/modules/${encodeURIComponent(strategy)}/tags/${encodeURIComponent(tag)}`,
		method: 'DELETE',
		searchParams,
		schema: strategyTagMutationResponseSchema,
	});
	return response as StrategyTagMutationResponse;
}

export async function refreshStrategyCatalog(
  payload?: StrategyRefreshRequest,
): Promise<StrategyRefreshResponse> {
  return requestJson({
    path: '/strategies/refresh',
    method: 'POST',
    body: payload,
    schema: strategyRefreshResponseSchema,
  });
}

export async function exportStrategyRegistry(): Promise<StrategyRegistryExport> {
  return requestJson({
    path: '/strategies/registry',
    schema: strategyRegistryExportSchema,
  });
}
