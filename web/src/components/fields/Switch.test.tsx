import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Switch from './Switch';

describe('Switch', () => {
  it('renders with role=switch and reflects checked=false', () => {
    render(<Switch checked={false} onChange={() => {}} ariaLabel="Toggle thing" />);
    const btn = screen.getByRole('switch', { name: 'Toggle thing' });
    expect(btn.getAttribute('aria-checked')).toBe('false');
  });

  it('reflects checked=true', () => {
    render(<Switch checked={true} onChange={() => {}} ariaLabel="Toggle thing" />);
    expect(screen.getByRole('switch').getAttribute('aria-checked')).toBe('true');
  });

  it('click fires onChange(!checked) once', () => {
    const onChange = vi.fn();
    render(<Switch checked={false} onChange={onChange} ariaLabel="X" />);
    fireEvent.click(screen.getByRole('switch'));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('click toggles from checked=true to false', () => {
    const onChange = vi.fn();
    render(<Switch checked={true} onChange={onChange} ariaLabel="X" />);
    fireEvent.click(screen.getByRole('switch'));
    expect(onChange).toHaveBeenCalledWith(false);
  });

  it('Space key toggles', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Switch checked={false} onChange={onChange} ariaLabel="X" />);
    const btn = screen.getByRole('switch');
    btn.focus();
    await user.keyboard(' ');
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('Enter key toggles', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Switch checked={false} onChange={onChange} ariaLabel="X" />);
    const btn = screen.getByRole('switch');
    btn.focus();
    await user.keyboard('{Enter}');
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('disabled prop blocks click', () => {
    const onChange = vi.fn();
    render(<Switch checked={false} onChange={onChange} ariaLabel="X" disabled />);
    fireEvent.click(screen.getByRole('switch'));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('disabled prop sets aria-disabled and disabled attribute', () => {
    render(<Switch checked={false} onChange={() => {}} ariaLabel="X" disabled />);
    const btn = screen.getByRole('switch') as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });
});
