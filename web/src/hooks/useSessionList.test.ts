import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useSessionList } from './useSessionList';

function mockFetchOnce(payload: unknown) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: async () => payload,
    headers: new Headers({ 'content-type': 'application/json' }),
  } as Response);
}

describe('useSessionList', () => {
  beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }); });
  afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

  it('loads sessions on mount', async () => {
    mockFetchOnce({ sessions: [{ id: 's1', title: 'hi', source: 'web' }], total: 1 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    expect(result.current.sessions[0].id).toBe('s1');
  });

  it('newSession returns a uuid but does NOT insert a placeholder', async () => {
    mockFetchOnce({ sessions: [], total: 0 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));
    let id: string | undefined;
    act(() => { id = result.current.newSession(); });
    expect(id).toMatch(/^[0-9a-f-]{36}$/);
    expect(result.current.sessions.length).toBe(0);
  });

  it('insertSession adds a new row (idempotent on same id)', async () => {
    mockFetchOnce({ sessions: [], total: 0 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));
    act(() => result.current.insertSession({ id: 's-new', title: 'hey', source: 'web' }));
    expect(result.current.sessions.length).toBe(1);
    act(() => result.current.insertSession({ id: 's-new', title: 'hey', source: 'web' }));
    expect(result.current.sessions.length).toBe(1); // idempotent
  });

  it('patchSession updates fields locally', async () => {
    mockFetchOnce({ sessions: [{ id: 's1', title: 'old', source: 'web' }], total: 1 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    act(() => result.current.patchSession('s1', { title: 'new name' }));
    expect(result.current.sessions[0].title).toBe('new name');
  });

  it('polls /api/sessions every 10s and merges results', async () => {
    mockFetchOnce({ sessions: [], total: 0 });  // initial mount
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));

    mockFetchOnce({
      sessions: [{ id: 'poll-1', title: 'from gateway', source: 'telegram' }],
      total: 1,
    });
    await act(async () => { vi.advanceTimersByTime(10_000); });
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    expect(result.current.sessions[0].source).toBe('telegram');
  });

  it('refetches on window.focus', async () => {
    mockFetchOnce({ sessions: [], total: 0 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));

    mockFetchOnce({
      sessions: [{ id: 'focus-1', title: 'late arrival', source: 'feishu' }],
      total: 1,
    });
    await act(async () => { window.dispatchEvent(new Event('focus')); });
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
  });
});
