import { useRef, useState } from 'react';
import styles from './ProviderEditor.module.css';
import fieldStyles from '../../fields/fields.module.css';
import ConfigSection from '../../ConfigSection';
import type { ConfigSection as ConfigSectionT, ConfigField } from '../../../api/schemas';

export interface ProviderEditorProps {
  sectionKey: string;
  instanceKey: string;
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  dirty: boolean;
  onField: (instanceKey: string, field: string, value: unknown) => void;
  onDelete: () => void;
  fetchModels: () => Promise<{ models: string[] }>;
  testConnection: () => Promise<{ ok: boolean; latency_ms: number }>;
  /** Full config snapshot — forwarded to ConfigSection so cross-section
   *  datalist_source hints on provider fields can resolve. Optional. */
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

function ModelSelect({
  field,
  value,
  models,
  onChange,
}: {
  field: ConfigField | undefined;
  value: string;
  models: string[];
  onChange: (v: string) => void;
}) {
  if (!field) return null;
  if (models.length === 0) {
    // Fall back to plain text input until models are fetched.
    return (
      <label className={fieldStyles.row}>
        <span className={fieldStyles.label}>
          {field.label}
          {field.required && <span className={fieldStyles.required}>*</span>}
        </span>
        <input
          type="text"
          className={fieldStyles.input}
          value={value}
          placeholder={field.default !== undefined ? String(field.default) : undefined}
          onChange={(e) => onChange(e.currentTarget.value)}
        />
        {field.help && <span className={fieldStyles.help}>{field.help}</span>}
      </label>
    );
  }
  return (
    <label className={fieldStyles.row}>
      <span className={fieldStyles.label}>
        {field.label}
        {field.required && <span className={fieldStyles.required}>*</span>}
      </span>
      <select className={fieldStyles.select} value={value} onChange={(e) => onChange(e.currentTarget.value)}>
        {!field.required && <option value="">—</option>}
        {models.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>
      {field.help && <span className={fieldStyles.help}>{field.help}</span>}
    </label>
  );
}

export default function ProviderEditor(props: ProviderEditorProps) {
  const [models, setModels] = useState<string[]>([]);
  const [fetchState, setFetchState] = useState<FetchState>({ status: 'idle' });
  const [testState, setTestState] = useState<TestState>({ status: 'idle' });
  const bodyRef = useRef<HTMLDivElement>(null);

  const modelField = props.section.fields.find((f) => f.name === 'model');
  const sectionWithoutModel: ConfigSectionT = {
    ...props.section,
    fields: props.section.fields.filter((f) => f.name !== 'model'),
  };

  // No DOM-patching needed — model is rendered as a native <select> below.

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

  function onDeleteClick() {
    if (window.confirm(`Delete provider "${props.instanceKey}"? This cannot be undone.`)) {
      props.onDelete();
    }
  }

  return (
    <section className={styles.editor}>
      <header className={styles.header}>
        <div className={styles.breadcrumb}>
          Models / Providers / <strong>{props.instanceKey}</strong>
        </div>
        <button type="button" className={styles.deleteBtn} onClick={onDeleteClick}>
          Delete
        </button>
      </header>
      <div ref={bodyRef} className={styles.body}>
        <ConfigSection
          section={sectionWithoutModel}
          value={props.value}
          originalValue={props.originalValue}
          onFieldChange={(field, v) => props.onField(props.instanceKey, field, v)}
          config={props.config}
        />
        <ModelSelect
          field={modelField}
          value={String(props.value['model'] ?? '')}
          models={models}
          onChange={(v) => props.onField(props.instanceKey, 'model', v)}
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
