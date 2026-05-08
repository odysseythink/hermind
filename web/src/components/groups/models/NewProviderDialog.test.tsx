import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import NewProviderDialog from './NewProviderDialog';

describe('NewProviderDialog', () => {
  const baseProps = {
    providerTypes: ['anthropic', 'openai', 'openrouter'],
    existingKeys: new Set<string>(['already_taken']),
    onCancel: () => {},
    onCreate: () => {},
  };

  it('renders the type dropdown populated from providerTypes', () => {
    render(<NewProviderDialog {...baseProps} />);
    const select = screen.getByLabelText(/provider type/i) as HTMLSelectElement;
    const options = Array.from(select.querySelectorAll('option')).map(o => o.value);
    expect(options).toEqual(['anthropic', 'openai', 'openrouter']);
  });

  it('rejects malformed keys', async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<NewProviderDialog {...baseProps} onCreate={onCreate} />);
    await user.type(screen.getByLabelText(/instance key/i), 'BAD KEY');
    await user.click(screen.getByRole('button', { name: /create/i }));
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByText(/lowercase letters, digits, underscore/i)).toBeInTheDocument();
  });

  it('rejects duplicate keys', async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<NewProviderDialog {...baseProps} onCreate={onCreate} />);
    await user.type(screen.getByLabelText(/instance key/i), 'already_taken');
    await user.click(screen.getByRole('button', { name: /create/i }));
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByText(/already exists/i)).toBeInTheDocument();
  });

  it('dispatches onCreate(key, type) for valid input', async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<NewProviderDialog {...baseProps} onCreate={onCreate} />);
    await user.type(screen.getByLabelText(/instance key/i), 'openai_bot');
    await user.selectOptions(screen.getByLabelText(/provider type/i), 'openai');
    await user.click(screen.getByRole('button', { name: /create/i }));
    expect(onCreate).toHaveBeenCalledWith('openai_bot', 'openai');
  });

  it('fires onCancel when the Cancel button is clicked', async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(<NewProviderDialog {...baseProps} onCancel={onCancel} />);
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalled();
  });
});
