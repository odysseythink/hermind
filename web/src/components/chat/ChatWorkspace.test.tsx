import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ChatWorkspace from './ChatWorkspace';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

function mockFetchOnce(payload: unknown) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: async () => payload,
    headers: new Headers({ 'content-type': 'application/json' }),
  } as Response);
}

describe('ChatWorkspace — "+ New conversation" does not insert a placeholder row', () => {
  beforeEach(() => {
    // Catch-all fetch for anything beyond the initial /api/sessions call.
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => jsonResponse({}, 200));
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('clicking "+ New conversation" routes to a fresh sessionId but does NOT insert into the sidebar', async () => {
    // Override the first call (the initial /api/sessions fetch) so the sidebar starts empty.
    mockFetchOnce({ sessions: [], total: 0 });

    const onChangeSession = vi.fn();
    render(
      <ChatWorkspace
        sessionId={null}
        onChangeSession={onChangeSession}
        providerConfigured={true}
      />,
    );

    // Wait for initial session list fetch to settle; sidebar remains row-less.
    await waitFor(() => {
      const sidebar = screen.getByRole('complementary');
      expect(within(sidebar).queryAllByRole('listitem')).toHaveLength(0);
    });

    const btn = screen.getByRole('button', { name: /\+ New conversation/i });
    await userEvent.click(btn);

    // onChangeSession called with a fresh UUID.
    expect(onChangeSession).toHaveBeenCalledTimes(1);
    expect(onChangeSession).toHaveBeenCalledWith(
      expect.stringMatching(/^[0-9a-f-]{36}$/),
    );

    // Sidebar still has no rows — no optimistic placeholder was inserted.
    const sidebar = screen.getByRole('complementary');
    expect(within(sidebar).queryAllByRole('listitem')).toHaveLength(0);
  });
});
