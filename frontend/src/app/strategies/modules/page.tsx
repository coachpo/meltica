'use client';

import Link from 'next/link';
import { ChangeEvent, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import {
  useStrategyModulesQuery,
  useStrategyModuleUsageQuery,
  useCreateStrategyModuleMutation,
  useUpdateStrategyModuleMutation,
  useDeleteStrategyModuleMutation,
  useRefreshStrategiesMutation,
  useExportStrategyRegistryQuery,
  useStrategyModuleSourceLoader,
  useAssignStrategyTagMutation,
  useDeleteStrategyTagMutation,
} from '@/lib/hooks';
import { StrategyValidationError } from '@/lib/api';
import {
  extractUsageConflictDetails,
  isRegistryUnavailableError,
  isSelectorNotRegisteredError,
  type UsageConflictDetails,
} from '@/lib/api/errors';
import type {
  StrategyDiagnostic,
  StrategyModuleRevision,
  StrategyModuleSummary,
  StrategyRefreshRequest,
  StrategyRefreshResult,
} from '@/lib/types';
import { CodeEditor, CodeViewer } from '@/components/code';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useToast } from '@/components/ui/toast-provider';
import { ConfirmDialog } from '@/components/confirm-dialog';
import {
  Activity,
  ArrowUpCircle,
  Copy,
  Download,
  Eye,
  FileCode,
  FilePlus,
  Loader2,
  Pencil,
  RefreshCw,
  Trash2,
  UploadCloud,
  ListFilter,
  Target,
  Tag,
} from 'lucide-react';

type ModuleFormMode = 'create' | 'edit';

type RefreshOptions = {
  silent?: boolean;
  notifySuccess?: boolean;
};

type ModuleFormState = {
  source: string;
};

const STRATEGY_DOCS_URL =
  'https://github.com/coachpo/meltica/blob/dev/docs/js-strategy-runtime.md';

export const STRATEGY_MODULE_TEMPLATE = `module.exports = {
  metadata: {
    name: "strategy-name",
    tag: "v1.0.0",
    displayName: "Strategy Display Name",
    description: "Describe the strategy's behaviour and requirements.",
    config: [
      {
        name: "threshold",
        type: "number",
        description: "Example configuration field.",
        default: 0.5,
        required: false
      }
    ],
    events: ["Trade"]
  },
  create: function (env) {
    return {
      onTrade: function (ctx, event) {
        env.helpers.log("Received trade", event.payload);
      }
    };
  }
};
`;

const STRATEGY_SOURCE_EDITOR_CLASS = 'h-full font-mono text-sm';
const STRATEGY_SOURCE_EDITOR_CONTAINER_CLASS =
  'relative w-full rounded-md border h-[320px] max-h-[60vh] lg:h-[440px]';

const CLEANUP_COMMAND = 'node strategies/gc.js --write';
const REGISTRY_UNAVAILABLE_HINT =
  'Strategy registry is missing or unreadable. Restore strategies/registry.json (create an empty {} if it never existed) and retry.';
const SELECTOR_UNREGISTERED_HINT =
  'Selector is not registered in the strategy registry. Register the revision here or edit strategies/registry.json before retrying the action.';

const defaultFormState: ModuleFormState = {
  source: '',
};

const STAGE_LABELS: Record<string, string> = {
  compile: 'Compile error',
  execute: 'Runtime init error',
  validation: 'Metadata validation',
};

const STAGE_ACTIONS: Record<string, string> = {
  compile: 'Fix the JavaScript syntax at the highlighted locations.',
  execute: 'Ensure module initialisation runs without throwing errors.',
  validation: 'Provide the required metadata fields before saving again.',
};

export const stageLabel = (rawStage: string | undefined): string => {
  if (!rawStage) {
    return 'Validation error';
  }
  const normalised = rawStage.toLowerCase();
  return STAGE_LABELS[normalised] ?? 'Validation error';
};

export const stageAction = (rawStage: string | undefined): string => {
  if (!rawStage) {
    return 'Review the highlighted issues before saving again.';
  }
  const normalised = rawStage.toLowerCase();
  return STAGE_ACTIONS[normalised] ?? 'Review the highlighted issues before saving again.';
};

export function nextValidationFeedbackAfterEdit(
  diagnostics: StrategyDiagnostic[],
  error: string | null,
): { diagnostics: StrategyDiagnostic[]; error: string | null } {
  if (diagnostics.length === 0 && error === null) {
    return { diagnostics, error };
  }
  return { diagnostics: [], error: null };
}

function formatBytes(size: number): string {
  if (!Number.isFinite(size) || size <= 0) {
    return '—';
  }
  const units = ['B', 'KB', 'MB', 'GB'];
  let index = 0;
  let value = size;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value % 1 === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
}

function directoryFromPath(path: string | undefined): string | null {
  if (!path) {
    return null;
  }
  const segments = path.split(/[\\/]/).filter(Boolean);
  if (segments.length === 0) {
    return null;
  }
  segments.pop();
  return segments.join('/');
}

const PINNED_REVISION_MESSAGE =
  'Revision is pinned by running instances. Stop or redeploy them before deleting.';

type UsageDialogState = {
  selector: string;
  moduleName: string;
  hash?: string;
};

const DEFAULT_MODULE_LIMIT = 25;
const DEFAULT_FILTERS = { strategy: '', hash: '' };
const MODULE_LIMIT_OPTIONS = [10, 25, 50, 100];
const DEFAULT_USAGE_LIMIT = 25;

function formatDateTime(value?: string | null): string {
  if (!value) {
    return '—';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date);
}

