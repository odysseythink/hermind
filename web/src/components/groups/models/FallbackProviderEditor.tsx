import { useEffect, useId, useRef, useState } from 'react';
import styles from './FallbackProviderEditor.module.css';
import ConfigSection from '../../ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../../../api/schemas';

export interface FallbackProviderEditorProps {
  sectionKey: string;
  index: number;
  length: number;
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  dirty: boolean;
  onField: (index: number, field: string, value: unknown) => void;
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  fetchModels: () => Promise<{ models: string[] }>;
  /** Full config snapshot — forwarded to ConfigSection so cross-section
   *  datalist_source hints on fallback fields can resolve. Optional. */
  config?: Record<string, unknown>;
}

type FetchState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; count: number }
  | { status: 'err'; error: string };

export default function FallbackProviderEditor(props: FallbackProviderEditorProps) {
  const [models, setModels] = useState<string[]>([]);
  const [fetchState, setFetchState] = useState<FetchState>({ status: 'idle' });
  const datalistId = useId();
  const bodyRef = useRef<HTMLDivElement>(null);

  // Wire the Model field's <input> to the sibling <datalist>. Same pattern
  // ProviderEditor uses — TextInput doesn't expose a list prop we can thread
  // from here, so we patch the DOM after render.
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
    if (window.confirm(`Delete fallback #${props.index + 1}? This cannot be undone.`)) {
      props.onDelete();
    }
  }

  return (
    <section className={styles.editor}>
      <header className={styles.header}>
        <div className={styles.breadcrumb}>
          Models / Fallback Providers / <strong>Fallback #{props.index + 1}</strong>
        </div>
        <div className={styles.headerBtns}>
          <button
            type="button"
            className={styles.moveBtn}
            aria-label="Move up"
            disabled={props.index === 0}
            onClick={props.onMoveUp}
          >
            ↑
          </button>
          <button
            type="button"
            className={styles.moveBtn}
            aria-label="Move down"
            disabled={props.index === props.length - 1}
            onClick={props.onMoveDown}
          >
            ↓
          </button>
          <button type="button" className={styles.deleteBtn} onClick={onDeleteClick}>
            Delete
          </button>
        </div>
      </header>
      <div ref={bodyRef} className={styles.body}>
        <ConfigSection
          section={props.section}
          value={props.value}
          originalValue={props.originalValue}
          onFieldChange={(field, v) => props.onField(props.index, field, v)}
          config={props.config}
        />
        <datalist id={datalistId} data-testid="fallback-model-datalist">
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
