import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { FakeEventSource } from '../test/fakeEventSource';
import { useChatStream } from './useChatStream';

beforeEach(() => {
  FakeEventSource.install();
  FakeEventSource.reset();
});

afterEach(() => vi.clearAllMocks());

describe('useChatStream', () => {
  it('subscribes on mount, closes on unmount', () => {
    const dispatch = vi.fn();
    const { unmount } = renderHook(() => useChatStream(dispatch));
    expect(FakeEventSource.instances.length).toBe(1);
    unmount();
    expect(FakeEventSource.instances[0].readyState).toBe(2);
  });

  it('dispatches stream/token on message_chunk event', async () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream(dispatch));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'message_chunk', data: { text: 'Hi' },
      });
    });
    await new Promise((r) => requestAnimationFrame(() => r(null)));
    expect(dispatch).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'chat/stream/token', delta: 'Hi' }),
    );
  });

  it('dispatches stream/toolCall on tool_call event', async () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream(dispatch));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'tool_call',
        data: { id: 't1', name: 'read_file', input: { path: '/x' } },
      });
    });
    expect(dispatch).toHaveBeenCalledWith({
      type: 'chat/stream/toolCall',
      call: expect.objectContaining({ id: 't1', name: 'read_file', state: 'running' }),
    });
  });

  it('dispatches stream/toolResult on tool_result event', async () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream(dispatch));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'tool_result',
        data: { id: 't1', result: 'ok' },
      });
    });
    expect(dispatch).toHaveBeenCalledWith({
      type: 'chat/stream/toolResult', id: 't1', result: 'ok',
    });
  });

  it('dispatches stream/done on done event', async () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream(dispatch));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({ type: 'done' });
    });
    expect(dispatch).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'chat/stream/done' }),
    );
  });

  it('dispatches stream/error on error event', async () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream(dispatch));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'error', data: { message: 'boom' },
      });
    });
    expect(dispatch).toHaveBeenCalledWith({
      type: 'chat/stream/error', message: 'boom',
    });
  });
});
