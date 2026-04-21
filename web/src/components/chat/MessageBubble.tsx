import StreamingCursor from './StreamingCursor';
import MessageContent from './MessageContent';
import ToolCallCard from './ToolCallCard';
import type { ChatMessage } from '../../state/chat';

type Props = { message: ChatMessage; streaming?: boolean };

export default function MessageBubble({ message, streaming }: Props) {
  const isUser = message.role === 'user';
  return (
    <div
      data-role={message.role}
      style={{
        textAlign: isUser ? 'right' : 'left',
        padding: '0.5rem 1rem',
      }}
    >
      <MessageContent content={message.content} />
      {streaming && <StreamingCursor />}
      {message.toolCalls?.map((c) => (
        <ToolCallCard key={c.id} call={c} />
      ))}
      {message.truncated && (
        <span style={{ color: 'var(--warn, #d29922)', fontSize: '0.85em' }}> — interrupted</span>
      )}
    </div>
  );
}
