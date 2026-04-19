import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { apiFetch, ApiError } from './api/client';
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import {
  dirtyGroups as selectDirtyGroups,
  initialState,
  instanceDirty,
  listInstances,
  reducer,
  totalDirtyCount,
} from './state';
import { migrateLegacyHash, parseHash, stringifyHash } from './shell/hash';
import type { GroupId } from './shell/groups';
import TopBar from './components/shell/TopBar';
import Sidebar from './components/shell/Sidebar';
import ContentPanel from './components/shell/ContentPanel';
import Footer from './components/Footer';
import NewInstanceDialog from './components/NewInstanceDialog';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [newDialogOpen, setNewDialogOpen] = useState(false);

  // Boot: fetch schema + config
  useEffect(() => {
    const ctrl = new AbortController();
    (async () => {
      try {
        const [schema, cfg] = await Promise.all([
          apiFetch('/api/platforms/schema', {
            schema: PlatformsSchemaResponseSchema,
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
      if (parsed.sub && parsed.group === 'gateway') {
        dispatch({ type: 'shell/selectSub', key: parsed.sub });
      }
    }
    // If parsed.group is null, stay in EmptyState — no dispatch needed.
  }, [state.status, state.shell.activeGroup, state.config.gateway?.platforms]);

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
  }, [state]);

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
        dirtyInstanceKeys={dirtyInstanceKeys}
        onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
        onSelectSub={(key: string) => {
          dispatch({ type: 'shell/selectGroup', group: 'gateway' });
          dispatch({ type: 'shell/selectSub', key });
        }}
        onToggleGroup={(id: GroupId) => dispatch({ type: 'shell/toggleGroup', group: id })}
        onNewInstance={() => setNewDialogOpen(true)}
      />
      <main>
        <ContentPanel
          activeGroup={state.shell.activeGroup}
          config={state.config}
          selectedKey={selectedKey}
          instance={selectedInstance}
          originalInstance={selectedOriginal}
          descriptor={selectedDescriptor}
          dirtyGateway={dirtyGroupIds.has('gateway')}
          busy={busy}
          onField={(field, value) =>
            selectedKey &&
            dispatch({ type: 'edit/field', key: selectedKey, field, value })
          }
          onToggleEnabled={enabled =>
            selectedKey &&
            dispatch({ type: 'edit/enabled', key: selectedKey, enabled })
          }
          onDelete={() => selectedKey && dispatch({ type: 'instance/delete', key: selectedKey })}
          onApply={onApplyGateway}
          onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
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
