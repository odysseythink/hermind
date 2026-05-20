import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import BrowserControlConfig from './BrowserControlConfig';

const baseProps = {
  name: 'browser_control',
  description: 'Control the browser via extension',
  enabled: true,
  onToggle: vi.fn(),
  onSectionField: vi.fn(),
  config: {},
};

describe('BrowserControlConfig', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders header with emoji and toggle', () => {
    render(<BrowserControlConfig {...baseProps} />);
    expect(screen.getByText('browser_control')).toBeInTheDocument();
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('dispatches onToggle when switch is clicked', () => {
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByRole('switch'));
    expect(baseProps.onToggle).toHaveBeenCalledWith(false);
  });

  it('shows unknown status initially', () => {
    render(<BrowserControlConfig {...baseProps} />);
    expect(screen.getByTestId('status-unknown')).toBeInTheDocument();
  });

  it('updates status on test connection success', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ connected: true, version: '1.2.0' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByTestId('test-connection-btn'));
    await waitFor(() => expect(screen.getByTestId('status-connected')).toBeInTheDocument());
    expect(screen.getByText('Extension version 1.2.0')).toBeInTheDocument();
  });

  it('updates status on test connection failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ connected: false, error: 'Extension not installed' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByTestId('test-connection-btn'));
    await waitFor(() => expect(screen.getByTestId('status-error')).toBeInTheDocument());
    expect(screen.getByText('Extension not installed')).toBeInTheDocument();
  });

  it('shows install guide when connection fails', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ connected: false, error: 'Extension not installed' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByTestId('test-connection-btn'));
    await waitFor(() => expect(screen.getByTestId('status-error')).toBeInTheDocument());
    expect(screen.getByTestId('install-guide')).toBeInTheDocument();
  });
});
