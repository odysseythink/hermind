import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { apiFetch, ApiError } from './api/client';
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  ConfigSchemaResponseSchema,
  PlatformsSchemaResponseSchema,
  ProviderModelsResponseSchema,
} from './api/schemas';
import {
  dirtyGroups as selectDirtyGroups,
  initialState,
  instanceDirty,
  listInstances,
  reducer,
  totalDirtyCount,
} from './state';
import { keyedInstanceDirty } from './shell/keyedInstances';
import { migrateLegacyHash, parseHash, stringifyHash } from './shell/hash';
import type { GroupId } from './shell/groups';
import TopBar from './components/shell/TopBar';
import Sidebar from './components/shell/Sidebar';
import ContentPanel from './components/shell/ContentPanel';
import Footer from './components/Footer';
import NewInstanceDialog from './components/NewInstanceDialog';
import NewProviderDialog from './components/groups/models/NewProviderDialog';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [newDialogOpen, setNewDialogOpen] = useState(false);

  // Boot: fetch schema + config
  useEffect(() => {
    const ctrl = new AbortController();
    (async () => {
      try {
        const [schema, cfgSchema, cfg] = await Promise.all([
          apiFetch('/api/platforms/schema', {
            schema: PlatformsSchemaResponseSchema,
            signal: ctrl.signal,
          }),
          apiFetch('/api/config/schema', {
            schema: ConfigSchemaResponseSchema,
            signal: ctrl.signal,
          }),
          apiFetch('/api/config', {
            schema: ConfigResponseSchema,
            signal: ctrl.signal,
          }),
        ]);
        dispatch({
          type: 'boot/loaded',
          descriptors: schema.descriptors,
          configSections: cfgSchema.sections,
          config: cfg.config,
        });
      } catch (err) {
        if (ctrl.signal.aborted) return;
        const msg = err instanceof Error ? err.message : 'boot failed';
        dispatch({ type: 'boot/failed', error: msg });
      }
    })();
    return () => ctrl.abort();
  }, []);

  // Resolve initial hash (including legacy migration) once config is available.
  useEffect(() => {
    if (state.status !== 'ready') return;
    if (state.shell.activeGroup !== null) return;
    const currentHash = window.location.hash;
    const platforms = Object.keys(state.config.gateway?.platforms ?? {});
    const migrated = migrateLegacyHash(currentHash, platforms);
    const effective = migrated ?? currentHash;
    if (migrated) {
      history.replaceState(null, '', window.location.pathname + window.location.search + migrated);
    }
    const parsed = parseHash(effective);
    if (parsed.group) {
      dispatch({ type: 'shell/selectGroup', group: parsed.group });
      if (parsed.sub) {
        dispatch({ type: 'shell/selectSub', key: parsed.sub });
      }
    }
    // If parsed.group is null, stay in EmptyState — no dispatch needed.
    // platforms is read here but deliberately NOT a dep — this effect is a
    // one-shot migration on status transition; re-running on platform edits
    // would re-dispatch selectGroup/selectSub unnecessarily (the inner guard
    // bails out anyway, but this is faster).
  }, [state.status, state.shell.activeGroup]); // eslint-disable-line react-hooks/exhaustive-deps

  // Sync hash whenever active group/sub changes.
  useEffect(() => {
    if (state.status === 'booting') return;
    const wanted = stringifyHash(state.shell.activeGroup, state.shell.activeSubKey);
    if (window.location.hash !== wanted) {
      if (wanted) {
        history.replaceState(null, '', window.location.pathname + window.location.search + wanted);
      } else if (window.location.hash) {
        history.replaceState(null, '', window.location.pathname + window.location.search);
      }
    }
  }, [state.shell.activeGroup, state.shell.activeSubKey, state.status]);

  const instances = useMemo(() => {
    const plats = state.config.gateway?.platforms ?? {};
    return listInstances(state).map(key => ({
      key,
      type: plats[key]?.type ?? '',
      enabled: plats[key]?.enabled ?? false,
    }));
  }, [state]);

  const dirtyInstanceKeys = useMemo(() => {
    const a = state.config.gateway?.platforms ?? {};
    const b = state.originalConfig.gateway?.platforms ?? {};
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    const out = new Set<string>();
    for (const k of keys) {
      if (instanceDirty(state, k)) out.add(k);
    }
    return out;
    // state is read here (via instanceDirty) but deliberately narrowed to only
    // the platform slices it accesses — this memo doesn't need to re-run on
    // other state changes like activeGroup or flash.
  }, [state.config.gateway?.platforms, state.originalConfig.gateway?.platforms]); // eslint-disable-line react-hooks/exhaustive-deps

  const providerInstances = useMemo(() => {
    const p = ((state.config as Record<string, unknown>).providers as
      | Record<string, Record<string, unknown>>
      | undefined) ?? {};
    return Object.keys(p)
      .sort()
      .map(k => ({
        key: k,
        type: (p[k].provider as string) ?? '',
      }));
  }, [state.config]);

  const dirtyProviderKeys = useMemo(() => {
    const cur = ((state.config as Record<string, unknown>).providers as
      | Record<string, unknown>
      | undefined) ?? {};
    const orig = ((state.originalConfig as Record<string, unknown>).providers as
      | Record<string, unknown>
      | undefined) ?? {};
    const keys = new Set<string>([...Object.keys(cur), ...Object.keys(orig)]);
    const out = new Set<string>();
    for (const k of keys) {
      if (keyedInstanceDirty(state, 'providers', k)) out.add(k);
    }
    return out;
  }, [state]);

  const [newProviderDialogOpen, setNewProviderDialogOpen] = useState(false);

  const onFetchProviderModels = useCallback(async (instanceKey: string) => {
    const res = await apiFetch(`/api/providers/${encodeURIComponent(instanceKey)}/models`, {
      method: 'POST',
      schema: ProviderModelsResponseSchema,
    });
    return res;
  }, []);

  const dirtyGroupIds = useMemo(() => selectDirtyGroups(state), [state]);
  const dirty = totalDirtyCount(state);
  const busy = state.status === 'saving' || state.status === 'applying';

  const onSave = useCallback(async () => {
    dispatch({ type: 'save/start' });
    try {
      await apiFetch('/api/config', {
        method: 'PUT',
        body: { config: state.config },
      });
      dispatch({ type: 'save/done' });
    } catch (err) {
      dispatch({ type: 'save/done', error: toErrMsg(err) });
    }
  }, [state.config]);

  const onApplyGateway = useCallback(async () => {
    dispatch({ type: 'apply/start' });
    try {
      const res = await apiFetch('/api/platforms/apply', {
        method: 'POST',
        schema: ApplyResultSchema,
      });
      if (res.ok) {
        dispatch({ type: 'apply/done' });
      } else {
        dispatch({ type: 'apply/done', error: res.error ?? 'apply failed' });
      }
    } catch (err) {
      dispatch({ type: 'apply/done', error: toErrMsg(err) });
    }
  }, []);

  if (state.status === 'booting') {
    return <div style={{ padding: '2rem' }}>Loading…</div>;
  }
  if (state.status === 'error' && state.descriptors.length === 0) {
    return (
      <div style={{ padding: '2rem', color: 'var(--error)' }}>
        Boot failed: {state.flash?.msg ?? 'unknown error'}
      </div>
    );
  }

  const selectedKey = state.shell.activeSubKey;
  const selectedInstance = selectedKey
    ? state.config.gateway?.platforms?.[selectedKey] ?? null
    : null;
  const selectedOriginal = selectedKey
    ? state.originalConfig.gateway?.platforms?.[selectedKey] ?? null
    : null;
  const selectedDescriptor = selectedInstance
    ? state.descriptors.find(d => d.type === selectedInstance.type) ?? null
    : null;

  return (
    <div className="app-shell">
      <TopBar dirtyCount={dirty} status={state.status} onSave={onSave} />
      <Sidebar
        activeGroup={state.shell.activeGroup}
        activeSubKey={state.shell.activeSubKey}
        expandedGroups={state.shell.expandedGroups}
        dirtyGroups={dirtyGroupIds}
        instances={instances}
        selectedKey={selectedKey}
        descriptors={state.descriptors}
        configSections={state.configSections}
        dirtyInstanceKeys={dirtyInstanceKeys}
        providerInstances={providerInstances}
        dirtyProviderKeys={dirtyProviderKeys}
        onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
        onSelectSub={(key: string) => dispatch({ type: 'shell/selectSub', key })}
        onToggleGroup={(id: GroupId) => dispatch({ type: 'shell/toggleGroup', group: id })}
        onNewInstance={() => setNewDialogOpen(true)}
        onNewProvider={() => setNewProviderDialogOpen(true)}
      />
      <main>
        <ContentPanel
          activeGroup={state.shell.activeGroup}
          activeSubKey={state.shell.activeSubKey}
          config={state.config}
          originalConfig={state.originalConfig}
          configSections={state.configSections}
          selectedKey={selectedKey}
          instance={selectedInstance}
          originalInstance={selectedOriginal}
          descriptor={selectedDescriptor}
          dirtyGateway={dirtyGroupIds.has('gateway')}
          busy={busy}
          onField={(field, value) =>
            selectedKey && dispatch({ type: 'edit/field', key: selectedKey, field, value })
          }
          onToggleEnabled={enabled =>
            selectedKey && dispatch({ type: 'edit/enabled', key: selectedKey, enabled })
          }
          onDelete={() => selectedKey && dispatch({ type: 'instance/delete', key: selectedKey })}
          onApply={onApplyGateway}
          onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
          onConfigField={(sectionKey, field, value) =>
            dispatch({ type: 'edit/config-field', sectionKey, field, value })
          }
          onConfigScalar={(sectionKey, value) =>
            dispatch({ type: 'edit/config-scalar', sectionKey, value })
          }
          onConfigKeyedField={(sectionKey, instanceKey, field, value) =>
            dispatch({ type: 'edit/keyed-instance-field', sectionKey, instanceKey, field, value })
          }
          onConfigKeyedDelete={(sectionKey, instanceKey) => {
            dispatch({ type: 'keyed-instance/delete', sectionKey, instanceKey });
            dispatch({ type: 'shell/selectSub', key: null });
          }}
          onFetchModels={onFetchProviderModels}
        />
      </main>
      <Footer flash={state.flash} />
      {newDialogOpen && (
        <NewInstanceDialog
          descriptors={state.descriptors}
          existingKeys={new Set(Object.keys(state.config.gateway?.platforms ?? {}))}
          onCancel={() => setNewDialogOpen(false)}
          onCreate={(key, platformType) => {
            dispatch({ type: 'instance/create', key, platformType });
            dispatch({ type: 'shell/selectGroup', group: 'gateway' });
            dispatch({ type: 'shell/selectSub', key });
            setNewDialogOpen(false);
          }}
        />
      )}
      {newProviderDialogOpen && (() => {
        const section = state.configSections.find(s => s.key === 'providers');
        const providerField = section?.fields.find(f => f.name === 'provider');
        const types = (providerField?.enum ?? []) as readonly string[];
        const existingKeys = new Set(
          Object.keys(
            ((state.config as Record<string, unknown>).providers as
              | Record<string, unknown>
              | undefined) ?? {},
          ),
        );
        return (
          <NewProviderDialog
            providerTypes={types}
            existingKeys={existingKeys}
            onCancel={() => setNewProviderDialogOpen(false)}
            onCreate={(key, providerType) => {
              dispatch({
                type: 'keyed-instance/create',
                sectionKey: 'providers',
                instanceKey: key,
                initial: { provider: providerType, base_url: '', api_key: '', model: '' },
              });
              dispatch({ type: 'shell/selectGroup', group: 'models' });
              dispatch({ type: 'shell/selectSub', key });
              setNewProviderDialogOpen(false);
            }}
          />
        );
      })()}
    </div>
  );
}

function toErrMsg(err: unknown): string {
  if (err instanceof ApiError) {
    if (typeof err.body === 'object' && err.body !== null && 'error' in err.body) {
      const e = (err.body as { error?: unknown }).error;
      if (typeof e === 'string') return e;
    }
    return `HTTP ${err.status}`;
  }
  return err instanceof Error ? err.message : String(err);
}
