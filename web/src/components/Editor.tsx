import styles from './Editor.module.css';
import type { PlatformInstance, SchemaDescriptor } from '../api/schemas';
import FieldList from './FieldList';
import TestConnection from './TestConnection';

export interface EditorProps {
  selectedKey: string | null;
  instance: PlatformInstance | null;
  originalInstance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
}

export default function Editor({
  selectedKey,
  instance,
  originalInstance,
  descriptor,
  onField,
  onToggleEnabled,
  onDelete,
}: EditorProps) {
  if (!selectedKey || !instance) {
    return (
      <div className={styles.wrapper}>
        <div className={styles.emptyCard}>
          <h2 className={styles.emptyTitle}>No instance selected</h2>
          <p className={styles.emptyBody}>
            Pick an instance from the sidebar, or click <em>+ New instance</em>
            to create one.
          </p>
        </div>
      </div>
    );
  }
  if (!descriptor) {
    return (
      <div className={styles.wrapper}>
        <div className={styles.emptyCard}>
          <h2 className={styles.emptyTitle}>Unknown platform type</h2>
          <p className={styles.emptyBody}>
            {selectedKey} is configured as type <code>{instance.type}</code>,
            which has no registered descriptor. Update the YAML directly or
            delete this instance.
          </p>
          <button
            type="button"
            className={styles.deleteBtn}
            onClick={() => {
              if (window.confirm(`Delete instance "${selectedKey}"?`)) onDelete();
            }}
          >
            Delete instance
          </button>
        </div>
      </div>
    );
  }

  const originalOptions = originalInstance?.options ?? {};
  const options = instance.options ?? {};
  const instanceIsDirty =
    !originalInstance ||
    (originalInstance.enabled ?? false) !== (instance.enabled ?? false) ||
    optionsDiffer(options, originalOptions);

  return (
    <div className={styles.wrapper}>
      <section className={styles.panel}>
        <header className={styles.panelHeader}>
          <h2 className={styles.title}>{selectedKey}</h2>
          <span className={styles.typeTag}>{descriptor.display_name}</span>
          <span className={styles.headerSpacer} />
          <label className={styles.enabledToggle}>
            <input
              type="checkbox"
              checked={instance.enabled ?? false}
              onChange={e => onToggleEnabled(e.currentTarget.checked)}
            />
            Enabled
          </label>
        </header>
        {descriptor.summary && (
          <p className={styles.summary}>{descriptor.summary}</p>
        )}
        <FieldList
          descriptor={descriptor}
          options={options}
          originalOptions={originalOptions}
          instanceKey={selectedKey}
          onChange={onField}
        />
        <TestConnection instanceKey={selectedKey} dirty={instanceIsDirty} />
        <div className={styles.dangerZone}>
          <button
            type="button"
            className={styles.deleteBtn}
            onClick={() => {
              if (window.confirm(`Delete instance "${selectedKey}"?`)) onDelete();
            }}
          >
            Delete instance
          </button>
        </div>
      </section>
    </div>
  );
}

function optionsDiffer(a: Record<string, string>, b: Record<string, string>): boolean {
  const keys = new Set<string>([...Object.keys(a), ...Object.keys(b)]);
  for (const k of keys) {
    if ((a[k] ?? '') !== (b[k] ?? '')) return true;
  }
  return false;
}
