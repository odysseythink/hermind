import { useMemo, useState } from 'react';
import styles from './AuxiliaryEditor.module.css';
import ConfigSection from '../../ConfigSection';
import type { ConfigField, ConfigSection as ConfigSectionT } from '../../../api/schemas';

export interface AuxiliaryEditorProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  dirty: boolean;
  onField: (field: string, value: unknown) => void;
  fetchModels: () => Promise<{ models: string[] }>;
  testConnection: () => Promise<{ ok: boolean; latency_ms: number }>;
  config?: Record<string, unknown>;
}

type FetchState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; count: number }
  | { status: 'err'; error: string };

type TestState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; latencyMs: number }
  | { status: 'err'; error: string };

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

export default function AuxiliaryEditor(props: AuxiliaryEditorProps) {
  const [models, setModels] = useState<string[]>([]);
  const [fetchState, setFetchState] = useState<FetchState>({ status: 'idle' });
  const [testState, setTestState] = useState<TestState>({ status: 'idle' });

  // Synthesize a section where the `model` text field is rebuilt as an enum,
  // so ConfigSection renders it via EnumSelect (a real <select>). Options are
  // the union of fetched models and the currently saved value, so the user
  // sees their existing model id even before clicking Fetch.
  const synthSection = useMemo<ConfigSectionT>(() => {
    const currentModel = asString(props.value.model);
    const merged = new Set<string>(models);
    if (currentModel) merged.add(currentModel);
    const enumValues = Array.from(merged).sort();

    const fields: ConfigField[] = props.section.fields.map(f => {
      if (f.name !== 'model') return f;
      return { ...f, kind: 'enum' as const, enum: enumValues };
    });
    return { ...props.section, fields };
  }, [props.section, props.value.model, models]);

  async function onFetchClick() {
    setFetchState({ status: 'loading' });
    try {
      const { models: got } = await props.fetchModels();
      setModels(got);
      setFetchState({ status: 'ok', count: got.length });
    } catch (err) {
      setFetchState({ status: 'err', error: err instanceof Error ? err.message : String(err) });
    }
  }

  async function onTestClick() {
    setTestState({ status: 'loading' });
    try {
      const { latency_ms } = await props.testConnection();
      setTestState({ status: 'ok', latencyMs: latency_ms });
    } catch (err) {
      setTestState({ status: 'err', error: err instanceof Error ? err.message : String(err) });
    }
  }

  return (
    <section className={styles.editor}>
      <div className={styles.body}>
        <ConfigSection
          section={synthSection}
          value={props.value}
          originalValue={props.originalValue}
          onFieldChange={(field, v) => props.onField(field, v)}
          config={props.config}
        />
      </div>
      <footer className={styles.footer}>
        <button
          type="button"
          className={styles.fetchBtn}
          disabled={props.dirty || fetchState.status === 'loading'}
          title={props.dirty ? 'Save first, then fetch models' : undefined}
          onClick={onFetchClick}
        >
          {fetchState.status === 'loading' ? 'Fetching…' : 'Fetch models'}
        </button>
        <button
          type="button"
          className={styles.testBtn}
          disabled={props.dirty || testState.status === 'loading'}
          title={props.dirty ? 'Save first, then test the connection' : undefined}
          onClick={onTestClick}
        >
          {testState.status === 'loading' ? 'Testing…' : 'Test'}
        </button>
        {fetchState.status === 'ok' && (
          <span className={styles.chipOk}>Connected ✓ ({fetchState.count} models)</span>
        )}
        {fetchState.status === 'err' && (
          <span className={styles.chipErr}>{fetchState.error}</span>
        )}
        {testState.status === 'ok' && (
          <span className={styles.chipOk}>Test passed ({testState.latencyMs}ms)</span>
        )}
        {testState.status === 'err' && (
          <span className={styles.chipErr}>{testState.error}</span>
        )}
      </footer>
    </section>
  );
}
