import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

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
