import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import HistoricalMessage from './HistoricalMessage';
import type { ChatMessage } from '../../state/chat';

describe('HistoricalMessage', () => {
  it('renders user message with bubble', () => {
    const msg: ChatMessage = { id: '1', role: 'user', content: 'hello', timestamp: 0 };
    render(<HistoricalMessage message={msg} />);
    expect(screen.getByText('hello')).toBeInTheDocument();
  });

  it('renders assistant message with avatar', () => {
    const msg: ChatMessage = { id: '2', role: 'assistant', content: 'hi there', timestamp: 0 };
    render(<HistoricalMessage message={msg} />);
    expect(screen.getByText('hi there')).toBeInTheDocument();
    expect(screen.getByLabelText('Assistant avatar')).toBeInTheDocument();
  });

  it('calls onDelete when delete action is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const msg: ChatMessage = { id: '3', role: 'assistant', content: 'test', timestamp: 0 };
    render(<HistoricalMessage message={msg} onDelete={onDelete} />);
    await user.click(screen.getByLabelText('Delete message'));
    expect(onDelete).toHaveBeenCalledWith('3');
  });
});
