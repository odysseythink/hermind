import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import MultiSelectField from './MultiSelectField';
import type { ConfigField } from '../../api/schemas';

const field: ConfigField = {
  name: 'disabled',
  label: 'Disabled skills',
  help: 'Check a skill to disable it',
  kind: 'multiselect',
  enum: ['alpha', 'beta', 'gamma'],
};

describe('MultiSelectField', () => {
  it('renders a checkbox per enum choice, reflecting initial value', () => {
    const { container } = render(
      <MultiSelectField field={field} value={['beta']} onChange={vi.fn()} />,
    );
    const boxes = container.querySelectorAll('input[type="checkbox"]');
    expect(boxes).toHaveLength(3);
    const alpha = screen.getByLabelText('alpha') as HTMLInputElement;
    const beta = screen.getByLabelText('beta') as HTMLInputElement;
    const gamma = screen.getByLabelText('gamma') as HTMLInputElement;
    expect(alpha.checked).toBe(false);
    expect(beta.checked).toBe(true);
    expect(gamma.checked).toBe(false);
  });

  it('calls onChange with the new sorted deduped array when a box is checked', () => {
    const onChange = vi.fn();
    render(<MultiSelectField field={field} value={['beta']} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText('alpha'));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(['alpha', 'beta']);
  });

  it('calls onChange with the remaining array when a box is unchecked', () => {
    const onChange = vi.fn();
    render(
      <MultiSelectField
        field={field}
        value={['alpha', 'beta']}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByLabelText('alpha'));
    expect(onChange).toHaveBeenCalledWith(['beta']);
  });

  it('renders the empty-state hint when field.enum is empty', () => {
    const empty: ConfigField = { ...field, enum: [] };
    render(<MultiSelectField field={empty} value={[]} onChange={vi.fn()} />);
    expect(screen.getByText(/no skills/i)).toBeInTheDocument();
  });
});
