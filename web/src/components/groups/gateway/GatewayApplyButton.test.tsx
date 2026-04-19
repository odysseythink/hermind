import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import GatewayApplyButton from './GatewayApplyButton';

describe('GatewayApplyButton', () => {
  it('is enabled when clean and idle', () => {
    render(<GatewayApplyButton dirty={false} busy={false} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeEnabled();
  });

  it('is disabled when the gateway slice is dirty', () => {
    render(<GatewayApplyButton dirty={true} busy={false} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled();
  });

  it('is disabled while busy', () => {
    render(<GatewayApplyButton dirty={false} busy={true} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled();
  });

  it('calls onApply when clicked', async () => {
    const onApply = vi.fn();
    render(<GatewayApplyButton dirty={false} busy={false} onApply={onApply} />);
    await userEvent.click(screen.getByRole('button', { name: /apply/i }));
    expect(onApply).toHaveBeenCalledTimes(1);
  });

  it('shows a tooltip hint when disabled due to dirty', () => {
    render(<GatewayApplyButton dirty={true} busy={false} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toHaveAttribute(
      'title',
      expect.stringMatching(/save first/i) as unknown as string,
    );
  });
});
