import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import TopBar from './TopBar';

describe('TopBar', () => {
  it('shows the brand label and "All saved" when clean', () => {
    render(<TopBar dirtyCount={0} status="ready" onSave={() => {}} />);
    expect(screen.getByText(/hermind/i)).toBeInTheDocument();
    expect(screen.getByText(/all saved/i)).toBeInTheDocument();
  });

  it('shows dirty count and enables Save when dirty', () => {
    render(<TopBar dirtyCount={3} status="ready" onSave={() => {}} />);
    const btn = screen.getByRole('button', { name: /save/i });
    expect(btn).toBeEnabled();
    expect(btn).toHaveTextContent(/3 changes/);
  });

  it('disables Save when clean', () => {
    render(<TopBar dirtyCount={0} status="ready" onSave={() => {}} />);
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
  });

  it('disables Save while status is saving', () => {
    render(<TopBar dirtyCount={1} status="saving" onSave={() => {}} />);
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
    expect(screen.getByText(/saving/i)).toBeInTheDocument();
  });

  it('calls onSave when the button is clicked', async () => {
    const onSave = vi.fn();
    render(<TopBar dirtyCount={1} status="ready" onSave={onSave} />);
    await userEvent.click(screen.getByRole('button', { name: /save/i }));
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it('does not render a Save-and-Apply button (regression guard)', () => {
    render(<TopBar dirtyCount={1} status="ready" onSave={() => {}} />);
    expect(screen.queryByText(/apply/i)).not.toBeInTheDocument();
  });

  it('shows mode toggle when onModeChange is provided; Settings click fires callback', async () => {
    const spy = vi.fn();
    render(
      <TopBar dirtyCount={0} status="ready" onSave={() => {}} mode="chat" onModeChange={spy} />,
    );
    const chat = screen.getByRole('button', { name: 'Chat' });
    const settings = screen.getByRole('button', { name: 'Settings' });
    expect(chat).toHaveAttribute('aria-pressed', 'true');
    expect(settings).toHaveAttribute('aria-pressed', 'false');
    await userEvent.click(settings);
    expect(spy).toHaveBeenCalledWith('settings');
  });

  it('hides mode toggle when onModeChange is not provided', () => {
    render(<TopBar dirtyCount={0} status="ready" onSave={() => {}} />);
    expect(screen.queryByRole('button', { name: 'Chat' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Settings' })).not.toBeInTheDocument();
  });
});
