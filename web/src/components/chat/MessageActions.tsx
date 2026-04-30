import styles from './MessageActions.module.css';

interface Props {
  messageId: string;
  role: string;
  visible?: boolean;
  onCopy?: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
  onRegenerate?: () => void;
}

export default function MessageActions({ messageId: _messageId, role, visible = true, onCopy, onEdit, onDelete, onRegenerate }: Props) {
  return (
    <div className={styles.actions} style={{ opacity: visible ? 1 : undefined }}>
      {onCopy && (
        <button className={styles.actionBtn} onClick={onCopy} aria-label="Copy message">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <rect x="9" y="9" width="13" height="13" rx="2" />
            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
          </svg>
        </button>
      )}
      {role === 'user' && onEdit && (
        <button className={styles.actionBtn} onClick={onEdit} aria-label="Edit message">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" />
            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" />
          </svg>
        </button>
      )}
      {onDelete && (
        <button className={styles.actionBtn} onClick={onDelete} aria-label="Delete message">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="3 6 5 6 21 6" />
            <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2" />
          </svg>
        </button>
      )}
      {role === 'assistant' && onRegenerate && (
        <button className={styles.actionBtn} onClick={onRegenerate} aria-label="Regenerate response">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="1 4 1 10 7 10" />
            <path d="M3.51 15a9 9 0 102.13-9.36L1 10" />
          </svg>
        </button>
      )}
    </div>
  );
}
