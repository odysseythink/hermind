import { useEffect, useId, useRef, useState } from 'react';
import styles from './ProviderEditor.module.css';
import ConfigSection from '../../ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../../../api/schemas';

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
  /** Full config snapshot — forwarded to ConfigSection so cross-section
   *  datalist_source hints on provider fields can resolve. Optional. */
  config?: Record<string, unknown>;
}

type FetchState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; count: number }
  | { status: 'err'; error: string };

export default function ProviderEditor(props: ProviderEditorProps) {
  const [models, setModels] = useState<string[]>([]);
  const [fetchState, setFetchState] = useState<FetchState>({ status: 'idle' });
  const datalistId = useId();
  const bodyRef = useRef<HTMLDivElement>(null);

  // Wire the Model field's <input> to the sibling <datalist> so the browser's
  // native autocomplete uses the fetched model list. ConfigSection renders
  // TextInput without a `list` attribute; rather than invading ConfigSection's
  // prop shape for this one case we set it from the outside after every render.
  //
  // TextInput wraps its <input> inside a <label> element whose label text is
  // a <span>. It does not set `aria-label` on the input, so we locate the
  // input by walking labels and matching the visible label text.
  useEffect(() => {
    const modelField = props.section.fields.find(f => f.name === 'model');
    if (!modelField || !bodyRef.current) return;
    const labels = bodyRef.current.querySelectorAll('label');
    for (const label of Array.from(labels)) {
      const span = label.querySelector('span');
      if (span && span.textContent?.trim().startsWith(modelField.label)) {
        const input = label.querySelector<HTMLInputElement>('input');
        if (input) input.setAttribute('list', datalistId);
        return;
      }
    }
  });

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
          section={props.section}
          value={props.value}
          originalValue={props.originalValue}
          onFieldChange={(field, v) => props.onField(props.instanceKey, field, v)}
          config={props.config}
        />
        <datalist id={datalistId} data-testid="provider-model-datalist">
          {models.map(m => (
            <option key={m} value={m} />
          ))}
        </datalist>
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
        {fetchState.status === 'ok' && (
          <span className={styles.chipOk}>Connected ✓ ({fetchState.count} models)</span>
        )}
        {fetchState.status === 'err' && (
          <span className={styles.chipErr}>{fetchState.error}</span>
        )}
      </footer>
    </section>
  );
}
