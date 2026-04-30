import styles from './HistoricalMessage.module.css';
import MessageActions from './MessageActions';
import MessageContent from './MessageContent';
import type { ChatMessage } from '../../state/chat';

interface Props {
  message: ChatMessage;
  onEdit?: (id: string, content: string) => void;
  onDelete?: (id: string) => void;
  onRegenerate?: (id: string) => void;
}

export default function HistoricalMessage({ message, onEdit, onDelete, onRegenerate }: Props) {
  const isUser = message.role === 'user';

  return (
    <div className={`${styles.message} ${isUser ? styles.user : styles.assistant}`}>
      {!isUser && (
        <div className={styles.avatar} aria-label="Assistant avatar">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="10" />
            <path d="M12 16v-4M12 8h.01" />
          </svg>
        </div>
      )}
      <div className={styles.bubbleWrapper}>
        <div className={styles.bubble}>
          <MessageContent content={message.content} />
        </div>
        <MessageActions
          messageId={message.id}
          role={message.role}
          onCopy={() => navigator.clipboard?.writeText(message.content)}
          onEdit={onEdit ? () => onEdit(message.id, message.content) : undefined}
          onDelete={onDelete ? () => onDelete(message.id) : undefined}
          onRegenerate={onRegenerate ? () => onRegenerate(message.id) : undefined}
        />
      </div>
    </div>
  );
}
