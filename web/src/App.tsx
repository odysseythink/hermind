import { useEffect, useMemo, useReducer } from 'react';
import { apiFetch } from './api/client';
import {
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import { initialState, listInstances, reducer } from './state';
import TopBar from './components/TopBar';
import Sidebar from './components/Sidebar';
import Footer from './components/Footer';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);

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

  const instances = useMemo(() => {
    const plats = state.config.gateway?.platforms ?? {};
    return listInstances(state).map(key => ({
      key,
      type: plats[key]?.type ?? '',
      enabled: plats[key]?.enabled ?? false,
    }));
  }, [state]);

  // Dirty count: Stage 3 doesn't yet have field-level edits, so this
  // is a placeholder 0. Stage 4 computes a real structural diff.
  const dirtyCount = 0;
  const busy = state.status === 'saving' || state.status === 'applying';

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

  return (
    <div className="app-shell">
      <TopBar dirtyCount={dirtyCount} status={state.status} />
      <Sidebar
        instances={instances}
        selectedKey={state.selectedKey}
        descriptors={state.descriptors}
        onSelect={key => dispatch({ type: 'select', key })}
        onNewInstance={() => console.log('TODO: new instance (Stage 4)')}
      />
      <main>
        {state.selectedKey
          ? <div style={{ padding: '2rem' }}>Editor for {state.selectedKey} — fields land in Stage 4.</div>
          : <div style={{ padding: '2rem', color: 'var(--muted)' }}>
              Select an instance from the sidebar, or click + New instance.
            </div>}
      </main>
      <Footer
        dirtyCount={dirtyCount}
        flash={state.flash}
        busy={busy}
        onSave={() => console.log('TODO: save (Stage 4)')}
        onSaveAndApply={() => console.log('TODO: save & apply (Stage 4)')}
      />
    </div>
  );
}
