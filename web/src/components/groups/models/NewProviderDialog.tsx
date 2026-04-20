import { useEffect, useRef, useState } from 'react';
import type React from 'react';
import styles from './NewProviderDialog.module.css';

export interface NewProviderDialogProps {
  providerTypes: readonly string[];
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string, providerType: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewProviderDialog({
  providerTypes,
  existingKeys,
  onCancel,
  onCreate,
}: NewProviderDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [key, setKey] = useState('');
  const [providerType, setProviderType] = useState(providerTypes[0] ?? '');
  const [keyError, setKeyError] = useState<string | null>(null);

  useEffect(() => {
    const d = dialogRef.current;
    if (!d) return;
    // showModal() is the native way to open a <dialog>; jsdom does not
    // always implement it, so we also set `open` manually to keep the
    // dialog's children in the accessibility tree for testing.
    if (typeof d.showModal === 'function') {
      try {
        d.showModal();
      } catch {
        d.setAttribute('open', '');
      }
    } else {
      d.setAttribute('open', '');
    }
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
    if (!providerType) {
      setKeyError('Pick a provider type.');
      return;
    }
    onCreate(trimmed, providerType);
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
          <h2 className={styles.title}>New provider</h2>
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
            <label className={styles.label} htmlFor="new-provider-type">
              Provider type
            </label>
            <select
              id="new-provider-type"
              className={styles.select}
              value={providerType}
              onChange={e => setProviderType(e.currentTarget.value)}
            >
              {providerTypes.map(t => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-provider-key">
              Instance key
            </label>
            <input
              id="new-provider-key"
              className={styles.input}
              value={key}
              placeholder="anthropic_main"
              autoFocus
              onChange={e => {
                setKey(e.currentTarget.value);
                setKeyError(null);
              }}
            />
            <span className={styles.hint}>
              Identifier under <code>providers.*</code>. Lowercase, underscores.
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
