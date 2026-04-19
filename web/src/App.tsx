import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { apiFetch, ApiError } from './api/client';
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import {
  dirtyCount as selectDirtyCount,
  initialState,
  instanceDirty,
  listInstances,
  reducer,
} from './state';
import TopBar from './components/TopBar';
import Sidebar from './components/Sidebar';
import Footer from './components/Footer';
import Editor from './components/Editor';
import NewInstanceDialog from './components/NewInstanceDialog';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [newDialogOpen, setNewDialogOpen] = useState(false);

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

  useEffect(() => {
    if (state.status === 'booting') return;
    const encoded = state.selectedKey ? encodeURIComponent(state.selectedKey) : '';
    const wanted = encoded ? '#' + encoded : '';
    if (window.location.hash !== wanted) {
      if (encoded) {
        window.location.hash = encoded;
      } else if (window.location.hash) {
        history.replaceState(null, '', window.location.pathname + window.location.search);
      }
    }
  }, [state.selectedKey, state.status]);

  useEffect(() => {
    if (state.status !== 'ready' || state.selectedKey !== null) return;
    const raw = window.location.hash.replace(/^#/, '');
    let fromHash = '';
    try {
      fromHash = decodeURIComponent(raw);
    } catch {
      fromHash = raw;
    }
    if (fromHash && state.config.gateway?.platforms?.[fromHash]) {
      dispatch({ type: 'select', key: fromHash });
    }
  }, [state.status, state.selectedKey, state.config.gateway?.platforms]);

  const instances = useMemo(() => {
    const plats = state.config.gateway?.platforms ?? {};
    return listInstances(state).map(key => ({
      key,
      type: plats[key]?.type ?? '',
      enabled: plats[key]?.enabled ?? false,
    }));
  }, [state]);

  const dirtyKeys = useMemo(() => {
    const a = state.config.gateway?.platforms ?? {};
    const b = state.originalConfig.gateway?.platforms ?? {};
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    const out = new Set<string>();
    for (const k of keys) {
      if (instanceDirty(state, k)) out.add(k);
    }
    return out;
  }, [state]);

  const dirty = selectDirtyCount(state);
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
      const msg = toErrMsg(err);
      dispatch({ type: 'save/done', error: msg });
    }
  }, [state.config]);

  const onSaveAndApply = useCallback(async () => {
    dispatch({ type: 'save/start' });
    try {
      await apiFetch('/api/config', {
        method: 'PUT',
        body: { config: state.config },
      });
      dispatch({ type: 'save/done' });
    } catch (err) {
      dispatch({ type: 'save/done', error: toErrMsg(err) });
      return;
    }
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
  }, [state.config]);

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

  const selectedInstance = state.selectedKey
    ? state.config.gateway?.platforms?.[state.selectedKey] ?? null
    : null;
  const selectedOriginal = state.selectedKey
    ? state.originalConfig.gateway?.platforms?.[state.selectedKey] ?? null
    : null;
  const selectedDescriptor = selectedInstance
    ? state.descriptors.find(d => d.type === selectedInstance.type) ?? null
    : null;

  return (
    <div className="app-shell">
      <TopBar dirtyCount={dirty} status={state.status} />
      <Sidebar
        instances={instances}
        selectedKey={state.selectedKey}
        descriptors={state.descriptors}
        dirtyKeys={dirtyKeys}
        onSelect={key => dispatch({ type: 'select', key })}
        onNewInstance={() => setNewDialogOpen(true)}
      />
      <main>
        <Editor
          selectedKey={state.selectedKey}
          instance={selectedInstance}
          originalInstance={selectedOriginal}
          descriptor={selectedDescriptor}
          onField={(field, value) =>
            state.selectedKey &&
            dispatch({ type: 'edit/field', key: state.selectedKey, field, value })
          }
          onToggleEnabled={enabled =>
            state.selectedKey &&
            dispatch({ type: 'edit/enabled', key: state.selectedKey, enabled })
          }
          onDelete={() =>
            state.selectedKey &&
            dispatch({ type: 'instance/delete', key: state.selectedKey })
          }
        />
      </main>
      <Footer
        dirtyCount={dirty}
        flash={state.flash}
        busy={busy}
        onSave={onSave}
        onSaveAndApply={onSaveAndApply}
      />
      {newDialogOpen && (
        <NewInstanceDialog
          descriptors={state.descriptors}
          existingKeys={new Set(Object.keys(state.config.gateway?.platforms ?? {}))}
          onCancel={() => setNewDialogOpen(false)}
          onCreate={(key, platformType) => {
            dispatch({ type: 'instance/create', key, platformType });
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
