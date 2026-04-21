import { useEffect, useRef } from 'react';
import MessageBubble from './MessageBubble';
import MessageContent from './MessageContent';
import StreamingCursor from './StreamingCursor';
import ToolCallCard from './ToolCallCard';
import type { ChatMessage, ToolCallSnapshot } from '../../state/chat';
import styles from './MessageList.module.css';
import bubble from './MessageBubble.module.css';

type Props = {
  messages: ChatMessage[];
  streamingDraft: string;
  streamingToolCalls: ToolCallSnapshot[];
  streamingSessionId: string | null;
  activeSessionId: string | null;
};

export default function MessageList({
  messages, streamingDraft, streamingToolCalls, streamingSessionId, activeSessionId,
}: Props) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (typeof el.scrollTo === 'function') {
      el.scrollTo({ top: el.scrollHeight });
    } else {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, streamingDraft]);

  const showStreamingBubble =
    streamingSessionId === activeSessionId && (!!streamingDraft || streamingToolCalls.length > 0);

  return (
    <div ref={ref} className={styles.list}>
      {messages.map((m) => (
        <MessageBubble key={m.id} message={m} />
      ))}
      {showStreamingBubble && (
        <div data-role="assistant" className={`${bubble.row} ${bubble.assistant}`}>
          <div className={bubble.roleTag}>assistant</div>
          <div className={bubble.bubble}>
            {streamingDraft && <MessageContent content={streamingDraft} />}
            <StreamingCursor />
            {streamingToolCalls.map((c) => (
              <ToolCallCard key={c.id} call={c} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
