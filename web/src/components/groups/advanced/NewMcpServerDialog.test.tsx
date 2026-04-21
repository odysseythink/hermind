import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import NewMcpServerDialog from './NewMcpServerDialog';

function baseProps(overrides: Partial<React.ComponentProps<typeof NewMcpServerDialog>> = {}) {
  return {
    existingKeys: new Set<string>(),
    onCancel: vi.fn(),
    onCreate: vi.fn(),
    ...overrides,
  };
}

import type React from 'react';

describe('NewMcpServerDialog', () => {
  it('renders with an empty name input', () => {
    render(<NewMcpServerDialog {...baseProps()} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    expect(input).toBeInTheDocument();
    expect((input as HTMLInputElement).value).toBe('');
  });

  it('Create button is disabled when name is empty', () => {
    render(<NewMcpServerDialog {...baseProps()} />);
    expect(screen.getByRole('button', { name: /create/i })).toBeDisabled();
  });

  it('typing a name enables Create button and click fires onCreate with trimmed name', async () => {
    const onCreate = vi.fn();
    render(<NewMcpServerDialog {...baseProps({ onCreate })} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    await userEvent.type(input, '  myserver  ');
    const btn = screen.getByRole('button', { name: /create/i });
    expect(btn).not.toBeDisabled();
    await userEvent.click(btn);
    expect(onCreate).toHaveBeenCalledWith('myserver');
  });

  it('shows duplicate error and disables button when name matches existingKeys', async () => {
    render(<NewMcpServerDialog {...baseProps({ existingKeys: new Set(['filesystem']) })} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    await userEvent.type(input, 'filesystem');
    expect(screen.getByText(/already exists/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create/i })).toBeDisabled();
  });

  it('Cancel button fires onCancel', async () => {
    const onCancel = vi.fn();
    render(<NewMcpServerDialog {...baseProps({ onCancel })} />);
    await userEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalled();
  });
});
