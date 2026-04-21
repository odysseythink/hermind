import StreamingCursor from './StreamingCursor';
import MessageContent from './MessageContent';
import ToolCallCard from './ToolCallCard';
import type { ChatMessage } from '../../state/chat';
import styles from './MessageBubble.module.css';

type Props = { message: ChatMessage; streaming?: boolean };

export default function MessageBubble({ message, streaming }: Props) {
  const isUser = message.role === 'user';
  return (
    <div
      data-role={message.role}
      className={`${styles.row} ${isUser ? styles.user : styles.assistant}`}
    >
      {isUser ? (
        <>
          <div className={styles.roleTag}>you</div>
          <div className={styles.bubble}>
            <MessageContent content={message.content} />
          </div>
        </>
      ) : (
        <>
          <div className={styles.roleTag}>assistant</div>
          <div className={styles.bubble}>
            <MessageContent content={message.content} />
            {streaming && <StreamingCursor />}
            {message.toolCalls?.map((c) => (
              <ToolCallCard key={c.id} call={c} />
            ))}
          </div>
        </>
      )}
      {message.truncated && (
        <span className={styles.truncated}>— interrupted</span>
      )}
    </div>
  );
}
