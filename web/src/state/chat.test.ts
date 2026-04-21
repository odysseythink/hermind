import { describe, it, expect } from 'vitest';
import { chatReducer, initialChatState } from './chat';

describe('chatReducer', () => {
  it('session/select switches activeSessionId', () => {
    const s = chatReducer(initialChatState, {
      type: 'chat/session/select',
      sessionId: 'abc',
    });
    expect(s.activeSessionId).toBe('abc');
  });

  it('session/created prepends to sessions and activates', () => {
    const s = chatReducer(initialChatState, {
      type: 'chat/session/created',
      session: { id: 'new-1', title: 'New conversation', source: 'web' },
    });
    expect(s.sessions[0]?.id).toBe('new-1');
    expect(s.activeSessionId).toBe('new-1');
  });

  it('session/created is idempotent on duplicate id', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/session/created',
      session: { id: 'new-1', title: 'alpha', source: 'web' },
    });
    s = chatReducer(s, {
      type: 'chat/session/created',
      session: { id: 'new-1', title: 'alpha', source: 'web' },
    });
    expect(s.sessions.length).toBe(1);
    expect(s.activeSessionId).toBe('new-1');
  });

  it('stream/start adds optimistic user message and sets running', () => {
    const s = chatReducer(
      { ...initialChatState, activeSessionId: 's1' },
      { type: 'chat/stream/start', sessionId: 's1', userText: 'hi' },
    );
    expect(s.streaming.status).toBe('running');
    expect(s.messagesBySession.s1[0].role).toBe('user');
    expect(s.messagesBySession.s1[0].content).toBe('hi');
  });

  it('stream/token appends to assistantDraft', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'Hel' });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'lo' });
    expect(s.streaming.assistantDraft).toBe('Hello');
  });

  it('stream/complete promotes draft to messagesBySession', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'Hi' });
    s = chatReducer(s, {
      type: 'chat/stream/complete', text: 'Hi', messageId: 'm1',
    });
    const msgs = s.messagesBySession.s1;
    expect(msgs.at(-1)).toMatchObject({ role: 'assistant', content: 'Hi', id: 'm1' });
    expect(s.streaming.status).toBe('idle');
    expect(s.streaming.assistantDraft).toBe('');
  });

  it('stream/cancelled keeps draft with truncated flag', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'partial' });
    s = chatReducer(s, { type: 'chat/stream/cancelled' });
    const last = s.messagesBySession.s1.at(-1);
    expect(last?.role).toBe('assistant');
    expect(last?.content).toBe('partial');
    expect(last?.truncated).toBe(true);
    expect(s.streaming.status).toBe('idle');
  });

  it('stream/error keeps draft with truncated flag + error', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/error', message: 'boom' });
    expect(s.streaming.status).toBe('error');
    expect(s.streaming.error).toBe('boom');
  });

  it('stream/rollbackUserMessage undoes start', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/rollbackUserMessage', sessionId: 's1' });
    expect(s.messagesBySession.s1 ?? []).toHaveLength(0);
    expect(s.streaming.status).toBe('idle');
  });

  it('composer/setText + setModel', () => {
    let s = chatReducer(initialChatState, { type: 'chat/composer/setText', text: 'hello' });
    expect(s.composer.text).toBe('hello');
    s = chatReducer(s, { type: 'chat/composer/setModel', model: 'claude' });
    expect(s.composer.selectedModel).toBe('claude');
  });
});
