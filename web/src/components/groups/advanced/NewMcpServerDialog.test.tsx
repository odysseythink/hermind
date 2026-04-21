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

  it('typing an invalid key (uppercase, space) shows format error and disables Create', async () => {
    const onCreate = vi.fn();
    render(<NewMcpServerDialog {...baseProps({ onCreate })} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    await userEvent.type(input, 'Bad Key');
    expect(screen.getByText(/lowercase letters, digits, underscore/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create/i })).toBeDisabled();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it('typing a valid key like fs_local enables Create button', async () => {
    const onCreate = vi.fn();
    render(<NewMcpServerDialog {...baseProps({ onCreate })} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    await userEvent.type(input, 'fs_local');
    expect(screen.queryByText(/lowercase letters, digits, underscore/i)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create/i })).not.toBeDisabled();
  });

  it('pressing Enter in a valid name input triggers onCreate', async () => {
    const onCreate = vi.fn();
    render(<NewMcpServerDialog {...baseProps({ onCreate })} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    await userEvent.type(input, 'filesystem');
    await userEvent.keyboard('{Enter}');
    expect(onCreate).toHaveBeenCalledWith('filesystem');
  });

  it('pressing Enter with an empty name does NOT trigger onCreate', async () => {
    const onCreate = vi.fn();
    render(<NewMcpServerDialog {...baseProps({ onCreate })} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    await userEvent.click(input);
    await userEvent.keyboard('{Enter}');
    expect(onCreate).not.toHaveBeenCalled();
  });

  it('pressing Enter with a duplicate name does NOT trigger onCreate', async () => {
    const onCreate = vi.fn();
    render(<NewMcpServerDialog {...baseProps({ existingKeys: new Set(['filesystem']), onCreate })} />);
    const input = screen.getByPlaceholderText(/filesystem/i);
    await userEvent.type(input, 'filesystem');
    await userEvent.keyboard('{Enter}');
    expect(onCreate).not.toHaveBeenCalled();
  });
});
