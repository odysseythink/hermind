import { describe, expect, it } from 'vitest';
import { chatReducer, initialChatState } from './chat';

describe('chatReducer', () => {
  it('loads history', () => {
    const s = chatReducer(initialChatState, {
      type: 'chat/history/loaded',
      messages: [
        { id: '1', role: 'user', content: 'hi', timestamp: 0 },
      ],
    });
    expect(s.messages).toHaveLength(1);
  });

  it('updates composer text', () => {
    const s = chatReducer(initialChatState, { type: 'chat/composer/setText', text: 'hello' });
    expect(s.composer.text).toBe('hello');
  });

  it('appends user message on stream/start and transitions to running', () => {
    const s = chatReducer(initialChatState, { type: 'chat/stream/start', userText: 'hi' });
    expect(s.messages).toHaveLength(1);
    expect(s.messages[0].role).toBe('user');
    expect(s.streaming.status).toBe('running');
  });

  it('accumulates tokens into the assistant draft', () => {
    let s = chatReducer(initialChatState, { type: 'chat/stream/start', userText: 'hi' });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'He' });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'llo' });
    expect(s.streaming.assistantDraft).toBe('Hello');
  });

  it('completes a turn — appends assistant message and clears draft', () => {
    let s = chatReducer(initialChatState, { type: 'chat/stream/start', userText: 'hi' });
    s = chatReducer(s, { type: 'chat/stream/done', assistantText: 'hello' });
    expect(s.messages).toHaveLength(2);
    expect(s.messages[1].role).toBe('assistant');
    expect(s.streaming.status).toBe('idle');
  });

  it('tracks tool calls and their results', () => {
    let s = chatReducer(initialChatState, { type: 'chat/stream/start', userText: 'hi' });
    s = chatReducer(s, {
      type: 'chat/stream/toolCall',
      call: { id: 't1', name: 'read_file', input: { path: '/x' }, state: 'running' },
    });
    expect(s.streaming.toolCalls).toHaveLength(1);
    s = chatReducer(s, { type: 'chat/stream/toolResult', id: 't1', result: 'ok' });
    expect(s.streaming.toolCalls[0].state).toBe('done');
    expect(s.streaming.toolCalls[0].result).toBe('ok');
  });

  it('enters error state on stream/error', () => {
    const s = chatReducer(initialChatState, { type: 'chat/stream/error', message: 'boom' });
    expect(s.streaming.status).toBe('error');
    expect(s.streaming.error).toBe('boom');
  });

  it('rolls back the user message on failure', () => {
    let s = chatReducer(initialChatState, { type: 'chat/stream/start', userText: 'hi' });
    s = chatReducer(s, { type: 'chat/stream/rollbackUserMessage' });
    expect(s.messages).toHaveLength(0);
    expect(s.streaming.status).toBe('idle');
  });
});
