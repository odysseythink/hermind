import styles from './PromptReply.module.css';
import MessageContent from './MessageContent';
import type { ToolCall } from '../../state/chat';

interface Props {
  draft: string;
  toolCalls: ToolCall[];
}

export default function PromptReply({ draft, toolCalls }: Props) {
  const hasRunningTool = toolCalls.some((t) => t.state === 'running');

  return (
    <div className={styles.reply}>
      <div className={styles.avatar} aria-label="Assistant avatar">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="10" />
          <path d="M12 16v-4M12 8h.01" />
        </svg>
      </div>
      <div className={styles.bubbleWrapper}>
        <div className={styles.bubble}>
          {draft ? (
            <MessageContent content={draft} />
          ) : hasRunningTool ? (
            <span className={styles.typing}>Running tool...</span>
          ) : (
            <span className={styles.typing}>Thinking...</span>
          )}
        </div>
        {toolCalls.length > 0 && (
          <div className={styles.toolCalls}>
            {toolCalls.map((tc) => (
              <div key={tc.id} className={styles.toolCall}>
                <span className={styles.toolName}>{tc.name}</span>
                <span className={styles.toolState}>{tc.state}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
