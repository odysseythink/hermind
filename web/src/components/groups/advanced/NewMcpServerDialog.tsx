import { useState } from 'react';
import type React from 'react';
import styles from './NewMcpServerDialog.module.css';

export interface NewMcpServerDialogProps {
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewMcpServerDialog({ existingKeys, onCancel, onCreate }: NewMcpServerDialogProps) {
  const [key, setKey] = useState('');
  const [keyError, setKeyError] = useState<string | null>(null);

  const trimmed = key.trim();
  const duplicate = trimmed !== '' && existingKeys.has(trimmed);
  const formatInvalid = trimmed !== '' && !KEY_REGEX.test(trimmed);
  const disabled = trimmed === '' || duplicate || formatInvalid;

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (disabled) return;
    if (!trimmed) {
      setKeyError('Instance key is required.');
      return;
    }
    if (!KEY_REGEX.test(trimmed)) {
      setKeyError('Use lowercase letters, digits, underscore. Must start with a letter.');
      return;
    }
    if (existingKeys.has(trimmed)) {
      setKeyError(`A server named "${trimmed}" already exists.`);
      return;
    }
    onCreate(trimmed);
  }

  return (
    <div className={styles.overlay} role="dialog" aria-labelledby="newMcpTitle" aria-modal="true">
      <div className={styles.panel}>
        <h2 id="newMcpTitle">New MCP server</h2>
        <form onSubmit={onSubmit}>
          <label className={styles.row}>
            Name
            <input
              type="text"
              value={key}
              onChange={e => {
                setKey(e.currentTarget.value);
                setKeyError(null);
              }}
              placeholder="e.g. filesystem"
              autoFocus
            />
          </label>
          {duplicate && <p className={styles.err}>A server named &quot;{trimmed}&quot; already exists.</p>}
          {formatInvalid && !duplicate && (
            <p className={styles.err}>Use lowercase letters, digits, underscore. Must start with a letter.</p>
          )}
          {keyError && !duplicate && !formatInvalid && <p className={styles.err}>{keyError}</p>}
          <div className={styles.actions}>
            <button type="button" onClick={onCancel}>Cancel</button>
            <button
              type="submit"
              disabled={disabled}
            >
              Create
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
