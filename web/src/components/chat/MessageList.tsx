import { useEffect, useRef } from 'react';
import MessageBubble from './MessageBubble';
import MessageContent from './MessageContent';
import StreamingCursor from './StreamingCursor';
import ToolCallCard from './ToolCallCard';
import type { ChatMessage, ToolCallSnapshot } from '../../state/chat';

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
      // jsdom: no scrollTo; fall back to direct assignment.
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, streamingDraft]);

  const showStreamingBubble =
    streamingSessionId === activeSessionId && (!!streamingDraft || streamingToolCalls.length > 0);

  return (
    <div ref={ref} style={{ flex: 1, overflowY: 'auto' }}>
      {messages.map((m) => (
        <MessageBubble key={m.id} message={m} />
      ))}
      {showStreamingBubble && (
        <div data-role="assistant" style={{ padding: '0.5rem 1rem' }}>
          {streamingDraft && <MessageContent content={streamingDraft} />}
          <StreamingCursor />
          {streamingToolCalls.map((c) => (
            <ToolCallCard key={c.id} call={c} />
          ))}
        </div>
      )}
    </div>
  );
}
