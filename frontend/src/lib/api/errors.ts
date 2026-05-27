import type {
  ApiErrorPayload,
  StrategyDiagnostic,
  StrategyErrorResponse,
  StrategyValidationErrorResponse,
} from '@/lib/types';

interface ApiErrorOptions {
  status?: number;
  payload?: ApiErrorPayload | StrategyErrorResponse | StrategyValidationErrorResponse | null;
  cause?: unknown;
}

export class ApiError extends Error {
  readonly status?: number;
  readonly payload: ApiErrorPayload | StrategyErrorResponse | StrategyValidationErrorResponse | null;

  constructor(message: string, options: ApiErrorOptions = {}) {
    super(message, { cause: options.cause });
    this.name = 'ApiError';
    this.status = options.status;
    this.payload = options.payload ?? null;
  }
}

export class StrategyValidationError extends ApiError {
  readonly response: StrategyValidationErrorResponse | StrategyErrorResponse | null;
  readonly diagnostics: StrategyDiagnostic[];

  constructor(
    message: string,
    options: {
      response?: StrategyValidationErrorResponse | StrategyErrorResponse | null;
      diagnostics?: StrategyDiagnostic[];
    } = {},
  ) {
    super(message, {
      status: 422,
      payload: options.response ?? null,
    });
    this.name = 'StrategyValidationError';
    this.response = options.response ?? null;
    this.diagnostics = options.diagnostics ?? this.response?.diagnostics ?? [];
  }
}

export type StrategyErrorPayload =
  | StrategyValidationErrorResponse
  | StrategyErrorResponse
  | ApiErrorPayload
  | null;

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) {
    return error;
  }
  if (error instanceof Error) {
    return new ApiError(error.message, { cause: error });
  }
  return new ApiError('Unknown API error', { cause: error });
}

const SELECTOR_NOT_REGISTERED_CODES = new Set([
  'strategy_module_not_found',
  'strategy_selector_not_found',
  'strategy_revision_not_found',
]);

const REGISTRY_UNAVAILABLE_CODES = new Set([
  'strategy_registry_unavailable',
  'strategy_registry_missing',
  'strategy_registry_unreadable',
]);

function extractErrorCode(payload: ApiError['payload']): string | undefined {
  if (!payload || typeof payload !== 'object') {
    return undefined;
  }
  const record = payload as Partial<{ error: unknown; status: unknown }>;
  const errorCode = typeof record.error === 'string' ? record.error.trim() : undefined;
  if (errorCode) {
    return errorCode;
  }
  const status = typeof record.status === 'string' ? record.status.trim() : undefined;
  return status && status.length > 0 ? status : undefined;
}

function extractMessage(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }
  if (value instanceof Error) {
    return value.message || '';
  }
  if (typeof (value as { message?: string })?.message === 'string') {
    return ((value as { message?: string }).message ?? '').toString();
  }
  return '';
}

export function isSelectorNotRegisteredError(error: unknown): error is ApiError {
  if (!isApiError(error)) {
    return false;
  }
  const code = extractErrorCode(error.payload);
  if (code && SELECTOR_NOT_REGISTERED_CODES.has(code)) {
    return true;
  }
  if (error.status === 404) {
    return true;
  }
  const message = extractMessage(error).toLowerCase();
  return message.includes('not found') || message.includes('not registered');
}

export function isRegistryUnavailableError(error: unknown): error is ApiError {
  if (!isApiError(error)) {
    return false;
  }
  const code = extractErrorCode(error.payload);
  if (code && REGISTRY_UNAVAILABLE_CODES.has(code)) {
    return true;
  }
  if (error.status === 500) {
    const message = extractMessage(error).toLowerCase();
    if (message.includes('registry')) {
      return true;
    }
  }
  return false;
}

export interface UsageConflictDetails {
  selector?: string;
  hash?: string;
  count?: number;
  instances?: string[];
}

export function extractUsageConflictDetails(error: unknown): UsageConflictDetails | null {
  if (!isApiError(error)) {
    return null;
  }
  const details = (error.payload as { details?: unknown })?.details;
  if (!details || typeof details !== 'object') {
    return null;
  }
  const record = details as Record<string, unknown>;
  const usageRaw = record.usage;
  if (!usageRaw || typeof usageRaw !== 'object') {
    return null;
  }
  const usageRecord = usageRaw as Record<string, unknown>;
  const instancesRaw = usageRecord.instances;
  const instances = Array.isArray(instancesRaw)
    ? instancesRaw.filter((entry): entry is string => typeof entry === 'string')
    : undefined;
  const countValue = usageRecord.count;
  const parsedCount =
    typeof countValue === 'number' && Number.isFinite(countValue)
      ? countValue
      : typeof countValue === 'string' && Number.isFinite(Number(countValue))
        ? Number(countValue)
        : undefined;
  const hashValue = usageRecord.hash;
  const selectorValue = record.selector;
  return {
    selector:
      typeof selectorValue === 'string' && selectorValue.trim().length > 0
        ? selectorValue.trim()
        : undefined,
    hash: typeof hashValue === 'string' && hashValue.trim().length > 0 ? hashValue : undefined,
    instances,
    count: parsedCount,
  };
}
