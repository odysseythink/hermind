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

describe('chatReducer — new actions', () => {
  it('chat/message/edit updates content and sets pending false', () => {
    const state = {
      ...initialChatState,
      messages: [
        { id: '1', role: 'user', content: 'old', timestamp: 0, chatId: 100 },
      ],
    };
    const next = chatReducer(state, {
      type: 'chat/message/edit',
      id: '1',
      content: 'new',
    });
    expect(next.messages[0].content).toBe('new');
    expect(next.messages[0].pending).toBe(false);
  });

  it('chat/message/delete removes the message', () => {
    const state = {
      ...initialChatState,
      messages: [
        { id: '1', role: 'user', content: 'hi', timestamp: 0 },
        { id: '2', role: 'assistant', content: 'hello', timestamp: 1 },
      ],
    };
    const next = chatReducer(state, {
      type: 'chat/message/delete',
      id: '1',
    });
    expect(next.messages).toHaveLength(1);
    expect(next.messages[0].id).toBe('2');
  });

  it('chat/message/regenerate clears assistant reply and sets streaming', () => {
    const state = {
      ...initialChatState,
      messages: [
        { id: '1', role: 'user', content: 'q', timestamp: 0 },
        { id: '2', role: 'assistant', content: 'a', timestamp: 1 },
      ],
    };
    const next = chatReducer(state, {
      type: 'chat/message/regenerate',
      id: '2',
    });
    expect(next.messages).toHaveLength(1);
    expect(next.streaming.status).toBe('running');
  });

  it('chat/composer/setAttachments replaces attachment list', () => {
    const next = chatReducer(initialChatState, {
      type: 'chat/composer/setAttachments',
      attachments: [{ id: 'a1', name: 'file.txt', type: 'text/plain', url: '/uploads/a1', size: 12 }],
    });
    expect(next.composer.attachments).toHaveLength(1);
    expect(next.composer.attachments[0].name).toBe('file.txt');
  });

  it('chat/suggestions/loaded replaces suggestions', () => {
    const next = chatReducer(initialChatState, {
      type: 'chat/suggestions/loaded',
      suggestions: ['What can you do?', 'Explain this code'],
    });
    expect(next.suggestions).toEqual(['What can you do?', 'Explain this code']);
  });
});
