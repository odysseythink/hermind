import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { useTranslation } from 'react-i18next';
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
import { listInstanceDirty } from './shell/listInstances';
import { migrateLegacyHash, parseHash, stringifyHash } from './shell/hash';
import type { GroupId } from './shell/groups';
import TopBar from './components/shell/TopBar';
import SettingsSidebar from './components/shell/SettingsSidebar';
import SettingsPanel from './components/shell/SettingsPanel';
import Footer from './components/Footer';
import NewInstanceDialog from './components/NewInstanceDialog';
import NewProviderDialog from './components/groups/models/NewProviderDialog';
import NewMcpServerDialog from './components/groups/advanced/NewMcpServerDialog';
import ChatWorkspace from './components/chat/ChatWorkspace';

export default function App() {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(reducer, initialState);
  const [newDialogOpen, setNewDialogOpen] = useState(false);

  // Hash-driven top-level mode router. parseHash returns { mode: 'chat' | 'settings', ... }.
  const [hashState, setHashState] = useState(() => parseHash(window.location.hash));
  useEffect(() => {
    const onChange = () => setHashState(parseHash(window.location.hash));
    window.addEventListener('hashchange', onChange);
    return () => window.removeEventListener('hashchange', onChange);
  }, []);

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
        const msg = err instanceof Error ? err.message : t('status.bootFailed');
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
    if (parsed.mode === 'settings') {
      dispatch({ type: 'shell/selectGroup', group: parsed.groupId });
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
    const wanted = state.shell.activeGroup
      ? stringifyHash({
          mode: 'settings',
          groupId: state.shell.activeGroup,
          sub: state.shell.activeSubKey ?? undefined,
        })
      : '';
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

  const fallbackProviders = useMemo(() => {
    const list = ((state.config as Record<string, unknown>).fallback_providers as
      | Array<Record<string, unknown>>
      | undefined) ?? [];
    return list.map(item => ({ provider: (item.provider as string) ?? '' }));
  }, [state.config]);

  const dirtyFallbackIndices = useMemo(() => {
    const cur = ((state.config as Record<string, unknown>).fallback_providers as
      | Array<unknown>
      | undefined) ?? [];
    const orig = ((state.originalConfig as Record<string, unknown>).fallback_providers as
      | Array<unknown>
      | undefined) ?? [];
    const len = Math.max(cur.length, orig.length);
    const out = new Set<number>();
    for (let i = 0; i < len; i++) {
      if (listInstanceDirty(state, 'fallback_providers', i)) out.add(i);
    }
    return out;
  }, [state]);

  const cronJobs = useMemo(() => {
    const sec = (state.config as Record<string, unknown>).cron as
      | { jobs?: Array<Record<string, unknown>> }
      | undefined;
    const list = sec?.jobs ?? [];
    return list.map(j => ({
      name: typeof j.name === 'string' ? j.name : '',
      schedule: typeof j.schedule === 'string' ? j.schedule : '',
    }));
  }, [state.config]);

  const dirtyCronIndices = useMemo(() => {
    const cur = ((state.config as Record<string, unknown>).cron as
      | { jobs?: Array<Record<string, unknown>> }
      | undefined)?.jobs ?? [];
    const orig = ((state.originalConfig as Record<string, unknown>).cron as
      | { jobs?: Array<Record<string, unknown>> }
      | undefined)?.jobs ?? [];
    const out = new Set<number>();
    const len = Math.max(cur.length, orig.length);
    for (let i = 0; i < len; i++) {
      if (!shallowEqualRecord(cur[i], orig[i])) out.add(i);
    }
    return out;
  }, [state.config, state.originalConfig]);

  const mcpInstances = useMemo(() => {
    const sec = (state.config as Record<string, unknown>).mcp as
      | { servers?: Record<string, Record<string, unknown>> }
      | undefined;
    const servers = sec?.servers ?? {};
    return Object.keys(servers)
      .sort()
      .map(key => {
        const inst = servers[key];
        return {
          key,
          command: typeof inst?.command === 'string' ? inst.command : '',
          enabled: inst?.enabled !== false, // default-true when unset (matches Go IsEnabled)
        };
      });
  }, [state.config]);

  const dirtyMcpKeys = useMemo(() => {
    const cur = ((state.config as Record<string, unknown>).mcp as
      | { servers?: Record<string, Record<string, unknown>> }
      | undefined)?.servers ?? {};
    const orig = ((state.originalConfig as Record<string, unknown>).mcp as
      | { servers?: Record<string, Record<string, unknown>> }
      | undefined)?.servers ?? {};
    const out = new Set<string>();
    const keys = new Set<string>([...Object.keys(cur), ...Object.keys(orig)]);
    for (const k of keys) {
      if (!shallowEqualRecord(cur[k], orig[k])) out.add(k);
    }
    return out;
  }, [state.config, state.originalConfig]);

  const sectionSubkey = useMemo(() => {
    const m = new Map<string, string | undefined>();
    for (const s of state.configSections) {
      m.set(s.key, s.subkey);
    }
    return (sectionKey: string): string | undefined => m.get(sectionKey);
  }, [state.configSections]);

  const [newProviderDialogOpen, setNewProviderDialogOpen] = useState(false);
  const [newMcpDialogOpen, setNewMcpDialogOpen] = useState(false);

  const onFetchProviderModels = useCallback(async (instanceKey: string) => {
    const res = await apiFetch(`/api/providers/${encodeURIComponent(instanceKey)}/models`, {
      method: 'POST',
      schema: ProviderModelsResponseSchema,
    });
    return res;
  }, []);

  const onFetchFallbackModels = useCallback(async (index: number) => {
    const res = await apiFetch(`/api/fallback_providers/${index}/models`, {
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
    return <div style={{ padding: '2rem' }}>{t('status.loading')}</div>;
  }
  if (state.status === 'error' && state.descriptors.length === 0) {
    return (
      <div style={{ padding: '2rem', color: 'var(--error)' }}>
        {t('status.bootFailedPrefix')} {state.flash?.msg ?? t('status.unknownError')}
      </div>
    );
  }

  const setMode = (m: 'chat' | 'settings') => {
    window.location.hash = stringifyHash(
      m === 'chat' ? { mode: 'chat' } : { mode: 'settings', groupId: 'models' },
    );
  };

  if (hashState.mode === 'chat') {
    const providerConfigured = Object.values(
      (state.config as { providers?: Record<string, { api_key?: string }> }).providers ?? {},
    ).some((p) => typeof p?.api_key === 'string' && p.api_key.length > 0);
    return (
      <div className="app-shell">
        <TopBar dirtyCount={0} status={state.status} onSave={() => {}} mode="chat" onModeChange={setMode} />
        <ChatWorkspace
          sessionId={hashState.sessionId ?? null}
          providerConfigured={providerConfigured}
          onChangeSession={(id) => {
            window.location.hash = stringifyHash({ mode: 'chat', sessionId: id });
          }}
        />
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
      <TopBar dirtyCount={dirty} status={state.status} onSave={onSave} mode="settings" onModeChange={setMode} />
      <SettingsSidebar
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
        fallbackProviders={fallbackProviders}
        dirtyFallbackIndices={dirtyFallbackIndices}
        onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
        onSelectSub={(key: string) => dispatch({ type: 'shell/selectSub', key })}
        onToggleGroup={(id: GroupId) => dispatch({ type: 'shell/toggleGroup', group: id })}
        onNewInstance={() => setNewDialogOpen(true)}
        onNewProvider={() => setNewProviderDialogOpen(true)}
        onAddFallback={() => {
          const list = ((state.config as Record<string, unknown>).fallback_providers as
            | Array<unknown>
            | undefined) ?? [];
          const section = state.configSections.find(s => s.key === 'fallback_providers');
          const providerField = section?.fields.find(f => f.name === 'provider');
          const firstType = providerField?.enum?.[0] ?? '';
          dispatch({
            type: 'list-instance/create',
            sectionKey: 'fallback_providers',
            initial: { provider: firstType, base_url: '', api_key: '', model: '' },
          });
          dispatch({ type: 'shell/selectGroup', group: 'models' });
          dispatch({ type: 'shell/selectSub', key: `fallback:${list.length}` });
        }}
        onMoveFallback={(index, direction) =>
          dispatch({
            type: direction === 'up' ? 'list-instance/move-up' : 'list-instance/move-down',
            sectionKey: 'fallback_providers',
            index,
          })
        }
        onReorderFallback={(from, to) =>
          dispatch({
            type: 'list-instance/move',
            sectionKey: 'fallback_providers',
            from,
            to,
          })
        }
        mcpInstances={mcpInstances}
        dirtyMcpKeys={dirtyMcpKeys}
        onAddMcpServer={() => setNewMcpDialogOpen(true)}
        cronJobs={cronJobs}
        dirtyCronIndices={dirtyCronIndices}
        onAddCronJob={() => {
          const list = (((state.config as Record<string, unknown>).cron as
            | { jobs?: Array<unknown> }
            | undefined)?.jobs) ?? [];
          dispatch({
            type: 'list-instance/create',
            sectionKey: 'cron',
            subkey: 'jobs',
            initial: { name: '', schedule: '', prompt: '', model: '' },
          });
          dispatch({ type: 'shell/selectGroup', group: 'advanced' });
          dispatch({ type: 'shell/selectSub', key: `cron:${list.length}` });
        }}
        onMoveCron={(index, direction) => {
          const list = (((state.config as Record<string, unknown>).cron as
            { jobs?: Array<unknown> } | undefined)?.jobs) ?? [];
          const newIndex = direction === 'up' ? index - 1 : index + 1;
          if (newIndex < 0 || newIndex >= list.length) {
            // Button is disabled at the edges; this is defensive only.
            return;
          }
          dispatch({
            type: direction === 'up' ? 'list-instance/move-up' : 'list-instance/move-down',
            sectionKey: 'cron',
            subkey: sectionSubkey('cron'),
            index,
          });
          dispatch({ type: 'shell/selectSub', key: `cron:${newIndex}` });
        }}
      />
      <main>
        <SettingsPanel
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
          onConfigKeyedField={(sectionKey, instanceKey, field, value) => {
            dispatch({ type: 'edit/keyed-instance-field', sectionKey, subkey: sectionSubkey(sectionKey), instanceKey, field, value });
          }}
          onConfigKeyedDelete={(sectionKey, instanceKey) => {
            dispatch({ type: 'keyed-instance/delete', sectionKey, subkey: sectionSubkey(sectionKey), instanceKey });
            dispatch({ type: 'shell/selectSub', key: null });
          }}
          onFetchModels={onFetchProviderModels}
          onFetchFallbackModels={onFetchFallbackModels}
          onConfigListField={(sectionKey, index, field, value) => {
            dispatch({ type: 'edit/list-instance-field', sectionKey, subkey: sectionSubkey(sectionKey), index, field, value });
          }}
          onConfigListDelete={(sectionKey, index) => {
            dispatch({ type: 'list-instance/delete', sectionKey, subkey: sectionSubkey(sectionKey), index });
            dispatch({ type: 'shell/selectSub', key: null });
          }}
          onConfigListMove={(sectionKey, index, direction) => {
            dispatch({
              type: direction === 'up' ? 'list-instance/move-up' : 'list-instance/move-down',
              sectionKey,
              subkey: sectionSubkey(sectionKey),
              index,
            });
            const newIndex = direction === 'up' ? index - 1 : index + 1;
            // fallback_providers uses the shorter "fallback:N" sub-key prefix (legacy
            // from stage-4c). All new list-shaped sections use "${sectionKey}:N".
            const prefix = sectionKey === 'fallback_providers' ? 'fallback:' : `${sectionKey}:`;
            dispatch({ type: 'shell/selectSub', key: `${prefix}${newIndex}` });
          }}
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
      {newMcpDialogOpen && (() => {
        const existingKeys = new Set(Object.keys(
          (((state.config as Record<string, unknown>).mcp as
            { servers?: Record<string, unknown> } | undefined)?.servers) ?? {}
        ));
        return (
          <NewMcpServerDialog
            existingKeys={existingKeys}
            onCancel={() => setNewMcpDialogOpen(false)}
            onCreate={key => {
              dispatch({
                type: 'keyed-instance/create',
                sectionKey: 'mcp',
                subkey: 'servers',
                instanceKey: key,
                initial: { command: '', enabled: true },
              });
              dispatch({ type: 'shell/selectGroup', group: 'advanced' });
              dispatch({ type: 'shell/selectSub', key: `mcp:${key}` });
              setNewMcpDialogOpen(false);
            }}
          />
        );
      })()}
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

function shallowEqualRecord(
  a: Record<string, unknown> | undefined,
  b: Record<string, unknown> | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  const ak = Object.keys(a);
  const bk = Object.keys(b);
  if (ak.length !== bk.length) return false;
  for (const k of ak) if (a[k] !== b[k]) return false;
  return true;
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
