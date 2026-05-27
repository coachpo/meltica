'use client';

import { useCallback } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  assignStrategyTag,
  createStrategyModule,
  deleteStrategyModule,
  deleteStrategyTag,
  exportStrategyRegistry,
  fetchStrategies,
  fetchStrategy,
  fetchStrategyModule,
  fetchStrategyModuleSource,
  fetchStrategyModules,
  fetchStrategyModuleUsage,
  refreshStrategyCatalog,
  updateStrategyModule,
  type StrategyModuleUsageFilters,
  type StrategyModulesFilters,
} from '@/lib/api/strategies';
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
} from '@/lib/types';
import { StrategyValidationError } from '@/lib/api';
import { isRegistryUnavailableError, isSelectorNotRegisteredError } from '@/lib/api/errors';
import { queryKeys } from './query-keys';
import { useApiNotifications } from './use-api-notifications';

function strategyModulesFiltersKey(filters?: StrategyModulesFilters) {
  if (!filters) {
    return undefined;
  }
  return {
    ...(filters.strategy ? { strategy: filters.strategy } : {}),
    ...(filters.hash ? { hash: filters.hash } : {}),
    ...(filters.limit !== undefined ? { limit: filters.limit } : {}),
    ...(filters.offset !== undefined ? { offset: filters.offset } : {}),
  } satisfies Record<string, unknown>;
}

function strategyModuleUsageFiltersKey(filters?: StrategyModuleUsageFilters) {
  if (!filters) {
    return undefined;
  }
  return {
    ...(filters.limit !== undefined ? { limit: filters.limit } : {}),
    ...(filters.offset !== undefined ? { offset: filters.offset } : {}),
    ...(filters.includeStopped !== undefined ? { includeStopped: filters.includeStopped } : {}),
  } satisfies Record<string, unknown>;
}

export function useStrategiesQuery(enabled = true) {
  return useQuery<Strategy[]>({
    queryKey: queryKeys.strategies(),
    queryFn: fetchStrategies,
    enabled,
    staleTime: 60_000,
  });
}

export function useStrategyQuery(name?: string, enabled = false) {
  const key = name ?? '__pending__';
  return useQuery<Strategy>({
    queryKey: queryKeys.strategy(key),
    queryFn: () => {
      if (!name) {
        throw new Error('Strategy name missing');
      }
      return fetchStrategy(name);
    },
    enabled: Boolean(name && enabled),
    staleTime: 60_000,
  });
}

export function useStrategyModulesQuery(filters?: StrategyModulesFilters, enabled = true) {
  return useQuery<StrategyModulesResponse>({
    queryKey: queryKeys.strategyModules(strategyModulesFiltersKey(filters)),
    queryFn: () => fetchStrategyModules(filters),
    enabled,
    staleTime: 15_000,
  });
}

export function useStrategyModuleQuery(identifier?: string, enabled = false) {
  const key = identifier ?? '__pending__';
  return useQuery<StrategyModuleSummary>({
    queryKey: queryKeys.strategyModule(key),
    queryFn: () => {
      if (!identifier) {
        throw new Error('Strategy module identifier missing');
      }
      return fetchStrategyModule(identifier);
    },
    enabled: Boolean(identifier && enabled),
    staleTime: 15_000,
  });
}

export function useStrategyModuleUsageQuery(
  selector?: string,
  filters?: StrategyModuleUsageFilters,
  enabled = false,
) {
  const key = selector ?? '__pending__';
  return useQuery<StrategyModuleUsageResponse>({
    queryKey: queryKeys.strategyModuleUsage(key, strategyModuleUsageFiltersKey(filters)),
    queryFn: () => {
      if (!selector) {
        throw new Error('Strategy module selector missing');
      }
      return fetchStrategyModuleUsage(selector, filters);
    },
    enabled: Boolean(selector && enabled),
  });
}

