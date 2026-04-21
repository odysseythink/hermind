import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';
import { FakeEventSource } from './test/fakeEventSource';

function mockBoot() {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : (input as Request).url;
    if (url.endsWith('/api/platforms/schema')) {
      return jsonResponse({ descriptors: [] });
    }
    if (url.endsWith('/api/config/schema')) {
      return jsonResponse({
        sections: [
          {
            key: 'storage',
            label: 'Storage',
            summary: 'Where hermind keeps data.',
            group_id: 'runtime',
            fields: [
              { name: 'driver', label: 'Driver', kind: 'enum',
                required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
              { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
                visible_when: { field: 'driver', equals: 'sqlite' } },
              { name: 'postgres_url', label: 'Postgres URL', kind: 'secret',
                visible_when: { field: 'driver', equals: 'postgres' } },
            ],
          },
        ],
      });
    }
    if (url.endsWith('/api/config') && (!input || !(input as Request).method || (input as Request).method === 'GET')) {
      return jsonResponse({
        config: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x' } },
      });
    }
    return jsonResponse({}, 200);
  });
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

describe('App integration — storage vertical slice', () => {
  beforeEach(() => {
    window.location.hash = '#runtime/storage';
    mockBoot();
  });
  afterEach(() => {
    vi.restoreAllMocks();
    window.location.hash = '';
  });

  it('renders the storage editor on #runtime/storage and flips fields on driver change', async () => {
    const user = userEvent.setup();
    render(<App />);

    // Boot → sidebar + storage fields appear.
    await waitFor(() => {
      expect(screen.getByLabelText(/driver/i)).toBeInTheDocument();
    });
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/postgres url/i)).not.toBeInTheDocument();

    // Flip driver → postgres.
    const driver = screen.getByLabelText(/driver/i) as HTMLSelectElement;
    await user.selectOptions(driver, 'postgres');

    expect(screen.queryByLabelText(/sqlite path/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText(/postgres url/i)).toBeInTheDocument();

    // TopBar badge shows one change.
    expect(screen.getByRole('button', { name: /save · 1 changes/i })).toBeInTheDocument();
  });
});

describe('App integration — chat mode', () => {
  beforeEach(() => {
    window.location.hash = '#/chat/s1';
    FakeEventSource.install();
    FakeEventSource.reset();
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : (input as Request).url;
      if (url.endsWith('/api/platforms/schema')) return jsonResponse({ descriptors: [] });
      if (url.endsWith('/api/config/schema')) return jsonResponse({ sections: [] });
      if (url.endsWith('/api/config')) {
        return jsonResponse({ config: { providers: { anthropic: { api_key: 'sk-test' } } } });
      }
      if (url.includes('/api/sessions?limit=50')) return jsonResponse({ sessions: [] });
      if (url.match(/\/api\/sessions\/[^/]+\/messages$/)) {
        const method = input instanceof Request ? input.method : 'GET';
        if (method === 'POST') return jsonResponse({ session_id: 's1', status: 'accepted' }, 202);
        return jsonResponse({ messages: [] });
      }
      return jsonResponse({}, 200);
    });
  });
  afterEach(() => {
    vi.restoreAllMocks();
    window.location.hash = '';
  });

  it('renders the chat composer in chat mode', async () => {
    render(<App />);
    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Type a message/i)).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /^Send$/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Chat' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('streamed tokens render the assistant draft', async () => {
    render(<App />);
    await waitFor(() => expect(FakeEventSource.instances.length).toBeGreaterThan(0));
    const es = FakeEventSource.instances.at(-1)!;
    es.dispatchMessage({ type: 'status', session_id: 's1', data: { state: 'running' } });
    es.dispatchMessage({ type: 'token', session_id: 's1', data: { text: 'Hello' } });
    // token dispatch is throttled via requestAnimationFrame; wait a frame
    await new Promise((r) => requestAnimationFrame(() => r(null)));
    await waitFor(() => {
      expect(screen.getByText('Hello')).toBeInTheDocument();
    });
  });
});
