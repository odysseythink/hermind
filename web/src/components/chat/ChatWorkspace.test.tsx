import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ChatWorkspace from './ChatWorkspace';
import { FakeEventSource } from '../../test/fakeEventSource';

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
        modelOptions={['', 'claude-opus-4-7', 'gpt-4']}
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

describe('ChatWorkspace — renders message history returned by GET /api/sessions/{id}/messages', () => {
  beforeEach(() => {
    FakeEventSource.install();
    FakeEventSource.reset();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('parses int64 message ids emitted as JSON numbers and renders prior turns', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
      if (url.includes('/api/sessions/telegram%3A760061130/messages')) {
        // Match the live server: ids are JSON numbers, not strings.
        return jsonResponse({
          messages: [
            { id: 1, role: 'user', content: 'hi there', timestamp: 1 },
            { id: 2, role: 'assistant', content: 'hello back', timestamp: 2 },
          ],
          total: 2,
        });
      }
      if (url.includes('/api/sessions?')) {
        return jsonResponse({ sessions: [], total: 0 });
      }
      return jsonResponse({}, 200);
    });

    render(
      <ChatWorkspace
        sessionId="telegram:760061130"
        onChangeSession={vi.fn()}
        providerConfigured={true}
        modelOptions={['', 'claude-opus-4-7', 'gpt-4']}
      />,
    );

    expect(await screen.findByText('hi there')).toBeInTheDocument();
    expect(await screen.findByText('hello back')).toBeInTheDocument();
  });
});

describe('ChatWorkspace — settings button pulls current session settings', () => {
  beforeEach(() => {
    FakeEventSource.install();
    FakeEventSource.reset();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches GET /api/sessions/{id} when the gear is clicked and hydrates the drawer with the response', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
      if (url.includes('/api/sessions/telegram%3A760061130/messages')) {
        return jsonResponse({ messages: [], total: 0 });
      }
      if (url.endsWith('/api/sessions/telegram%3A760061130')) {
        return jsonResponse({
          id: 'telegram:760061130',
          source: 'telegram',
          model: 'claude-opus-4-7',
          system_prompt: 'Ruthless Life Reviewer',
          title: 'hi',
          started_at: 1,
          message_count: 0,
        });
      }
      // Sidebar list: deliberately omit this session so we can prove the
      // drawer hydrates from the single-session GET, not the list cache.
      if (url.includes('/api/sessions?')) {
        return jsonResponse({ sessions: [], total: 0 });
      }
      return jsonResponse({}, 200);
    });

    render(
      <ChatWorkspace
        sessionId="telegram:760061130"
        onChangeSession={vi.fn()}
        providerConfigured={true}
        modelOptions={['', 'claude-opus-4-7', 'gpt-4']}
      />,
    );

    // Wait for initial renders to settle (sidebar fetch + messages fetch).
    await waitFor(() => expect(fetchSpy).toHaveBeenCalled());

    const gear = await screen.findByRole('button', { name: /session settings/i });
    await userEvent.click(gear);

    // Drawer mounted with the server's system_prompt.
    await waitFor(() => {
      const prompt = screen.getByLabelText(/system prompt/i) as HTMLTextAreaElement;
      expect(prompt.value).toBe('Ruthless Life Reviewer');
    });

    // Single-session GET was made.
    const getCalls = fetchSpy.mock.calls.filter(([input]) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : (input as Request).url;
      return url.endsWith('/api/sessions/telegram%3A760061130');
    });
    expect(getCalls.length).toBeGreaterThanOrEqual(1);
  });

  it('triggers onEnsureModelsLoaded when the gear is clicked so the dropdown populates without a Settings detour', async () => {
    // Repro: user opens chat, hits the gear, sees "默认" only because the
    // providers panel was never visited. Drawer should request models on
    // open instead of relying on a side trip through Settings.
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : (input as Request).url;
      if (url.includes('/messages')) return jsonResponse({ messages: [], total: 0 });
      if (url.endsWith('/api/sessions/sess-1')) {
        return jsonResponse({
          id: 'sess-1',
          source: 'web',
          model: 'claude-opus-4-6',
          system_prompt: 'p',
          title: 't',
          started_at: 1,
          message_count: 0,
        });
      }
      if (url.includes('/api/sessions?')) return jsonResponse({ sessions: [], total: 0 });
      return jsonResponse({}, 200);
    });

    const onEnsureModelsLoaded = vi.fn().mockResolvedValue(undefined);
    render(
      <ChatWorkspace
        sessionId="sess-1"
        onChangeSession={vi.fn()}
        providerConfigured={true}
        modelOptions={['']}
        onEnsureModelsLoaded={onEnsureModelsLoaded}
      />,
    );

    const gear = await screen.findByRole('button', { name: /session settings/i });
    await userEvent.click(gear);

    await waitFor(() => expect(onEnsureModelsLoaded).toHaveBeenCalled());
  });
});
