import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TextAreaInput from './TextAreaInput';

describe('TextAreaInput', () => {
  it('renders current value and fires onChange with new text', () => {
    const onChange = vi.fn();
    render(
      <TextAreaInput
        value="hello"
        onChange={onChange}
        placeholder="type here"
      />,
    );
    const ta = screen.getByPlaceholderText('type here') as HTMLTextAreaElement;
    expect(ta.value).toBe('hello');
    fireEvent.change(ta, { target: { value: 'world' } });
    expect(onChange).toHaveBeenCalledWith('world');
  });
});
