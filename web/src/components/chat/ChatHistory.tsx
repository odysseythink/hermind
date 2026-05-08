import { useRef, useEffect } from 'react';
import styles from './ChatHistory.module.css';
import { useScrollToBottom } from '../../hooks/useScrollToBottom';
import EmptyState from './EmptyState';
import HistoricalMessage from './HistoricalMessage';
import PromptReply from './PromptReply';
import ScrollToBottomButton from './ScrollToBottomButton';
import type { ChatMessage, ToolCall } from '../../state/chat';

interface Props {
  messages: ChatMessage[];
  streamingDraft: string;
  streamingToolCalls: ToolCall[];
  suggestions: string[];
  onSuggestionClick: (text: string) => void;
  onEdit?: (id: string, content: string) => void;
  onDelete?: (id: string) => void;
  onRegenerate?: (id: string) => void;
}

export default function ChatHistory({
  messages,
  streamingDraft,
  streamingToolCalls,
  suggestions,
  onSuggestionClick,
  onEdit,
  onDelete,
  onRegenerate,
}: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { isAtBottom, scrollToBottom } = useScrollToBottom(containerRef);

  // Auto-scroll when new content arrives if user is at bottom
  useEffect(() => {
    if (isAtBottom) {
      scrollToBottom('auto');
    }
  }, [messages, streamingDraft, streamingToolCalls, isAtBottom, scrollToBottom]);

  const isEmpty = messages.length === 0 && !streamingDraft && streamingToolCalls.length === 0;

  return (
    <div className={styles.history} ref={containerRef}>
      {isEmpty ? (
        <EmptyState suggestions={suggestions} onSuggestionClick={onSuggestionClick} />
      ) : (
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
      )}
      {!isAtBottom && <ScrollToBottomButton onClick={() => scrollToBottom('smooth')} />}
    </div>
  );
}
