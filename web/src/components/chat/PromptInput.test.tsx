import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { useState } from 'react';
import userEvent from '@testing-library/user-event';
import PromptInput from './PromptInput';

function StatefulPromptInput({ onSubmit, onTextChangeSpy }: { onSubmit?: () => void; onTextChangeSpy?: (text: string) => void }) {
  const [text, setText] = useState('');
  return (
    <PromptInput
      text={text}
      onTextChange={(t) => {
        setText(t);
        onTextChangeSpy?.(t);
      }}
      onSubmit={onSubmit ?? vi.fn()}
    />
  );
}

describe('PromptInput', () => {
  it('renders textarea and send button', () => {
    render(<StatefulPromptInput />);
    expect(screen.getByPlaceholderText(/type a message/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/send/i)).toBeInTheDocument();
  });

  it('calls onTextChange when typing', async () => {
    const user = userEvent.setup();
    const onTextChange = vi.fn();
    render(<StatefulPromptInput onTextChangeSpy={onTextChange} />);
    await user.type(screen.getByPlaceholderText(/type a message/i), 'hi');
    expect(onTextChange).toHaveBeenCalledWith('hi');
  });

  it('calls onSubmit when send button clicked', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<PromptInput text="hello" onTextChange={vi.fn()} onSubmit={onSubmit} />);
    await user.click(screen.getByLabelText(/send/i));
    expect(onSubmit).toHaveBeenCalled();
  });

  it('calls onSubmit on Enter without shift', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<PromptInput text="hello" onTextChange={vi.fn()} onSubmit={onSubmit} />);
    await user.type(screen.getByPlaceholderText(/type a message/i), '{Enter}');
    expect(onSubmit).toHaveBeenCalled();
  });

  it('renders attach, mention, and tools buttons', () => {
    render(<StatefulPromptInput />);
    expect(screen.getByLabelText('Attach file')).toBeInTheDocument();
    expect(screen.getByLabelText('Mention')).toBeInTheDocument();
    expect(screen.getByLabelText('Tools')).toBeInTheDocument();
  });
});
