import { describe, it, expect, vi, beforeAll, afterAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import ChatHistory from './ChatHistory';
import type { ChatMessage } from '../../state/chat';

describe('ChatHistory', () => {
  const originalScrollTo = HTMLElement.prototype.scrollTo;

  beforeAll(() => {
    HTMLElement.prototype.scrollTo = vi.fn();
  });

  afterAll(() => {
    HTMLElement.prototype.scrollTo = originalScrollTo;
  });

  it('renders EmptyState when no messages and not streaming', () => {
    render(
      <ChatHistory
        messages={[]}
        streamingDraft=""
        streamingToolCalls={[]}
        suggestions={['Hi']}
        onSuggestionClick={vi.fn()}
      />
    );
    expect(screen.getByText('How can I help you today?')).toBeInTheDocument();
  });

  it('renders messages when provided', () => {
    const messages: ChatMessage[] = [
      { id: '1', role: 'user', content: 'hello', timestamp: 0 },
      { id: '2', role: 'assistant', content: 'world', timestamp: 1 },
    ];
    render(
      <ChatHistory
        messages={messages}
        streamingDraft=""
        streamingToolCalls={[]}
        suggestions={[]}
        onSuggestionClick={vi.fn()}
      />
    );
    expect(screen.getByText('hello')).toBeInTheDocument();
    expect(screen.getByText('world')).toBeInTheDocument();
  });
});
