import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { FakeEventSource } from '../test/fakeEventSource';
import { useChatStream } from './useChatStream';

beforeEach(() => {
  FakeEventSource.install();
  FakeEventSource.reset();
});

describe('useChatStream', () => {
  it('subscribes on mount, closes on unmount', () => {
    const dispatch = vi.fn();
    const { unmount } = renderHook(() => useChatStream('s1', dispatch));
    expect(FakeEventSource.instances.length).toBe(1);
    unmount();
    expect(FakeEventSource.instances[0].readyState).toBe(2);
  });

  it('reconnects on activeSessionId change', () => {
    const dispatch = vi.fn();
    const { rerender } = renderHook(({ id }) => useChatStream(id, dispatch), {
      initialProps: { id: 's1' as string | null },
    });
    expect(FakeEventSource.instances.length).toBe(1);
    rerender({ id: 's2' });
    expect(FakeEventSource.instances.length).toBe(2);
    expect(FakeEventSource.instances[0].readyState).toBe(2);
  });

  it('dispatches stream/token on token event', async () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream('s1', dispatch));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'token', session_id: 's1', data: { text: 'Hi' },
      });
    });
    await new Promise((r) => requestAnimationFrame(() => r(null)));
    expect(dispatch).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'chat/stream/token', delta: 'Hi' }),
    );
  });

  it('filters events for stale session_id', () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream('s1', dispatch));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'token', session_id: 'OTHER', data: { text: 'stale' },
      });
    });
    expect(dispatch).not.toHaveBeenCalled();
  });

  it('dispatches chat/session/created on session_created SSE event', async () => {
    const dispatch = vi.fn();
    const onSessionCreated = vi.fn();
    renderHook(() => useChatStream('s-new', dispatch, onSessionCreated));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'session_created',
        session_id: 's-new',
        data: {
          id: 's-new',
          title: 'Build me ',
          source: 'web',
          model: 'm',
          started_at: 1713724000,
          ended_at: 0,
          message_count: 1,
        },
      });
    });
    expect(dispatch).toHaveBeenCalledWith({
      type: 'chat/session/created',
      session: expect.objectContaining({ id: 's-new', title: 'Build me ' }),
    });
    expect(onSessionCreated).toHaveBeenCalledWith(
      expect.objectContaining({ id: 's-new', title: 'Build me ' }),
    );
  });
});

describe('useChatStream session_updated', () => {
  afterEach(() => vi.clearAllMocks());

  it('invokes onSessionUpdated when a session_updated event arrives', async () => {
    const dispatch = vi.fn();
    const onSessionUpdated = vi.fn();
    renderHook(() =>
      useChatStream('sess-1', dispatch, undefined, onSessionUpdated),
    );
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'session_updated',
        session_id: 'sess-1',
        data: { model: 'claude-sonnet-4-6', system_prompt: 'new', title: 't' },
      });
    });

    expect(onSessionUpdated).toHaveBeenCalledWith('sess-1', {
      model: 'claude-sonnet-4-6',
      system_prompt: 'new',
      title: 't',
    });
  });
});
