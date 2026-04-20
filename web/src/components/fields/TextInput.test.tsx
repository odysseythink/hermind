import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import TextInput from './TextInput';
import type { SchemaField } from '../../api/schemas';

const baseField: SchemaField = {
  name: 'thing',
  label: 'Thing',
  kind: 'string',
};

describe('TextInput', () => {
  it('renders without a datalist when no suggestions are passed', () => {
    const { container } = render(
      <TextInput field={baseField} value="" onChange={() => {}} />,
    );
    expect(container.querySelector('datalist')).toBeNull();
    const input = screen.getByLabelText(/thing/i) as HTMLInputElement;
    expect(input.getAttribute('list')).toBeNull();
  });

  it('renders a datalist with one <option> per suggestion', () => {
    const { container } = render(
      <TextInput
        field={baseField}
        value=""
        onChange={() => {}}
        datalist={['anthropic/claude-opus-4-7', 'openai/gpt-4o']}
      />,
    );
    const list = container.querySelector('datalist');
    expect(list).not.toBeNull();
    const options = list!.querySelectorAll('option');
    expect(options).toHaveLength(2);
    expect(options[0].getAttribute('value')).toBe('anthropic/claude-opus-4-7');
    expect(options[1].getAttribute('value')).toBe('openai/gpt-4o');
  });

  it('wires the input.list attribute to the rendered datalist id', () => {
    const { container } = render(
      <TextInput
        field={baseField}
        value=""
        onChange={() => {}}
        datalist={['a', 'b']}
      />,
    );
    const list = container.querySelector('datalist');
    const input = screen.getByLabelText(/thing/i);
    expect(input.getAttribute('list')).toBe(list!.id);
  });

  it('still emits onChange on typing when datalist is present', async () => {
    const onChange = vi.fn();
    render(
      <TextInput
        field={baseField}
        value=""
        onChange={onChange}
        datalist={['a']}
      />,
    );
    await userEvent.type(screen.getByLabelText(/thing/i), 'x');
    expect(onChange).toHaveBeenCalledWith('x');
  });

  it('omits the datalist when suggestions is an empty array', () => {
    const { container } = render(
      <TextInput field={baseField} value="" onChange={() => {}} datalist={[]} />,
    );
    expect(container.querySelector('datalist')).toBeNull();
  });
});
