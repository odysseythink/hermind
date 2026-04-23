import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EmptyState from './EmptyState';

describe('EmptyState', () => {
  it('renders a card for every group', () => {
    render(<EmptyState onSelectGroup={() => {}} />);
    for (const label of [
      'Models',
      'Memory',
      'Skills',
      'Runtime',
      'Advanced',
      'Observability',
    ]) {
      expect(screen.getByRole('button', { name: new RegExp(label, 'i') })).toBeInTheDocument();
    }
  });

  it('calls onSelectGroup with the right id when a card is clicked', async () => {
    const onSelectGroup = vi.fn();
    render(<EmptyState onSelectGroup={onSelectGroup} />);
    await userEvent.click(screen.getByRole('button', { name: /memory/i }));
    expect(onSelectGroup).toHaveBeenCalledWith('memory');
  });
});