function parseListInput(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function canonicalUsageSelector(moduleName: string, hash?: string | null, tag?: string | null): string {
  const trimmedName = moduleName.trim();
  if (hash && hash.trim()) {
    return `${trimmedName}@${hash.trim()}`;
  }
  if (tag && tag.trim()) {
    return `${trimmedName}:${tag.trim()}`;
  }
  return trimmedName;
}

function friendlyDeletionMessage(message: string): string {
  const lower = message.toLowerCase();
  if (lower.includes('in use') || lower.includes('pinned')) {
    return PINNED_REVISION_MESSAGE;
  }
  if (lower.includes('not registered')) {
    return SELECTOR_UNREGISTERED_HINT;
  }
  if (lower.includes('registry')) {
    return REGISTRY_UNAVAILABLE_HINT;
  }
  return message;
}

function UsageConflictNotice({ conflict }: { conflict: UsageConflictDetails | null }) {
  if (!conflict) {
    return null;
  }
  const instances = Array.isArray(conflict.instances)
    ? conflict.instances.filter((entry): entry is string => typeof entry === 'string' && entry.trim().length > 0)
    : [];
  const count =
    typeof conflict.count === 'number' && Number.isFinite(conflict.count)
      ? conflict.count
      : instances.length;
  return (
    <div className="space-y-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-xs text-muted-foreground">
      <p className="font-semibold text-destructive">Blocked by running instances</p>
      <p>
        {count > 0
          ? `${count} instance${count === 1 ? '' : 's'} still reference this revision.`
          : 'Active usage detected.'}{' '}
        {conflict.selector ? `Selector ${conflict.selector}.` : ''}
      </p>
      {conflict.hash ? (
        <p className="flex items-center gap-2">
          <span>Hash</span>
          <code className="font-mono text-[11px]">{conflict.hash}</code>
        </p>
      ) : null}
      {instances.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {instances.slice(0, 4).map((instanceId) => (
            <Badge key={instanceId} variant="outline" className="font-mono">
              {instanceId}
            </Badge>
          ))}
          {instances.length > 4 ? (
            <span className="text-muted-foreground">+{instances.length - 4} more</span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function friendlySaveError(message: string): string {
  const lower = message.toLowerCase();
  if (lower.includes('metadata') && lower.includes('name')) {
    return 'Set metadata.name inside module.exports.metadata before uploading this module.';
  }
  if (lower.includes('metadata tag')) {
    return 'Strategy tag is required. Edit module.exports.metadata.tag (for example v1.2.0) inside the source before uploading.';
  }
  if (lower.includes('tag') && lower.includes('already exists')) {
    return 'Tag already exists for this strategy. Choose a new tag or retire the conflicting revision first.';
  }
  if (lower.includes('registry') && (lower.includes('missing') || lower.includes('unavailable') || lower.includes('unreadable'))) {
    return REGISTRY_UNAVAILABLE_HINT;
  }
  if (lower.includes('not registered') || lower.includes('selector not found')) {
    return SELECTOR_UNREGISTERED_HINT;
  }
  return message;
}

function buildRevisionSelector(module: StrategyModuleSummary, revision: StrategyModuleRevision): string {
  if (revision.hash) {
    return `${module.name}@${revision.hash}`;
  }
  if (revision.tag) {
    return `${module.name}:${revision.tag}`;
  }
  return module.name;
}

function moduleIdentifier(module?: StrategyModuleSummary | null): string {
  if (!module) {
    return '';
  }
  const name = module.name?.trim();
  return name ?? '';
}

function StrategyModulesPageContent() {
  const searchParams = useSearchParams();
  const [filterDraft, setFilterDraft] = useState(() => ({ ...DEFAULT_FILTERS }));
  const [filters, setFilters] = useState(() => ({ ...DEFAULT_FILTERS }));
  const [limit, setLimit] = useState(DEFAULT_MODULE_LIMIT);
  const [offset, setOffset] = useState(0);
  const [usageDialog, setUsageDialog] = useState<UsageDialogState | null>(null);
  const [usageLimit, setUsageLimit] = useState(DEFAULT_USAGE_LIMIT);
  const [usageOffset, setUsageOffset] = useState(0);
  const [usageIncludeStopped, setUsageIncludeStopped] = useState(false);
  const [appliedUsageSelector, setAppliedUsageSelector] = useState<string | null>(null);
  const [refreshDialogOpen, setRefreshDialogOpen] = useState(false);
  const [refreshSelectorInput, setRefreshSelectorInput] = useState('');
  const [refreshHashInput, setRefreshHashInput] = useState('');
  const [refreshProcessing, setRefreshProcessing] = useState(false);
  const [refreshResults, setRefreshResults] = useState<StrategyRefreshResult[]>([]);
  const [refreshError, setRefreshError] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<ModuleFormMode>('create');
  const [formData, setFormData] = useState(defaultFormState);
  const [formError, setFormError] = useState<string | null>(null);
  const [formProcessing, setFormProcessing] = useState(false);
  const [formPrefillLoading, setFormPrefillLoading] = useState(false);
  const [formTarget, setFormTarget] = useState<StrategyModuleSummary | null>(null);
  const [formDiagnostics, setFormDiagnostics] = useState<StrategyDiagnostic[]>([]);
  const [uploadedFileInfo, setUploadedFileInfo] = useState<{ name: string; size: number } | null>(
    null,
  );
  const [detailModule, setDetailModule] = useState<StrategyModuleSummary | null>(null);
  const [sourceModule, setSourceModule] = useState<StrategyModuleSummary | null>(null);
  const [sourceContent, setSourceContent] = useState('');
  const [sourceLoading, setSourceLoading] = useState(false);
  const [sourceError, setSourceError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<StrategyModuleSummary | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteConflictUsage, setDeleteConflictUsage] = useState<UsageConflictDetails | null>(null);
  const [revisionToDelete, setRevisionToDelete] = useState<{
    module: StrategyModuleSummary;
    revision: StrategyModuleRevision;
  } | null>(null);
  const [revisionActionBusy, setRevisionActionBusy] = useState<string | null>(null);
  const [revisionDeleteError, setRevisionDeleteError] = useState<string | null>(null);
  const [revisionConflictUsage, setRevisionConflictUsage] = useState<UsageConflictDetails | null>(null);
  const [promoteTarget, setPromoteTarget] = useState<{
    module: StrategyModuleSummary;
    revision: StrategyModuleRevision;
  } | null>(null);
  const [templateConfirmOpen, setTemplateConfirmOpen] = useState(false);
  const [tagEditorState, setTagEditorState] = useState<{
    module: StrategyModuleSummary;
    revision: StrategyModuleRevision;
  } | null>(null);
  const [tagEditorValue, setTagEditorValue] = useState('');
  const [tagEditorRefresh, setTagEditorRefresh] = useState(true);
  const [tagEditorError, setTagEditorError] = useState<string | null>(null);
  const [tagDeleteTarget, setTagDeleteTarget] = useState<{
    module: StrategyModuleSummary;
    tag: string;
  } | null>(null);
  const [tagDeleteAllowOrphan, setTagDeleteAllowOrphan] = useState(false);
  const [tagDeleteError, setTagDeleteError] = useState<string | null>(null);
  const [tagDeleteConflictUsage, setTagDeleteConflictUsage] = useState<UsageConflictDetails | null>(null);
  const moduleQueryParams = useMemo(
    () => ({
      strategy: filters.strategy.trim() || undefined,
      hash: filters.hash.trim() || undefined,
      limit,
      offset,
    }),
    [filters, limit, offset],
  );
  const refreshStrategiesMutation = useRefreshStrategiesMutation();
  const exportRegistryQuery = useExportStrategyRegistryQuery(false);
  const loadModuleSource = useStrategyModuleSourceLoader();
  const createModuleMutation = useCreateStrategyModuleMutation();
  const updateModuleMutation = useUpdateStrategyModuleMutation();
  const deleteModuleMutation = useDeleteStrategyModuleMutation();
  const assignTagMutation = useAssignStrategyTagMutation();
  const deleteTagMutation = useDeleteStrategyTagMutation();
  const modulesQuery = useStrategyModulesQuery(moduleQueryParams, true);
  const { refetch: refetchModules } = modulesQuery;
  const modules = useMemo(
    () => modulesQuery.data?.modules ?? [],
    [modulesQuery.data],
  );
  const total = modulesQuery.data?.total ?? modules.length;
  const apiStrategyDirectory =
    modulesQuery.data?.strategyDirectory?.trim() ?? null;
  const loading = modulesQuery.isLoading;
  const refreshing =
    refreshStrategiesMutation.isPending ||
    (modulesQuery.isFetching && modulesQuery.isFetched);
  const error = modulesQuery.error as Error | null;
  const usageFilters = useMemo(
    () => ({
      limit: usageLimit,
      offset: usageOffset,
      includeStopped: usageIncludeStopped,
    }),
    [usageIncludeStopped, usageLimit, usageOffset],
  );
  const usageQuery = useStrategyModuleUsageQuery(
    usageDialog?.selector,
    usageFilters,
    Boolean(usageDialog),
  );
  const usageResponse = usageQuery.data ?? null;
  const usageLoading = usageQuery.isLoading;
  const usageError = usageQuery.error as Error | null;
  const exportingRegistry = exportRegistryQuery.isFetching;

  const sourceEditorReadOnly = formPrefillLoading || formProcessing;

  const detailMetadata = detailModule?.metadata;
  const detailModuleTag = detailModule?.selectedRevisionTag ?? null;
  const detailDescription =
    typeof detailMetadata?.description === 'string' && detailMetadata.description.trim().length > 0
      ? detailMetadata.description
      : null;
  const detailEvents = Array.isArray(detailMetadata?.events) ? detailMetadata.events : [];
  const detailConfig = Array.isArray(detailMetadata?.config) ? detailMetadata.config : [];
  const detailTagAliases = useMemo(() => {
    if (!detailModule?.tagAliases) {
      return [] as Array<{ alias: string; hash: string }>;
    }
    return Object.entries(detailModule.tagAliases)
      .filter(([alias, hash]) => alias.trim().length > 0 && hash.trim().length > 0)
      .map(([alias, hash]) => ({ alias: alias.trim(), hash: hash.trim() }))
      .sort((a, b) => {
        if (a.alias === 'latest') return -1;
        if (b.alias === 'latest') return 1;
        return a.alias.localeCompare(b.alias);
      });
  }, [detailModule?.tagAliases]);

  const openTagEditor = useCallback((module: StrategyModuleSummary, revision: StrategyModuleRevision) => {
    setTagEditorState({ module, revision });
    setTagEditorValue(revision.tag ?? '');
    setTagEditorRefresh(true);
    setTagEditorError(null);
  }, []);

  const closeTagEditor = useCallback(() => {
    setTagEditorState(null);
    setTagEditorValue('');
    setTagEditorError(null);
  }, []);

  const resetUsageState = useCallback(() => {
    setUsageOffset(0);
    setUsageLimit(DEFAULT_USAGE_LIMIT);
    setUsageIncludeStopped(false);
  }, []);

  const openUsageDialog = useCallback(
    (selector: string, moduleName: string, hash?: string | null) => {
      resetUsageState();
      setUsageDialog({
        selector,
        moduleName,
        hash: hash ?? undefined,
      });
    },
    [resetUsageState],
  );

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const { show: showToast } = useToast();

  const clearValidationFeedback = useCallback(() => {
    const next = nextValidationFeedbackAfterEdit(formDiagnostics, formError);
    if (next.diagnostics !== formDiagnostics) {
      setFormDiagnostics(next.diagnostics);
    }
    if (next.error !== formError) {
      setFormError(next.error);
    }
  }, [formDiagnostics, formError]);

  const emitValidationTelemetry = useCallback((diagnostics: StrategyDiagnostic[]) => {
    if (typeof window === 'undefined') {
      return;
    }
    const primaryStage = diagnostics.find((entry) => entry.stage)?.stage ?? 'unknown';
    window.dispatchEvent(
      new CustomEvent('strategy_module.validation_failure', {
        detail: {
          stage: primaryStage,
          diagnostics: diagnostics.length,
        },
      }),
    );
  }, []);

  const handleSourceChange = useCallback(
    (nextSource: string) => {
      setFormData((prev) => ({ ...prev, source: nextSource }));
      clearValidationFeedback();
      setUploadedFileInfo(null);
    },
    [clearValidationFeedback],
  );

  const applyTemplateSource = useCallback(() => {
    setFormData((prev) => ({ ...prev, source: STRATEGY_MODULE_TEMPLATE }));
    clearValidationFeedback();
    setUploadedFileInfo(null);
  }, [clearValidationFeedback]);

  const handleTemplateInsert = useCallback(() => {
    const trimmed = formData.source.trim();
    if (trimmed.length === 0) {
      applyTemplateSource();
      return;
    }
    setTemplateConfirmOpen(true);
  }, [applyTemplateSource, formData.source]);

  const sortedModules = useMemo(
    () => [...modules].sort((a, b) => a.name.localeCompare(b.name)),
    [modules],
  );

  const strategyDirectory = useMemo(() => {
    const fromApi =
      typeof apiStrategyDirectory === 'string' ? apiStrategyDirectory.trim() : '';
    if (fromApi) {
      return fromApi;
    }
    const candidate = modules.find((module) => module.path);
    return directoryFromPath(candidate?.path ?? undefined);
  }, [apiStrategyDirectory, modules]);

  const applyFilters = useCallback(() => {
    setFilters({ ...filterDraft });
    setOffset(0);
  }, [filterDraft]);

  const resetFilters = useCallback(() => {
    const next = { ...DEFAULT_FILTERS };
    setFilterDraft(next);
    setFilters(next);
    setOffset(0);
  }, []);

  const handleLimitChange = useCallback((value: string) => {
    const numeric = Number(value);
    if (!Number.isFinite(numeric) || numeric <= 0) {
      return;
    }
    setLimit(numeric);
    setOffset(0);
  }, []);

  const goToPreviousPage = useCallback(() => {
    setOffset((current) => Math.max(current - limit, 0));
  }, [limit]);

  const goToNextPage = useCallback(() => {
    setOffset((current) => {
      const next = current + limit;
      if (next >= total) {
        return current;
      }
      return next;
    });
  }, [limit, total]);

  useEffect(() => {
    const selectorParam = searchParams?.get('usage');
    if (!selectorParam) {
      return;
    }
    if (appliedUsageSelector === selectorParam) {
      return;
    }
    const moduleName = selectorParam.split(/[@:]/)[0] || selectorParam;
    openUsageDialog(selectorParam, moduleName);
    setAppliedUsageSelector(selectorParam);
  }, [appliedUsageSelector, openUsageDialog, searchParams]);

  useEffect(() => {
    setDetailModule((current) => {
      if (!current) {
        return current;
      }
      const next = modules.find((entry) => entry.name === current.name);
      if (!next) {
        return null;
      }
      if (next === current) {
        return current;
      }
      return next;
    });
  }, [modules]);

  const refreshCatalog = useCallback(
    async ({ silent = false, notifySuccess = !silent }: RefreshOptions = {}) => {
      try {
        const result = await refreshStrategiesMutation.mutateAsync(undefined);
        await refetchModules();
        if (notifySuccess) {
          showToast({
            title: 'Strategy catalog refreshed',
            description:
              result.status?.toLowerCase() === 'refreshed'
                ? 'Latest JavaScript modules loaded into the runtime.'
                : 'Strategy runtime acknowledged refresh command.',
            variant: 'info',
          });
        }
        return result;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : 'Failed to refresh strategy modules';
        if (!silent) {
          showToast({
            title: 'Refresh failed',
            description: message,
            variant: 'destructive',
          });
        }
        throw err;
      }
    },
    [refetchModules, refreshStrategiesMutation, showToast],
  );

  const handleAssignTag = useCallback(async () => {
    if (!tagEditorState) {
      return;
    }
    const trimmedTag = tagEditorValue.trim();
    if (!trimmedTag) {
      setTagEditorError('Tag name is required.');
      return;
    }
    const hash = tagEditorState.revision?.hash;
    if (!hash) {
      setTagEditorError('Revision hash unavailable.');
      return;
    }
    setTagEditorError(null);
    try {
      await assignTagMutation.mutateAsync({
        strategy: tagEditorState.module.name,
        tag: trimmedTag,
        hash,
        refresh: tagEditorRefresh,
      });
      await refetchModules();
      closeTagEditor();
    } catch (err) {
      if (isRegistryUnavailableError(err)) {
        setTagEditorError(REGISTRY_UNAVAILABLE_HINT);
      } else if (isSelectorNotRegisteredError(err)) {
        setTagEditorError(SELECTOR_UNREGISTERED_HINT);
      } else {
        setTagEditorError(err instanceof Error ? err.message : 'Failed to update tag.');
      }
    }
  }, [assignTagMutation, tagEditorRefresh, tagEditorState, tagEditorValue, refetchModules, closeTagEditor]);

  const handleDeleteTag = useCallback(async () => {
    if (!tagDeleteTarget) {
      return;
    }
    setTagDeleteError(null);
    setTagDeleteConflictUsage(null);
    try {
      await deleteTagMutation.mutateAsync({
        strategy: tagDeleteTarget.module.name,
        tag: tagDeleteTarget.tag,
        allowOrphan: tagDeleteAllowOrphan,
      });
      await refetchModules();
      setTagDeleteTarget(null);
      setTagDeleteAllowOrphan(false);
      setTagDeleteError(null);
      setTagDeleteConflictUsage(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete tag';
      setTagDeleteError(message);
      setTagDeleteConflictUsage(extractUsageConflictDetails(err));
    }
  }, [deleteTagMutation, refetchModules, tagDeleteAllowOrphan, tagDeleteTarget]);

  const handleExportRegistry = useCallback(async () => {
    try {
      const snapshotResult = await exportRegistryQuery.refetch();
      if (!snapshotResult.data) {
        throw new Error('Registry export returned empty payload');
      }
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      const blob = new Blob([JSON.stringify(snapshotResult.data, null, 2)], {
        type: 'application/json',
      });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `strategy-registry-${timestamp}.json`;
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
      URL.revokeObjectURL(url);
      showToast({
        title: 'Registry downloaded',
        description: 'Exported registry metadata with current usage counters.',
        variant: 'success',
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to download registry export';
      showToast({
        title: 'Export failed',
        description: message,
        variant: 'destructive',
      });
    }
  }, [exportRegistryQuery, showToast]);

  const closeUsageDialog = useCallback(() => {
    setUsageDialog(null);
    resetUsageState();
  }, [resetUsageState]);

  const handleUsageLimitChange = useCallback((value: string) => {
    const numeric = Number(value);
    if (!Number.isFinite(numeric) || numeric <= 0) {
      return;
    }
    setUsageLimit(numeric);
    setUsageOffset(0);
  }, []);

  const goToNextUsagePage = useCallback(() => {
    setUsageOffset((current) => {
      const next = current + usageLimit;
      if (usageResponse && next >= usageResponse.total) {
        return current;
      }
      return next;
    });
  }, [usageLimit, usageResponse]);

  const goToPreviousUsagePage = useCallback(() => {
    setUsageOffset((current) => Math.max(current - usageLimit, 0));
  }, [usageLimit]);

  const toggleUsageIncludeStopped = useCallback((checked: boolean | 'indeterminate') => {
    setUsageIncludeStopped(Boolean(checked));
    setUsageOffset(0);
  }, []);

  const resetRefreshDialogState = useCallback(() => {
    setRefreshSelectorInput('');
    setRefreshHashInput('');
    setRefreshResults([]);
    setRefreshError(null);
    setRefreshProcessing(false);
  }, []);

  const closeRefreshDialog = useCallback(() => {
    setRefreshDialogOpen(false);
    resetRefreshDialogState();
  }, [resetRefreshDialogState]);

  const submitTargetedRefresh = useCallback(async () => {
    const selectors = parseListInput(refreshSelectorInput);
    const hashes = parseListInput(refreshHashInput);
    const payload: StrategyRefreshRequest | undefined =
      selectors.length === 0 && hashes.length === 0
        ? undefined
        : {
            ...(selectors.length ? { strategies: selectors } : {}),
            ...(hashes.length ? { hashes } : {}),
          };

    setRefreshProcessing(true);
    setRefreshError(null);
    try {
      const response = await refreshStrategiesMutation.mutateAsync(payload);
      setRefreshResults(response.results ?? []);
      await refetchModules();
      showToast({
        title: 'Refresh dispatched',
        description:
          response.status?.toLowerCase() === 'refreshed'
            ? 'Runtime reloaded all strategies.'
            : 'Targeted refresh command accepted by runtime.',
        variant: 'info',
      });
    } catch (err) {
      let message: string;
      if (isSelectorNotRegisteredError(err)) {
        message = SELECTOR_UNREGISTERED_HINT;
      } else if (isRegistryUnavailableError(err)) {
        message = REGISTRY_UNAVAILABLE_HINT;
      } else {
        message = err instanceof Error ? err.message : 'Failed to refresh strategies';
      }
      setRefreshError(message);
      showToast({
        title: 'Refresh failed',
        description: message,
        variant: 'destructive',
      });
    } finally {
      setRefreshProcessing(false);
    }
  }, [refetchModules, refreshHashInput, refreshSelectorInput, refreshStrategiesMutation, showToast]);

  const openCreateDialog = () => {
    setFormMode('create');
    setFormTarget(null);
    setFormData(defaultFormState);
    clearValidationFeedback();
    setUploadedFileInfo(null);
    setFormPrefillLoading(false);
    setFormProcessing(false);
    setFormOpen(true);
  };

  const openEditDialog = async (module: StrategyModuleSummary) => {
    setFormMode('edit');
    setFormTarget(module);
    clearValidationFeedback();
    setUploadedFileInfo(null);
    setFormProcessing(false);
    setFormPrefillLoading(true);
    setFormData({ source: '' });
    setFormOpen(true);
    try {
      const identifier = moduleIdentifier(module);
      if (!identifier) {
        throw new Error('Strategy identifier unavailable for this module.');
      }
      const source = await loadModuleSource(identifier);
      setFormData({ source });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to load strategy source';
      setFormError(message);
      showToast({
        title: 'Load failed',
        description: message,
        variant: 'destructive',
      });
    } finally {
      setFormPrefillLoading(false);
    }
  };

  const handleFilePickerClick = () => {
    fileInputRef.current?.click();
  };

  const handleFileSelected = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    try {
      const text = await file.text();
      setFormData({ source: text });
      clearValidationFeedback();
      setUploadedFileInfo({ name: file.name, size: file.size });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to read selected file';
      setFormError(message);
      setUploadedFileInfo(null);
    } finally {
      // reset input so same file can be reselected
      event.target.value = '';
    }
  };

  const validateForm = () => {
    if (!formData.source || formData.source.trim().length === 0) {
      setFormError('Strategy source code cannot be empty. Include module.exports.metadata before uploading.');
      return false;
    }
    setFormError(null);
    return true;
  };

  const handleFormSubmit = async () => {
    if (!validateForm()) {
      return;
    }
    const payload = {
      source: formData.source,
    };
    setFormProcessing(true);
    try {
      if (formMode === 'create') {
        await createModuleMutation.mutateAsync(payload);
      } else if (formTarget) {
        const targetIdentifier = moduleIdentifier(formTarget);
        if (!targetIdentifier) {
          throw new Error('Strategy identifier unavailable for this module.');
        }
        await updateModuleMutation.mutateAsync({ identifier: targetIdentifier, payload });
      }
      await refreshCatalog({ silent: true, notifySuccess: false });
      showToast({
        title: 'Runtime refreshed',
        description: 'JavaScript strategy catalog now reflects the latest source. Assign tags via Manage tags when you need to update aliases or latest pointers.',
        variant: 'info',
      });
      setFormOpen(false);
      setFormTarget(null);
      setFormData(defaultFormState);
      setFormDiagnostics([]);
      setUploadedFileInfo(null);
      setFormError(null);
    } catch (err) {
      if (err instanceof StrategyValidationError) {
        const diagnostics = Array.isArray(err.diagnostics) ? err.diagnostics : [];
        setFormDiagnostics(diagnostics);
        setFormError(err.message || 'Strategy module validation failed');
        if (diagnostics.length > 0) {
          emitValidationTelemetry(diagnostics);
        }
      } else {
        const message = friendlySaveError(
          err instanceof Error ? err.message : 'Failed to save strategy module',
        );
        setFormDiagnostics([]);
        setFormError(message);
        showToast({
          title: 'Save failed',
          description: message,
          variant: 'destructive',
        });
      }
    } finally {
      setFormProcessing(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    const moduleName = deleteTarget.name;
    const identifier = moduleIdentifier(deleteTarget);
    if (!identifier) {
      setDeleteError('Strategy identifier unavailable for this module.');
      return;
    }
    setDeleting(true);
    setDeleteError(null);
    setDeleteConflictUsage(null);
    try {
      await deleteModuleMutation.mutateAsync(identifier);
      await refreshCatalog({ silent: true, notifySuccess: false });
      setDetailModule((current) => (current?.name === moduleName ? null : current));
      setDeleteTarget(null);
      setDeleteError(null);
      setDeleteConflictUsage(null);
    } catch (err) {
      const messageRaw =
        err instanceof Error ? err.message : 'Failed to delete strategy module';
      const message = friendlyDeletionMessage(messageRaw);
      setDeleteError(message);
      setDeleteConflictUsage(extractUsageConflictDetails(err));
      // Mutation hook already surfaced toast; surface inline message too.
    } finally {
      setDeleting(false);
    }
  };

  const openSourceDialog = async (module: StrategyModuleSummary) => {
    setSourceModule(module);
    setSourceContent('');
    setSourceError(null);
    setSourceLoading(true);
    try {
      const identifier = moduleIdentifier(module);
      if (!identifier) {
        throw new Error('Strategy identifier unavailable for this module.');
      }
      const source = await loadModuleSource(identifier);
      setSourceContent(source);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to load strategy source';
      setSourceError(message);
      showToast({
        title: 'Load failed',
        description: message,
        variant: 'destructive',
      });
    } finally {
      setSourceLoading(false);
    }
  };

  const copyHash = async (hash: string, label = 'Hash') => {
    try {
      if (typeof navigator === 'undefined' || !navigator.clipboard) {
        throw new Error('Clipboard API unavailable in this environment');
      }
      await navigator.clipboard.writeText(hash);
      showToast({
        title: `${label} copied`,
        description: `${label} copied to clipboard.`,
        variant: 'success',
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to copy hash';
      showToast({
        title: 'Copy failed',
        description: message,
        variant: 'destructive',
      });
    }
  };

  const copyCleanupCommand = async () => {
    try {
      if (typeof navigator === 'undefined' || !navigator.clipboard) {
        throw new Error('Clipboard API unavailable in this environment');
      }
      await navigator.clipboard.writeText(CLEANUP_COMMAND);
      showToast({
        title: 'Cleanup command copied',
        description: 'Run it on the gateway host after deleting registry entries.',
        variant: 'success',
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to copy command';
      showToast({
        title: 'Copy failed',
        description: message,
        variant: 'destructive',
      });
    }
  };

  const copySource = async () => {
    if (!sourceContent) {
      return;
    }
    try {
      if (typeof navigator === 'undefined' || !navigator.clipboard) {
        throw new Error('Clipboard API unavailable in this environment');
      }
      await navigator.clipboard.writeText(sourceContent);
      showToast({
        title: 'Source copied',
        description: 'Strategy JavaScript copied to clipboard.',
        variant: 'success',
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to copy strategy source';
      showToast({
        title: 'Copy failed',
        description: message,
        variant: 'destructive',
      });
    }
  };

  const revisionKey = (
    module: StrategyModuleSummary,
    revision: StrategyModuleRevision,
    action: string,
  ) => `${module.name}:${revision.hash || revision.tag || 'latest'}:${action}`;

  const revisionLabel = (revision: StrategyModuleRevision) => {
    const tag = revision.tag?.trim();
    if (tag) {
      return tag;
    }
    if (revision.hash) {
      return `${revision.hash.slice(0, 12)}…`;
    }
    return 'revision';
  };

  const handlePromoteRevision = async (
    module: StrategyModuleSummary,
    revision: StrategyModuleRevision,
  ) => {
    const key = revisionKey(module, revision, 'promote');
    const hash = revision.hash;
    if (!hash) {
      showToast({
        title: 'Promotion unavailable',
        description: 'Revision hash missing. Select a hashed revision before promoting.',
        variant: 'destructive',
      });
      return;
    }
    setRevisionActionBusy(key);
    try {
      await assignTagMutation.mutateAsync({
        strategy: module.name,
        tag: 'latest',
        hash,
        refresh: true,
      });
      await refetchModules();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to promote revision';
      showToast({
        title: 'Promotion failed',
        description: message,
        variant: 'destructive',
      });
    } finally {
      setRevisionActionBusy(null);
      setPromoteTarget(null);
    }
  };

  const handleDeleteRevision = async (
    module: StrategyModuleSummary,
    revision: StrategyModuleRevision,
  ) => {
    const key = revisionKey(module, revision, 'delete');
    setRevisionActionBusy(key);
    setRevisionDeleteError(null);
    setRevisionConflictUsage(null);
    try {
      const selector = buildRevisionSelector(module, revision);
      await deleteModuleMutation.mutateAsync(selector);
      await refreshCatalog({ silent: true, notifySuccess: false });
    } catch (err) {
      const messageRaw =
        err instanceof Error ? err.message : 'Failed to delete revision';
      const message = friendlyDeletionMessage(messageRaw);
      setRevisionDeleteError(message);
      setRevisionConflictUsage(extractUsageConflictDetails(err));
      showToast({
        title: 'Deletion failed',
        description: message,
        variant: 'destructive',
      });
    } finally {
      setRevisionActionBusy(null);
      setRevisionToDelete(null);
    }
  };

  const moduleCount = modules.length;
  const usageSummary = usageResponse?.usage ?? null;
  const usageInstances = usageResponse?.instances ?? [];
  const usageTotal = usageResponse?.total ?? 0;
  const usageOffsetResolved = usageResponse?.offset ?? usageOffset;
  const usageLimitResolved = usageResponse?.limit ?? usageLimit;
  const usagePageCount = usageLimitResolved > 0 ? Math.max(1, Math.ceil(usageTotal / usageLimitResolved)) : 1;
  const usageCurrentPage = usageLimitResolved > 0 ? Math.floor(usageOffsetResolved / usageLimitResolved) + 1 : 1;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-2">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Strategy Modules</h1>
            <p className="text-muted-foreground">
              Register, edit, and refresh JavaScript trading strategies; only selectors recorded in{' '}
              <code className="mx-1 font-mono text-xs">strategies/registry.json</code>{' '}
              are visible to the runtime.
            </p>
          </div>
          <div className="space-y-3">
            <Alert variant="info" className="max-w-4xl">
              <AlertTitle className="flex items-center gap-2 text-sm font-semibold">
                Revision pointers
              </AlertTitle>
              <AlertDescription className="space-y-2 text-xs sm:text-sm">
                <p>
                  <span className="font-medium">Latest hash</span> is the revision operators reach when they
                  reference only the strategy name. Promote tags to change what <code>name</code> resolves to.
                </p>
                <p>
                  <span className="font-medium">Pinned hash</span> is the digest currently recorded for running
                  instances. Instances stay on their pinned hash until they are refreshed or redeployed.
                </p>
              </AlertDescription>
            </Alert>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button onClick={openCreateDialog} variant="default">
            <UploadCloud className="mr-2 h-4 w-4" />
            Register module
          </Button>
          <Button
            onClick={() => void refreshCatalog()}
            variant="outline"
            disabled={refreshing}
          >
            {refreshing ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="mr-2 h-4 w-4" />
            )}
            Refresh catalog
          </Button>
          <Button
            variant="outline"
            onClick={() => setRefreshDialogOpen(true)}
            disabled={refreshProcessing}
          >
            {refreshProcessing ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Target className="mr-2 h-4 w-4" />
            )}
            Targeted refresh
          </Button>
          <Button
            variant="outline"
            onClick={() => void handleExportRegistry()}
            disabled={exportingRegistry}
          >
            {exportingRegistry ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Download className="mr-2 h-4 w-4" />
            )}
            Download registry
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <CardTitle className="flex items-center gap-2 text-base font-semibold">
                <ListFilter className="h-4 w-4" />
                Filters
              </CardTitle>
              <CardDescription className="text-sm">
                Narrow the module catalogue by strategy name, exact hash, or active usage state.
              </CardDescription>
            </div>
            <div className="text-xs text-muted-foreground lg:text-sm">
              Showing{' '}
              <span className="font-medium">
                {total === 0 ? 0 : Math.min(total, offset + modules.length)}
              </span>{' '}
              of <span className="font-medium">{total}</span>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="filter-strategy">Strategy name</Label>
              <Input
                id="filter-strategy"
                value={filterDraft.strategy}
                onChange={(event) =>
                  setFilterDraft((prev) => ({ ...prev, strategy: event.target.value }))
                }
                placeholder="grid"
                autoComplete="off"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="filter-hash">Hash</Label>
              <Input
                id="filter-hash"
                value={filterDraft.hash}
                onChange={(event) =>
                  setFilterDraft((prev) => ({ ...prev, hash: event.target.value }))
                }
                placeholder="sha256:..."
                autoComplete="off"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="module-page-size">Page size</Label>
              <Select value={String(limit)} onValueChange={handleLimitChange}>
                <SelectTrigger id="module-page-size">
                  <SelectValue placeholder={`${DEFAULT_MODULE_LIMIT}`} />
                </SelectTrigger>
                <SelectContent>
                  {MODULE_LIMIT_OPTIONS.map((option) => (
                    <SelectItem key={option} value={String(option)}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-4">
            <div className="flex flex-1 justify-end gap-2">
              <Button variant="ghost" onClick={resetFilters} disabled={loading && modules.length === 0}>
                Reset
              </Button>
              <Button onClick={applyFilters} disabled={loading && modules.length === 0}>
                Apply filters
              </Button>
            </div>
          </div>
          <div className="flex flex-col gap-4 border-t pt-4 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
            <div>
              Page{' '}
              <span className="font-medium">
                {total === 0 ? 0 : Math.floor(offset / limit) + 1}
              </span>{' '}
              / <span className="font-medium">{total === 0 ? 1 : Math.ceil(total / limit)}</span>
            </div>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={goToPreviousPage}
                disabled={offset === 0 || loading}
              >
                Previous
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={goToNextPage}
                disabled={offset + limit >= total || loading || modules.length === 0}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {strategyDirectory ? (
        <Alert variant="info">
          <AlertTitle>Strategy directory</AlertTitle>
          <AlertDescription className="mt-1 space-y-2 text-xs sm:text-sm">
            <p>
              Sources live under{' '}
              <span className="inline-flex items-center whitespace-nowrap font-mono">{strategyDirectory}</span>. The
              registry is updated first, then files are written to this directory.
            </p>
            <p>
              Removing a registry entry does not delete files. Run{' '}
              <code className="mx-1 font-mono text-xs">{CLEANUP_COMMAND}</code> on the gateway to prune stray
              directories.{' '}
              <Button
                type="button"
                variant="link"
                size="sm"
                className="h-auto px-0"
                onClick={() => void copyCleanupCommand()}
              >
                Copy cleanup command
              </Button>
            </p>
          </AlertDescription>
        </Alert>
      ) : null}

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>Unable to load strategy modules</AlertTitle>
          <AlertDescription>{error.message}</AlertDescription>
        </Alert>
      ) : null}

      {loading ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            Loading strategy modules…
          </CardContent>
        </Card>
      ) : moduleCount === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>
              {total === 0 ? 'No registered strategies detected' : 'No modules matched your filters'}
            </CardTitle>
            <CardDescription>
              {total === 0
                ? 'Register a JavaScript module to seed strategies/registry.json so the runtime can load code.'
                : 'Adjust your filters to view available modules or clear them to see the entire catalogue.'}
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap items-center gap-2">
            <Button onClick={openCreateDialog}>
              <UploadCloud className="mr-2 h-4 w-4" />
              Register module
            </Button>
            {total !== 0 ? (
              <Button variant="ghost" onClick={resetFilters}>
                Reset filters
              </Button>
            ) : null}
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <CardTitle>Loaded modules</CardTitle>
              <CardDescription>
                {moduleCount} registered module{moduleCount === 1 ? '' : 's'} currently loaded from the manifest.
              </CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <ScrollArea
              className="-mx-6"
              aria-label="Strategy modules table"
              allowXScroll
              allowYScroll={false}
            >
              <div className="min-w-[1450px] px-6 [&_[data-slot=table-container]]:overflow-visible">
                <Table containerClassName="overflow-visible">
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Display Name</TableHead>
                  <TableHead>Tags</TableHead>
                  <TableHead>Latest hash</TableHead>
                  <TableHead>Size</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedModules.map((module) => {
                  const moduleTags = module.tags
                    .map((entry) => entry.trim())
                    .filter((entry) => entry.length > 0);
                  return (
                    <TableRow key={module.name}>
                    <TableCell>
                      <span className="font-mono text-xs sm:text-sm">{module.name}</span>
                    </TableCell>
                    <TableCell>{module.metadata.displayName || '—'}</TableCell>
                    <TableCell>
                      {moduleTags.length > 0 ? (
                        <div className="flex flex-wrap gap-2">
                          {moduleTags.map((tag) => (
                            <Badge key={tag} variant="outline">
                              {tag}
                            </Badge>
                          ))}
                        </div>
                      ) : (
                        <span className="text-xs text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="space-y-2 text-xs">
                        <div>
                          <div className="flex items-center gap-2">
                            <Badge variant="outline">Hash</Badge>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              className="h-auto px-0 text-muted-foreground hover:text-foreground"
                              onClick={() => copyHash(module.selectedRevisionHash)}
                            >
                              <span className="font-mono">{module.selectedRevisionHash.slice(0, 12)}…</span>
                              <Copy className="h-3 w-3" />
                            </Button>
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>{formatBytes(module.size)}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() =>
                            openUsageDialog(
                              canonicalUsageSelector(module.name, module.selectedRevisionHash, module.selectedRevisionTag),
                              module.name,
                              module.selectedRevisionHash,
                            )
                          }
                          title="View usage"
                        >
                          <Activity className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setDetailModule(module)}
                          title="View metadata"
                        >
                          <Eye className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => void openSourceDialog(module)}
                          title="View source"
                        >
                          <FileCode className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => void openEditDialog(module)}
                          title="Edit source"
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setDeleteTarget(module)}
                          title="Delete module"
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                  );
                })}
              </TableBody>
            </Table>
              </div>
            </ScrollArea>
          </CardContent>
        </Card>
      )}

      <Sheet
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open);
          if (!open) {
            setFormTarget(null);
            clearValidationFeedback();
            setFormProcessing(false);
            setFormPrefillLoading(false);
            setFormData(defaultFormState);
            setUploadedFileInfo(null);
          }
        }}
      >
        <SheetContent side="right" className="flex h-full flex-col sm:max-w-[80rem]">
          <SheetHeader>
            <SheetTitle>
              {formMode === 'create' ? 'Register strategy module' : `Edit ${formTarget?.name ?? ''}`}
            </SheetTitle>
            <SheetDescription>
              {formMode === 'create'
                ? 'Upload the raw JavaScript source. module.exports.metadata supplies the name, tag, and description; tag aliases are updated after the save.'
                : 'Update the persisted JavaScript source. Edit module.exports.metadata in the file and adjust tag aliases separately once the upload succeeds.'}
            </SheetDescription>
          </SheetHeader>
          <ScrollArea
            className="flex-1 min-h-0 pr-1"
            allowYScroll
            allowXScroll={false}
          >
            <div className="grid gap-6 py-2 lg:grid-cols-[minmax(0,22rem)_minmax(0,1fr)]">
              <div className="space-y-4">
                <div className="rounded-md border bg-muted/40 p-4 text-sm leading-relaxed text-muted-foreground">
                  <p className="mb-1 font-medium text-foreground">Source is the contract</p>
                  <p>
                    Populate <code>module.exports.metadata</code>—including{' '}
                    <code>name</code>, <code>tag</code>, <code>displayName</code>,{' '}
                    <code>description</code>, config, and events—directly inside the JavaScript file.
                    The upload API now ignores external fields and relies entirely on the compiled metadata.
                  </p>
                </div>
                <div className="rounded-md border bg-muted/40 p-4 text-sm leading-relaxed text-muted-foreground">
                  <p className="mb-1 font-medium text-foreground">Manage tags separately</p>
                  <p>
                    Reassign aliases or promote <code>latest</code> after saving via the{' '}
                    <span className="font-semibold">Manage tags</span> actions or the{' '}
                    <code>/strategies/modules/&lt;name&gt;/tags/&lt;tag&gt;</code> endpoints. Uploading
                    no longer performs tag changes.
                  </p>
                </div>
                <div className="rounded-md border bg-muted/40 p-4 text-sm leading-relaxed text-muted-foreground">
                  <p className="mb-1 font-medium text-foreground">Helpful tips</p>
                  <ul className="list-disc space-y-1 pl-5">
                    <li>Insert the template for a valid metadata skeleton (now with <code>tag</code>).</li>
                    <li>Load an existing .js/.mjs file to keep editing inline.</li>
                    <li>Press <kbd className="font-semibold">Mod</kbd> + <kbd className="font-semibold">Enter</kbd> to save without leaving the editor.</li>
                  </ul>
                </div>
              </div>
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <Label htmlFor="strategy-source">Source</Label>
                  <p className="text-xs text-muted-foreground">
                    Paste or load the JavaScript module to compile. Ensure <code>module.exports.metadata</code> includes{' '}
                    <code>name</code>, <code>tag</code>, <span className="font-medium">displayName</span>, at least one{' '}
                    <code>events</code> entry, and any required configuration fields.
                    {' '}
                    <Link
                      href={STRATEGY_DOCS_URL}
                      target="_blank"
                      rel="noreferrer"
                      className="underline-offset-4 hover:underline"
                    >
                      View docs
                    </Link>
                    .
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleTemplateInsert}
                    disabled={formProcessing || formPrefillLoading}
                  >
                    <FilePlus className="mr-2 h-4 w-4" />
                    Insert template
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleFilePickerClick}
                    disabled={formProcessing || formPrefillLoading}
                  >
                    <UploadCloud className="mr-2 h-4 w-4" />
                    Load from file
                  </Button>
                </div>
              </div>
              {uploadedFileInfo ? (
                <p className="text-xs text-muted-foreground">
                  Loaded file:{' '}
                  <span className="font-medium">{uploadedFileInfo.name}</span> ·{' '}
                  {formatBytes(uploadedFileInfo.size)}
                </p>
              ) : null}
              <CodeEditor
                data-slot="module-source-editor"
                value={formData.source}
                onChange={handleSourceChange}
                language="javascript"
                allowHorizontalScroll={!sourceEditorReadOnly}
                wrapLines={sourceEditorReadOnly}
                height="100%"
                readOnly={sourceEditorReadOnly}
                onSubmitShortcut={() => void handleFormSubmit()}
                className={STRATEGY_SOURCE_EDITOR_CONTAINER_CLASS}
                editorClassName={STRATEGY_SOURCE_EDITOR_CLASS}
                aria-label="Strategy JavaScript source"
              />
              <input
                type="file"
                accept=".js,.mjs,application/javascript"
                ref={fileInputRef}
                className="hidden"
                onChange={handleFileSelected}
              />
              {formPrefillLoading ? (
                <span className="flex items-center text-xs text-muted-foreground">
                  <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                  Loading current source…
                </span>
              ) : null}
            </div>
          </div>
          {formDiagnostics.length > 0 ? (
            <Alert variant="destructive">
              <AlertTitle>Resolve validation issues</AlertTitle>
              <AlertDescription className="space-y-2 text-sm">
                {formError ? <p>{formError}</p> : null}
                <ul className="space-y-1">
                  {formDiagnostics.map((diagnostic, index) => {
                    const location =
                      typeof diagnostic.line === 'number' && diagnostic.line > 0
                        ? ` (line ${diagnostic.line}${
                            typeof diagnostic.column === 'number' && diagnostic.column > 0
                              ? `, column ${diagnostic.column}`
                              : ''
                          })`
                        : '';
                    const hint = diagnostic.hint ? ` — ${diagnostic.hint}` : '';
                    return (
                      <li key={`${diagnostic.stage}-${diagnostic.message}-${index}`}>
                        <span className="font-medium">{stageLabel(diagnostic.stage)}</span>: {diagnostic.message}
                        {location}
                        {hint}
                      </li>
                    );
                  })}
                </ul>
                <p className="text-xs text-muted-foreground">
                  {stageAction(formDiagnostics[0]?.stage)}
                </p>
              </AlertDescription>
            </Alert>
          ) : formError ? (
            <Alert variant="destructive">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          ) : null}
          </ScrollArea>
          <SheetFooter className="gap-2 sm:gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => setFormOpen(false)}
              disabled={formProcessing}
            >
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => void handleFormSubmit()}
              disabled={formProcessing || formPrefillLoading}
            >
              {formProcessing ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Saving…
                </>
              ) : formMode === 'create' ? (
                'Register & refresh'
              ) : (
                'Update & refresh'
              )}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Dialog
        open={Boolean(usageDialog)}
        onOpenChange={(open) => {
          if (!open) {
            closeUsageDialog();
          }
        }}
      >
        <DialogContent className="flex flex-col gap-4 overflow-hidden max-w-[min(85vw,64rem)]">
          <DialogHeader className="space-y-1">
            <DialogTitle>
              Revision usage{usageDialog ? ` · ${usageDialog.moduleName}` : ''}
            </DialogTitle>
            <DialogDescription>
              Inspect running instances referencing{' '}
              <code className="mx-1 font-mono text-xs">
                {usageDialog?.selector ?? ''}
              </code>
              and review first/last activity timestamps. Selectors must exist in the registry—unregistered names now
              return errors.
            </DialogDescription>
          </DialogHeader>
          {usageError ? (
            <Alert variant="destructive">
              <AlertTitle>Error loading usage data</AlertTitle>
              <AlertDescription>{usageError.message}</AlertDescription>
            </Alert>
          ) : null}
          {usageLoading ? (
            <div className="flex flex-1 items-center justify-center text-muted-foreground">
              <Loader2 className="mr-2 h-5 w-5 animate-spin" />
              Loading revision usage…
            </div>
          ) : usageResponse ? (
            <div className="flex flex-1 flex-col gap-4 overflow-hidden">
              <div className="grid gap-3 sm:grid-cols-3">
                <div className="rounded-md border p-3">
                  <p className="text-xs uppercase text-muted-foreground">Active instances</p>
                  <p className="text-2xl font-semibold">{usageSummary?.count ?? 0}</p>
                </div>
                <div className="rounded-md border p-3">
                  <p className="text-xs uppercase text-muted-foreground">First seen</p>
                  <p className="text-sm font-medium">{formatDateTime(usageSummary?.firstSeen)}</p>
                </div>
                <div className="rounded-md border p-3">
                  <p className="text-xs uppercase text-muted-foreground">Last seen</p>
                  <p className="text-sm font-medium">{formatDateTime(usageSummary?.lastSeen)}</p>
                </div>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground">
                <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3">
                  <span>
                    Hash:{' '}
                    <Button
                      type="button"
                      variant="link"
                      size="sm"
                      className="h-auto px-0 text-foreground"
                      onClick={() => usageSummary?.hash && copyHash(usageSummary.hash, 'Revision hash')}
                    >
                      <span className="font-mono">
                        {usageSummary?.hash ? usageSummary.hash.slice(0, 18) : '—'}
                      </span>
                      <Copy className="h-3 w-3" />
                    </Button>
                  </span>
                  <span>Selector: <span className="font-mono">{usageResponse.selector}</span></span>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="usage-include-stopped"
                    checked={usageIncludeStopped}
                    onCheckedChange={(nextChecked) =>
                      toggleUsageIncludeStopped(nextChecked === true)
                    }
                  />
                    <Label htmlFor="usage-include-stopped" className="text-xs">
                      Include stopped instances
                    </Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <span>Page size</span>
                    <Select value={String(usageLimit)} onValueChange={handleUsageLimitChange}>
                      <SelectTrigger className="h-8 w-[5rem]">
                        <SelectValue placeholder={`${DEFAULT_USAGE_LIMIT}`} />
                      </SelectTrigger>
                      <SelectContent>
                        {MODULE_LIMIT_OPTIONS.map((option) => (
                          <SelectItem key={option} value={String(option)}>
                            {option}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </div>
              <div className="flex-1 overflow-hidden rounded-md border">
                <ScrollArea className="h-full min-h-0" allowYScroll allowXScroll={false}>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Instance</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Hash</TableHead>
                        <TableHead>Providers</TableHead>
                        <TableHead>Last seen</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {usageInstances.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={5} className="text-center text-sm text-muted-foreground">
                            No instances matched this selector.
                          </TableCell>
                        </TableRow>
                      ) : (
                        usageInstances.map((instance) => (
                          <TableRow key={instance.id}>
                            <TableCell>
                              <div className="flex flex-col gap-1">
                                <div className="flex items-center gap-2">
                                  <span className="font-mono text-xs sm:text-sm">{instance.id}</span>
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-7 w-7"
                                    onClick={() => copyHash(instance.id, 'Instance id')}
                                    title="Copy instance id"
                                  >
                                    <Copy className="h-3 w-3" />
                                  </Button>
                                </div>
                                {instance.links?.self ? (
                                  <span className="text-[11px] text-muted-foreground">
                                    API: {instance.links.self}
                                  </span>
                                ) : null}
                              </div>
                            </TableCell>
                            <TableCell>
                              <Badge variant={instance.running ? 'success' : 'muted'}>
                                {instance.running ? 'Running' : 'Stopped'}
                              </Badge>
                            </TableCell>
                            <TableCell>
                                {instance.strategyHash ? (
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-auto px-0 text-xs text-muted-foreground hover:text-foreground"
                                    onClick={() => copyHash(instance.strategyHash ?? '', 'Instance hash')}
                                  >
                                    <span className="font-mono">
                                      {instance.strategyHash.slice(0, 12)}…
                                    </span>
                                    <Copy className="h-3 w-3" />
                                  </Button>
                                ) : (
                                <span className="text-xs text-muted-foreground">—</span>
                              )}
                            </TableCell>
                            <TableCell>
                              <div className="flex flex-wrap gap-1">
                                {instance.providers.map((provider) => (
                                  <Badge key={provider} variant="outline">
                                    {provider}
                                  </Badge>
                                ))}
                              </div>
                            </TableCell>
                            <TableCell className="text-xs text-muted-foreground">
                              {formatDateTime(instance.usage?.lastSeen)}
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </ScrollArea>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground">
                <span>
                  Page <span className="font-medium">{usageCurrentPage}</span> /{' '}
                  <span className="font-medium">{usagePageCount}</span>
                </span>
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={goToPreviousUsagePage}
                    disabled={usageCurrentPage <= 1 || usageLoading}
                  >
                    Previous
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={goToNextUsagePage}
                    disabled={usageCurrentPage >= usagePageCount || usageLoading}
                  >
                    Next
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex flex-1 items-center justify-center text-muted-foreground">
              No usage data available.
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Sheet
        open={refreshDialogOpen}
        onOpenChange={(open) => {
          setRefreshDialogOpen(open);
          if (!open) {
            resetRefreshDialogState();
          }
        }}
      >
        <SheetContent side="right" className="flex h-full flex-col sm:max-w-[48rem]">
          <SheetHeader>
            <SheetTitle>Targeted refresh</SheetTitle>
            <SheetDescription>
              Refresh specific selectors or exact hashes directly from the registry. Selectors that are not present in{' '}
              <code className="mx-1 font-mono text-xs">registry.json</code>{' '}
              now fail with HTTP 404 instead of falling back to loose files.
            </SheetDescription>
          </SheetHeader>
          <ScrollArea
            className="flex-1 min-h-0 pr-1"
            allowYScroll
            allowXScroll={false}
          >
            <div className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="refresh-strategies">Selectors</Label>
                <CodeEditor
                  id="refresh-strategies"
                  value={refreshSelectorInput}
                  onChange={setRefreshSelectorInput}
                  language="text"
                  wrapLines
                  height="8rem"
                  className="max-h-[60vh] rounded-md border"
                  editorClassName="font-mono text-xs"
                  placeholder={`grid:canary
delay@sha256:def...`}
                />
                <p className="text-xs text-muted-foreground">
                  One selector per line (or comma separated). Examples: <code>grid</code>, <code>grid:v2.1.0</code>, <code>grid@sha256:abc...</code> Selectors must already exist in the registry.
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="refresh-hashes">Hashes</Label>
                <CodeEditor
                  id="refresh-hashes"
                  value={refreshHashInput}
                  onChange={setRefreshHashInput}
                  language="text"
                  wrapLines
                  height="8rem"
                  className="max-h-[60vh] rounded-md border"
                  editorClassName="font-mono text-xs"
                  placeholder={`sha256:abc...
sha256:def...`}
                />
                <p className="text-xs text-muted-foreground">
                  Provide raw digests to refresh everything pinned to those hashes. Hashes must be present in registry.json.
                </p>
              </div>
            </div>
            {refreshError ? (
              <Alert variant="destructive">
                <AlertTitle>Refresh failed</AlertTitle>
                <AlertDescription>{refreshError}</AlertDescription>
              </Alert>
            ) : null}
            {refreshResults.length > 0 ? (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Selector</TableHead>
                      <TableHead>Hash</TableHead>
                      <TableHead>Instances</TableHead>
                      <TableHead>Reason</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {refreshResults.map((result) => {
                      const instances = Array.isArray(result.instances) ? result.instances : [];
                      const reason = result.reason ?? 'unknown';
                      const reasonVariant =
                        reason === 'refreshed'
                          ? 'default'
                          : reason === 'alreadyPinned'
                            ? 'secondary'
                            : reason === 'missingFile' || reason === 'registryMismatch'
                              ? 'destructive'
                              : 'outline';
                      return (
                        <TableRow key={`${result.selector}-${result.hash ?? 'unknown'}`}>
                          <TableCell className="font-mono text-xs sm:text-sm">{result.selector}</TableCell>
                          <TableCell>
                            {result.hash ? (
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="h-auto px-0 text-xs text-muted-foreground hover:text-foreground"
                                onClick={() => copyHash(result.hash ?? '', 'Revision hash')}
                              >
                                <span className="font-mono">{result.hash.slice(0, 12)}…</span>
                                <Copy className="h-3 w-3" />
                              </Button>
                            ) : (
                              <span className="text-xs text-muted-foreground">—</span>
                            )}
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground">
                            {instances.length > 0 ? (
                              <span>{instances.length} ({instances.slice(0, 3).join(', ')}{instances.length > 3 ? '…' : ''})</span>
                            ) : (
                              <span>—</span>
                            )}
                          </TableCell>
                          <TableCell>
                            <Badge variant={reasonVariant}>
                              {reason}
                            </Badge>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            ) : null}
            </div>
          </ScrollArea>
          <SheetFooter className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <Button variant="ghost" onClick={closeRefreshDialog} disabled={refreshProcessing}>
              Close
            </Button>
            <div className="flex gap-2">
              <Button
                variant="outline"
                onClick={resetRefreshDialogState}
                disabled={refreshProcessing}
              >
                Clear inputs
              </Button>
              <Button onClick={() => void submitTargetedRefresh()} disabled={refreshProcessing}>
                {refreshProcessing ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                {refreshProcessing ? 'Refreshing…' : 'Execute refresh'}
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Sheet
        open={Boolean(detailModule)}
        onOpenChange={(open) => {
          if (!open) {
            setDetailModule(null);
            setRevisionToDelete(null);
            setPromoteTarget(null);
          }
        }}
      >
        <SheetContent side="right" className="flex h-full flex-col sm:max-w-[64rem]">
          <SheetHeader>
            <SheetTitle>{detailModule?.metadata.displayName ?? detailModule?.name ?? 'Strategy'}</SheetTitle>
            <SheetDescription>
              Runtime metadata exported by the JavaScript module.
            </SheetDescription>
          </SheetHeader>
          <ScrollArea
            className="flex-1 min-h-0 pr-1 max-h-full"
            allowYScroll
            allowXScroll={false}
            style={{
              maxHeight: 'calc(var(--dialog-content-max-height, 85vh) - 6rem)',
            }}
          >
            {detailModule ? (
              <div className="space-y-6">
              <div>
                <h4 className="text-sm font-semibold">Identifiers</h4>
                <div className="mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                  <div>
                    <p className="text-xs text-muted-foreground uppercase">Strategy name</p>
                    <p className="font-mono text-sm">{detailModule.name}</p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground uppercase">Module file</p>
                    <p className="font-mono text-sm">{detailModule.file}</p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground uppercase">Tag</p>
                    <p className="font-mono text-sm">{detailModuleTag || '—'}</p>
                  </div>
                </div>
              </div>
              <div>
                <h4 className="text-sm font-semibold">Hash & size</h4>
                <div className="mt-2 flex flex-wrap items-center gap-4">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-auto px-0 text-xs text-muted-foreground hover:text-foreground"
                    onClick={() => copyHash(detailModule.selectedRevisionHash)}
                  >
                    <span className="font-mono">{detailModule.selectedRevisionHash}</span>
                    <Copy className="h-3 w-3" />
                  </Button>
                  <span className="text-xs text-muted-foreground">
                    {formatBytes(detailModule.size)}
                  </span>
                </div>
              </div>
              <div>
                <h4 className="text-sm font-semibold">Pointer summary</h4>
                <div className="mt-2 rounded-md border p-3 text-xs">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <Badge variant="outline">Hash</Badge>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-auto px-0 text-muted-foreground hover:text-foreground"
                        onClick={() => copyHash(detailModule.selectedRevisionHash)}
                      >
                        <span className="font-mono">{detailModule.selectedRevisionHash}</span>
                        <Copy className="h-3 w-3" />
                      </Button>
                    </div>
                    <p className="text-[11px] text-muted-foreground">
                      Instances referencing this module use this hash until they are refreshed with a new revision.
                    </p>
                  </div>
                </div>
              </div>
              <div>
                <h4 className="text-sm font-semibold">Tags</h4>
                <div className="mt-2 rounded-md border">
                  {detailTagAliases.length > 0 ? (
                    <Table containerClassName="overflow-hidden text-xs">
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-1/3">Tag</TableHead>
                          <TableHead>Hash</TableHead>
                          <TableHead className="text-right">Actions</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {detailTagAliases.map(({ alias, hash }) => {
                          const isLatest = alias === 'latest';
                          return (
                            <TableRow key={alias}>
                              <TableCell>
                                <div className="flex items-center gap-2">
                                  <Badge variant={isLatest ? 'secondary' : 'outline'}>
                                    {alias}
                                  </Badge>
                                  {alias === detailModuleTag ? (
                                    <span className="text-[11px] text-muted-foreground">
                                      default
                                    </span>
                                  ) : null}
                                </div>
                              </TableCell>
                              <TableCell>
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-auto px-0 text-muted-foreground hover:text-foreground"
                                  onClick={() => copyHash(hash)}
                                >
                                  <span className="font-mono">{hash.slice(0, 24)}…</span>
                                  <Copy className="h-3 w-3" />
                                </Button>
                              </TableCell>
                              <TableCell>
                                <div className="flex justify-end gap-1">
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-7 px-2"
                                    onClick={() => {
                                      const revision = detailModule.revisions?.find((rev) => rev.hash === hash);
                                      const targetRevision =
                                        revision ?? {
                                          hash,
                                          tag: alias,
                                          path: detailModule.path,
                                          size: detailModule.size,
                                          retired: false,
                                        };
                                      openTagEditor(detailModule, targetRevision);
                                      setTagEditorValue(alias);
                                    }}
                                  >
                                    <Pencil className="mr-1 h-3 w-3" /> Move
                                  </Button>
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-7 px-2 text-destructive"
                                    disabled={isLatest}
                                    onClick={() => {
                                      setTagDeleteAllowOrphan(false);
                                      setTagDeleteTarget({ module: detailModule, tag: alias });
                                    }}
                                  >
                                    <Trash2 className="mr-1 h-3 w-3" /> Remove
                                  </Button>
                                </div>
                              </TableCell>
                            </TableRow>
                          );
                        })}
                      </TableBody>
                    </Table>
                  ) : (
                    <p className="px-3 py-4 text-xs text-muted-foreground">
                      No tag aliases are registered for this strategy.
                    </p>
                  )}
                </div>
              </div>
              <div>
                <h4 className="text-sm font-semibold">Revision history</h4>
                {detailModule.revisions && detailModule.revisions.length > 0 ? (
                  <ScrollArea
                    className="mt-2 rounded-md border"
                    aria-label="Revision history table"
                    allowXScroll
                    allowYScroll={false}
                  >
                    <div className="min-w-[720px] [&_[data-slot=table-container]]:overflow-visible">
                      <Table containerClassName="overflow-visible">
                      <TableHeader>
                        <TableRow>
                          <TableHead>Tag</TableHead>
                          <TableHead>Hash</TableHead>
                          <TableHead>Size</TableHead>
                          <TableHead className="text-right">Actions</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {detailModule.revisions.map((revision) => {
                          const promoteBusy =
                            revisionActionBusy === revisionKey(detailModule, revision, 'promote');
                          const deleteBusy =
                            revisionActionBusy === revisionKey(detailModule, revision, 'delete');
                          const revisionTag = revision.tag?.trim() || null;
                          return (
                            <TableRow key={`${revision.hash}-${revision.tag ?? 'untagged'}`}>
                              <TableCell>
                                <div className="flex flex-col gap-1">
                                  <div className="flex flex-wrap items-center gap-2">
                                    {revisionTag ? (
                                      <Badge variant="secondary">{revisionTag}</Badge>
                                    ) : (
                                      <span className="text-xs text-muted-foreground">—</span>
                                    )}
                                    {revision.retired ? (
                                      <Badge variant="warning">Retired</Badge>
                                    ) : null}
                                  </div>
                                  <p className="text-xs text-muted-foreground">
                                    {revision.path || '—'}
                                  </p>
                                </div>
                              </TableCell>
                              <TableCell>
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-auto px-0 text-xs text-muted-foreground hover:text-foreground"
                                  onClick={() => copyHash(revision.hash)}
                                >
                                  <span className="font-mono">{revision.hash.slice(0, 18)}…</span>
                                  <Copy className="h-3 w-3" />
                                </Button>
                              </TableCell>
                              <TableCell>{formatBytes(revision.size)}</TableCell>
                              <TableCell>
                                <div className="flex justify-end gap-2">
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    onClick={() =>
                                      setPromoteTarget({ module: detailModule, revision })
                                    }
                                    disabled={promoteBusy || deleteBusy}
                                    className="h-8 px-2"
                                  >
                                    {promoteBusy ? (
                                      <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                                    ) : (
                                      <ArrowUpCircle className="mr-1 h-3 w-3" />
                                    )}
                                    Promote
                                  </Button>
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 px-2"
                                    onClick={() => openTagEditor(detailModule, revision)}
                                    disabled={deleteBusy || promoteBusy}
                                  >
                                    <Tag className="mr-1 h-3 w-3" /> Tag
                                  </Button>
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 px-2"
                                    onClick={() =>
                                      openUsageDialog(
                                        buildRevisionSelector(detailModule, revision),
                                        detailModule.name,
                                        revision.hash,
                                      )
                                    }
                                    disabled={deleteBusy || promoteBusy}
                                  >
                                    <Activity className="mr-1 h-3 w-3" /> Usage
                                  </Button>
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 px-2 text-destructive"
                                    onClick={() => setRevisionToDelete({ module: detailModule, revision })}
                                    disabled={deleteBusy || promoteBusy}
                                  >
                                    {deleteBusy ? (
                                      <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                                    ) : (
                                      <Trash2 className="mr-1 h-3 w-3" />
                                    )}
                                    Delete
                                  </Button>
                                </div>
                              </TableCell>
                            </TableRow>
                          );
                        })}
                      </TableBody>
                    </Table>
                  </div>
                  </ScrollArea>
                ) : (
                  <p className="mt-2 text-sm text-muted-foreground">
                    No revision history available for this strategy yet.
                  </p>
                )}
              </div>
              {detailDescription ? (
                <div>
                  <h4 className="text-sm font-semibold">Description</h4>
                  <p className="mt-2 text-sm text-muted-foreground">{detailDescription}</p>
                </div>
              ) : null}
              <div>
                <h4 className="text-sm font-semibold">Events</h4>
                <div className="mt-2 flex flex-wrap gap-2">
                  {detailEvents.length > 0 ? (
                    detailEvents.map((event) => (
                      <Badge key={event} variant="secondary">
                        {event}
                      </Badge>
                    ))
                  ) : (
                    <span className="text-sm text-muted-foreground">None declared</span>
                  )}
                </div>
              </div>
              <div>
                <h4 className="text-sm font-semibold">Configuration fields</h4>
                <div className="mt-2 space-y-3">
                  {detailConfig.length === 0 ? (
                    <p className="text-sm text-muted-foreground">No configurable fields exported.</p>
                  ) : (
                    detailConfig.map((field) => (
                      <div key={field.name} className="rounded-md border p-3">
                        <div className="flex items-center justify-between">
                          <span className="font-mono text-sm">{field.name}</span>
                          <Badge variant="outline">{field.type}</Badge>
                        </div>
                        {field.description ? (
                          <p className="mt-1 text-sm text-muted-foreground">{field.description}</p>
                        ) : null}
                        <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                          <span>{field.required ? 'Required' : 'Optional'}</span>
                          {field.default !== undefined ? (
                            <>
                              <Separator orientation="vertical" className="h-4" />
                              <span>
                                Default:{' '}
                                <span className="font-mono">
                                  {typeof field.default === 'string'
                                    ? field.default
                                    : JSON.stringify(field.default)}
                                </span>
                              </span>
                            </>
                          ) : null}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
              </div>
            ) : null}
          </ScrollArea>
        </SheetContent>
      </Sheet>

      <Sheet
        open={Boolean(sourceModule)}
        onOpenChange={(open) => {
          if (!open) {
            setSourceModule(null);
            setSourceContent('');
            setSourceError(null);
          }
        }}
      >
        <SheetContent side="right" className="flex h-full flex-col sm:max-w-[80rem]">
          <SheetHeader>
            <SheetTitle>
              {sourceModule ? `Source: ${sourceModule.file || sourceModule.name}` : 'Source'}
            </SheetTitle>
            <SheetDescription>Read-only view of the on-disk JavaScript module.</SheetDescription>
          </SheetHeader>
          <ScrollArea
            className="flex-1 min-h-0 pr-1"
            allowYScroll
            allowXScroll={false}
          >
            {sourceLoading ? (
              <div className="flex items-center gap-2 py-8 text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Loading strategy source…
              </div>
            ) : sourceError ? (
              <Alert variant="destructive">
                <AlertDescription>{sourceError}</AlertDescription>
              </Alert>
            ) : (
              <div className="h-[60vh] rounded-md border">
                <CodeViewer
                  value={sourceContent}
                  language="javascript"
                  allowHorizontalScroll
                  wrapLines={false}
                  height="100%"
                  className="h-full w-full"
                  editorClassName={STRATEGY_SOURCE_EDITOR_CLASS}
                />
              </div>
            )}
          </ScrollArea>
          <SheetFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => copySource()}
              disabled={!sourceContent}
            >
              <Copy className="mr-2 h-4 w-4" />
              Copy source
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
            setDeleteError(null);
            setDeleteConflictUsage(null);
          }
        }}
        title="Delete strategy module"
        description={
          <span>
            Are you sure you want to delete{' '}
            <span className="font-semibold">{deleteTarget?.name}</span>? This action removes the registry entry; files stay on disk until you run the cleanup script.
          </span>
        }
        confirmLabel="Delete"
        confirmVariant="destructive"
        loading={deleting}
        errorMessage={deleteError}
        body={
          <div className="space-y-3 text-xs text-muted-foreground">
            <div className="space-y-2 rounded-md border p-3">
              <p>Deleting the registry entry disconnects the module immediately, but files remain on disk.</p>
              <p className="flex flex-wrap items-center gap-3">
                Run <code className="font-mono text-[11px]">{CLEANUP_COMMAND}</code> on the gateway host to prune stray directories.
                <Button size="sm" variant="outline" className="h-7 px-2 text-xs" onClick={() => void copyCleanupCommand()}>
                  Copy command
                </Button>
              </p>
            </div>
            <UsageConflictNotice conflict={deleteConflictUsage} />
          </div>
        }
        onConfirm={() => void handleDelete()}
      />
      <ConfirmDialog
        open={Boolean(revisionToDelete)}
        onOpenChange={(open) => {
          if (!open) {
            setRevisionToDelete(null);
            setRevisionDeleteError(null);
            setRevisionConflictUsage(null);
          }
        }}
        title="Delete revision"
        description={
          revisionToDelete ? (
            <span>
              Are you sure you want to delete revision{' '}
              <span className="font-semibold">
                {revisionLabel(revisionToDelete.revision)}
              </span>{' '}
              for <span className="font-semibold">{revisionToDelete.module.name}</span>?
            </span>
          ) : undefined
        }
        confirmLabel="Delete"
        confirmVariant="destructive"
        loading={Boolean(
          revisionToDelete &&
            revisionActionBusy ===
              revisionKey(revisionToDelete.module, revisionToDelete.revision, 'delete'),
        )}
        errorMessage={revisionDeleteError}
        body={
          <div className="space-y-3 text-xs text-muted-foreground">
            <div className="rounded-md border p-3">
              <p>
                Removing a revision detaches it from the registry only. Run{' '}
                <code className="mx-1 font-mono text-[11px]">{CLEANUP_COMMAND}</code> afterwards if you also want to delete the file.
              </p>
              <Button size="sm" variant="outline" className="mt-2 h-7 px-2 text-xs" onClick={() => void copyCleanupCommand()}>
                Copy cleanup command
              </Button>
            </div>
            <UsageConflictNotice conflict={revisionConflictUsage} />
          </div>
        }
        onConfirm={() =>
          revisionToDelete
            ? void handleDeleteRevision(revisionToDelete.module, revisionToDelete.revision)
            : undefined
        }
      />
      <ConfirmDialog
        open={Boolean(promoteTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setPromoteTarget(null);
          }
        }}
        title="Promote revision to latest"
        description={
          promoteTarget ? (
            <span>
              Move the <span className="font-semibold">latest</span> tag to revision{' '}
              <span className="font-semibold">{revisionLabel(promoteTarget.revision)}</span>?
            </span>
          ) : undefined
        }
        confirmLabel="Promote"
        confirmVariant="default"
        loading={Boolean(
          promoteTarget &&
            revisionActionBusy ===
              revisionKey(promoteTarget.module, promoteTarget.revision, 'promote'),
        )}
        onConfirm={() =>
          promoteTarget
            ? void handlePromoteRevision(promoteTarget.module, promoteTarget.revision)
            : undefined
        }
      />
      <ConfirmDialog
        open={templateConfirmOpen}
        onOpenChange={(open) => setTemplateConfirmOpen(open)}
        title="Replace current source?"
        description="Inserting the template will overwrite the editor contents."
        confirmLabel="Insert template"
        confirmVariant="default"
        onConfirm={() => {
          applyTemplateSource();
          setTemplateConfirmOpen(false);
        }}
      />
      <Dialog
        open={Boolean(tagEditorState)}
        onOpenChange={(open) => {
          if (!open) {
            closeTagEditor();
          }
        }}
      >
        <DialogContent className="max-w-[min(85vw,28rem)]">
          <DialogHeader>
            <DialogTitle>Assign tag</DialogTitle>
            <DialogDescription>
              Move an existing tag or create a new alias that points to the selected revision.
            </DialogDescription>
          </DialogHeader>
          {tagEditorState ? (
            <div className="space-y-4">
              <div className="rounded-md border p-3 text-xs">
                <p className="font-semibold">{tagEditorState.module.name}</p>
                <p className="mt-1 font-mono text-muted-foreground">{tagEditorState.revision.hash}</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="tag-editor-input">Tag name</Label>
                <Input
                  id="tag-editor-input"
                  value={tagEditorValue}
                  onChange={(event) => setTagEditorValue(event.target.value)}
                  placeholder="prod"
                  autoComplete="off"
                />
                <p className="text-xs text-muted-foreground">
                  Reusing a tag reassigns it, similar to Docker image tags.
                </p>
              </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="tag-editor-refresh"
                    checked={tagEditorRefresh}
                    onCheckedChange={(nextChecked) =>
                      setTagEditorRefresh(nextChecked === true)
                    }
                  />
                <Label htmlFor="tag-editor-refresh" className="text-sm">
                  Refresh runtime after updating tag
                </Label>
              </div>
              {tagEditorError ? (
                <Alert variant="destructive">
                  <AlertDescription>{tagEditorError}</AlertDescription>
                </Alert>
              ) : null}
            </div>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeTagEditor} disabled={assignTagMutation.isPending}>
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => void handleAssignTag()}
              disabled={assignTagMutation.isPending}
            >
              {assignTagMutation.isPending ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : null}
              Assign tag
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={Boolean(tagDeleteTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setTagDeleteTarget(null);
            setTagDeleteAllowOrphan(false);
            setTagDeleteError(null);
            setTagDeleteConflictUsage(null);
          }
        }}
        title="Remove tag"
        description={
          tagDeleteTarget ? (
            <span>
              Remove tag <span className="font-semibold">{tagDeleteTarget.tag}</span> from{' '}
              <span className="font-semibold">{tagDeleteTarget.module.name}</span>? The underlying hash remains available through other tags.
            </span>
          ) : undefined
        }
        body={
          <div className="space-y-3 rounded-md border p-3">
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="allow-orphan-toggle"
                  checked={tagDeleteAllowOrphan}
                  onCheckedChange={(nextChecked) =>
                    setTagDeleteAllowOrphan(nextChecked === true)
                  }
                />
              <Label htmlFor="allow-orphan-toggle" className="text-sm">
                Allow orphaned hash
              </Label>
            </div>
            <p className="mt-2 text-xs text-muted-foreground">
              Enable only when you intend to remove the final tag pointing to this revision.
            </p>
            <UsageConflictNotice conflict={tagDeleteConflictUsage} />
          </div>
        }
        confirmLabel="Remove"
        confirmVariant="destructive"
        loading={deleteTagMutation.isPending}
        errorMessage={tagDeleteError}
        onConfirm={() => void handleDeleteTag()}
      />
    </div>
  );
}

export default function StrategyModulesPage() {
  return (
    <Suspense fallback={<div className="py-6 text-sm text-muted-foreground">Loading modules…</div>}>
      <StrategyModulesPageContent />
    </Suspense>
  );
}
