import { useState } from 'react';
import styles from './NewMcpServerDialog.module.css';

export interface NewMcpServerDialogProps {
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string) => void;
}

export default function NewMcpServerDialog({ existingKeys, onCancel, onCreate }: NewMcpServerDialogProps) {
  const [key, setKey] = useState('');
  const trimmed = key.trim();
  const duplicate = trimmed !== '' && existingKeys.has(trimmed);
  const disabled = trimmed === '' || duplicate;

  return (
    <div className={styles.overlay} role="dialog" aria-labelledby="newMcpTitle" aria-modal="true">
      <div className={styles.panel}>
        <h2 id="newMcpTitle">New MCP server</h2>
        <label className={styles.row}>
          Name
          <input
            type="text"
            value={key}
            onChange={e => setKey(e.currentTarget.value)}
            placeholder="e.g. filesystem"
            autoFocus
          />
        </label>
        {duplicate && <p className={styles.err}>A server named &quot;{trimmed}&quot; already exists.</p>}
        <div className={styles.actions}>
          <button type="button" onClick={onCancel}>Cancel</button>
          <button
            type="button"
            disabled={disabled}
            onClick={() => onCreate(trimmed)}
          >
            Create
          </button>
        </div>
      </div>
    </div>
  );
}
