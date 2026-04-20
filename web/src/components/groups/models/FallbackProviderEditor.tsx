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
  /** Full config snapshot — forwarded to ConfigSection so cross-section
   *  datalist_source hints on fallback fields can resolve. Optional. */
  config?: Record<string, unknown>;
}

export default function FallbackProviderEditor(props: FallbackProviderEditorProps) {
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
      <div className={styles.body}>
        <ConfigSection
          section={props.section}
          value={props.value}
          originalValue={props.originalValue}
          onFieldChange={(field, v) => props.onField(props.index, field, v)}
          config={props.config}
        />
      </div>
    </section>
  );
}