export function useStrategyModuleSourceQuery(identifier?: string, enabled = false) {
  const key = identifier ?? '__pending__';
  return useQuery<string>({
    queryKey: queryKeys.strategyModuleSource(key),
    queryFn: () => {
      if (!identifier) {
        throw new Error('Strategy module identifier missing');
      }
      return fetchStrategyModuleSource(identifier);
    },
    enabled: Boolean(identifier && enabled),
  });
}

export function useStrategyModuleSourceLoader() {
  const queryClient = useQueryClient();
  return useCallback(
    async (identifier: string) => {
      if (!identifier) {
        throw new Error('Strategy module identifier missing');
      }
      try {
        return await queryClient.fetchQuery({
          queryKey: queryKeys.strategyModuleSource(identifier),
          queryFn: () => fetchStrategyModuleSource(identifier),
        });
      } catch (error) {
        if (isSelectorNotRegisteredError(error)) {
          throw new Error(SELECTOR_NOT_REGISTERED_MESSAGE, {
            cause: error instanceof Error ? error : undefined,
          });
        }
        if (isRegistryUnavailableError(error)) {
          throw new Error(REGISTRY_UNAVAILABLE_MESSAGE, {
            cause: error instanceof Error ? error : undefined,
          });
        }
        throw error;
      }
    },
    [queryClient],
  );
}

export function useExportStrategyRegistryQuery(enabled = false) {
  return useQuery<StrategyRegistryExport>({
    queryKey: ['strategy-registry'],
    queryFn: exportStrategyRegistry,
    enabled,
  });
}

function invalidateStrategyCaches(queryClient: ReturnType<typeof useQueryClient>) {
  void queryClient.invalidateQueries({ queryKey: ['strategy-modules'] });
  void queryClient.invalidateQueries({ queryKey: queryKeys.strategies() });
}

const REGISTRY_UNAVAILABLE_MESSAGE =
  'Strategy registry is missing or unreadable. Ensure strategies/registry.json exists (even if empty) and is valid JSON before retrying.';
const SELECTOR_NOT_REGISTERED_MESSAGE =
  'Selector is not registered in the strategy registry. Register the module (via this UI or by editing strategies/registry.json) before retrying.';

function describeStrategyError(
  error: unknown,
  options: { includeSelector?: boolean } = {},
): string | null {
  if (isRegistryUnavailableError(error)) {
    return REGISTRY_UNAVAILABLE_MESSAGE;
  }
  if (options.includeSelector !== false && isSelectorNotRegisteredError(error)) {
    return SELECTOR_NOT_REGISTERED_MESSAGE;
  }
  return null;
}

export function useCreateStrategyModuleMutation() {
  const queryClient = useQueryClient();
  const { notifySuccess, notifyError } = useApiNotifications();

  return useMutation<StrategyModuleOperationResponse, unknown, StrategyModulePayload>({
    mutationFn: (payload) => createStrategyModule(payload),
    onSuccess: (response) => {
      const identifier =
        response.module?.name ?? response.module?.file ?? response.module?.hash ?? 'strategy module';
      notifySuccess({
        title: 'Module saved',
        description: `Saved ${identifier}.`,
      });
      invalidateStrategyCaches(queryClient);
    },
    onError: (error) => {
      if (error instanceof StrategyValidationError) {
        return;
      }
      notifyError({
        title: 'Module save failed',
        error,
        description: describeStrategyError(error, { includeSelector: false }) ?? undefined,
        fallbackMessage: 'Unable to save strategy module.',
      });
    },
  });
}

export function useUpdateStrategyModuleMutation() {
  const queryClient = useQueryClient();
  const { notifySuccess, notifyError } = useApiNotifications();

  return useMutation<
    StrategyModuleOperationResponse,
    unknown,
    { identifier: string; payload: StrategyModulePayload }
  >({
    mutationFn: ({ identifier, payload }) => updateStrategyModule(identifier, payload),
    onSuccess: (response) => {
      const identifier =
        response.module?.name ?? response.module?.file ?? response.module?.hash ?? 'strategy module';
      notifySuccess({
        title: 'Module updated',
        description: `Updated ${identifier}.`,
      });
      invalidateStrategyCaches(queryClient);
    },
    onError: (error) => {
      if (error instanceof StrategyValidationError) {
        return;
      }
      notifyError({
        title: 'Update failed',
        error,
        description: describeStrategyError(error) ?? undefined,
        fallbackMessage: 'Unable to update strategy module.',
      });
    },
  });
}

