import { useEffect, useRef, useState } from 'react';
import styles from './NewInstanceDialog.module.css';
import type { SchemaDescriptor } from '../api/schemas';

export interface NewInstanceDialogProps {
  descriptors: SchemaDescriptor[];
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string, platformType: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewInstanceDialog({
  descriptors,
  existingKeys,
  onCancel,
  onCreate,
}: NewInstanceDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [key, setKey] = useState('');
  const [platformType, setPlatformType] = useState(
    descriptors[0]?.type ?? '',
  );
  const [keyError, setKeyError] = useState<string | null>(null);

  useEffect(() => {
    dialogRef.current?.showModal();
  }, []);

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = key.trim();
    if (!trimmed) {
      setKeyError('Instance key is required.');
      return;
    }
    if (!KEY_REGEX.test(trimmed)) {
      setKeyError('Use lowercase letters, digits, underscore. Must start with a letter.');
      return;
    }
    if (existingKeys.has(trimmed)) {
      setKeyError(`An instance named "${trimmed}" already exists.`);
      return;
    }
    if (!platformType) {
      setKeyError('Pick a platform type.');
      return;
    }
    onCreate(trimmed, platformType);
  }

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      onCancel={e => {
        e.preventDefault();
        onCancel();
      }}
      onClose={() => onCancel()}
    >
      <form onSubmit={onSubmit}>
        <header className={styles.header}>
          <h2 className={styles.title}>New instance</h2>
          <span className={styles.spacer} />
          <button
            type="button"
            className={styles.close}
            onClick={onCancel}
            aria-label="Close"
          >
            ✕
          </button>
        </header>
        <div className={styles.body}>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-instance-type">
              Platform type
            </label>
            <select
              id="new-instance-type"
              className={styles.select}
              value={platformType}
              onChange={e => setPlatformType(e.currentTarget.value)}
            >
              {descriptors.map(d => (
                <option key={d.type} value={d.type}>
                  {d.display_name} ({d.type})
                </option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-instance-key">
              Instance key
            </label>
            <input
              id="new-instance-key"
              className={styles.input}
              value={key}
              placeholder="tg_main"
              autoFocus
              onChange={e => {
                setKey(e.currentTarget.value);
                setKeyError(null);
              }}
            />
            <span className={styles.hint}>
              Identifier under <code>gateway.platforms.*</code>. Lowercase, underscores.
              Immutable after creation.
            </span>
            {keyError && <span className={styles.error}>{keyError}</span>}
          </div>
        </div>
        <footer className={styles.footer}>
          <button
            type="button"
            className={`${styles.btn} ${styles.secondary}`}
            onClick={onCancel}
          >
            Cancel
          </button>
          <span className={styles.footerSpacer} />
          <button
            type="submit"
            className={`${styles.btn} ${styles.primary}`}
          >
            Create
          </button>
        </footer>
      </form>
    </dialog>
  );
}
