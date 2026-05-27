// Contracts are generated from frontend-api.yaml. Run `pnpm generate:api-types` after making spec changes.
import type { components } from './api-types';

type Schemas = components['schemas'];

export type StrategyConfig = Schemas['StrategyConfig'];
export type Strategy = Schemas['Strategy'];
export type StrategyModuleRevision = Schemas['StrategyModuleRevision'];
export type StrategyModuleResolution = Schemas['StrategyModuleResolution'];
export type StrategyModuleSummary = Schemas['StrategyModuleSummary'];
export type StrategyModulesResponse = Schemas['StrategyModulesResponse'];
export type StrategyModulePayload = Schemas['StrategyModulePayload'];
export type StrategyModuleOperationResponse = Schemas['StrategyModuleOperationResponse'];
export type StrategyTagAssignmentRequest = Schemas['StrategyTagAssignmentRequest'];
export type StrategyTagMutationResponse = Schemas['StrategyTagMutationResponse'];
export type StrategyDiagnostic = {
  stage?: string;
  message?: string;
  line?: number;
  column?: number;
  hint?: string;
};
export type ApiErrorPayload = Schemas['ApiErrorPayload'];
export interface StrategyErrorResponse extends ApiErrorPayload {
  status?: string;
  message?: string;
  diagnostics?: StrategyDiagnostic[];
}
export type StrategyValidationErrorResponse = StrategyErrorResponse;
export type ProviderSettings = Schemas['ProviderSettings'];
export type ProviderStatus = Schemas['ProviderStatus'];
export type Provider = Schemas['Provider'];
export type Instrument = Schemas['Instrument'];
export type AdapterMetadata = Schemas['AdapterMetadata'];
export type SettingsSchema = AdapterMetadata['settingsSchema'][number];
type ProviderDetailSchema = Schemas['ProviderDetail'];
export type ProviderDetail = Omit<ProviderDetailSchema, 'adapter'> & {
  adapter: AdapterMetadata;
};
export type ProviderRequest = Schemas['ProviderRequest'];
export type ProviderSymbols = Schemas['ProviderSymbols'];
export type LambdaStrategySpec = Schemas['LambdaStrategySpec'];
export type InstanceLinks = Schemas['InstanceLinks'];
export type ModuleRevisionUsage = Schemas['ModuleRevisionUsage'];
export type InstanceSummary = Schemas['InstanceSummary'];
export type InstanceSpec = Schemas['InstanceSpec'];
export type InstanceSnapshotResponse = Schemas['InstanceSnapshotResponse'];
export type ExecutionRecord = Schemas['ExecutionRecord'];
export type ExecutionHistoryResponse = Schemas['ExecutionHistoryResponse'];
export type OrderRecord = Schemas['OrderRecord'];
export type OrderHistoryResponse = Schemas['OrderHistoryResponse'];
export type BalanceRecord = Schemas['BalanceRecord'];
export type BalanceHistoryResponse = Schemas['BalanceHistoryResponse'];
export type RiskConfig = Schemas['RiskConfig'];
export type RuntimeConfig = {
  eventbus: Record<string, unknown>;
  pools: Record<string, unknown>;
  risk: Record<string, unknown>;
  apiServer: Record<string, unknown>;
  telemetry: Record<string, unknown>;
};
export type RuntimeConfigSource = 'runtime' | 'file' | 'bootstrap';
export type RuntimeConfigSnapshot = {
  config: RuntimeConfig;
  source: RuntimeConfigSource;
  persistedAt?: string | null;
  filePath?: string | null;
  metadata?: Record<string, unknown> | null;
};
export type ContextBackupPayload = Schemas['ContextBackupPayload'];
export type RestoreContextResponse = Schemas['RestoreContextResponse'];
export type StrategyRefreshRequest = Schemas['StrategyRefreshRequest'];
export type StrategyRefreshResult = Schemas['StrategyRefreshResult'];
export type StrategyRefreshResponse = Schemas['StrategyRefreshResponse'];
export type StrategyModuleUsageResponse = Schemas['StrategyModuleUsageResponse'];
export type StrategyRegistryExport = Schemas['StrategyRegistryExport'];
