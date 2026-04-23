import { useState } from 'react';
import type { ToolCall } from '../../state/chat';
import styles from './ToolCallCard.module.css';

type Props = { call: ToolCall };

export default function ToolCallCard({ call }: Props) {
  const [open, setOpen] = useState(false);
  return (
    <div className={styles.card}>
      <button type="button" className={styles.head} onClick={() => setOpen((o) => !o)}>
        <span className={styles.caret}>{open ? '▼' : '▶'}</span>
        <span>tool:</span>
        <span className={styles.name}>{call.name}</span>
        <span className={styles.state}>({call.state})</span>
      </button>
      {open && (
        <div className={styles.body}>
          <div>
            <label>input:</label>
            <code>{JSON.stringify(call.input)}</code>
          </div>
          {call.result && (
            <div>
              <label>result:</label>
              <pre>{call.result}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