function isRevisionInUseError(error: unknown): boolean {
  const message =
    typeof error === 'string'
      ? error
      : error instanceof Error
        ? error.message
        : typeof (error as { message?: string })?.message === 'string'
          ? (error as { message?: string }).message ?? ''
          : '';
  return typeof message === 'string' && message.toLowerCase().includes('is in use');
}

export function useDeleteStrategyModuleMutation() {
  const queryClient = useQueryClient();
  const { notifySuccess, notifyError } = useApiNotifications();

  return useMutation<void, unknown, string>({
    mutationFn: (identifier) => deleteStrategyModule(identifier),
    onSuccess: () => {
      notifySuccess({
        title: 'Module removed',
        description: 'Removed registry entry. Run `node strategies/gc.js --write` to clean stray files.',
      });
      invalidateStrategyCaches(queryClient);
    },
    onError: (error) => {
      notifyError({
        title: 'Delete failed',
        error,
        description: describeStrategyError(error) ?? undefined,
        fallbackMessage: 'Unable to delete strategy module.',
        suppressConsole: isRevisionInUseError(error),
      });
    },
  });
}

export function useRefreshStrategiesMutation() {
  const queryClient = useQueryClient();
  const { notifySuccess, notifyError } = useApiNotifications();

  return useMutation<StrategyRefreshResponse, unknown, StrategyRefreshRequest | undefined>({
    mutationFn: (payload) => refreshStrategyCatalog(payload),
    onSuccess: (response) => {
      const refreshed = response.results?.length ?? 0;
      notifySuccess({
        title: 'Runtime refreshed',
        description: refreshed > 0 ? `Updated ${refreshed} strategies.` : 'Strategy runtime refreshed.',
      });
      invalidateStrategyCaches(queryClient);
    },
    onError: (error) => {
      notifyError({
        title: 'Refresh failed',
        error,
        description: describeStrategyError(error, { includeSelector: false }) ?? undefined,
        fallbackMessage: 'Unable to refresh strategies.',
      });
    },
  });
}

export function useAssignStrategyTagMutation() {
  const queryClient = useQueryClient();
  const { notifySuccess, notifyError } = useApiNotifications();

  return useMutation<
    StrategyTagMutationResponse,
    unknown,
    { strategy: string; tag: string; hash: string; refresh?: boolean }
  >({
    mutationFn: ({ strategy, tag, hash, refresh }) =>
      assignStrategyTag(strategy, tag, { hash, refresh }),
    onSuccess: (response) => {
      notifySuccess({
        title: 'Tag updated',
        description: `${response.tag} now points to ${response.hash?.slice(0, 12) ?? 'the selected hash'}.`,
      });
      invalidateStrategyCaches(queryClient);
    },
    onError: (error) => {
      notifyError({
        title: 'Tag update failed',
        error,
        description: describeStrategyError(error) ?? undefined,
        fallbackMessage: 'Unable to update tag.',
      });
    },
  });
}

export function useDeleteStrategyTagMutation() {
  const queryClient = useQueryClient();
  const { notifySuccess, notifyError } = useApiNotifications();

  return useMutation<
    StrategyTagMutationResponse,
    unknown,
    { strategy: string; tag: string; allowOrphan?: boolean }
  >({
    mutationFn: ({ strategy, tag, allowOrphan }) =>
      deleteStrategyTag(strategy, tag, allowOrphan ? { allowOrphan } : undefined),
    onSuccess: (response) => {
      notifySuccess({
        title: 'Tag removed',
        description: `Removed tag ${response.tag}.`,
      });
      invalidateStrategyCaches(queryClient);
    },
    onError: (error) => {
      notifyError({
        title: 'Tag removal failed',
        error,
        description: describeStrategyError(error) ?? undefined,
        fallbackMessage: 'Unable to delete tag.',
      });
    },
  });
}
