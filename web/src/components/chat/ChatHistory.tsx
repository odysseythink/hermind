import { useRef, useEffect } from 'react';
import styles from './ChatHistory.module.css';
import { useScrollToBottom } from '../../hooks/useScrollToBottom';
import HistoricalMessage from './HistoricalMessage';
import PromptReply from './PromptReply';
import ScrollToBottomButton from './ScrollToBottomButton';
import type { ChatMessage, ToolCall } from '../../state/chat';

interface Props {
  messages: ChatMessage[];
  streamingDraft: string;
  streamingToolCalls: ToolCall[];
  onEdit?: (id: string, content: string) => void;
  onDelete?: (id: string) => void;
  onRegenerate?: (id: string) => void;
}

export default function ChatHistory({
  messages,
  streamingDraft,
  streamingToolCalls,
  onEdit,
  onDelete,
  onRegenerate,
}: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { isAtBottom, scrollToBottom } = useScrollToBottom(containerRef);

  useEffect(() => {
    if (isAtBottom) {
      scrollToBottom('auto');
    }
  }, [messages, streamingDraft, streamingToolCalls, isAtBottom, scrollToBottom]);

  return (
    <div className={styles.history} ref={containerRef}>
      <div className={styles.messages}>
        {messages.map((msg) => (
          <HistoricalMessage
            key={msg.id}
            message={msg}
            onEdit={onEdit}
            onDelete={onDelete}
            onRegenerate={onRegenerate}
          />
        ))}
        {(streamingDraft || streamingToolCalls.length > 0) && (
          <PromptReply draft={streamingDraft} toolCalls={streamingToolCalls} />
        )}
      </div>
      {!isAtBottom && <ScrollToBottomButton onClick={() => scrollToBottom('smooth')} />}
    </div>
  );
}
