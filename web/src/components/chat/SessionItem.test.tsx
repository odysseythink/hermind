import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SessionItem from './SessionItem';

const baseSession = { id: 's1', title: 'alpha', source: 'web' };

describe('SessionItem rename', () => {
  it('double-click enters editing state with preselected input', async () => {
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={() => {}} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    expect(input).toHaveFocus();
    expect((input as HTMLInputElement).selectionStart).toBe(0);
    expect((input as HTMLInputElement).selectionEnd).toBe('alpha'.length);
  });

  it('Enter commits: onRename called, exits editing', async () => {
    const onRename = vi.fn().mockResolvedValue(undefined);
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.type(input, 'beta');
    await userEvent.keyboard('{Enter}');
    expect(onRename).toHaveBeenCalledWith('s1', 'beta');
  });

  it('Esc cancels: onRename NOT called, exits editing', async () => {
    const onRename = vi.fn();
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.type(input, 'changed');
    await userEvent.keyboard('{Escape}');
    expect(onRename).not.toHaveBeenCalled();
    // After Esc the original title renders back in the readonly view.
    expect(screen.getByText('alpha')).toBeInTheDocument();
  });

  it('blur commits with trimmed value', async () => {
    const onRename = vi.fn().mockResolvedValue(undefined);
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.type(input, '  gamma  ');
    await userEvent.tab(); // blur
    expect(onRename).toHaveBeenCalledWith('s1', 'gamma');
  });

  it('empty title blocks save (no API call, stays in editing state)', async () => {
    const onRename = vi.fn();
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.keyboard('{Enter}');
    expect(onRename).not.toHaveBeenCalled();
    expect(input).toHaveFocus();
  });

  it('renders source badge', () => {
    render(<SessionItem session={{ ...baseSession, source: 'telegram' }} active={false} onSelect={() => {}} onRename={() => {}} />);
    expect(screen.getByText(/telegram/i)).toBeInTheDocument();
  });
});
