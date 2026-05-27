import type {
  ExecutionHistoryResponse,
  InstanceSnapshotResponse,
  InstanceSpec,
  InstanceSummary,
  OrderHistoryResponse,
} from '@/lib/types';
import {
  executionHistoryResponseSchema,
  instanceActionResponseSchema,
  instanceSnapshotResponseSchema,
  instancesResponseSchema,
  orderHistoryResponseSchema,
  type InstanceActionResponse,
} from './schemas';
import { requestJson } from './http';

export interface InstanceListFilters {
  running?: boolean;
}

export interface OrderHistoryFilters {
  limit?: number;
  provider?: string;
  state?: string[];
}

export interface ExecutionHistoryFilters {
  limit?: number;
  provider?: string;
  orderId?: string;
}

export async function fetchInstances(): Promise<InstanceSummary[]> {
  const data = await requestJson({
    path: '/strategy/instances',
    schema: instancesResponseSchema,
  });
  return data.instances;
}

export async function fetchInstance(id: string): Promise<InstanceSnapshotResponse> {
  return requestJson({
    path: `/strategy/instances/${encodeURIComponent(id)}`,
    schema: instanceSnapshotResponseSchema,
  });
}

export async function createInstance(spec: InstanceSpec): Promise<InstanceSnapshotResponse> {
  return requestJson({
    path: '/strategy/instances',
    method: 'POST',
    body: spec,
    schema: instanceSnapshotResponseSchema,
  });
}

export async function updateInstance(id: string, spec: InstanceSpec): Promise<InstanceSnapshotResponse> {
  return requestJson({
    path: `/strategy/instances/${encodeURIComponent(id)}`,
    method: 'PUT',
    body: spec,
    schema: instanceSnapshotResponseSchema,
  });
}

export async function deleteInstance(id: string): Promise<void> {
  await requestJson({
    path: `/strategy/instances/${encodeURIComponent(id)}`,
    method: 'DELETE',
  });
}

export async function startInstance(id: string): Promise<InstanceActionResponse> {
  return requestJson({
    path: `/strategy/instances/${encodeURIComponent(id)}/start`,
    method: 'POST',
    schema: instanceActionResponseSchema,
  });
}

export async function stopInstance(id: string): Promise<InstanceActionResponse> {
  return requestJson({
    path: `/strategy/instances/${encodeURIComponent(id)}/stop`,
    method: 'POST',
    schema: instanceActionResponseSchema,
  });
}

export async function fetchInstanceOrders(
  id: string,
  filters?: OrderHistoryFilters,
): Promise<OrderHistoryResponse> {
  const searchParams = filters
    ? {
        ...(filters.limit !== undefined ? { limit: filters.limit } : {}),
        ...(filters.provider ? { provider: filters.provider } : {}),
        ...(filters.state?.length ? { state: filters.state } : {}),
      }
    : undefined;
  return requestJson({
    path: `/strategy/instances/${encodeURIComponent(id)}/orders`,
    searchParams,
    schema: orderHistoryResponseSchema,
  });
}

export async function fetchInstanceExecutions(
  id: string,
  filters?: ExecutionHistoryFilters,
): Promise<ExecutionHistoryResponse> {
  const searchParams = filters
    ? {
        ...(filters.limit !== undefined ? { limit: filters.limit } : {}),
        ...(filters.provider ? { provider: filters.provider } : {}),
        ...(filters.orderId ? { orderId: filters.orderId } : {}),
      }
    : undefined;
  return requestJson({
    path: `/strategy/instances/${encodeURIComponent(id)}/executions`,
    searchParams,
    schema: executionHistoryResponseSchema,
  });
}
