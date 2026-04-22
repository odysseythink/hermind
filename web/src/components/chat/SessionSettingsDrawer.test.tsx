import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import SessionSettingsDrawer from './SessionSettingsDrawer';
import type { SessionSummary } from '../../api/schemas';

const session: SessionSummary = {
  id: 'sess-1',
  source: 'web',
  title: 't',
  model: 'claude-opus-4-7',
  system_prompt: 'orig prompt',
};

describe('SessionSettingsDrawer', () => {
  it('renders current model and system prompt when open', () => {
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['', 'claude-opus-4-7', 'claude-sonnet-4-6']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByRole('combobox')).toHaveValue('claude-opus-4-7');
    expect(screen.getByRole('textbox')).toHaveValue('orig prompt');
  });

  it('does not render when closed', () => {
    render(
      <SessionSettingsDrawer
        open={false}
        session={session}
        modelOptions={['']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('calls onSave with only the fields that changed', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['', 'claude-opus-4-7', 'claude-sonnet-4-6']}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );
    fireEvent.change(screen.getByRole('combobox'), {
      target: { value: 'claude-sonnet-4-6' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await Promise.resolve(); // flush microtask

    expect(onSave).toHaveBeenCalledWith({ model: 'claude-sonnet-4-6' });
  });

  it('cancel discards draft and calls onClose', () => {
    const onClose = vi.fn();
    const onSave = vi.fn();
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['']}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'changed' },
    });
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it('Esc key closes the drawer', () => {
    const onClose = vi.fn();
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['']}
        onClose={onClose}
        onSave={vi.fn()}
      />,
    );
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
    expect(onClose).toHaveBeenCalled();
  });

  it('shows conflict banner when session prop updates while drawer is open with unsaved draft', () => {
    const { rerender } = render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'local draft' },
    });
    rerender(
      <SessionSettingsDrawer
        open
        session={{ ...session, system_prompt: 'remote change' }}
        modelOptions={['']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByText(/updated in another/i)).toBeInTheDocument();
  });
});
