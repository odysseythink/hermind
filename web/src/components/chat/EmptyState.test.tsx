import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EmptyState from './EmptyState';

describe('EmptyState', () => {
  it('renders greeting and suggestions', () => {
    render(<EmptyState suggestions={['What can you do?', 'Help me code']} onSuggestionClick={vi.fn()} />);
    expect(screen.getByText('What can you do?')).toBeInTheDocument();
    expect(screen.getByText('Help me code')).toBeInTheDocument();
  });

  it('calls onSuggestionClick when a suggestion is clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<EmptyState suggestions={['Test suggestion']} onSuggestionClick={onClick} />);
    await user.click(screen.getByText('Test suggestion'));
    expect(onClick).toHaveBeenCalledWith('Test suggestion');
  });
});
