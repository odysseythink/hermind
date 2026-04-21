import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useSessionList } from './useSessionList';

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('useSessionList', () => {
  it('loads sessions via GET /api/sessions', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(
        JSON.stringify({ sessions: [{ id: 's1', title: 'First' }] }),
        { status: 200, headers: { 'content-type': 'application/json' } },
      ),
    );
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    expect(result.current.sessions[0].id).toBe('s1');
  });

  it('handles fetch errors gracefully', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('network'));
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.error).toBeTruthy());
  });

  it('newSession generates uuid and prepends', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ sessions: [] }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    );
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));
    let created = '';
    act(() => { created = result.current.newSession(); });
    expect(result.current.sessions[0].id).toBe(created);
    expect(created).toMatch(/^[0-9a-f-]{36}$/);
  });
});
